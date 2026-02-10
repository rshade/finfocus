//go:build e2e
// +build e2e

package e2e

import (
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_Output_NDJSON(t *testing.T) {
	binary := findFinFocusBinary()
	require.NotEmpty(t, binary)

	planPath, err := filepath.Abs("../fixtures/plans/aws/simple.json")
	require.NoError(t, err)

	// Run with NDJSON output
	cmd := exec.Command(binary, "cost", "projected", "--pulumi-json", planPath, "--output", "ndjson")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			require.NoError(t, err, "command failed with stderr: %s", string(exitErr.Stderr))
		}
		require.NoError(t, err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var obj map[string]interface{}
		err := json.Unmarshal([]byte(line), &obj)
		require.NoError(t, err, "Line should be valid JSON: %s", line)

		// Schema validation
		assert.Contains(t, obj, "resourceType", "Missing resourceType field in NDJSON: %s", line)
		assert.Contains(t, obj, "currency", "Missing currency field in NDJSON: %s", line)
		assert.Contains(t, obj, "resourceId", "Missing resourceId field in NDJSON: %s", line)
	}
}
