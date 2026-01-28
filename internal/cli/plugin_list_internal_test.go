package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestFormatProviders tests the provider list formatting function.
// This validates T071 - SupportedProviders display formatting.
func TestFormatProviders(t *testing.T) {
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
			result := formatProviders(tt.providers)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFormatCapabilities tests the capabilities list formatting function.
// This validates T072 - capabilities display formatting.
func TestFormatCapabilities(t *testing.T) {
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
			result := formatCapabilities(tt.capabilities)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEnrichedPluginInfoDisplayVersion tests the version display logic.
func TestEnrichedPluginInfoDisplayVersion(t *testing.T) {
	tests := []struct {
		name           string
		runtimeVersion string
		version        string
		expected       string
	}{
		{
			name:           "runtime version available",
			runtimeVersion: "1.2.3",
			version:        "1.0.0",
			expected:       "1.2.3",
		},
		{
			name:           "runtime version N/A falls back to version",
			runtimeVersion: notAvailable,
			version:        "1.0.0",
			expected:       "1.0.0",
		},
		{
			name:           "both N/A returns N/A",
			runtimeVersion: notAvailable,
			version:        notAvailable,
			expected:       notAvailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := enrichedPluginInfo{
				RuntimeVersion: tt.runtimeVersion,
			}
			info.Version = tt.version
			result := info.displayVersion()
			assert.Equal(t, tt.expected, result)
		})
	}
}
