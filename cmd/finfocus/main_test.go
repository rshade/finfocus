package main

import (
	"errors"
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
		if root == nil {
			t.Error("expected root command to be non-nil")
		}
		if root.Use == "" {
			t.Error("expected root command to have a use string")
		}
	})
}

// T002: Test that BudgetExitError is correctly detected via errors.As and
// the custom exit code is extracted. This tests the extractBudgetExitCode
// helper that main() will use.
func TestExtractBudgetExitCode(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantExitCode int
		wantIsBudget bool
	}{
		{
			name:         "BudgetExitError with exit code 2",
			err:          &cli.BudgetExitError{ExitCode: 2, Reason: "budget exceeded"},
			wantExitCode: 2,
			wantIsBudget: true,
		},
		{
			name:         "BudgetExitError with exit code 42",
			err:          &cli.BudgetExitError{ExitCode: 42, Reason: "over limit"},
			wantExitCode: 42,
			wantIsBudget: true,
		},
		{
			name:         "wrapped BudgetExitError",
			err:          errors.Join(errors.New("outer"), &cli.BudgetExitError{ExitCode: 3, Reason: "wrapped budget"}),
			wantExitCode: 3,
			wantIsBudget: true,
		},
		{
			name:         "non-BudgetExitError falls through",
			err:          errors.New("generic error"),
			wantExitCode: 1,
			wantIsBudget: false,
		},
		{
			name:         "nil error returns 0",
			err:          nil,
			wantExitCode: 0,
			wantIsBudget: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exitCode := extractBudgetExitCode(tt.err)
			if tt.err == nil {
				assert.Equal(t, 0, exitCode, "nil error should return 0")
				return
			}

			var budgetErr *cli.BudgetExitError
			isBudget := errors.As(tt.err, &budgetErr)
			assert.Equal(t, tt.wantIsBudget, isBudget)

			if tt.wantIsBudget {
				require.True(t, isBudget)
				assert.Equal(t, tt.wantExitCode, budgetErr.ExitCode)
			}

			assert.Equal(t, tt.wantExitCode, exitCode)
		})
	}
}
