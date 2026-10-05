package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/rshade/finfocus/internal/config"
)

func TestAlertConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		alert     config.AlertConfig
		wantErr   bool
		errString string
	}{
		{
			name:    "valid actual alert at 80%",
			alert:   config.AlertConfig{Threshold: 80.0, Type: config.AlertTypeActual},
			wantErr: false,
		},
		{
			name:    "valid forecasted alert at 100%",
			alert:   config.AlertConfig{Threshold: 100.0, Type: config.AlertTypeForecasted},
			wantErr: false,
		},
		{
			name:    "valid alert at 0%",
			alert:   config.AlertConfig{Threshold: 0.0, Type: config.AlertTypeActual},
			wantErr: false,
		},
		{
			name:    "valid alert at max threshold",
			alert:   config.AlertConfig{Threshold: config.MaxThresholdPercent, Type: config.AlertTypeActual},
			wantErr: false,
		},
		{
			name:      "negative threshold",
			alert:     config.AlertConfig{Threshold: -10.0, Type: config.AlertTypeActual},
			wantErr:   true,
			errString: "threshold must be between 0 and 1000",
		},
		{
			name:      "threshold exceeds max",
			alert:     config.AlertConfig{Threshold: 1001.0, Type: config.AlertTypeActual},
			wantErr:   true,
			errString: "threshold must be between 0 and 1000",
		},
		{
			name:      "invalid alert type",
			alert:     config.AlertConfig{Threshold: 80.0, Type: "invalid"},
			wantErr:   true,
			errString: "alert type must be 'actual' or 'forecasted'",
		},
		{
			name:      "empty alert type",
			alert:     config.AlertConfig{Threshold: 80.0, Type: ""},
			wantErr:   true,
			errString: "alert type must be 'actual' or 'forecasted'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.alert.Validate()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errString)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBudgetConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		budget    config.BudgetConfig
		wantErr   bool
		errString string
	}{
		{
			name: "valid budget with alerts",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Period:   "monthly",
				Alerts: []config.AlertConfig{
					{Threshold: 80.0, Type: config.AlertTypeActual},
					{Threshold: 100.0, Type: config.AlertTypeForecasted},
				},
			},
			wantErr: false,
		},
		{
			name: "valid budget without alerts",
			budget: config.BudgetConfig{
				Amount:   500.0,
				Currency: "EUR",
			},
			wantErr: false,
		},
		{
			name: "disabled budget (amount zero) is valid",
			budget: config.BudgetConfig{
				Amount: 0.0,
			},
			wantErr: false,
		},
		{
			name: "disabled budget ignores missing currency",
			budget: config.BudgetConfig{
				Amount:   0.0,
				Currency: "",
			},
			wantErr: false,
		},
		{
			name:      "negative amount",
			budget:    config.BudgetConfig{Amount: -100.0, Currency: "USD"},
			wantErr:   true,
			errString: "budget amount cannot be negative",
		},
		{
			name: "missing currency when enabled",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "",
			},
			wantErr:   true,
			errString: "currency is required when budget amount is greater than 0",
		},
		{
			name: "invalid alert propagates error",
			budget: config.BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []config.AlertConfig{
					{Threshold: -10.0, Type: config.AlertTypeActual},
				},
			},
			wantErr:   true,
			errString: "alert[0]:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.budget.Validate()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errString)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBudgetConfig_IsEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		amount   float64
		expected bool
	}{
		{"zero amount is disabled", 0.0, false},
		{"positive amount is enabled", 100.0, true},
		{"small positive amount is enabled", 0.01, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := config.BudgetConfig{Amount: tc.amount}
			assert.Equal(t, tc.expected, budget.IsEnabled())
			assert.Equal(t, !tc.expected, budget.IsDisabled())
		})
	}
}

func TestBudgetConfig_GetPeriod(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		period   string
		expected string
	}{
		{"empty defaults to monthly", "", "monthly"},
		{"monthly returns monthly", "monthly", "monthly"},
		{"weekly returns weekly", "weekly", "weekly"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := config.BudgetConfig{Period: tc.period}
			assert.Equal(t, tc.expected, budget.GetPeriod())
		})
	}
}

