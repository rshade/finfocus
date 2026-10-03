package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestPricingSpecFallbackEnabled(t *testing.T) {
	t.Parallel()

	cfgOn := &config.Config{Cost: config.CostConfig{PricingSpecFallback: true}}
	assert.True(t, pricingSpecFallbackEnabled(false, false, cfgOn))
	assert.False(t, pricingSpecFallbackEnabled(true, false, cfgOn))
	assert.True(t, pricingSpecFallbackEnabled(true, true, &config.Config{}))
	assert.False(t, pricingSpecFallbackEnabled(false, true, nil))
	assert.False(t, pricingSpecFallbackEnabled(false, false, &config.Config{}))
}

func TestCostProjectedPricingSpecFlagDefault(t *testing.T) {
	t.Parallel()

	cmd := NewCostProjectedCmd()
	flag := cmd.Flags().Lookup("pricing-spec-fallback")
	require.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)
	assert.Equal(t, "bool", flag.Value.Type())
}

func TestCostProjectedExplainFlagDefault(t *testing.T) {
	t.Parallel()

	cmd := NewCostProjectedCmd()
	flag := cmd.Flags().Lookup("explain")
	require.NotNil(t, flag)
	assert.Equal(t, "false", flag.DefValue)
	assert.Equal(t, "bool", flag.Value.Type())
}
