package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

//nolint:paralleltest // changes the working directory and executes a fake Terraform binary.
func TestCostRecommendationsTerraformSources(t *testing.T) {
	requirePOSIXShell(t)
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), []byte("# Terraform root\n"), 0o600))
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	bin := t.TempDir()
	marker := filepath.Join(bin, "ran")
	t.Setenv("TEST_TERRAFORM_MARKER", marker)
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tofu"), []byte(`#!/bin/sh
printf '%s\n' "$1" >> "$TEST_TERRAFORM_MARKER"
case "$1" in
 version) echo '{"terraform_version":"1.6.0"}';;
 plan) for arg in "$@"; do case "$arg" in -out=*) touch "${arg#-out=}";; esac; done;;
 show) echo '{"format_version":"1.2","resource_changes":[]}';;
 *) exit 1;;
esac
`), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(project)
	plan := filepath.Join(project, "plan.json")
	require.NoError(t, os.WriteFile(plan, []byte(`{"format_version":"1.2","resource_changes":[]}`), 0o600))
	for _, source := range []string{"saved", "detected"} {
		t.Run(source, func(t *testing.T) {
			args := []string{"--output", "json"}
			if source == "saved" {
				args = append(args, "--terraform-plan", plan)
			}
			cmd := cli.NewCostRecommendationsCmd()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(args)
			require.NoError(t, cmd.Execute(), output.String())
			if source == "saved" {
				assert.NoFileExists(t, marker)
			} else {
				assert.FileExists(t, marker)
			}
		})
	}
}

//nolint:paralleltest // changes the working directory and PATH to detect accidental plan execution.
func TestRecommendationLifecycleTerraformDoesNotPlan(t *testing.T) {
	requirePOSIXShell(t)
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), []byte("# Terraform root\n"), 0o600))
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	bin := t.TempDir()
	marker := filepath.Join(bin, "ran")
	t.Setenv("TEST_TERRAFORM_MARKER", marker)
	require.NoError(t, os.WriteFile(
		filepath.Join(bin, "tofu"), []byte("#!/bin/sh\ntouch \"$TEST_TERRAFORM_MARKER\"\nexit 1\n"), 0o700,
	))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(project)
	for _, action := range []string{"dismiss", "snooze"} {
		t.Run(action, func(t *testing.T) {
			args := []string{action, "terraform-rec-" + action, "--force"}
			if action == "dismiss" {
				args = append(args, "--reason", "not-applicable")
			} else {
				args = append(args, "--until", "2099-01-01")
			}
			cmd := cli.NewCostRecommendationsCmd()
			cmd.SetArgs(args)
			require.NoError(t, cmd.Execute())
			assert.NoFileExists(t, marker)
			cmd = cli.NewCostRecommendationsCmd()
			cmd.SetArgs(append(args, "--terraform-plan", "plan.json"))
			err := cmd.Execute()
			require.ErrorContains(t, err, "unknown flag: --terraform-plan")
			assert.NoFileExists(t, marker)
		})
	}
}
