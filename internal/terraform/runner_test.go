//go:build unix

package terraform

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // writes a binary and executes it
func TestRunPlanSuccess(t *testing.T) {
	project := t.TempDir()
	record := filepath.Join(project, "args.txt")
	meta := filepath.Join(project, "meta.txt")
	script := `#!/bin/sh
echo "$@" >> __RECORD__
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
  echo "$out" > __META__
  (stat -c %a "$(dirname "$out")" 2>/dev/null || stat -f %Lp "$(dirname "$out")") >> __META__
  : > "$out"
  exit 0
fi
if [ "$1" = "show" ]; then
  (stat -c %a "$4" 2>/dev/null || stat -f %Lp "$4") >> __META__
  echo '{"format_version":"1.2","marker":"do-not-log"}'
  exit 0
fi
exit 1
`
	installFake(t, script, map[string]string{"__RECORD__": record, "__META__": meta})
	var logs bytes.Buffer
	ctx := zerolog.New(&logs).WithContext(context.Background())
	got, err := RunPlan(ctx, project)
	require.NoError(t, err)
	assert.Contains(t, string(got), "do-not-log")
	assert.NotContains(t, logs.String(), "do-not-log")

	args, err := os.ReadFile(record)
	require.NoError(t, err)
	text := string(args)
	assert.Contains(t, text, "plan -input=false -lock=false -no-color -out=")
	assert.Contains(t, text, "show -json -no-color ")

	metaText, err := os.ReadFile(meta)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(metaText)), "\n")
	require.Len(t, lines, 3)
	planPath := lines[0]
	assert.False(t, strings.HasPrefix(planPath, project+string(os.PathSeparator)))
	assert.Equal(t, "700", lines[1])
	assert.Equal(t, "600", lines[2])
	_, statErr := os.Stat(planPath)
	require.ErrorIs(t, statErr, os.ErrNotExist)
	_, dirErr := os.Stat(filepath.Dir(planPath))
	assert.ErrorIs(t, dirErr, os.ErrNotExist)
}

//nolint:paralleltest // writes a binary and executes it
func TestRunPlanHints(t *testing.T) {
	project := t.TempDir()
	t.Run("missing binary", func(t *testing.T) {
		t.Setenv("PATH", project)
		_, err := RunPlan(context.Background(), project)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "tofu command on PATH")
	})

	cases := []struct {
		name   string
		stderr string
		want   string
	}{
		{name: "init", stderr: "Error: terraform init is required\n", want: "not initialized"},
		{
			name:   "aws creds",
			stderr: "No valid credential sources found\n",
			want:   "credentials or the backend",
		},
		{
			name:   "aws refresh",
			stderr: "failed to refresh cached credentials\n",
			want:   "credentials or the backend",
		},
		{
			name:   "expired",
			stderr: "ExpiredToken: token expired\n",
			want:   "credentials or the backend",
		},
		{
			name:   "invalid token",
			stderr: "InvalidClientTokenId\n",
			want:   "credentials or the backend",
		},
		{
			name:   "azure arm",
			stderr: "Error building ARM Config\n",
			want:   "credentials or the backend",
		},
		{name: "azure login", stderr: "run az login\n", want: "credentials or the backend"},
		{
			name:   "azure authorizer",
			stderr: "unable to build authorizer\n",
			want:   "credentials or the backend",
		},
		{
			name:   "gcp",
			stderr: "could not find default credentials\n",
			want:   "credentials or the backend",
		},
		{
			name:   "gcp google",
			stderr: "google: could not find default credentials\n",
			want:   "credentials or the backend",
		},
		{
			name:   "backend",
			stderr: "error configuring the backend\n",
			want:   "credentials or the backend",
		},
		{name: "state", stderr: "Error loading state\n", want: "credentials or the backend"},
		{
			name:   "workspaces",
			stderr: "Failed to get existing workspaces\n",
			want:   "credentials or the backend",
		},
		{name: "inspect", stderr: "Error inspecting states\n", want: "credentials or the backend"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installPlanFailure(t, tc.stderr, `echo '{"terraform_version":"1.6.0"}'`)
			_, err := RunPlan(context.Background(), project)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.NotContains(t, err.Error(), "Projected")
			matches, err := filepath.Glob(filepath.Join(project, "plan.bin"))
			require.NoError(t, err)
			assert.Empty(t, matches)
		})
	}

	t.Run("unrecognized stderr keeps the last 20 lines", func(t *testing.T) {
		var stderr strings.Builder
		for i := 1; i <= 25; i++ {
			stderr.WriteString("line " + strconv.Itoa(i) + "\n")
		}
		installPlanFailure(t, stderr.String(), `echo '{"terraform_version":"1.6.0"}'`)
		_, err := RunPlan(context.Background(), project)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "OpenTofu plan failed.")
		assert.NotContains(t, err.Error(), "line 5\n")
		assert.Contains(t, err.Error(), "line 6\n")
		assert.Contains(t, err.Error(), "line 25")
	})

	t.Run("old OpenTofu", func(t *testing.T) {
		installPlanFailure(t, "", `echo '{"terraform_version":"1.0.0"}'`)
		_, err := RunPlan(context.Background(), project)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "OpenTofu 1.6.0")
		assert.NotContains(t, err.Error(), "panic")
	})

	t.Run("old tofu", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		installNamed(
			t,
			"tofu",
			versionThenExit(`echo '{"terraform_version":"1.5.9"}'`, "echo old >&2\nexit 1\n"),
		)
		_, err := RunPlan(context.Background(), project)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "OpenTofu 1.6.0")
	})
}

