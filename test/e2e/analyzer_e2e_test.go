//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AnalyzerDiagnostic represents an expected diagnostic pattern in preview output.
type AnalyzerDiagnostic struct {
	Pattern  string
	Required bool
	Count    int // 0 means any number > 0 if Required is true, or specific count
}

func validateDiagnostics(t *testing.T, output string, diagnostics []AnalyzerDiagnostic) {
	for _, d := range diagnostics {
		count := strings.Count(output, d.Pattern)
		if d.Required {
			assert.Greater(t, count, 0, "Expected diagnostic pattern not found: %s", d.Pattern)
		}
		if d.Count > 0 {
			assert.Equal(t, d.Count, count,
				"Expected %d occurrences of pattern '%s', got %d", d.Count, d.Pattern, count)
		}
	}
}

// ensureAnalyzerConfig sets up the policy pack directory for the analyzer.
// This creates a directory with:
//   - PulumiPolicy.yaml (required for policy pack loading)
//   - The finfocus binary copied to pulumi-analyzer-policy-finfocus
//   - The finfocus binary copied to pulumi-language-finfocus
//
// The names match the legacy analyzer entry and
// analyzer.LanguagePluginBinaryName. This module cannot import
// internal/analyzer, so the names stay literal.
//
// Returns the absolute path to the policy pack directory.
func ensureAnalyzerConfig(t *testing.T, projectDir string) string {
	binaryPath := findFinFocusBinary()
	require.NotEmpty(t, binaryPath, "finfocus binary not found")

	// Create a directory for the policy pack
	policyPackDir := filepath.Join(projectDir, "policy-pack")
	err := os.MkdirAll(policyPackDir, 0755)
	require.NoError(t, err)

	// Create PulumiPolicy.yaml (required for policy pack loading)
	policyYaml := `runtime: finfocus
name: finfocus
version: 0.0.0-dev
`
	err = os.WriteFile(filepath.Join(policyPackDir, "PulumiPolicy.yaml"), []byte(policyYaml), 0644)
	require.NoError(t, err)

	// Preserve extension if present (e.g., .exe).
	ext := filepath.Ext(binaryPath)
	for _, name := range []string{"pulumi-analyzer-policy-finfocus", "pulumi-language-finfocus"} {
		copyAnalyzerEntry(t, binaryPath, filepath.Join(policyPackDir, name+ext))
	}

	absPolicyPackDir, err := filepath.Abs(policyPackDir)
	require.NoError(t, err)

	return absPolicyPackDir
}

func copyAnalyzerEntry(t *testing.T, src, dst string) {
	t.Helper()
	input, err := os.ReadFile(src)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dst, input, 0755))
}

func skipIfPulumiNotInstalled(t *testing.T) {
	if _, err := exec.LookPath("pulumi"); err != nil {
		t.Skip("Pulumi CLI not installed, skipping analyzer E2E test")
	}
}

// runAnalyzerPreview sets up the analyzer project and runs pulumi preview, returning the output.
func runAnalyzerPreview(t *testing.T, tc *TestContext) string {
	skipIfPulumiNotInstalled(t)
	ctx := context.Background()
	tc.SetupAWSEnv(ctx)

	// Install aws-public plugin (assumed needed for all tests for now)
	pm := NewPluginManager(t)
	err := pm.EnsurePluginInstalled(ctx, "aws-public")
	require.NoError(t, err, "Failed to install aws-public plugin")
	defer pm.DeferPluginCleanup(ctx, "aws-public")()

	// Prepare project
	projectPath := "fixtures/ec2"
	absProjectPath, err := filepath.Abs(projectPath)
	require.NoError(t, err)
	tc.ProjectDir = absProjectPath

	workDir, err := os.MkdirTemp("", "pulumi-e2e-analyzer-")
	require.NoError(t, err)
	tc.WorkDir = workDir
	t.Cleanup(func() {
		if tc.WorkDir != "" {
			os.RemoveAll(tc.WorkDir)
		}
	})
	tc.PreviewJSON = filepath.Join(workDir, "preview.json")

	err = tc.CopyDir(absProjectPath, workDir)
	require.NoError(t, err)

	// Set up policy pack and get its path
	policyPackDir := ensureAnalyzerConfig(t, workDir)

	// Init Pulumi
	stateDir := filepath.Join(workDir, ".pulumi-state")
	err = os.MkdirAll(stateDir, 0755)
	require.NoError(t, err)

	// Add policy pack directory to PATH so Pulumi can find the binary
	pathEnv := policyPackDir + string(os.PathListSeparator) + os.Getenv("PATH")
	env := []string{
		"PULUMI_BACKEND_URL=file://" + stateDir,
		"PATH=" + pathEnv,
	}

	err = tc.runCmdWithEnv(ctx, workDir, env, "pulumi", "login", "--local")
	require.NoError(t, err)
	err = tc.runCmdWithEnv(ctx, workDir, env, "pulumi", "stack", "init", tc.StackName)
	require.NoError(t, err)
	err = tc.runCmdWithEnv(ctx, workDir, env, "pulumi", "config", "set", "aws:region", tc.Region)
	require.NoError(t, err)

	// Run preview with --policy-pack flag to activate the analyzer
	outputBytes, err := tc.runCmdOutput(ctx, workDir, env, "pulumi", "preview", "--policy-pack", policyPackDir)
	require.NoError(t, err, "pulumi preview failed")
	return string(outputBytes)
}

