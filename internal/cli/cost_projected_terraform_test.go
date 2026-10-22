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

func TestCostProjectedTerraformNoChanges(t *testing.T) {
	project := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(
			filepath.Join(project, "main.tf"),
			[]byte("resource \"null_resource\" \"n\" {}\n"),
			0o600,
		),
	)
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
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

	result := axtest.Run(
		context.Background(),
		t,
		cli.NewRootCmd("test"),
		[]string{"cost", "projected"},
	)
	assert.Equal(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stdout), "No changes. Projected monthly cost is zero.")
}

func TestCostProjectedTerraformPlanFile(t *testing.T) {
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