//nolint:paralleltest // writes a binary and executes it
func TestRunPlanCancelKillsProcessGroup(t *testing.T) {
	project := t.TempDir()
	pidFile := filepath.Join(project, "child.pid")
	script := `#!/bin/sh
if [ "$1" = "version" ]; then
  echo '{"terraform_version":"1.6.0"}'
  exit 0
fi
sleep 30 &
echo $! > __PIDFILE__
wait
`
	installFake(t, script, map[string]string{"__PIDFILE__": pidFile})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := RunPlan(ctx, project)
		done <- err
	}()
	require.Eventually(t, func() bool {
		_, err := os.Stat(pidFile)
		return err == nil
	}, 2*time.Second, 20*time.Millisecond)
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("plan did not return after cancel")
	}
	pidText, err := os.ReadFile(pidFile)
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidText)))
	require.NoError(t, err)
	assert.Eventually(t, func() bool {
		return syscall.Kill(pid, 0) != nil
	}, 2*time.Second, 10*time.Millisecond, "cancelled child must terminate")
}

func installPlanFailure(t *testing.T, stderr, versionBody string) {
	t.Helper()
	script := versionThenExit(versionBody, "echo "+shellQuote(stderr)+" >&2\nexit 1\n")
	installFake(t, script, nil)
}

func versionThenExit(versionBody, failure string) string {
	return "#!/bin/sh\nif [ \"$1\" = \"version\" ]; then\n  " + versionBody + "\n  exit 0\nfi\n" + failure
}

func installNamed(t *testing.T, name, script string) {
	t.Helper()
	dir := filepath.Dir(os.Getenv("PATH"))
	if !strings.Contains(os.Getenv("PATH"), string(os.PathListSeparator)) {
		dir = os.Getenv("PATH")
	}
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(script), 0o700))
}

func installFake(t *testing.T, script string, env map[string]string) {
	t.Helper()
	dir := t.TempDir()
	for key, value := range env {
		script = strings.ReplaceAll(script, key, shellQuote(value))
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tofu"), []byte(script), 0o700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+"/usr/bin:/bin")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func TestVersionOrdering(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		version, minimum string
		older, valid     bool
	}{
		{"1.1.0", "1.1.0", false, true}, {"1.0.9", "1.1.0", true, true}, {"0.99.0", "1.1.0", true, true},
		{"2.0.0", "1.1.0", false, true}, {"1.1", "1.1.0", false, true}, {"1.1.0-beta+build", "1.1.0", false, true},
		{"bogus", "1.1.0", false, false}, {"1.x.0", "1.1.0", false, false}, {"1.1.0", "invalid", false, false},
	} {
		t.Run(tc.version+"/"+tc.minimum, func(t *testing.T) {
			t.Parallel()
			older, valid := versionOlder(tc.version, tc.minimum)
			assert.Equal(t, tc.older, older)
			assert.Equal(t, tc.valid, valid)
		})
	}
}