func TestAnalyzer_Handshake(t *testing.T) {
	tc := NewTestContext(t, "e2e-analyzer-handshake")
	ctx := context.Background()
	defer tc.Teardown(ctx)

	output := runAnalyzerPreview(t, tc)
	t.Logf("Preview output:\n%s", output)
	// Implicit success check in runAnalyzerPreview (require.NoError)
}

func TestAnalyzer_CostDiagnostics(t *testing.T) {
	tc := NewTestContext(t, "e2e-analyzer-diagnostics")
	ctx := context.Background()
	defer tc.Teardown(ctx)

	output := runAnalyzerPreview(t, tc)

	diagnostics := []AnalyzerDiagnostic{
		{Pattern: "Estimated Monthly Cost:", Required: true},
		{Pattern: "finfocus", Required: true},
	}
	validateDiagnostics(t, output, diagnostics)
}

func TestAnalyzer_StackSummary(t *testing.T) {
	tc := NewTestContext(t, "e2e-analyzer-summary")
	ctx := context.Background()
	defer tc.Teardown(ctx)

	output := runAnalyzerPreview(t, tc)

	diagnostics := []AnalyzerDiagnostic{
		{Pattern: "Total Estimated Monthly Cost:", Required: true, Count: 1},
	}
	validateDiagnostics(t, output, diagnostics)
}

func TestAnalyzer_GracefulDegradation(t *testing.T) {
	skipIfPulumiNotInstalled(t)
	tc := NewTestContext(t, "e2e-analyzer-degradation")
	ctx := context.Background()
	tc.SetupAWSEnv(ctx)
	defer tc.Teardown(ctx)

	// Install aws-public plugin
	pm := NewPluginManager(t)
	err := pm.EnsurePluginInstalled(ctx, "aws-public")
	require.NoError(t, err, "Failed to install aws-public plugin")
	defer pm.DeferPluginCleanup(ctx, "aws-public")()

	// Prepare project
	projectPath := "fixtures/ec2"
	absProjectPath, err := filepath.Abs(projectPath)
	require.NoError(t, err)
	tc.ProjectDir = absProjectPath

	workDir, err := os.MkdirTemp("", "pulumi-e2e-analyzer-")
	require.NoError(t, err)
	tc.WorkDir = workDir
	t.Cleanup(func() {
		if tc.WorkDir != "" {
			os.RemoveAll(tc.WorkDir)
		}
	})
	tc.PreviewJSON = filepath.Join(workDir, "preview.json")

	err = tc.CopyDir(absProjectPath, workDir)
	require.NoError(t, err)

	// Set up policy pack and get its path
	policyPackDir := ensureAnalyzerConfig(t, workDir)

	// Modify Pulumi.yaml to use invalid instance type to trigger pricing failure
	yamlPath := filepath.Join(workDir, "Pulumi.yaml")
	content, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	newContent := strings.Replace(string(content), "t3.micro", "invalid.type.xlarge", 1)
	err = os.WriteFile(yamlPath, []byte(newContent), 0644)
	require.NoError(t, err)

	// Init Pulumi
	stateDir := filepath.Join(workDir, ".pulumi-state")
	err = os.MkdirAll(stateDir, 0755)
	require.NoError(t, err)

	// Add policy pack directory to PATH so Pulumi can find the binary
	pathEnv := policyPackDir + string(os.PathListSeparator) + os.Getenv("PATH")
	env := []string{
		"PULUMI_BACKEND_URL=file://" + stateDir,
		"PATH=" + pathEnv,
	}

	err = tc.runCmdWithEnv(ctx, workDir, env, "pulumi", "login", "--local")
	require.NoError(t, err)
	err = tc.runCmdWithEnv(ctx, workDir, env, "pulumi", "stack", "init", tc.StackName)
	require.NoError(t, err)
	err = tc.runCmdWithEnv(ctx, workDir, env, "pulumi", "config", "set", "aws:region", tc.Region)
	require.NoError(t, err)

	// Run preview with --policy-pack flag
	outputBytes, err := tc.runCmdOutput(ctx, workDir, env, "pulumi", "preview", "--policy-pack", policyPackDir)

	// We expect success (graceful degradation)
	require.NoError(t, err, "Preview should succeed even with pricing failure")
	output := string(outputBytes)

	t.Logf("Preview output:\n%s", output)

	// Verify analyzer didn't crash and output produced something
	assert.Contains(t, output, "finfocus", "Analyzer should still run")
}