//nolint:paralleltest // subtests share the parent-scoped fixture budget (composite value mutated by a subtest)
func TestBudgetConfig_GetAlertsByType(t *testing.T) {
	budget := config.BudgetConfig{
		Amount:   1000.0,
		Currency: "USD",
		Alerts: []config.AlertConfig{
			{Threshold: 50.0, Type: config.AlertTypeActual},
			{Threshold: 80.0, Type: config.AlertTypeActual},
			{Threshold: 100.0, Type: config.AlertTypeForecasted},
			{Threshold: 120.0, Type: config.AlertTypeForecasted},
		},
	}

	t.Run("GetActualAlerts", func(t *testing.T) {
		actual := budget.GetActualAlerts()
		assert.Len(t, actual, 2)
		assert.InDelta(t, 50.0, actual[0].Threshold, 1e-9)
		assert.InDelta(t, 80.0, actual[1].Threshold, 1e-9)
	})

	t.Run("GetForecastedAlerts", func(t *testing.T) {
		forecasted := budget.GetForecastedAlerts()
		assert.Len(t, forecasted, 2)
		assert.InDelta(t, 100.0, forecasted[0].Threshold, 1e-9)
		assert.InDelta(t, 120.0, forecasted[1].Threshold, 1e-9)
	})

	t.Run("empty alerts", func(t *testing.T) {
		emptyBudget := config.BudgetConfig{}
		assert.Nil(t, emptyBudget.GetActualAlerts())
		assert.Nil(t, emptyBudget.GetForecastedAlerts())
	})
}

func TestCostConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cost    config.CostConfig
		wantErr bool
	}{
		{
			name: "valid cost config",
			cost: config.CostConfig{
				Budgets: &config.BudgetsConfig{
					Global: &config.ScopedBudget{
						Amount:   1000.0,
						Currency: "USD",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty cost config is valid",
			cost:    config.CostConfig{},
			wantErr: false,
		},
		{
			name: "invalid budget propagates error",
			cost: config.CostConfig{
				Budgets: &config.BudgetsConfig{
					Global: &config.ScopedBudget{
						Amount: -100.0,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cost.Validate()
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCostConfig_HasBudget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cost     config.CostConfig
		expected bool
	}{
		{
			name:     "empty config has no budget",
			cost:     config.CostConfig{},
			expected: false,
		},
		{
			name: "zero amount has no budget",
			cost: config.CostConfig{
				Budgets: &config.BudgetsConfig{
					Global: &config.ScopedBudget{Amount: 0.0},
				},
			},
			expected: false,
		},
		{
			name: "positive amount has budget",
			cost: config.CostConfig{
				Budgets: &config.BudgetsConfig{
					Global: &config.ScopedBudget{Amount: 100.0, Currency: "USD"},
				},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.cost.HasBudget())
		})
	}
}

func TestBudgetConfig_YAMLParsing(t *testing.T) {
	t.Parallel()

	yamlData := `
cost:
  budgets:
    global:
      amount: 1000
      currency: USD
      period: monthly
      alerts:
        - threshold: 50
          type: actual
        - threshold: 80
          type: actual
        - threshold: 100
          type: forecasted
`

	var cfg struct {
		Cost config.CostConfig `yaml:"cost"`
	}

	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)

	require.NotNil(t, cfg.Cost.Budgets)
	require.NotNil(t, cfg.Cost.Budgets.Global)
	assert.InDelta(t, 1000.0, cfg.Cost.Budgets.Global.Amount, 1e-9)
	assert.Equal(t, "USD", cfg.Cost.Budgets.Global.Currency)
	assert.Equal(t, "monthly", cfg.Cost.Budgets.Global.Period)
	assert.Len(t, cfg.Cost.Budgets.Global.Alerts, 3)

	// Validate alert parsing
	assert.InDelta(t, 50.0, cfg.Cost.Budgets.Global.Alerts[0].Threshold, 1e-9)
	assert.Equal(t, config.AlertTypeActual, cfg.Cost.Budgets.Global.Alerts[0].Type)
	assert.InDelta(t, 80.0, cfg.Cost.Budgets.Global.Alerts[1].Threshold, 1e-9)
	assert.Equal(t, config.AlertTypeActual, cfg.Cost.Budgets.Global.Alerts[1].Type)
	assert.InDelta(t, 100.0, cfg.Cost.Budgets.Global.Alerts[2].Threshold, 1e-9)
	assert.Equal(t, config.AlertTypeForecasted, cfg.Cost.Budgets.Global.Alerts[2].Type)

	// Validate the parsed config
	require.NoError(t, cfg.Cost.Validate())
}

func TestBudgetConfig_YAMLRoundTrip(t *testing.T) {
	t.Parallel()

	original := config.CostConfig{
		Budgets: &config.BudgetsConfig{
			Global: &config.ScopedBudget{
				Amount:   1500.50,
				Currency: "EUR",
				Period:   "monthly",
				Alerts: []config.AlertConfig{
					{Threshold: 75.0, Type: config.AlertTypeActual},
					{Threshold: 100.0, Type: config.AlertTypeForecasted},
				},
			},
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back
	var parsed config.CostConfig
	err = yaml.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// Verify equality
	require.NotNil(t, parsed.Budgets)
	require.NotNil(t, parsed.Budgets.Global)
	assert.InDelta(t, original.Budgets.Global.Amount, parsed.Budgets.Global.Amount, 1e-9)
	assert.Equal(t, original.Budgets.Global.Currency, parsed.Budgets.Global.Currency)
	assert.Equal(t, original.Budgets.Global.Period, parsed.Budgets.Global.Period)
	assert.Len(t, parsed.Budgets.Global.Alerts, 2)
	assert.InDelta(t, original.Budgets.Global.Alerts[0].Threshold, parsed.Budgets.Global.Alerts[0].Threshold, 1e-9)
	assert.Equal(t, original.Budgets.Global.Alerts[0].Type, parsed.Budgets.Global.Alerts[0].Type)
}

func TestConfig_CostIntegration(t *testing.T) {
	t.Parallel()

	// Test that cost config integrates properly with main config
	t.Run("set and get cost values", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{}

		// Set cost values
		err := cfg.Set("cost.budgets.amount", "1000")
		require.NoError(t, err)
		err = cfg.Set("cost.budgets.currency", "USD")
		require.NoError(t, err)
		err = cfg.Set("cost.budgets.period", "monthly")
		require.NoError(t, err)

		// Get cost values
		amount, err := cfg.Get("cost.budgets.amount")
		require.NoError(t, err)
		assert.InDelta(t, 1000.0, amount, 1e-9)

		currency, err := cfg.Get("cost.budgets.currency")
		require.NoError(t, err)
		assert.Equal(t, "USD", currency)

		period, err := cfg.Get("cost.budgets.period")
		require.NoError(t, err)
		assert.Equal(t, "monthly", period)
	})

	t.Run("get entire cost config", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Cost: config.CostConfig{
				Budgets: &config.BudgetsConfig{
					Global: &config.ScopedBudget{
						Amount:   500.0,
						Currency: "EUR",
					},
				},
			},
		}

		cost, err := cfg.Get("cost")
		require.NoError(t, err)
		costConfig, ok := cost.(config.CostConfig)
		require.True(t, ok)
		require.NotNil(t, costConfig.Budgets)
		require.NotNil(t, costConfig.Budgets.Global)
		assert.InDelta(t, 500.0, costConfig.Budgets.Global.Amount, 1e-9)
	})

	t.Run("get entire budgets config", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{
			Cost: config.CostConfig{
				Budgets: &config.BudgetsConfig{
					Global: &config.ScopedBudget{
						Amount:   750.0,
						Currency: "GBP",
					},
				},
			},
		}

		budgets, err := cfg.Get("cost.budgets")
		require.NoError(t, err)
		budgetsConfig, ok := budgets.(*config.BudgetsConfig)
		require.True(t, ok)
		require.NotNil(t, budgetsConfig.Global)
		assert.InDelta(t, 750.0, budgetsConfig.Global.Amount, 1e-9)
	})

	t.Run("invalid set value", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{}
		err := cfg.Set("cost.budgets.amount", "not-a-number")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a number")
	})

	t.Run("unknown cost setting", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{}
		err := cfg.Set("cost.unknown", "value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown cost setting")
	})

	t.Run("unknown budgets setting", func(t *testing.T) {
		t.Parallel()
		cfg := &config.Config{}
		err := cfg.Set("cost.budgets.unknown", "value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown cost.budgets setting")
	})
}

