package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestRoutingIssues_PluginNames(t *testing.T) {
	t.Parallel()

	awsPublic := &pluginhost.Client{
		Name: "finfocus-plugin-aws-public", InstallName: "aws-public",
		Aliases: []string{"finfocus-plugin-aws-public"},
	}
	impostor := &pluginhost.Client{Name: "aws-public", InstallName: "custom"}

	tests := []struct {
		name         string
		clients      []*pluginhost.Client
		entry        string
		wantWarning  string
		wantProblems int
	}{
		{name: "install name", clients: []*pluginhost.Client{awsPublic}, entry: "aws-public"},
		{
			name: "alias is a warning, not a problem", clients: []*pluginhost.Client{awsPublic},
			entry: "finfocus-plugin-aws-public", wantWarning: "will be retired",
		},
		{
			name: "ambiguous name is a warning, not a problem", clients: []*pluginhost.Client{impostor, awsPublic},
			entry: "aws-public", wantWarning: `also matches the plugin installed as "custom"`,
		},
		{
			name:    "plugin-load fallback with configured names stays valid",
			clients: []*pluginhost.Client{{Name: "aws-publik"}}, entry: "aws-publik",
		},
		{
			name:         "unknown name is a problem",
			clients:      []*pluginhost.Client{awsPublic},
			entry:        "aws-publik",
			wantProblems: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			routing := &config.RoutingConfig{Plugins: []config.PluginRouting{{Name: tc.entry}}}

			warnings, problems := routingIssues(routing, tc.clients)

			assert.Len(t, problems, tc.wantProblems)
			if tc.wantWarning == "" {
				assert.Empty(t, warnings)
				return
			}
			require.Len(t, warnings, 1)
			assert.Contains(t, warnings[0], tc.wantWarning)
		})
	}
}