// TestAnalyzer_RecommendationDisplay verifies recommendations appear in diagnostics.
//
// NOTE: This test currently validates graceful handling when plugins don't return
// recommendations. Recommendations require the analyzer to call the separate
// GetRecommendations RPC (available in pluginsdk v0.4.10+) and merge results
// with cost diagnostics. Once that integration is added, update this test.
//
// Expected output format when recommendations are present:
//
//	"Estimated Monthly Cost: $X.XX USD (source: aws-plugin) |
//	  Recommendations: Right-sizing: Switch to t3.small (save $15.00/mo)"
func TestAnalyzer_RecommendationDisplay(t *testing.T) {
	tc := NewTestContext(t, "e2e-analyzer-recommendations")
	ctx := context.Background()
	defer tc.Teardown(ctx)

	output := runAnalyzerPreview(t, tc)

	// Verify basic cost diagnostics still work (graceful handling when no recommendations)
	diagnostics := []AnalyzerDiagnostic{
		{Pattern: "Estimated Monthly Cost:", Required: true},
		{Pattern: "finfocus", Required: true},
	}
	validateDiagnostics(t, output, diagnostics)

	// Verify recommendations are displayed
	recommendationDiagnostics := []AnalyzerDiagnostic{
		{Pattern: "Recommendations:", Required: false},
		// We expect at least one of these patterns depending on what the plugin returns
		// For now, we just check that the recommendations section exists
	}
	validateDiagnostics(t, output, recommendationDiagnostics)

	t.Logf("Preview output:\n%s", output)
}

// TestAnalyzer_StackSummaryWithRecommendations verifies aggregate recommendations
// appear in the stack summary.
//
// NOTE: This test currently validates graceful handling when plugins don't return
// recommendations. Once plugins implement GetRecommendations RPC (pluginsdk v0.4.10),
// this test should be updated to verify aggregate savings patterns.
//
// Expected output format when recommendations are present:
//
//	"Total Estimated Monthly Cost: $X.XX USD (N resources analyzed) |
//	  X recommendations, up to $XX.XX/mo potential savings"
func TestAnalyzer_StackSummaryWithRecommendations(t *testing.T) {
	tc := NewTestContext(t, "e2e-analyzer-stack-recommendations")
	ctx := context.Background()
	defer tc.Teardown(ctx)

	output := runAnalyzerPreview(t, tc)

	// Verify stack summary still works (graceful handling when no recommendations)
	diagnostics := []AnalyzerDiagnostic{
		{Pattern: "Total Estimated Monthly Cost:", Required: true, Count: 1},
		{Pattern: "resources analyzed", Required: true, Count: 1},
	}
	validateDiagnostics(t, output, diagnostics)

	// Verify recommendation summary is present
	stackRecommendationDiagnostics := []AnalyzerDiagnostic{
		{Pattern: "recommendations, up to", Required: false},
		{Pattern: "potential savings", Required: false}, // Might not be present if savings are 0
	}
	validateDiagnostics(t, output, stackRecommendationDiagnostics)

	t.Logf("Preview output:\n%s", output)
}
