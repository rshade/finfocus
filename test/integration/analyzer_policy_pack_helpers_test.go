package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	policyPackTestVersion = "v0.0.0-policypack"
	policyPackTimeout     = 3 * time.Minute
	analyzerServingMarker = "analyzer server listening"
	languagePluginEntry   = "pulumi-language-finfocus"
	legacyAnalyzerEntry   = "pulumi-analyzer-policy-finfocus"
	emptyProgramYAML      = "name: policy-pack-smoke\nruntime: yaml\n"
)

// policyPackPulumi returns the pulumi binary named by FINFOCUS_TEST_PULUMI, or
// skips: the result depends on the Pulumi version, so the check is opt-in.
// The path must be absolute, not a mise shim, which resolves per directory.
func policyPackPulumi(t *testing.T) string {
	t.Helper()
	pulumiBin := os.Getenv("FINFOCUS_TEST_PULUMI")
	if pulumiBin == "" {
		t.Skip("set FINFOCUS_TEST_PULUMI to a pulumi binary to run the policy pack check")
	}
	require.True(t, filepath.IsAbs(pulumiBin), "FINFOCUS_TEST_PULUMI must be an absolute path, got %q", pulumiBin)
	return pulumiBin
}

func buildPolicyPackBinary(t *testing.T) string {
	t.Helper()
	name := "finfocus"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	out, err := exec.Command("go", "build",
		"-ldflags", "-X github.com/rshade/finfocus/pkg/version.version="+policyPackTestVersion,
		"-o", binary, "../../cmd/finfocus").CombinedOutput()
	require.NoError(t, err, "build finfocus: %s", out)
	return binary
}

func pulumiVersionOutput(t *testing.T, pulumiBin string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), policyPackTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, pulumiBin, "version")
	// Its own PULUMI_HOME keeps the version check's log out of the real home.
	cmd.Env = append(os.Environ(), "PULUMI_SKIP_UPDATE_CHECK=true", "PULUMI_HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "pulumi version: %s", out)
	return string(out)
}

type policyPackRun struct {
	t                  *testing.T
	pulumiBin          string
	binary             string
	work               string
	finfocusHome       string
	pulumiHome         string
	packDir            string
	project            string
	disableAcquisition bool
	extraEnv           []string
}

func newPolicyPackRun(t *testing.T, pulumiBin, binary string, disableAcquisition bool) *policyPackRun {
	t.Helper()
	work := t.TempDir()
	finfocusHome := filepath.Join(work, "finfocus-home")
	project := filepath.Join(work, "project")
	require.NoError(t, os.MkdirAll(project, 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(work, "state"), 0o750))
	return &policyPackRun{
		t:                  t,
		pulumiBin:          pulumiBin,
		binary:             binary,
		work:               work,
		finfocusHome:       finfocusHome,
		pulumiHome:         filepath.Join(work, "pulumi-home"),
		packDir:            filepath.Join(finfocusHome, "analyzer"),
		project:            project,
		disableAcquisition: disableAcquisition,
	}
}

func (r *policyPackRun) writeProgram() {
	r.t.Helper()
	require.NoError(r.t, os.WriteFile(filepath.Join(r.project, "Pulumi.yaml"), []byte(emptyProgramYAML), 0o600))
}

func (r *policyPackRun) writeProjectConfig(content string) {
	r.t.Helper()
	dir := filepath.Join(r.project, ".finfocus")
	require.NoError(r.t, os.MkdirAll(dir, 0o750))
	require.NoError(r.t, os.WriteFile(filepath.Join(dir, "config.hujson"), []byte(content), 0o600))
}

func (r *policyPackRun) environ() []string {
	env := append(environWithoutPrefixes("FINFOCUS_", "PULUMI_"),
		"FINFOCUS_HOME="+r.finfocusHome,
		"FINFOCUS_LOG_LEVEL=info",
		"PULUMI_HOME="+r.pulumiHome,
		"PULUMI_BACKEND_URL=file://"+filepath.ToSlash(filepath.Join(r.work, "state")),
		"PULUMI_CONFIG_PASSPHRASE=",
		"PULUMI_SKIP_UPDATE_CHECK=true",
		"PATH="+strings.Join([]string{r.packDir, filepath.Dir(r.pulumiBin), os.Getenv("PATH")},
			string(os.PathListSeparator)),
	)
	if r.disableAcquisition {
		env = append(env, "PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=true")
	}
	return append(env, r.extraEnv...)
}

func (r *policyPackRun) run(dir, name string, args ...string) (string, error) {
	r.t.Helper()
	ctx, cancel := context.WithTimeout(r.t.Context(), policyPackTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = r.environ()
	combined, runErr := cmd.CombinedOutput()
	return string(combined), runErr
}

func (r *policyPackRun) install() {
	r.t.Helper()
	installOut, err := r.run(
		r.work, r.binary, "analyzer", "install", "--target-dir", filepath.Join(r.pulumiHome, "plugins"),
	)
	require.NoError(r.t, err, "finfocus analyzer install: %s", installOut)
	assert.Contains(r.t, installOut, "  Version: "+policyPackTestVersion+"\n", "one v before the stamped version")
	assert.Contains(r.t, installOut, `pulumi preview --policy-pack "`+r.packDir+`"`)
	stackOut, err := r.run(r.project, r.pulumiBin, "stack", "init", "dev", "--non-interactive")
	require.NoError(r.t, err, "pulumi stack init: %s", stackOut)
}

func (r *policyPackRun) preview() string {
	r.t.Helper()
	previewOut, err := r.run(r.project, r.pulumiBin, "preview", "--policy-pack", r.packDir, "--non-interactive")
	require.NoError(r.t, err, "pulumi preview --policy-pack failed:\n%s", previewOut)
	return previewOut
}

func assertCostSummary(t *testing.T, summaryDir string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(summaryDir, "last-cost-summary.json"))
	require.NoError(t, err)
	var summary struct {
		Stack   string `json:"stack"`
		Project string `json:"project"`
	}
	require.NoError(t, json.Unmarshal(data, &summary))
	assert.Equal(t, "dev", summary.Stack)
	assert.Equal(t, "policy-pack-smoke", summary.Project)
}

func environWithoutPrefixes(prefixes ...string) []string {
	var kept []string
	for _, kv := range os.Environ() {
		drop := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(kv, prefix) {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, kv)
		}
	}
	return kept
}
