//go:build e2e
// +build e2e

package e2e

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE2E_GCP_ProjectedCost(t *testing.T) {
	binary := findFinFocusBinary()
	require.NotEmpty(t, binary)

	planPath, err := filepath.Abs("../fixtures/plans/gcp/simple.json")
	require.NoError(t, err)

	cmd := exec.Command(binary, "cost", "projected", "--pulumi-json", planPath, "--output", "json")
	output, err := cmd.Output()
	require.NoError(t, err)

	var result map[string]interface{}
	err = json.Unmarshal(output, &result)
	require.NoError(t, err)
}