func TestConfig_List_IncludesCost(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Cost: config.CostConfig{
			Budgets: &config.BudgetsConfig{
				Global: &config.ScopedBudget{
					Amount:   1000.0,
					Currency: "USD",
				},
			},
		},
	}

	list := cfg.List()
	cost, exists := list["cost"]
	require.True(t, exists)

	costConfig, ok := cost.(config.CostConfig)
	require.True(t, ok)
	require.NotNil(t, costConfig.Budgets)
	require.NotNil(t, costConfig.Budgets.Global)
	assert.InDelta(t, 1000.0, costConfig.Budgets.Global.Amount, 1e-9)
}

// T004: Unit test for ErrExitCodeOutOfRange error type.
func TestErrExitCodeOutOfRange(t *testing.T) {
	t.Parallel()

	// Verify the error variable exists and has the expected message
	require.Error(t, config.ErrExitCodeOutOfRange)
	assert.Contains(t, config.ErrExitCodeOutOfRange.Error(), "exit code must be between 0 and 255")
}

// T005: Unit test for BudgetConfig.GetExitCode() method.
func TestBudgetConfig_GetExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		budget   config.BudgetConfig
		expected int
	}{
		{
			name:     "default exit code when not set",
			budget:   config.BudgetConfig{Amount: 100.0, Currency: "USD"},
			expected: 1,
		},
		{
			name:     "explicit exit code 0",
			budget:   config.BudgetConfig{Amount: 100.0, Currency: "USD", ExitCode: 0, ExitOnThreshold: true},
			expected: 0,
		},
		{
			name:     "explicit exit code 2",
			budget:   config.BudgetConfig{Amount: 100.0, Currency: "USD", ExitCode: 2},
			expected: 2,
		},
		{
			name:     "max exit code 255",
			budget:   config.BudgetConfig{Amount: 100.0, Currency: "USD", ExitCode: 255},
			expected: 255,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.budget.GetExitCode())
		})
	}
}

// T006: Unit test for BudgetConfig.ShouldExitOnThreshold() method.
func TestBudgetConfig_ShouldExitOnThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		budget   config.BudgetConfig
		expected bool
	}{
		{
			name:     "default is disabled",
			budget:   config.BudgetConfig{Amount: 100.0, Currency: "USD"},
			expected: false,
		},
		{
			name:     "explicitly enabled",
			budget:   config.BudgetConfig{Amount: 100.0, Currency: "USD", ExitOnThreshold: true},
			expected: true,
		},
		{
			name:     "explicitly disabled",
			budget:   config.BudgetConfig{Amount: 100.0, Currency: "USD", ExitOnThreshold: false},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.budget.ShouldExitOnThreshold())
		})
	}
}

