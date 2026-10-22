//go:build unix

package integration_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
)

// TestAnalyzerPolicyPack_LoadsInPulumi runs `pulumi preview --policy-pack` on a
// no-resource YAML program, using the policy pack `finfocus analyzer install`
// writes, and checks that Pulumi started the finfocus analyzer (#1895). Pulumi
// 3.270.0 removed the pulumi-analyzer-policy-<runtime> fallback that the pack
// relies on, so this fails on those releases until finfocus ships a
// pulumi-language-finfocus plugin.
//
// The test is opt-in because the result depends on the Pulumi version: set
// FINFOCUS_TEST_PULUMI to the absolute path of a pulumi binary (not a mise
// shim, which resolves per directory).
//
//nolint:paralleltest // builds and executes a binary (text file busy, golang/go#22315)
func TestAnalyzerPolicyPack_LoadsInPulumi(t *testing.T) {
	pulumiBin := os.Getenv("FINFOCUS_TEST_PULUMI")
	if pulumiBin == "" {
		t.Skip("set FINFOCUS_TEST_PULUMI to a pulumi binary to run the policy pack check")
	}
	require.True(t, filepath.IsAbs(pulumiBin), "FINFOCUS_TEST_PULUMI must be an absolute path, got %q", pulumiBin)

	work := t.TempDir()
	finfocusHome := filepath.Join(work, "finfocus-home")
	pulumiHome := filepath.Join(work, "pulumi-home")
	packDir := filepath.Join(finfocusHome, "analyzer")
	project := filepath.Join(work, "project")
	state := filepath.Join(work, "state")
	require.NoError(t, os.MkdirAll(project, 0o750))
	require.NoError(t, os.MkdirAll(state, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(project, "Pulumi.yaml"),
		[]byte("name: policy-pack-smoke\nruntime: yaml\n"), 0o600))

	binary := filepath.Join(work, "finfocus")
	out, err := exec.Command("go", "build",
		"-ldflags", "-X github.com/rshade/finfocus/pkg/version.version="+policyPackTestVersion,
		"-o", binary, "../../cmd/finfocus").CombinedOutput()
	require.NoError(t, err, "build finfocus: %s", out)

	env := append(environWithoutPrefixes("FINFOCUS_", "PULUMI_"),
		"FINFOCUS_HOME="+finfocusHome,
		"FINFOCUS_LOG_LEVEL=info",
		"PULUMI_HOME="+pulumiHome,
		"PULUMI_BACKEND_URL=file://"+state,
		"PULUMI_CONFIG_PASSPHRASE=",
		"PULUMI_SKIP_UPDATE_CHECK=true",
		"PULUMI_DISABLE_AUTOMATIC_PLUGIN_ACQUISITION=true",
		"PATH="+strings.Join([]string{packDir, filepath.Dir(pulumiBin), os.Getenv("PATH")},
			string(os.PathListSeparator)),
	)

	run := func(t *testing.T, dir, name string, args ...string) (string, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), policyPackTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Dir = dir
		cmd.Env = env
		combined, runErr := cmd.CombinedOutput()
		return string(combined), runErr
	}

	version, err := run(t, project, pulumiBin, "version")
	require.NoError(t, err, "pulumi version: %s", version)
	t.Logf("pulumi %s", strings.TrimSpace(version))

	installOut, err := run(t, work, binary, "analyzer", "install", "--target-dir", filepath.Join(pulumiHome, "plugins"))
	require.NoError(t, err, "finfocus analyzer install: %s", installOut)
	assert.Contains(t, installOut, `pulumi preview --policy-pack "`+packDir+`"`)

	stackOut, err := run(t, project, pulumiBin, "stack", "init", "dev", "--non-interactive")
	require.NoError(t, err, "pulumi stack init: %s", stackOut)

	previewOut, err := run(t, project, pulumiBin, "preview", "--policy-pack", packDir, "--non-interactive")
	require.NoError(t, err, "pulumi preview --policy-pack failed:\n%s", previewOut)
	assert.Contains(t, previewOut, analyzerServingMarker,
		"pulumi preview succeeded but the finfocus analyzer never served:\n%s", previewOut)
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
