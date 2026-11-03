package webui

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

const (
	redactionTokenValue  = "tok-9f8e7d6c5b4a-do-not-leak"
	redactionPulumiValue = "pulumi-ciphertext-do-not-leak"
	redactionInternalKey = "__defaults"
	redactionPulumiSig   = "4dabf18193072939515e22adb298388d"
)

func writePluginSecrets(t *testing.T, hub *weblog.Hub) {
	t.Helper()
	line := `{"level":"warn","message":"price lookup slow","region":"us-east-1",` +
		`"api_token":"` + redactionTokenValue + `",` +
		`"nested":{"password":"` + redactionTokenValue + `","ok":"visible"},` +
		`"input":{"` + redactionPulumiSig + `":"1b47061264138c4ac30375b4e7b8a1b5","ciphertext":"` +
		redactionPulumiValue + `"},` +
		`"list":[{"` + redactionPulumiSig + `":"1b47061264138c4ac30375b4e7b8a1b5","ciphertext":"` +
		redactionPulumiValue + `"}],` +
		`"` + redactionInternalKey + `":["hidden"]}` + "\n"
	w := hub.Writer("aws-public")
	_, err := w.Write([]byte(line))
	require.NoError(t, err)
	require.NoError(t, w.Close())
}

func assertNoSecrets(t *testing.T, label, body string) {
	t.Helper()
	assert.NotContains(t, body, redactionTokenValue, label)
	assert.NotContains(t, body, redactionPulumiValue, label)
	assert.NotContains(t, body, redactionPulumiSig, label)
	assert.NotContains(t, body, redactionInternalKey, label)
}

func TestLogsRedactionEndToEnd(t *testing.T) {
	t.Parallel()
	hub := weblog.NewHub(50)
	writePluginSecrets(t, hub)
	s := newRunningServer(t, Options{Logs: hub})

	statusResp := overviewRequest(t, s, http.MethodGet, "/api/logs/status", "")
	require.Equal(t, http.StatusOK, statusResp.StatusCode)
	assertNoSecrets(t, "status", readBody(t, statusResp))

	reader, _ := openLogStream(t, s, "")
	snap := readFrame(t, reader)
	require.Equal(t, "snapshot", snap.event)
	encoded, err := json.Marshal(snap.data)
	require.NoError(t, err)
	body := string(encoded)
	assertNoSecrets(t, "snapshot", body)
	assert.Contains(t, body, "[redacted]")
	assert.Contains(t, body, "visible")
	assert.Contains(t, body, "us-east-1")

	writePluginSecrets(t, hub)
	live := readFrame(t, reader)
	require.Equal(t, "log", live.event)
	liveEncoded, err := json.Marshal(live.data)
	require.NoError(t, err)
	assertNoSecrets(t, "live", string(liveEncoded))

	downloaded, _, cancel := hub.Subscribe(0)
	defer cancel()
	require.NotEmpty(t, downloaded.Entries)
	for _, entry := range downloaded.Entries {
		raw, marshalErr := json.Marshal(entry)
		require.NoError(t, marshalErr)
		assertNoSecrets(t, "download entry "+strings.TrimSpace(entry.Message), string(raw))
	}
}