// T007: Unit test for exit code validation (0-255 range).
func TestBudgetConfig_Validate_ExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		budget    config.BudgetConfig
		wantErr   bool
		errString string
	}{
		{
			name: "valid exit code 0",
			budget: config.BudgetConfig{
				Amount:          100.0,
				Currency:        "USD",
				ExitOnThreshold: true,
				ExitCode:        0,
			},
			wantErr: false,
		},
		{
			name: "valid exit code 1 (default)",
			budget: config.BudgetConfig{
				Amount:          100.0,
				Currency:        "USD",
				ExitOnThreshold: true,
				ExitCode:        1,
			},
			wantErr: false,
		},
		{
			name: "valid exit code 255 (max)",
			budget: config.BudgetConfig{
				Amount:          100.0,
				Currency:        "USD",
				ExitOnThreshold: true,
				ExitCode:        255,
			},
			wantErr: false,
		},
		{
			name: "invalid exit code 256 (exceeds max)",
			budget: config.BudgetConfig{
				Amount:          100.0,
				Currency:        "USD",
				ExitOnThreshold: true,
				ExitCode:        256,
			},
			wantErr:   true,
			errString: "exit code must be between 0 and 255",
		},
		{
			name: "invalid exit code -1 (negative)",
			budget: config.BudgetConfig{
				Amount:          100.0,
				Currency:        "USD",
				ExitOnThreshold: true,
				ExitCode:        -1,
			},
			wantErr:   true,
			errString: "exit code must be between 0 and 255",
		},
		{
			name: "exit code validation skipped when exit disabled",
			budget: config.BudgetConfig{
				Amount:          100.0,
				Currency:        "USD",
				ExitOnThreshold: false,
				ExitCode:        999, // Invalid but not validated when disabled
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.budget.Validate()
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errString)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// T006: Tests for HistoryConfig and AllocationConfig YAML deserialization.

func TestHistoryConfig_YAMLDefaults(t *testing.T) {
	t.Parallel()

	yamlData := `
cost:
  history: {}
`
	var cfg struct {
		Cost config.CostConfig `yaml:"cost"`
	}

	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)

	assert.Nil(t, cfg.Cost.History.Enabled, "nil when omitted")
	assert.Nil(t, cfg.Cost.History.RetentionDays, "nil when omitted")
	assert.Empty(t, cfg.Cost.History.Directory, "empty when omitted")
}

func TestHistoryRetention_YAMLAndValidate(t *testing.T) {
	t.Parallel()

	yamlData := `
cost:
  history:
    retention:
      max_snapshots: 1000
      max_age_days: 365
      auto_prune: true
`
	var cfg struct {
		Cost config.CostConfig `yaml:"cost"`
	}
	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)
	assert.Equal(t, 1000, cfg.Cost.History.Retention.MaxSnapshots)
	assert.Equal(t, 365, cfg.Cost.History.Retention.MaxAgeDays)
	assert.True(t, cfg.Cost.History.Retention.AutoPrune)
	require.NoError(t, cfg.Cost.History.Validate())

	negative := config.HistoryConfig{}
	negative.Retention.MaxSnapshots = -1
	require.ErrorContains(t, negative.Validate(), "max_snapshots")
	negative.Retention.MaxSnapshots = 0
	negative.Retention.MaxAgeDays = -5
	require.ErrorContains(t, negative.Validate(), "max_age_days")
}

func TestHistoryConfig_YAMLExplicitValues(t *testing.T) {
	t.Parallel()

	yamlData := `
cost:
  history:
    enabled: true
    retention_days: 180
    directory: /tmp/custom-history
`
	var cfg struct {
		Cost config.CostConfig `yaml:"cost"`
	}

	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)

	require.NotNil(t, cfg.Cost.History.Enabled)
	assert.True(t, *cfg.Cost.History.Enabled)
	require.NotNil(t, cfg.Cost.History.RetentionDays)
	assert.Equal(t, 180, *cfg.Cost.History.RetentionDays)
	assert.Equal(t, "/tmp/custom-history", cfg.Cost.History.Directory)
}

func TestHistoryConfig_NestedUnderCost(t *testing.T) {
	t.Parallel()

	yamlData := `
cost:
  history:
    enabled: true
    retention_days: 90
  cache:
    enabled: true
    ttl_seconds: 3600
`
	var cfg struct {
		Cost config.CostConfig `yaml:"cost"`
	}

	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)

	require.NotNil(t, cfg.Cost.History.Enabled)
	assert.True(t, *cfg.Cost.History.Enabled)
	require.NotNil(t, cfg.Cost.History.RetentionDays)
	assert.Equal(t, 90, *cfg.Cost.History.RetentionDays)
	assert.True(t, cfg.Cost.Cache.Enabled)
	assert.Equal(t, 3600, cfg.Cost.Cache.TTLSeconds)
}

func TestHistoryConfig_YAMLRoundTrip(t *testing.T) {
	t.Parallel()

	boolTrue := true
	retentionDays := 120
	original := config.CostConfig{
		History: config.HistoryConfig{
			Enabled:       &boolTrue,
			RetentionDays: &retentionDays,
			Directory:     "/var/data/history",
		},
	}

	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var parsed config.CostConfig
	err = yaml.Unmarshal(data, &parsed)
	require.NoError(t, err)

	require.NotNil(t, parsed.History.Enabled)
	assert.Equal(t, *original.History.Enabled, *parsed.History.Enabled)
	require.NotNil(t, parsed.History.RetentionDays)
	assert.Equal(t, *original.History.RetentionDays, *parsed.History.RetentionDays)
	assert.Equal(t, original.History.Directory, parsed.History.Directory)
}

