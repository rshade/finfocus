//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCLIExecution performs a basic smoke test to verify the finfocus binary can be executed.
func TestCLIExecution(t *testing.T) {
	// Locate the binary (assume it's built in bin/)
	rootDir, err := filepath.Abs("../../../")
	require.NoError(t, err)

	binaryPath := filepath.Join(rootDir, "bin", "finfocus")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Skipf("Binary not found at %s, skipping black-box test", binaryPath)
	}

	// Run help command as a basic smoke test
	cmd := newCommand(context.Background(), binaryPath, "--help")
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to run CLI help command")
	require.Contains(t, string(output), "Usage:", "Help output should contain 'Usage:'")

	// NOTE: In a full E2E run, we would:
	// 1. Deploy a stack using the white-box harness
	// 2. Run the CLI against that stack: `finfocus --stack <stack-name>`
	// 3. Parse the CLI JSON output and validate costs
	// 4. Destroy the stack via harness
}
