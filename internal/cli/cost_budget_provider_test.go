package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

func TestEvaluateScopedBudgets_ProviderKeys(t *testing.T) {
	t.Parallel()

	costs := []engine.CostResult{
		{ResourceType: "aws:ec2/instance:Instance", Monthly: 100},
		{ResourceType: "aws-native:ec2:Instance", Monthly: 50},
		{ResourceType: "azurerm_linux_virtual_machine", Monthly: 30},
	}

	tests := []struct {
		name      string
		providers map[string]*config.ScopedBudget
		wantSpend map[string]float64
	}{
		{
			name:      "cloud key counts package-named resources",
			providers: map[string]*config.ScopedBudget{"aws": {Amount: 1000, Currency: "USD"}},
			wantSpend: map[string]float64{"aws": 150},
		},
		{
			name:      "package-named key reports the cloud's spend",
			providers: map[string]*config.ScopedBudget{"aws-native": {Amount: 1000, Currency: "USD"}},
			wantSpend: map[string]float64{"aws-native": 150},
		},
		{
			name:      "mixed-case key reports the cloud's spend",
			providers: map[string]*config.ScopedBudget{"AWS": {Amount: 1000, Currency: "USD"}},
			wantSpend: map[string]float64{"AWS": 150},
		},
		{
			name: "a duplicate key that loses is not reported",
			providers: map[string]*config.ScopedBudget{
				"aws":        {Amount: 1000, Currency: "USD"},
				"aws-native": {Amount: 10, Currency: "USD"},
			},
			wantSpend: map[string]float64{"aws": 150},
		},
		{
			name:      "azurerm terraform type counts toward azure",
			providers: map[string]*config.ScopedBudget{"azure": {Amount: 1000, Currency: "USD"}},
			wantSpend: map[string]float64{"azure": 30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.BudgetsConfig{
				Global:    &config.ScopedBudget{Amount: 10000, Currency: "USD"},
				Providers: tt.providers,
			}
			eval := engine.NewScopedBudgetEvaluator(cfg)

			result := evaluateScopedBudgets(context.Background(), eval, cfg, costs, BudgetFlagOverrides{})

			require.Len(t, result.ByProvider, len(tt.wantSpend))
			for key, want := range tt.wantSpend {
				require.Contains(t, result.ByProvider, key)
				assert.InDelta(t, want, result.ByProvider[key].CurrentSpend, 1e-9)
			}
		})
	}
}
