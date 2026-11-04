package webui

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

func TestLogsSnapshotParityWithDecode(t *testing.T) {
	t.Parallel()
	file, err := os.Open("../weblog/testdata/sample.ndjson")
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	var decoded []weblog.Entry
	for entry, decodeErr := range weblog.Decode(file, "") {
		require.NoError(t, decodeErr)
		decoded = append(decoded, entry)
	}
	require.NotEmpty(t, decoded)

	hub := weblog.NewHub(len(decoded) + 1)
	for _, entry := range decoded {
		hub.Append(entry)
	}
	s := newRunningServer(t, Options{Logs: hub})

	reader, resp := openLogStream(t, s, "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	frame := readFrame(t, reader)
	require.Equal(t, "snapshot", frame.event)

	raw, err := json.Marshal(frame.data["entries"])
	require.NoError(t, err)
	var streamed []weblog.Entry
	require.NoError(t, json.Unmarshal(raw, &streamed))
	require.Len(t, streamed, len(decoded))

	for i, want := range decoded {
		want.Display = weblog.Display(want)
		assert.Equal(t, want, streamed[i], "entry %d", i)
	}
	assert.Equal(t, strconv.Itoa(len(decoded)), frame.id)
}

func TestWebUIImportsStayOffCLIRegistryPluginhost(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	forbidden := []string{"/internal/cli", "/internal/registry", "/internal/pluginhost"}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, parseErr := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		require.NoError(t, parseErr)
		for _, imp := range parsed.Imports {
			for _, bad := range forbidden {
				assert.NotContains(t, imp.Path.Value, bad, name)
			}
		}
	}
}
