package cli_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

func runPluginInitForSkill(t *testing.T, opts *cli.PluginInitOptions) (string, string) {
	t.Helper()
	opts.Name = "skill-plugin"
	opts.Author = "Test Author"
	opts.Providers = []string{"aws"}
	opts.OutputDir = t.TempDir()
	opts.Force = true

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetContext(context.Background())
	require.NoError(t, cli.RunPluginInit(cmd.Context(), cmd, opts))
	return buf.String(), filepath.Join(opts.OutputDir, opts.Name)
}

func TestPluginInitInstallsSkill(t *testing.T) {
	t.Parallel()
	out, projectDir := runPluginInitForSkill(t, &cli.PluginInitOptions{})

	assert.True(t, skillInstalledIn(t, projectDir))
	assert.Contains(
		t,
		out,
		"Installed FinFocus agent skills:\n  .agents/skills/finfocus-plugin-dev\n  skills-lock.json\n",
	)
}

func TestPluginInitSkipsSkill(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		opts   cli.PluginInitOptions
		reason string
	}{
		{"no-skill", cli.PluginInitOptions{NoSkill: true}, "--no-skill"},
		{"offline", cli.PluginInitOptions{Offline: true}, "--offline"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opts := tc.opts
			out, projectDir := runPluginInitForSkill(t, &opts)

			assert.False(t, skillInstalledIn(t, projectDir))
			assert.Contains(t, out, "skill install skipped ("+tc.reason+")")
			assert.Contains(t, out, "To add the agent skills later, run in the plugin directory:\n  npx -y skills@")
		})
	}
}

func TestPluginInitDockerOnlySkipsSkill(t *testing.T) {
	t.Parallel()
	out, projectDir := runPluginInitForSkill(t, &cli.PluginInitOptions{DockerOnly: true})

	assert.False(t, skillInstalledIn(t, projectDir))
	assert.NotContains(t, out, "agent skills")
}

func TestPluginInitDryRunShowsSkillCommand(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	tmpDir := t.TempDir()
	root := cli.NewRootCmd("test")
	result := axtest.Run(context.Background(), t, root, []string{
		"plugin", "init", "dry-plugin", "--author", "A", "--providers", "aws",
		"--output-dir", tmpDir, "--dry-run",
	})

	require.Zero(t, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), "Would install the FinFocus agent skills with:\n  npx -y skills@")
	assert.False(t, skillInstalledIn(t, filepath.Join(tmpDir, "dry-plugin")))
}
