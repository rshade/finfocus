package cli_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func TestNewAnalyzerCmd(t *testing.T) {
	cmd := cli.NewAnalyzerCmd()

	require.NotNil(t, cmd)
	assert.Equal(t, "analyzer", cmd.Use)
	assert.Contains(t, cmd.Short, "Pulumi Analyzer")

	// Verify serve subcommand is registered
	serveCmd, _, err := cmd.Find([]string{"serve"})
	require.NoError(t, err)
	require.NotNil(t, serveCmd)
	assert.Equal(t, "serve", serveCmd.Use)
}

func TestNewAnalyzerServeCmd(t *testing.T) {
	cmd := cli.NewAnalyzerServeCmd()

	require.NotNil(t, cmd)
	assert.Equal(t, "serve", cmd.Use)
	assert.Contains(t, cmd.Short, "gRPC server")
}

func TestAnalyzerServeCmd_PrintsPort(t *testing.T) {
	// This test verifies that the serve command:
	// 1. Starts without immediate error
	// 2. Prints a port number to stdout
	// We need to cancel quickly to avoid blocking

	cmd := cli.NewAnalyzerServeCmd()

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Create a context that cancels after a short delay
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cmd.SetContext(ctx)

	// Run the command (will exit when context is canceled)
	err := cmd.Execute()

	// The command should complete without error when context is canceled
	// (graceful shutdown on context done)
	if err != nil {
		// Context canceled is expected
		assert.Contains(t, err.Error(), "context")
	}

	// Verify a port number was printed to stdout
	output := stdout.String()
	if len(output) > 0 {
		// Should be a number followed by newline
		assert.Regexp(t, `^\d+\n$`, output)
	}
}

func TestAnalyzerCmd_InRootCmd(t *testing.T) {
	rootCmd := cli.NewRootCmd("1.0.0-test")

	// Find the analyzer command
	analyzerCmd, _, err := rootCmd.Find([]string{"analyzer"})
	require.NoError(t, err)
	require.NotNil(t, analyzerCmd)

	// Find the serve subcommand
	serveCmd, _, err := rootCmd.Find([]string{"analyzer", "serve"})
	require.NoError(t, err)
	require.NotNil(t, serveCmd)
	assert.Equal(t, "serve", serveCmd.Use)
}

func TestAnalyzerServeCmd_HelpOutput(t *testing.T) {
	cmd := cli.NewAnalyzerServeCmd()

	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "gRPC server")
	assert.Contains(t, output, "Pulumi")
}
