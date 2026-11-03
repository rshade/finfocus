package cli_test

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

// The dashboard stream and `finfocus logs` must produce the same entry for the
// same line (SC-009). A raw line is the one legitimate difference: a file has
// no writer tag and no arrival time, so Decode stamps "unknown" and the read
// time where a live writer stamps its own name and the arrival time.
//
//nolint:paralleltest // root command and t.Setenv
func TestLogs_NDJSONMatchesHubEntries(t *testing.T) {
	isolatedHome(t)
	file := filepath.Join(t.TempDir(), "s.log")
	copySample(t, file)

	result := runLogs(t, "--file", file, "--output", "ndjson", "--tail", "0")
	require.Equal(t, 0, result.ExitCode, "stderr: %s", result.Stderr)
	got := ndjsonEntries(t, result.Stdout)

	f, err := os.Open(weblogSamplePath())
	require.NoError(t, err)
	defer f.Close()
	hub := weblog.NewHub(100)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		hub.Append(weblog.NewEntry(scanner.Bytes(), "", time.Now()))
	}
	require.NoError(t, scanner.Err())
	snapshot, _, cancel := hub.Subscribe(0)
	defer cancel()

	require.Len(t, got, len(snapshot.Entries))
	for i, want := range snapshot.Entries {
		if want.Raw {
			assert.True(t, got[i].Raw)
			assert.Equal(t, want.Message, got[i].Message)
			assert.Equal(t, want.Level, got[i].Level)
			continue
		}
		assert.Equal(t, want, got[i], "entry %d", i+1)
	}
}
