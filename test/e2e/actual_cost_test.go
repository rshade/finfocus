//go:build integration
// +build integration

// Package e2e contains end-to-end and integration tests.
// This file contains integration tests for the "cost actual" command using
// static fixture files. These are classified as integration tests (not e2e)
// because they use pre-generated state fixtures rather than live Pulumi output.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// commandTimeout is the maximum duration to wait for finfocus commands.
const commandTimeout = 60 * time.Second

// runFinFocusCommand executes the finfocus binary with the given arguments and returns
// the unmarshaled JSON result. It uses a context with timeout to prevent hanging tests.
// The test fails if the command times out, fails, or returns invalid JSON.
func runFinFocusCommand(t *testing.T, binary string, args ...string) []map[string]interface{} {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Check if the error was due to context deadline exceeded
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			require.FailNow(t, "Command timed out after %v: %s\nstderr: %s",
				commandTimeout, binary, stderr.String())
		}
		require.NoError(t, err, "Command failed: %s\nstderr: %s", binary, stderr.String())
	}

	var result []map[string]interface{}
	err = json.Unmarshal(stdout.Bytes(), &result)
	require.NoError(t, err, "Failed to unmarshal JSON output: %s", stdout.String())
	return result
}

// TestIntegration_ActualCost_StaticFixtures tests the "cost actual" command using
// pre-generated static state fixture files. This validates CLI parsing and output
// formatting without requiring live Pulumi infrastructure.
//
// Note: These are integration tests (not e2e) because they use static fixtures
// rather than live `pulumi preview --json` output.
func TestIntegration_ActualCost_StaticFixtures(t *testing.T) {
	binary := findFinFocusBinary()
	require.NotEmpty(t, binary)

	ctx := context.Background()
	pm := NewPluginManager(t)
	if !pm.IsPluginInstalled(ctx, "aws-public") {
		t.Skip("aws-public plugin not installed - required for actual cost integration tests")
	}

	t.Run("WithTimestamps", func(t *testing.T) {
		statePath, err := filepath.Abs("../fixtures/state/valid-state.json")
		require.NoError(t, err)

		result := runFinFocusCommand(t, binary, "cost", "actual", "--pulumi-state", statePath, "--output", "json")

		// valid-state.json has 4 AWS resources (1 provider + 3 resources)
		assert.NotEmpty(t, result)

		for _, res := range result {
			assert.Contains(t, res, "resourceType")
			assert.Contains(t, res, "currency")
			// Depending on whether plugin succeeded or failed, it might have monthly or notes
			if _, ok := res["monthly"]; !ok {
				assert.Contains(t, res, "notes")
			}
		}
	})

	t.Run("ImportedResources", func(t *testing.T) {
		statePath, err := filepath.Abs("../fixtures/state/imported-resources.json")
		require.NoError(t, err)

		result := runFinFocusCommand(t, binary, "cost", "actual", "--pulumi-state", statePath, "--output", "json")

		// imported-resources.json has 4 AWS resources
		assert.NotEmpty(t, result)
	})

	t.Run("MissingTimestamps", func(t *testing.T) {
		statePath, err := filepath.Abs("../fixtures/state/no-timestamps.json")
		require.NoError(t, err)

		// Use a recent date range to avoid "range too large" error
		result := runFinFocusCommand(t, binary, "cost", "actual", "--pulumi-state", statePath, "--from", "2025-01-01", "--to", "2025-01-31", "--output", "json")

		// no-timestamps.json custom resources should be present but with 0 actual cost
		// because they lack timestamps for lifecycle verification
		assert.NotEmpty(t, result)
		for _, res := range result {
			assert.Equal(t, float64(0), res["monthly"], "Expected 0 cost for resource without timestamps")
			assert.Contains(t, res, "notes")
		}
	})
}
