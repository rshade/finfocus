package weblog_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/weblog"
)

const secretKey = "4dabf18193072939515e22adb298388d"

func TestRedaction(t *testing.T) {
	t.Parallel()
	line := `{"message":"m",` +
		`"api_token":"abc",` +
		`"keep":"visible",` +
		`"__defaults":["x"],` +
		`"nested":{"password":"p","host":"h","__provider":"z"},` +
		`"list":[{"secret":"s","ok":1},"plain"],` +
		`"prop":{"` + secretKey + `":"1","ciphertext":"c"},` +
		`"deep":[{"inner":{"` + secretKey + `":"1"}}]}`
	e := weblog.NewEntry([]byte(line), "", fixedNow)
	want := map[string]any{
		"api_token": "[redacted]",
		"keep":      "visible",
		"nested":    map[string]any{"password": "[redacted]", "host": "h"},
		"list":      []any{map[string]any{"secret": "[redacted]", "ok": float64(1)}, "plain"},
		"prop":      "[redacted]",
		"deep":      []any{map[string]any{"inner": "[redacted]"}},
	}
	assert.Equal(t, want, e.Fields)
}

func TestRedactionFollowsSharedRule(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"api_token", "password", "Authorization", "client_secret", "region", "resources", "host"} {
		e := weblog.NewEntry([]byte(`{"message":"m","`+key+`":"value"}`), "", fixedNow)
		require.Contains(t, e.Fields, key, key)
		if config.IsCredentialName(key) {
			assert.Equal(t, "[redacted]", e.Fields[key], key)
		} else {
			assert.Equal(t, "value", e.Fields[key], key)
		}
	}
}

func TestRedactionOnlyUnderscoreKeysLeavesNoFields(t *testing.T) {
	t.Parallel()
	e := weblog.NewEntry([]byte(`{"message":"m","__defaults":1}`), "", fixedNow)
	assert.Nil(t, e.Fields)
}
