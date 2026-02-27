package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/analyzer"
)

func TestAnalyzerCheckCmd_AllPass(t *testing.T) {
	originalRunChecks := runAnalyzerChecks
	t.Cleanup(func() {
		runAnalyzerChecks = originalRunChecks
	})

	runAnalyzerChecks = func(_ context.Context) (*analyzer.CheckReport, error) {
		return &analyzer.CheckReport{
			Checks: []analyzer.CheckResult{
				{
					Name:        "policy_pack_dir",
					DisplayName: "Policy pack directory",
					Status:      "pass",
					Message:     "directory exists",
				},
				{
					Name:        "pulumi_policy_yaml",
					DisplayName: "PulumiPolicy.yaml",
					Status:      "pass",
					Message:     "configuration valid",
				},
			},
			AllPass: true,
		}, nil
	}

	cmd := NewAnalyzerCheckCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Policy pack directory")
	assert.Contains(t, output, "PulumiPolicy.yaml")
	assert.Contains(t, output, "PASS")
	assert.Contains(t, output, "All checks passed")
}

func TestAnalyzerCheckCmd_JSONOutput(t *testing.T) {
	originalRunChecks := runAnalyzerChecks
	t.Cleanup(func() {
		runAnalyzerChecks = originalRunChecks
	})

	expected := &analyzer.CheckReport{
		Checks: []analyzer.CheckResult{
			{
				Name:        "policy_pack_dir",
				DisplayName: "Policy pack directory",
				Status:      "pass",
				Message:     "directory exists",
			},
			{
				Name:        "pulumi_policy_yaml",
				DisplayName: "PulumiPolicy.yaml",
				Status:      "pass",
				Message:     "configuration valid",
			},
		},
		AllPass: true,
	}

	runAnalyzerChecks = func(_ context.Context) (*analyzer.CheckReport, error) {
		return expected, nil
	}

	cmd := NewAnalyzerCheckCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--output", "json"})

	err := cmd.Execute()
	require.NoError(t, err)

	var actual analyzer.CheckReport
	require.NoError(t, json.Unmarshal(buf.Bytes(), &actual))
	assert.Equal(t, expected.AllPass, actual.AllPass)
	assert.Equal(t, expected.Checks, actual.Checks)
}
