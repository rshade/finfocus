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

func TestPricingSpecFallbackForCmdReadsChangedFlag(t *testing.T) {
	t.Parallel()

	cfgOn := &config.Config{Cost: config.CostConfig{PricingSpecFallback: true}}
	cfgOff := &config.Config{}
	tests := []struct {
		name string
		args []string
		cfg  *config.Config
		want bool
	}{
		{"flag unset follows config on", nil, cfgOn, true},
		{"flag unset follows config off", nil, cfgOff, false},
		{"explicit false overrides config on", []string{"--pricing-spec-fallback=false"}, cfgOn, false},
		{"explicit true overrides config off", []string{"--pricing-spec-fallback"}, cfgOff, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := NewCostProjectedCmd()
			require.NoError(t, cmd.ParseFlags(tt.args))
			value, err := cmd.Flags().GetBool("pricing-spec-fallback")
			require.NoError(t, err)
			assert.Equal(t, tt.want, pricingSpecFallbackForCmd(cmd, value, tt.cfg))
		})
	}
}
