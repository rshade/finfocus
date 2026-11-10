package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// envelopeText returns s as it appears inside a string of the JSON error
// envelope that a non-terminal run writes to stderr. A Windows path doubles
// its backslashes there; on Linux and macOS a path is unchanged.
func envelopeText(t *testing.T, s string) string {
	t.Helper()
	quoted, err := json.Marshal(s)
	require.NoError(t, err)
	return strings.Trim(string(quoted), `"`)
}