//nolint:paralleltest // Writes and executes fake binaries and changes PATH.
func TestRunnerFailureAndVersionEdges(t *testing.T) {
	project := t.TempDir()
	for _, tc := range []struct{ name, version, body, want string }{
		{"malformed version", "echo invalid", "echo failure >&2; exit 1", "OpenTofu plan failed."},
		{"missing version", "echo '{}'", "echo failure >&2; exit 1", "OpenTofu plan failed."},
		{"version command failed", "exit 1", "echo failure >&2; exit 1", "OpenTofu plan failed."},
		{"plan file absent", `echo '{"terraform_version":"1.6.0"}'`, "exit 0", "OpenTofu plan failed."},
		{"show failure", `echo '{"terraform_version":"1.6.0"}'`, `if [ "$1" = plan ]; then for arg in "$@"; do case "$arg" in -out=*) : > "${arg#-out=}";; esac; done; exit 0; fi; echo show-failed >&2; exit 1`, "show-failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			installFake(t, versionThenExit(tc.version, tc.body), nil)
			_, err := RunPlan(context.Background(), project)
			require.ErrorContains(t, err, tc.want)
		})
	}
	t.Run("temp creation failure", func(t *testing.T) {
		installFake(
			t,
			versionThenExit(`echo '{"terraform_version":"1.6.0"}'`, "exit 0"),
			nil,
		)
		t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
		_, err := RunPlan(context.Background(), project)
		require.ErrorContains(t, err, "creating plan directory")
	})
	t.Run("cancelled before start", func(t *testing.T) {
		installFake(
			t,
			versionThenExit(`echo '{"terraform_version":"1.6.0"}'`, "exit 0"),
			nil,
		)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := RunPlan(ctx, project)
		require.ErrorIs(t, err, context.Canceled)
	})
}

func TestRunPlanUsesOnlyOpenTofu(t *testing.T) {
	for _, withTofu := range []bool{true, false} {
		t.Run(strconv.FormatBool(withTofu), func(t *testing.T) {
			bin := t.TempDir()
			marker := filepath.Join(t.TempDir(), "terraform-was-run")
			forbidden := "#!/bin/sh\necho forbidden > " + shellQuote(marker) + "\nexit 1\n"
			require.NoError(
				t,
				os.WriteFile(filepath.Join(bin, "terraform"), []byte(forbidden), 0o700),
			)
			if withTofu {
				tofu := `#!/bin/sh
if [ "$1" = version ]; then echo '{"terraform_version":"1.6.0"}'; exit 0; fi
if [ "$1" = plan ]; then for arg in "$@"; do case "$arg" in -out=*) : > "${arg#-out=}";; esac; done; exit 0; fi
if [ "$1" = show ]; then echo '{"format_version":"1.2","tool":"tofu"}'; exit 0; fi
exit 1
`
				require.NoError(t, os.WriteFile(filepath.Join(bin, "tofu"), []byte(tofu), 0o700))
			}
			t.Setenv("PATH", bin)
			got, err := RunPlan(context.Background(), t.TempDir())
			if withTofu {
				require.NoError(t, err)
				assert.Contains(t, string(got), `"tool":"tofu"`)
			} else {
				var hint *HintError
				require.ErrorAs(t, err, &hint)
				assert.Equal(t, HintBinaryMissing, hint.Kind)
				assert.Contains(t, hint.Text, "OpenTofu")
			}
			_, err = os.Stat(marker)
			assert.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

//nolint:paralleltest // Installs and executes fake binaries and changes PATH.
func TestRunPlanRequiresSupportedOpenTofuVersion(t *testing.T) {
	for _, version := range []string{`echo '{"terraform_version":"1.5.9"}'`, `echo invalid`, `echo '{}'`, `exit 1`} {
		t.Run(version, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "plan-was-run")
			script := versionThenExit(version, "echo ran > "+shellQuote(marker)+"\nexit 1\n")
			installFake(t, script, nil)
			_, err := RunPlan(context.Background(), t.TempDir())
			require.Error(t, err)
			_, err = os.Stat(marker)
			assert.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}
