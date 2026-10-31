package cli_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/rshade/ax-go/axtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

const noChangesSentence = "No changes. Projected monthly cost is zero."

// useNoChangeTofu enters a Terraform project whose tofu reports a plan with no changes.
func useNoChangeTofu(t *testing.T) {
	requirePOSIXShell(t)
	t.Helper()
	project := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(project, "main.tf"),
			[]byte("resource \"null_resource\" \"n\" {}\n"),
			0o600,
		),
	)
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	bin := t.TempDir()
	script := []byte(`#!/bin/sh
if [ "$1" = "version" ]; then
  echo '{"terraform_version":"1.6.0"}'
  exit 0
fi
if [ "$1" = "plan" ]; then
  out=""
  for arg in "$@"; do
    case "$arg" in
      -out=*) out="${arg#-out=}" ;;
    esac
  done
  : > "$out"
  exit 0
fi
if [ "$1" = "show" ]; then
  echo '{"format_version":"1.2","resource_changes":[]}'
  exit 0
fi
exit 1
`)
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tofu"), script, 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	t.Chdir(project)
}

//nolint:paralleltest // useNoChangeTofu sets the environment, changes directory, and runs a script.
func TestCostProjectedTerraformNoChanges(t *testing.T) {
	useNoChangeTofu(t)

	result := axtest.Run(
		context.Background(),
		t,
		cli.NewRootCmd("test"),
		[]string{"cost", "projected"},
	)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stdout), noChangesSentence)
}

// assertNoChangeMachineOutput runs every command that reports a Terraform plan
// with no changes, in every machine format (--output and ax's --format): stdout
// holds only the command's own empty document, the sentence moves to stderr,
// and the exit code stays 0.
func assertNoChangeMachineOutput(t *testing.T, input ...string) {
	t.Helper()
	commands := []struct {
		args        []string
		ndjsonLines int
	}{
		{args: []string{"cost", "projected"}},
		{args: []string{"cost", "recommendations"}, ndjsonLines: 1},
		{args: []string{"cost", "forecast"}, ndjsonLines: 1},
		{args: []string{"cost", "estimate"}},
		{args: []string{"overview"}},
	}
	formats := [][]string{
		{"--output", "json"},
		{"--output", "ndjson"},
		{"--format", "json"},
	}
	for _, command := range commands {
		for _, format := range formats {
			t.Run(strings.Join(slices.Concat(command.args, format), " "), func(t *testing.T) {
				args := slices.Concat(command.args, input, format)
				result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"), args)
				assert.Equal(t, 0, result.ExitCode, string(result.Stderr))
				assert.NotContains(t, string(result.Stdout), noChangesSentence)
				assert.Contains(t, string(result.Stderr), noChangesSentence)
				if format[1] == "ndjson" {
					lines := slices.Collect(strings.Lines(string(result.Stdout)))
					assert.Len(t, lines, command.ndjsonLines, string(result.Stdout))
					for _, line := range lines {
						assert.True(t, json.Valid([]byte(line)), line)
					}
					return
				}
				require.True(t, json.Valid(result.Stdout), string(result.Stdout))
				if command.args[len(command.args)-1] == "projected" {
					assertUnchangedProjectedJSON(t, result.Stdout)
				}
			})
		}
	}
}

//nolint:paralleltest // useNoChangeTofu sets the environment, changes directory, and runs a script.
func TestTerraformAutoPlanNoChangesMachineOutput(t *testing.T) {
	useNoChangeTofu(t)
	assertNoChangeMachineOutput(t)
}

// Sets the environment and changes directory, so it does not run in parallel.
func TestTerraformSavedPlanNoChangesMachineOutput(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	t.Setenv("PATH", t.TempDir())
	t.Chdir(t.TempDir())
	plan := filepath.Join(t.TempDir(), "plan.json")
	require.NoError(t, os.WriteFile(plan, []byte(`{"format_version":"1.2","resource_changes":[]}`), 0o600))
	assertNoChangeMachineOutput(t, "--terraform-plan", plan)
}

func assertUnchangedProjectedJSON(t *testing.T, stdout []byte) {
	t.Helper()
	var doc struct {
		Finfocus struct {
			EmptyPlan bool              `json:"emptyPlan"`
			Resources []json.RawMessage `json:"resources"`
			Stack     json.RawMessage   `json:"stack"`
			Summary   struct {
				TotalMonthly float64 `json:"totalMonthly"`
				Currency     string  `json:"currency"`
			} `json:"summary"`
		} `json:"finfocus"`
	}
	require.NoError(t, json.Unmarshal(stdout, &doc), string(stdout))
	assert.True(t, doc.Finfocus.EmptyPlan)
	assert.NotNil(t, doc.Finfocus.Resources)
	assert.Empty(t, doc.Finfocus.Resources)
	assert.Nil(t, doc.Finfocus.Stack)
	assert.Zero(t, doc.Finfocus.Summary.TotalMonthly)
	assert.Equal(t, "USD", doc.Finfocus.Summary.Currency)
}

func TestCostProjectedTerraformPlanFile(t *testing.T) {
	requirePOSIXShell(t)
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")
	bin := t.TempDir()
	marker := filepath.Join(home, "ran")
	script := "#!/bin/sh\necho ran > '" + marker + "'\nexit 1\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "tofu"), []byte(script), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+"/usr/bin:/bin")
	t.Chdir(t.TempDir())

	plan := filepath.Join(home, "plan.json")
	require.NoError(
		t,
		os.WriteFile(plan, []byte(`{"format_version":"1.2","resource_changes":[]}`), 0o600),
	)
	result := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"cost", "projected", "--terraform-plan", plan})
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stdout), "No changes. Projected monthly cost is zero.")
	_, err := os.Stat(marker)
	require.ErrorIs(t, err, os.ErrNotExist)

	combined := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"cost", "projected", "--terraform-plan", plan, "--pulumi-json", plan})
	assert.Equal(t, 2, combined.ExitCode)

	missing := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"cost", "projected", "--terraform-plan", filepath.Join(home, "missing.json")})
	assert.Equal(t, 2, missing.ExitCode)

	bad := filepath.Join(home, "bad.json")
	require.NoError(
		t,
		os.WriteFile(bad, []byte(`{"format_version":"2.0","resource_changes":[]}`), 0o600),
	)
	badResult := axtest.Run(context.Background(), t, cli.NewRootCmd("test"),
		[]string{"cost", "projected", "--terraform-plan", bad})
	assert.Equal(t, 2, badResult.ExitCode)
}

func TestCostProjectedTerraformPlanSizeLimit(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "large.json")
	file, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, file.Truncate(32*1024*1024+1))
	require.NoError(t, file.Close())
	result := axtest.Run(
		context.Background(),
		t,
		cli.NewRootCmd("test"),
		[]string{"cost", "projected", "--terraform-plan", path},
	)
	assert.Equal(t, 2, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "32 MiB")
}
