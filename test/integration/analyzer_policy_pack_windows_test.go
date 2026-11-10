//go:build windows

package integration_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestAnalyzerPolicyPack_Windows runs `pulumi preview --policy-pack` on a
// no-resource YAML program with the pack `finfocus analyzer install` writes on
// Windows: both `.exe` entries, Pulumi finding pulumi-language-finfocus.exe on
// PATH, and the analyzer starting (#1918). Plugin cleanup is not checked:
// Windows does not promise it (spec 1734 FR-007), and the unix test's process
// checks do not port. Opt-in through FINFOCUS_TEST_PULUMI, like that test.
//
//nolint:paralleltest // builds and executes a binary
func TestAnalyzerPolicyPack_Windows(t *testing.T) {
	pulumiBin := policyPackPulumi(t)
	binary := buildPolicyPackBinary(t)
	t.Logf("pulumi %s", strings.TrimSpace(pulumiVersionOutput(t, pulumiBin)))

	run := newPolicyPackRun(t, pulumiBin, binary, false)
	run.writeProgram()
	run.install()
	for _, name := range []string{legacyAnalyzerEntry + ".exe", languagePluginEntry + ".exe", "PulumiPolicy.yaml"} {
		assert.FileExists(t, filepath.Join(run.packDir, name))
	}

	previewOut := run.preview()
	assert.Contains(t, previewOut, analyzerServingMarker)
	assert.Contains(t, previewOut, "using "+languagePluginEntry)
	assert.NotContains(t, previewOut, "pulumi-finfocus")
	assert.NotContains(t, previewOut, "could not find latest version")
}
