package weblog_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

const (
	leakedToken  = "tok-leak-9f3a"
	leakedSecret = "4dabf18193072939515e22adb298388d"

	secretEnvelope = `{\"4dabf18193072939515e22adb298388d\":\"1b47061264138c4ac30d75fd1eb44270\",` +
		`\"ciphertext\":\"` + leakedSecret + `\"}`
)

func oversizedEvent(field, value string) string {
	return `{"level":"info","message":"m","` + field + `":"` + value + `","padding":"` +
		strings.Repeat("x", weblog.MaxLineBytes) + `"}`
}

func assertWithheld(t *testing.T, e weblog.Entry) {
	t.Helper()
	encoded, err := json.Marshal(e)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), leakedToken)
	assert.NotContains(t, string(encoded), leakedSecret)
	assert.True(t, e.Raw)
	assert.Contains(t, e.Message, "withheld")
}

func TestUnparseableStructuredLineIsWithheld(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		line string
	}{
		{"credential field before the cut", oversizedEvent("api_token", leakedToken)},
		{"pulumi secret envelope before the cut", oversizedEvent("value", secretEnvelope)},
		{"truncated object", `{"level":"info","api_token":"` + leakedToken},
		{"malformed object", `{"level":"info","password":"` + leakedToken + `",`},
		{"leading space", `  {"api_token":"` + leakedToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			line := tt.line
			if len(line) > weblog.MaxLineBytes {
				line = line[:weblog.MaxLineBytes]
			}
			assertWithheld(t, weblog.NewEntry([]byte(line), "", fixedNow))
		})
	}
}

func TestOversizedCredentialEventIsWithheldOnEveryPath(t *testing.T) {
	t.Parallel()
	event := oversizedEvent("api_token", leakedToken)

	t.Run("core tee", func(t *testing.T) {
		t.Parallel()
		hub := weblog.NewHub(10)
		_, err := hub.Tee().Write([]byte(event + "\n"))
		require.NoError(t, err)
		snap, _, cancel := hub.Subscribe(0)
		defer cancel()
		require.Len(t, snap.Entries, 1)
		assertWithheld(t, snap.Entries[0])
		assert.True(t, snap.Entries[0].Truncated)
	})

	t.Run("plugin writer", func(t *testing.T) {
		t.Parallel()
		hub := weblog.NewHub(10)
		w := hub.Writer("aws-public")
		_, err := w.Write([]byte(event + "\n"))
		require.NoError(t, err)
		require.NoError(t, w.Close())
		snap, _, cancel := hub.Subscribe(0)
		defer cancel()
		require.Len(t, snap.Entries, 1)
		assertWithheld(t, snap.Entries[0])
		assert.Equal(t, "aws-public", snap.Entries[0].Source)
	})

	t.Run("file decoder", func(t *testing.T) {
		t.Parallel()
		count := 0
		for e, err := range weblog.Decode(strings.NewReader(event+"\n"), "") {
			require.NoError(t, err)
			assertWithheld(t, e)
			count++
		}
		assert.Equal(t, 1, count)
	})
}

func TestPlainTextAndValidJSONAreNotWithheld(t *testing.T) {
	t.Parallel()
	plain := weblog.NewEntry([]byte("PORT=36677"), "aws-public", fixedNow)
	assert.True(t, plain.Raw)
	assert.Equal(t, "PORT=36677", plain.Message)

	valid := weblog.NewEntry([]byte(`{"level":"info","message":"ok","api_token":"`+leakedToken+`"}`), "", fixedNow)
	assert.False(t, valid.Raw)
	assert.Equal(t, "ok", valid.Message)
	encoded, err := json.Marshal(valid)
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), leakedToken)
}
