package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// TestSetupAnalyzerInfra_DisabledPlugin verifies that plugins with Enabled: false in
// cfg.Analyzer.Plugins are excluded from the active client list. Issue #751.
func TestSetupAnalyzerInfra_DisabledPlugin(t *testing.T) {
	t.Run("disabled plugin is excluded", func(t *testing.T) {
		clients := []*pluginhost.Client{
			{Name: "enabled-plugin"},
			{Name: "disabled-plugin"},
		}
		analyzerPlugins := map[string]config.AnalyzerPlugin{
			"disabled-plugin": {Enabled: false},
		}

		result := filterDisabledClients(clients, analyzerPlugins)

		require.Len(t, result, 1, "only the enabled plugin should remain")
		assert.Equal(t, "enabled-plugin", result[0].Name)
	})

	t.Run("plugin absent from map defaults to enabled", func(t *testing.T) {
		clients := []*pluginhost.Client{
			{Name: "unknown-plugin"},
		}
		analyzerPlugins := map[string]config.AnalyzerPlugin{
			"other-plugin": {Enabled: false},
		}

		result := filterDisabledClients(clients, analyzerPlugins)

		require.Len(t, result, 1, "plugin not in map must default to enabled")
		assert.Equal(t, "unknown-plugin", result[0].Name)
	})

	t.Run("empty plugin config passes all clients through", func(t *testing.T) {
		clients := []*pluginhost.Client{
			{Name: "plugin-a"},
			{Name: "plugin-b"},
		}

		result := filterDisabledClients(clients, nil)

		assert.Len(t, result, 2, "nil plugin map must pass all clients through")
	})

	t.Run("explicitly enabled plugin is kept", func(t *testing.T) {
		clients := []*pluginhost.Client{
			{Name: "my-plugin"},
		}
		analyzerPlugins := map[string]config.AnalyzerPlugin{
			"my-plugin": {Enabled: true},
		}

		result := filterDisabledClients(clients, analyzerPlugins)

		require.Len(t, result, 1, "explicitly enabled plugin must be kept")
		assert.Equal(t, "my-plugin", result[0].Name)
	})
}