func TestAllocationConfig_YAMLDefaults(t *testing.T) {
	t.Parallel()

	yamlData := `
cost:
  allocation: {}
`
	var cfg struct {
		Cost config.CostConfig `yaml:"cost"`
	}

	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)

	assert.False(t, cfg.Cost.Allocation.Enabled, "zero value when omitted")
	assert.Empty(t, cfg.Cost.Allocation.Tags, "empty when omitted")
}

func TestAllocationConfig_YAMLExplicitValues(t *testing.T) {
	t.Parallel()

	yamlData := `
cost:
  allocation:
    enabled: true
    tags:
      - pulumi:project
      - env
      - team
`
	var cfg struct {
		Cost config.CostConfig `yaml:"cost"`
	}

	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)

	assert.True(t, cfg.Cost.Allocation.Enabled)
	require.Len(t, cfg.Cost.Allocation.Tags, 3)
	assert.Equal(t, "pulumi:project", cfg.Cost.Allocation.Tags[0])
	assert.Equal(t, "env", cfg.Cost.Allocation.Tags[1])
	assert.Equal(t, "team", cfg.Cost.Allocation.Tags[2])
}

func TestAllocationConfig_NestedUnderCost(t *testing.T) {
	t.Parallel()

	yamlData := `
cost:
  allocation:
    enabled: true
    tags:
      - pulumi:project
  history:
    enabled: true
    retention_days: 90
`
	var cfg struct {
		Cost config.CostConfig `yaml:"cost"`
	}

	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)

	assert.True(t, cfg.Cost.Allocation.Enabled)
	require.Len(t, cfg.Cost.Allocation.Tags, 1)
	assert.Equal(t, "pulumi:project", cfg.Cost.Allocation.Tags[0])
	require.NotNil(t, cfg.Cost.History.Enabled)
	assert.True(t, *cfg.Cost.History.Enabled)
	require.NotNil(t, cfg.Cost.History.RetentionDays)
	assert.Equal(t, 90, *cfg.Cost.History.RetentionDays)
}

func TestAllocationConfig_YAMLRoundTrip(t *testing.T) {
	t.Parallel()

	original := config.CostConfig{
		Allocation: config.AllocationConfig{
			Enabled: true,
			Tags:    []string{"pulumi:project", "env"},
		},
	}

	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var parsed config.CostConfig
	err = yaml.Unmarshal(data, &parsed)
	require.NoError(t, err)

	assert.Equal(t, original.Allocation.Enabled, parsed.Allocation.Enabled)
	assert.Equal(t, original.Allocation.Tags, parsed.Allocation.Tags)
}

func TestHistoryConfig_DefaultConstants(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 90, config.HistoryDefaultRetentionDays)
	assert.True(t, config.HistoryDefaultEnabled)
	assert.False(t, config.AllocationDefaultEnabled)
}

// ---------------------------------------------------------------------------
// T038: AllocationConfig validation tests
// ---------------------------------------------------------------------------

