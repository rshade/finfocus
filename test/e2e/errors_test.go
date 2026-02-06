//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// DefaultE2ETimeout is the default timeout for E2E tests (SC-002 fix for CI reliability)
const DefaultE2ETimeout = 30 * time.Second

func TestE2E_Errors_MissingFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultE2ETimeout)
	defer cancel()

	binary := findFinFocusBinary()
	require.NotEmpty(t, binary)

	// Run with non-existent file
	cmd := exec.CommandContext(ctx, binary, "cost", "projected", "--pulumi-json", "nonexistent.json")
	output, err := cmd.CombinedOutput()

	// Verify timeout didn't fire
	require.NoError(t, ctx.Err(), "test timeout expired; command may not have completed naturally")

	// Should fail with exit code 1
	require.Error(t, err)
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "expected exec.ExitError")
	assert.Equal(t, 1, exitErr.ExitCode(), "expected exit code 1 for missing file")
	assert.Contains(t, string(output), "no such file")
}

func TestE2E_Errors_InvalidFormat(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultE2ETimeout)
	defer cancel()

	binary := findFinFocusBinary()
	require.NotEmpty(t, binary)

	// Run with invalid format (--pulumi-json required to reach format validation)
	cmd := exec.CommandContext(ctx, binary, "cost", "projected",
		"--pulumi-json", os.DevNull, "--output", "invalid")
	output, err := cmd.CombinedOutput()

	// Verify timeout didn't fire
	require.NoError(t, ctx.Err(), "test timeout expired; command may not have completed naturally")

	// Should fail with exit code 1
	require.Error(t, err)
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "expected exec.ExitError")
	assert.Equal(t, 1, exitErr.ExitCode(), "expected exit code 1 for invalid format")
	assert.Contains(t, string(output), "invalid")
}
