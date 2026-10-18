package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // sets env and the global config singleton
func TestConfigListResolvesProjectTier(t *testing.T) {
	notifyHome(t, globalTierConfig)
	project := writeProject(t, "config.hujson", `{"plugins":{"both":{"log_level":"debug"},
  "aws-public":{"log_level":"trace"}}}`)

	for _, as := range []string{"yaml", "json"} {
		t.Run(as, func(t *testing.T) {
			result := runCLI(t, "config", "list", "--as", as, "--project-dir", project)
			require.Equal(t, 0, result.ExitCode, string(result.Stderr))
			out := string(result.Stdout)
			assert.Contains(t, out, "aws-public")
			assert.Contains(t, out, "trace")
			assert.Contains(t, out, "debug")
			assert.NotContains(t, out, "global-only")
			assert.NotContains(t, out, "us-east-1")
			assert.Contains(t, out, "50")
		})
	}
}

//nolint:paralleltest // sets env and the global config singleton
func TestConfigListFallsBackToGlobalPlugins(t *testing.T) {
	notifyHome(t, globalTierConfig)
	project := writeProject(t, "config.hujson", `{"output":{"precision":4}}`)

	result := runCLI(t, "config", "list", "--project-dir", project)
	require.Equal(t, 0, result.ExitCode, string(result.Stderr))
	assert.Contains(t, string(result.Stdout), "global-only")
}