func TestAllocationConfig_Validate_ValidWithTags(t *testing.T) {
	t.Parallel()

	cfg := config.AllocationConfig{
		Enabled: true,
		Tags:    []string{"pulumi:project", "env", "team"},
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestAllocationConfig_Validate_DisabledEmptyTags(t *testing.T) {
	t.Parallel()

	cfg := config.AllocationConfig{
		Enabled: false,
		Tags:    nil,
	}
	err := cfg.Validate()
	require.NoError(t, err, "disabled config with empty tags should be valid")
}

func TestAllocationConfig_Validate_EnabledEmptyTags(t *testing.T) {
	t.Parallel()

	cfg := config.AllocationConfig{
		Enabled: true,
		Tags:    nil,
	}
	err := cfg.Validate()
	require.NoError(t, err, "enabled with empty tags is valid (warning-only)")
}

func TestAllocationConfig_Validate_EmptyTagKey(t *testing.T) {
	t.Parallel()

	cfg := config.AllocationConfig{
		Enabled: true,
		Tags:    []string{"valid", ""},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "tag key at index 1 is empty")
}

func TestAllocationConfig_Validate_TagKeyTooLong(t *testing.T) {
	t.Parallel()

	longKey := string(make([]byte, 129))
	for i := range longKey {
		longKey = longKey[:i] + "a" + longKey[i+1:]
	}
	cfg := config.AllocationConfig{
		Enabled: true,
		Tags:    []string{longKey},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum length of 128")
}

func TestAllocationConfig_Validate_TagsPreservedThroughYAML(t *testing.T) {
	t.Parallel()

	yamlData := `
enabled: true
tags:
  - pulumi:project
  - cost-center
  - team
`
	var cfg config.AllocationConfig
	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)

	require.NoError(t, cfg.Validate())
	assert.True(t, cfg.Enabled)
	require.Len(t, cfg.Tags, 3)
	assert.Equal(t, "pulumi:project", cfg.Tags[0])
	assert.Equal(t, "cost-center", cfg.Tags[1])
	assert.Equal(t, "team", cfg.Tags[2])
}

func TestAllocationConfig_Validate_DisabledWithInvalidTags(t *testing.T) {
	t.Parallel()

	cfg := config.AllocationConfig{
		Enabled: false,
		Tags:    []string{"valid", ""},
	}
	err := cfg.Validate()
	require.NoError(t, err, "disabled config should skip tag validation")
}

// Test YAML parsing with exit code fields.
func TestBudgetConfig_YAMLParsing_ExitCode(t *testing.T) {
	t.Parallel()

	yamlData := `
cost:
  budgets:
    global:
      amount: 1000
      currency: USD
      exit_on_threshold: true
      exit_code: 2
`

	var cfg struct {
		Cost config.CostConfig `yaml:"cost"`
	}

	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)

	require.NotNil(t, cfg.Cost.Budgets)
	require.NotNil(t, cfg.Cost.Budgets.Global)
	require.NotNil(t, cfg.Cost.Budgets.Global.ExitOnThreshold)
	assert.True(t, *cfg.Cost.Budgets.Global.ExitOnThreshold)
	require.NotNil(t, cfg.Cost.Budgets.Global.ExitCode)
	assert.Equal(t, 2, *cfg.Cost.Budgets.Global.ExitCode)
	shouldExit := cfg.Cost.Budgets.Global.ShouldExitOnThreshold()
	require.NotNil(t, shouldExit)
	assert.True(t, *shouldExit)
	exitCode := cfg.Cost.Budgets.Global.GetExitCode()
	require.NotNil(t, exitCode)
	assert.Equal(t, 2, *exitCode)
}

func TestAlertConfig_ValidateNotifications(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		alert    config.AlertConfig
		wantErr  error
		contains string
	}{
		{
			name: "valid destinations",
			alert: config.AlertConfig{
				Threshold: 80,
				Type:      config.AlertTypeActual,
				Notifications: []config.NotificationDestination{
					{Type: "slack", URL: "${FINFOCUS_NOTIFY_SLACK_URL}"},
					{Type: "webhook", URL: "https://api.example.com/hook"},
				},
			},
		},
		{
			name: "second destination invalid carries its index",
			alert: config.AlertConfig{
				Threshold: 80,
				Type:      config.AlertTypeActual,
				Notifications: []config.NotificationDestination{
					{Type: "slack", URL: "https://hooks.slack.com/x"},
					{Type: "webhook", URL: "http://api.example.com"},
				},
			},
			wantErr:  config.ErrNotificationHTTPSRequired,
			contains: "notifications[1]: url",
		},
		{
			name: "non-prefixed variable names the variable",
			alert: config.AlertConfig{
				Threshold: 80,
				Type:      config.AlertTypeActual,
				Notifications: []config.NotificationDestination{
					{
						Type:    "webhook",
						URL:     "https://api.example.com",
						Headers: map[string]string{"X-Key": "${API_TOKEN}"},
					},
				},
			},
			wantErr:  config.ErrNotificationVariableNotAllowed,
			contains: "API_TOKEN",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.alert.Validate()
			if tc.wantErr == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tc.wantErr)
			assert.Contains(t, err.Error(), tc.contains)
		})
	}
}
