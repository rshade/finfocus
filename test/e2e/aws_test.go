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

func TestE2E_AWS_ProjectedCost(t *testing.T) {
	binary := findFinFocusBinary()
	require.NotEmpty(t, binary)

	planPath, err := filepath.Abs("../fixtures/plans/aws/simple.json")
	require.NoError(t, err)

	cmd := exec.Command(binary, "cost", "projected", "--pulumi-json", planPath, "--output", "json")
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			require.NoError(t, err, "command failed with stderr: %s", string(exitErr.Stderr))
		}
		require.NoError(t, err)
	}

	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err)

	// JSON output wraps under "finfocus" key
	finfocus, ok := result["finfocus"].(map[string]interface{})
	require.True(t, ok, "expected finfocus wrapper key")

	// Verify we got valid output structure
	resources, ok := finfocus["resources"].([]interface{})
	require.True(t, ok, "expected resources to be an array")
	assert.NotEmpty(t, resources, "expected at least one resource in output")
}
