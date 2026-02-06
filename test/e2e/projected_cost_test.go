//go:build e2e
// +build e2e

package e2e

import (
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestE2E_ProjectedCost tests the projected cost workflow using the compiled binary.
func TestE2E_ProjectedCost(t *testing.T) {
	binary := findFinFocusBinary()
	require.NotEmpty(t, binary, "finfocus binary not found")

	// Use fixture plan
	planPath, err := filepath.Abs("../fixtures/plans/aws/simple.json")
	require.NoError(t, err)
	require.FileExists(t, planPath)

	// Run command
	cmd := exec.Command(binary, "cost", "projected", "--pulumi-json", planPath, "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			require.NoError(t, err, "command failed with stderr: %s", string(exitErr.Stderr))
		}
		require.NoError(t, err)
	}

	// Verify JSON output
	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err, "Failed to parse JSON output: %s", string(output))

	// JSON output wraps under "finfocus" key
	finfocus, ok := result["finfocus"].(map[string]interface{})
	require.True(t, ok, "expected finfocus wrapper key")

	// Verify structure
	assert.Contains(t, finfocus, "summary")
	assert.Contains(t, finfocus, "resources")
}
