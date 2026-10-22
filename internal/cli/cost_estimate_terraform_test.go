package cli_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

//nolint:paralleltest // changes the working directory and executes a fake Terraform binary.
func TestCostEstimateTerraformSources(t *testing.T) {
	plan, err := filepath.Abs("../ingest/testdata/terraform/plan/show.json")
	require.NoError(t, err)
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), []byte("# Terraform root\n"), 0o600))
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("TEST_TERRAFORM_PLAN", plan)
	bin := t.TempDir()
	marker := filepath.Join(bin, "ran")
	t.Setenv("TEST_TERRAFORM_MARKER", marker)
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tofu"), []byte(`#!/bin/sh
printf '%s\n' "$1" >> "$TEST_TERRAFORM_MARKER"
case "$1" in
 version) echo '{"terraform_version":"1.6.0"}';;
 plan) for arg in "$@"; do case "$arg" in -out=*) touch "${arg#-out=}";; esac; done;;
 show) cat "$TEST_TERRAFORM_PLAN";;
 *) exit 1;;
esac
`), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(project)
	for _, source := range []string{"saved", "detected"} {
		t.Run(source, func(t *testing.T) {
			args := []string{"--output", "json", "--modify", "aws_s3_bucket.site:bucket=alternate-site"}
			if source == "saved" {
				args = append(args, "--terraform-plan", plan)
			}
			cmd := cli.NewCostEstimateCmd()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs(args)
			require.NoError(t, cmd.Execute(), output.String())
			assert.NotContains(t, output.String(), "did not match")
			if source == "saved" {
				assert.NoFileExists(t, marker)
			} else {
				assert.FileExists(t, marker)
			}
		})
	}
}

//nolint:paralleltest // changes process environment and working directory for CLI input validation.
func TestEstimateRecommendationsTerraformInputErrors(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Chdir(t.TempDir())
	bad := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(bad, []byte(`{"format_version":"2.0","resource_changes":[]}`), 0o600))
	for _, command := range []string{"estimate", "recommendations"} {
		t.Run(command, func(t *testing.T) {
			for _, plan := range []string{bad, filepath.Join(t.TempDir(), "missing.json")} {
				result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
					[]string{"cost", command, "--terraform-plan", plan})
				assert.Equal(t, 2, result.ExitCode, string(result.Stderr))
			}
		})
	}
}
