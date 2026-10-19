package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/registry"
)

// TestFormatProviders tests the provider list formatting function.
// This validates T071 - SupportedProviders display formatting.
func TestFormatProviders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		providers []string
		expected  string
	}{
		{
			name:      "empty providers is global",
			providers: []string{},
			expected:  "*",
		},
		{
			name:      "nil providers is global",
			providers: nil,
			expected:  "*",
		},
		{
			name:      "wildcard is global",
			providers: []string{"*"},
			expected:  "*",
		},
		{
			name:      "single provider",
			providers: []string{"aws"},
			expected:  "aws",
		},
		{
			name:      "multiple providers",
			providers: []string{"aws", "gcp"},
			expected:  "aws, gcp",
		},
		{
			name:      "three providers",
			providers: []string{"aws", "gcp", "azure"},
			expected:  "aws, gcp, azure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := formatProviders(tt.providers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFormatCapabilities tests the capabilities list formatting function.
// This validates T072 - capabilities display formatting.
func TestFormatCapabilities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		capabilities []string
		expected     string
	}{
		{
			name:         "empty capabilities shows dash",
			capabilities: []string{},
			expected:     "-",
		},
		{
			name:         "nil capabilities shows dash",
			capabilities: nil,
			expected:     "-",
		},
		{
			name:         "single capability",
			capabilities: []string{"ProjectedCosts"},
			expected:     "ProjectedCosts",
		},
		{
			name:         "default capabilities",
			capabilities: []string{"ProjectedCosts", "ActualCosts"},
			expected:     "ProjectedCosts, ActualCosts",
		},
		{
			name:         "all capabilities",
			capabilities: []string{"ProjectedCosts", "ActualCosts", "Recommendations", "Carbon"},
			expected:     "ProjectedCosts, ActualCosts, Recommendations, Carbon",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := formatCapabilities(tt.capabilities)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestPluginListRowsUseInstallVersion keeps the version column on the
// install directory name when only the active binary was started.
func TestPluginListRowsUseInstallVersion(t *testing.T) {
	t.Parallel()

	rows := []enrichedPluginInfo{
		{
			PluginInfo: registry.PluginInfo{
				Name:    "aws-public",
				Version: "v0.2.0",
				Path:    "/plugins/aws-public/v0.2.0/finfocus-plugin-aws-public",
			},
			SpecVersion:        "v0.7.6",
			RuntimeVersion:     "0.2.0",
			SupportedProviders: []string{"aws"},
			Capabilities:       []string{"projected_costs"},
			Active:             true,
			probed:             true,
		},
		{
			PluginInfo: registry.PluginInfo{
				Name:    "aws-public",
				Version: "v0.1.9",
				Path:    "/plugins/aws-public/v0.1.9/finfocus-plugin-aws-public",
			},
			SpecVersion:    notAvailable,
			RuntimeVersion: notAvailable,
		},
	}

	var jsonBuf bytes.Buffer
	require.NoError(t, renderPluginsJSON(&jsonBuf, rows))
	var entries []PluginJSONEntry
	require.NoError(t, json.Unmarshal(jsonBuf.Bytes(), &entries))
	require.Len(t, entries, 2)
	assert.Equal(t, "v0.2.0", entries[0].Version)
	assert.Equal(t, "0.2.0", entries[0].RuntimeVersion)
	assert.Equal(t, []string{"aws"}, entries[0].SupportedProviders)
	assert.Equal(t, "v0.1.9", entries[1].Version)
	assert.Equal(t, notAvailable, entries[1].RuntimeVersion)
	assert.Nil(t, entries[1].SupportedProviders)
	assert.Nil(t, entries[1].Capabilities)
	assert.False(t, entries[1].Active)

	var tableBuf bytes.Buffer
	writer := tabwriter.NewWriter(&tableBuf, 0, 0, 2, ' ', 0)
	require.NoError(t, writePluginTable(writer, rows, false, true))
	var versions []string
	for _, line := range strings.Split(tableBuf.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "aws-public" {
			versions = append(versions, fields[1])
		}
	}
	assert.Equal(t, []string{"v0.2.0", "v0.1.9"}, versions)
}
