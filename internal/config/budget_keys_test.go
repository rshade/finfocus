package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestCostBudgetsKeysRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key    string
		value  string
		want   any
		number bool
	}{
		{key: "cost.budgets.global.amount", value: "10", want: 10.0, number: true},
		{key: "cost.budgets.global.currency", value: "USD", want: "USD"},
		{key: "cost.budgets.global.period", value: "monthly", want: "monthly"},
		{key: "cost.budgets.amount", value: "12.5", want: 12.5, number: true},
		{key: "cost.budgets.currency", value: "EUR", want: "EUR"},
		{key: "cost.budgets.period", value: "monthly", want: "monthly"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			cfg := &config.Config{}
			require.NoError(t, cfg.Set(tt.key, tt.value))

			got, err := cfg.Get(tt.key)
			require.NoError(t, err)
			if tt.number {
				assert.InDelta(t, tt.want, got, 1e-9)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCostBudgetsFlatKeyReadsGlobalAmount(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	require.NoError(t, cfg.Set("cost.budgets.global.amount", "10"))
	got, err := cfg.Get("cost.budgets.amount")
	require.NoError(t, err)
	assert.InDelta(t, 10.0, got, 1e-9)
}

func TestCostBudgetsGlobalAmountUnset(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	_, err := cfg.Get("cost.budgets.global.amount")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no budget configured")
}
