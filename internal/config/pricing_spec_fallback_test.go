package config_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/rshade/finfocus/internal/config"
)

func TestCostConfig_PricingSpecFallbackDecode(t *testing.T) {
	t.Parallel()

	var fromJSON config.CostConfig
	require.NoError(t, json.Unmarshal([]byte(`{"pricing_spec_fallback":true}`), &fromJSON))
	assert.True(t, fromJSON.PricingSpecFallback)

	var fromYAML config.CostConfig
	require.NoError(t, yaml.Unmarshal([]byte("pricing_spec_fallback: true\n"), &fromYAML))
	assert.True(t, fromYAML.PricingSpecFallback)

	var defaults config.CostConfig
	require.NoError(t, json.Unmarshal([]byte(`{}`), &defaults))
	assert.False(t, defaults.PricingSpecFallback)
}
