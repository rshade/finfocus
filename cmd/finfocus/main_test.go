package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/pkg/version"
)

func TestRun(t *testing.T) {
	t.Parallel()

	// Test that run() can be called without panicking
	// Note: This is a basic smoke test. More comprehensive testing
	// would require mocking the CLI execution, which is complex
	// for a main package test.

	// We can't easily test the full execution without setting up
	// complex test harnesses, but we can test that the function
	// exists and can be called
	t.Run("run function exists", func(t *testing.T) {
		t.Parallel()
		// This test mainly ensures the function can be called
		// In a real scenario, we'd mock dependencies
		_ = run
	})
}

//nolint:paralleltest // SetResolvedProjectDir sets the process-wide project directory (via NewRootCmd)
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

func TestStartModeFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		exe  string
		want startMode
	}{
		{exe: "pulumi-language-finfocus", want: startModeLanguageHost},
		{exe: "pulumi-language-finfocus.exe", want: startModeLanguageHost},
		{
			exe:  filepath.Join("/opt/finfocus/analyzer", "pulumi-language-finfocus"),
			want: startModeLanguageHost,
		},
		{exe: "pulumi-analyzer-policy-finfocus", want: startModeAnalyzer},
		{exe: "pulumi-analyzer-policy-finfocus.exe", want: startModeAnalyzer},
		{exe: "pulumi-analyzer-finfocus", want: startModeAnalyzer},
		{exe: "pulumi-analyzer-finfocus.exe", want: startModeAnalyzer},
		{exe: "finfocus", want: startModeCLI},
		{exe: "finfocus.exe", want: startModeCLI},
	}
	for _, tt := range tests {
		t.Run(tt.exe, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, startModeFor(tt.exe))
		})
	}
}

// Budget-exceeded exit-code preservation is now verified where the conversion
// actually happens: internal/cli's TestToAxExitError (cost_budget_test.go)
// exercises toAxExitError + ax.ErrorExitCode directly. main()'s run() delegates
// exit-code resolution entirely to ax.Execute, which has no error value left to
// inspect here (Execute returns only the resolved int).
