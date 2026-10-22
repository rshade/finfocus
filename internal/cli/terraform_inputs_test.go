package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// Tests execute fake binaries and change the working directory.
//
//nolint:paralleltest // Changes process environment and executes a fake binary.
func TestTerraformPlanInputs(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Chdir(t.TempDir())
	plan := filepath.Join(t.TempDir(), "plan.json")
	require.NoError(
		t,
		os.WriteFile(plan, []byte(`{"format_version":"1.2","resource_changes":[]}`), 0o600),
	)
	for _, command := range [][]string{{"cost", "projected"}, {"cost", "forecast"}, {"cost", "estimate"}, {"cost", "recommendations"}, {"overview"}} {
		t.Run(command[len(command)-1], func(t *testing.T) {
			for _, other := range []string{"--pulumi-json", "--terraform-state", "--pulumi-state"} {
				args := append(
					append([]string{}, command...),
					"--terraform-plan",
					plan,
					other,
					plan,
				)
				result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), args)
				assert.Equal(t, 2, result.ExitCode, "%v: %s", args, result.Stderr)
				assert.Contains(t, string(result.Stderr), "mutually exclusive")
			}
			result := axtest.Run(
				context.Background(),
				t,
				cli.NewRootCmd("test"),
				append(append([]string{}, command...), "--terraform-plan", plan),
			)
			assert.Equal(t, 0, result.ExitCode, "%s", result.Stderr)
		})
	}
}

//nolint:paralleltest // Subtests share the working directory and process environment.
func TestMissingInfrastructureProject(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Chdir(t.TempDir())
	for _, command := range [][]string{{"cost", "projected"}, {"cost", "forecast"}, {"cost", "estimate"}, {"cost", "recommendations"}, {"cost", "actual"}, {"overview"}} {
		t.Run(command[len(command)-1], func(t *testing.T) {
			result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), command)
			assert.Equal(t, 2, result.ExitCode, string(result.Stderr))
			assert.Contains(t, string(result.Stderr), "Pulumi")
			assert.Contains(t, string(result.Stderr), "Terraform")
		})
	}
}
