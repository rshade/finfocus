package router_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/router"
)

func routingFor(names ...string) *config.RoutingConfig {
	cfg := &config.RoutingConfig{}
	for _, name := range names {
		cfg.Plugins = append(cfg.Plugins, config.PluginRouting{Name: name})
	}
	return cfg
}

func TestValidateRoutingConfig_PluginNames(t *testing.T) {
	t.Parallel()

	awsPublic := &pluginhost.Client{Name: "finfocus-plugin-aws-public", InstallName: "aws-public"}
	acme := &pluginhost.Client{Name: "acme", InstallName: "acme"}
	impostor := &pluginhost.Client{Name: "aws-public", InstallName: "custom"}

	tests := []struct {
		name         string
		clients      []*pluginhost.Client
		entry        string
		wantValid    bool
		errContains  string
		warnContains string
	}{
		{name: "install name", clients: []*pluginhost.Client{awsPublic}, entry: "aws-public", wantValid: true},
		{
			name: "reported name", clients: []*pluginhost.Client{awsPublic},
			entry: "finfocus-plugin-aws-public", wantValid: true,
		},
		{
			name: "unknown name", clients: []*pluginhost.Client{awsPublic},
			entry: "aws-publik", errContains: "plugin not found",
		},
		{
			name: "binary prefix suggests the install name", clients: []*pluginhost.Client{acme},
			entry: "finfocus-plugin-acme", errContains: `plugin not found (did you mean "acme"?)`,
		},
		{
			name:    "install name of one plugin and reported name of another",
			clients: []*pluginhost.Client{impostor, awsPublic}, entry: "aws-public", wantValid: true,
			warnContains: `also matches the plugin installed as "custom"; using the plugin installed as "aws-public"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := router.ValidateRoutingConfig(routingFor(tc.entry), tc.clients)

			assert.Equal(t, tc.wantValid, result.Valid)
			if tc.errContains == "" {
				assert.False(t, result.HasErrors(), "errors: %v", result.ErrorMessages())
			} else {
				require.Len(t, result.Errors, 1)
				assert.Contains(t, result.Errors[0].Error(), tc.errContains)
			}
			if tc.warnContains == "" {
				assert.False(t, result.HasWarnings(), "warnings: %v", result.WarningMessages())
			} else {
				assert.True(t, containsAnyWarningMessage(result.Warnings, tc.warnContains),
					"warnings: %v", result.WarningMessages())
			}
		})
	}
}

func TestValidateRoutingConfig_CatalogAliases(t *testing.T) {
	t.Parallel()

	preRename := &pluginhost.Client{
		Name: "finfocus-plugin-aws-public", InstallName: "aws-public",
		Aliases: []string{"finfocus-plugin-aws-public"},
	}
	postRename := &pluginhost.Client{
		Name: "aws-public", InstallName: "aws-public",
		Aliases: []string{"finfocus-plugin-aws-public"},
	}
	installedAsShared := &pluginhost.Client{Name: "shared", InstallName: "shared"}
	aliasedShared := &pluginhost.Client{Name: "y", InstallName: "y", Aliases: []string{"shared"}}

	tests := []struct {
		name     string
		clients  []*pluginhost.Client
		entry    string
		wantWarn string
	}{
		{
			name:     "alias that is still the reported name",
			clients:  []*pluginhost.Client{preRename},
			entry:    "finfocus-plugin-aws-public",
			wantWarn: `"finfocus-plugin-aws-public" will be retired; use "aws-public"`,
		},
		{
			name: "alias after the plugin is renamed", clients: []*pluginhost.Client{postRename},
			entry:    "finfocus-plugin-aws-public",
			wantWarn: `"finfocus-plugin-aws-public" is a former name of "aws-public"; use "aws-public"`,
		},
		{
			name:    "an alias never wins over another plugin's install name",
			clients: []*pluginhost.Client{aliasedShared, installedAsShared}, entry: "shared",
			wantWarn: `also matches the plugin installed as "y"; using the plugin installed as "shared"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := router.ValidateRoutingConfig(routingFor(tc.entry), tc.clients)

			assert.True(t, result.Valid, "errors: %v", result.ErrorMessages())
			assert.True(t, containsAnyWarningMessage(result.Warnings, tc.wantWarn),
				"warnings: %v", result.WarningMessages())
		})
	}
}

func TestValidateRoutingConfig_TwoEntriesForOnePlugin(t *testing.T) {
	t.Parallel()

	awsPublic := &pluginhost.Client{
		Name: "finfocus-plugin-aws-public", InstallName: "aws-public",
		Aliases: []string{"finfocus-plugin-aws-public"},
	}
	result := router.ValidateRoutingConfig(routingFor("aws-public", "finfocus-plugin-aws-public"),
		[]*pluginhost.Client{awsPublic})

	assert.True(t, result.Valid)
	assert.True(t, containsAnyWarningMessage(
		result.Warnings,
		`"aws-public" and "finfocus-plugin-aws-public" refer to the same plugin; only "finfocus-plugin-aws-public" applies`,
	),
		"warnings: %v", result.WarningMessages())
}

func TestValidateRoutingConfig_LaterEntryOverridesOneOfTwoPluginsSharingAReportedName(t *testing.T) {
	t.Parallel()

	first := &pluginhost.Client{Name: "shared", InstallName: "shared-a"}
	second := &pluginhost.Client{Name: "shared", InstallName: "shared-b"}
	result := router.ValidateRoutingConfig(routingFor("shared", "shared-b"),
		[]*pluginhost.Client{first, second})

	assert.True(t, result.Valid)
	require.Len(t, result.Warnings, 1, "only the plugin installed as shared-b loses the earlier rule")
	assert.Equal(t, `"shared" and "shared-b" refer to the same plugin; only "shared-b" applies`,
		result.Warnings[0].Message)
}
