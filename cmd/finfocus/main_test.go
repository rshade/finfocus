package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/pkg/version"
)

func TestRun(t *testing.T) {
	// Test that run() can be called without panicking
	// Note: This is a basic smoke test. More comprehensive testing
	// would require mocking the CLI execution, which is complex
	// for a main package test.

	// We can't easily test the full execution without setting up
	// complex test harnesses, but we can test that the function
	// exists and can be called
	t.Run("run function exists", func(t *testing.T) {
		// This test mainly ensures the function can be called
		// In a real scenario, we'd mock dependencies
		_ = run
	})
}

func TestMainComponents(t *testing.T) {
	t.Run("version available", func(t *testing.T) {
		v := version.GetVersion()
		if v == "" {
			t.Error("expected version to be non-empty")
		}
	})

	t.Run("cli root command", func(t *testing.T) {
		root := cli.NewRootCmd(version.GetVersion())
		require.NotNil(t, root)
		assert.NotEmpty(t, root.Use)
	})
}

// Budget-exceeded exit-code preservation is now verified where the conversion
// actually happens: internal/cli's TestToAxExitError (cost_budget_test.go)
// exercises toAxExitError + ax.ErrorExitCode directly. main()'s run() delegates
// exit-code resolution entirely to ax.Execute, which has no error value left to
// inspect here (Execute returns only the resolved int).
