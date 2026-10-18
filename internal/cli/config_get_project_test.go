package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const globalTierConfig = `{"plugins":{"global-only":{"region":"eu-west-1"},
  "both":{"log_level":"info","region":"us-east-1"}},
  "cost":{"budgets":{"global":{"amount":50,"currency":"USD"}}}}`

//nolint:paralleltest // sets env and the global config singleton
func TestConfigGetResolvesProjectTierPlugins(t *testing.T) {
	notifyHome(t, globalTierConfig)
	project := writeProject(t, "config.hujson", `{"plugins":{"both":{"log_level":"debug"},
  "aws-public":{"log_level":"trace"}}}`)

	tests := []struct {
		name string
		key  string
		want string
	}{
		{"set only in project tier", "plugins.aws-public.log_level", "trace\n"},
		{"project tier wins at the top-level key", "plugins.both.log_level", "debug\n"},
		{"global budget is untouched", "cost.budgets.global.amount", "50\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := runCLI(t, "config", "get", tt.key, "--project-dir", project)
			require.Equal(t, 0, result.ExitCode, string(result.Stderr))
			assert.Equal(t, tt.want, string(result.Stdout))
		})
	}

	for key, msg := range map[string]string{
		"plugins.nonexistent.key":    "plugin not found",
		"plugins.global-only.region": "plugin not found",
		"plugins.both.region":        "config key not found",
	} {
		t.Run("error for "+key, func(t *testing.T) {
			result := runCLI(t, "config", "get", key, "--project-dir", project)
			assert.NotEqual(t, 0, result.ExitCode)
			assert.Contains(t, string(result.Stderr), msg)
		})
	}
}

//nolint:paralleltest // sets env and the global config singleton
func TestConfigGetFallsBackToGlobalPlugins(t *testing.T) {
	notifyHome(t, globalTierConfig)
	project := writeProject(t, "config.hujson", `{"output":{"precision":4}}`)

	result := runCLI(t, "config", "get", "plugins.global-only.region", "--project-dir", project)
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Equal(t, "eu-west-1\n", string(result.Stdout))

	result = runCLI(t, "config", "get", "plugins.nonexistent.key", "--project-dir", project)
	assert.NotEqual(t, 0, result.ExitCode)
	assert.Contains(t, string(result.Stderr), "plugin not found")
}
