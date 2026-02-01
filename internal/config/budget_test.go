package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAlertConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		alert     AlertConfig
		wantErr   bool
		errString string
	}{
		{
			name:    "valid actual alert at 80%",
			alert:   AlertConfig{Threshold: 80.0, Type: AlertTypeActual},
			wantErr: false,
		},
		{
			name:    "valid forecasted alert at 100%",
			alert:   AlertConfig{Threshold: 100.0, Type: AlertTypeForecasted},
			wantErr: false,
		},
		{
			name:    "valid alert at 0%",
			alert:   AlertConfig{Threshold: 0.0, Type: AlertTypeActual},
			wantErr: false,
		},
		{
			name:    "valid alert at max threshold",
			alert:   AlertConfig{Threshold: MaxThresholdPercent, Type: AlertTypeActual},
			wantErr: false,
		},
		{
			name:      "negative threshold",
			alert:     AlertConfig{Threshold: -10.0, Type: AlertTypeActual},
			wantErr:   true,
			errString: "threshold must be between 0 and 1000",
		},
		{
			name:      "threshold exceeds max",
			alert:     AlertConfig{Threshold: 1001.0, Type: AlertTypeActual},
			wantErr:   true,
			errString: "threshold must be between 0 and 1000",
		},
		{
			name:      "invalid alert type",
			alert:     AlertConfig{Threshold: 80.0, Type: "invalid"},
			wantErr:   true,
			errString: "alert type must be 'actual' or 'forecasted'",
		},
		{
			name:      "empty alert type",
			alert:     AlertConfig{Threshold: 80.0, Type: ""},
			wantErr:   true,
			errString: "alert type must be 'actual' or 'forecasted'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
	tests := []struct {
		name      string
		budget    BudgetConfig
		wantErr   bool
		errString string
	}{
		{
			name: "valid budget with alerts",
			budget: BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Period:   "monthly",
				Alerts: []AlertConfig{
					{Threshold: 80.0, Type: AlertTypeActual},
					{Threshold: 100.0, Type: AlertTypeForecasted},
				},
			},
			wantErr: false,
		},
		{
			name: "valid budget without alerts",
			budget: BudgetConfig{
				Amount:   500.0,
				Currency: "EUR",
			},
			wantErr: false,
		},
		{
			name: "disabled budget (amount zero) is valid",
			budget: BudgetConfig{
				Amount: 0.0,
			},
			wantErr: false,
		},
		{
			name: "disabled budget ignores missing currency",
			budget: BudgetConfig{
				Amount:   0.0,
				Currency: "",
			},
			wantErr: false,
		},
		{
			name:      "negative amount",
			budget:    BudgetConfig{Amount: -100.0, Currency: "USD"},
			wantErr:   true,
			errString: "budget amount cannot be negative",
		},
		{
			name: "missing currency when enabled",
			budget: BudgetConfig{
				Amount:   1000.0,
				Currency: "",
			},
			wantErr:   true,
			errString: "currency is required when budget amount is greater than 0",
		},
		{
			name: "invalid alert propagates error",
			budget: BudgetConfig{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []AlertConfig{
					{Threshold: -10.0, Type: AlertTypeActual},
				},
			},
			wantErr:   true,
			errString: "alert[0]:",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			budget := BudgetConfig{Amount: tc.amount}
			assert.Equal(t, tc.expected, budget.IsEnabled())
			assert.Equal(t, !tc.expected, budget.IsDisabled())
		})
	}
}

func TestBudgetConfig_GetPeriod(t *testing.T) {
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
			budget := BudgetConfig{Period: tc.period}
			assert.Equal(t, tc.expected, budget.GetPeriod())
		})
	}
}

func TestBudgetConfig_GetAlertsByType(t *testing.T) {
	budget := BudgetConfig{
		Amount:   1000.0,
		Currency: "USD",
		Alerts: []AlertConfig{
			{Threshold: 50.0, Type: AlertTypeActual},
			{Threshold: 80.0, Type: AlertTypeActual},
			{Threshold: 100.0, Type: AlertTypeForecasted},
			{Threshold: 120.0, Type: AlertTypeForecasted},
		},
	}

	t.Run("GetActualAlerts", func(t *testing.T) {
		actual := budget.GetActualAlerts()
		assert.Len(t, actual, 2)
		assert.Equal(t, 50.0, actual[0].Threshold)
		assert.Equal(t, 80.0, actual[1].Threshold)
	})

	t.Run("GetForecastedAlerts", func(t *testing.T) {
		forecasted := budget.GetForecastedAlerts()
		assert.Len(t, forecasted, 2)
		assert.Equal(t, 100.0, forecasted[0].Threshold)
		assert.Equal(t, 120.0, forecasted[1].Threshold)
	})

	t.Run("empty alerts", func(t *testing.T) {
		emptyBudget := BudgetConfig{}
		assert.Nil(t, emptyBudget.GetActualAlerts())
		assert.Nil(t, emptyBudget.GetForecastedAlerts())
	})
}

func TestCostConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cost    CostConfig
		wantErr bool
	}{
		{
			name: "valid cost config",
			cost: CostConfig{
				Budgets: &BudgetsConfig{
					Global: &ScopedBudget{
						Amount:   1000.0,
						Currency: "USD",
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "empty cost config is valid",
			cost:    CostConfig{},
			wantErr: false,
		},
		{
			name: "invalid budget propagates error",
			cost: CostConfig{
				Budgets: &BudgetsConfig{
					Global: &ScopedBudget{
						Amount: -100.0,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
	tests := []struct {
		name     string
		cost     CostConfig
		expected bool
	}{
		{
			name:     "empty config has no budget",
			cost:     CostConfig{},
			expected: false,
		},
		{
			name: "zero amount has no budget",
			cost: CostConfig{
				Budgets: &BudgetsConfig{
					Global: &ScopedBudget{Amount: 0.0},
				},
			},
			expected: false,
		},
		{
			name: "positive amount has budget",
			cost: CostConfig{
				Budgets: &BudgetsConfig{
					Global: &ScopedBudget{Amount: 100.0, Currency: "USD"},
				},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.cost.HasBudget())
		})
	}
}

func TestBudgetConfig_YAMLParsing(t *testing.T) {
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
		Cost CostConfig `yaml:"cost"`
	}

	err := yaml.Unmarshal([]byte(yamlData), &cfg)
	require.NoError(t, err)

	require.NotNil(t, cfg.Cost.Budgets)
	require.NotNil(t, cfg.Cost.Budgets.Global)
	assert.Equal(t, 1000.0, cfg.Cost.Budgets.Global.Amount)
	assert.Equal(t, "USD", cfg.Cost.Budgets.Global.Currency)
	assert.Equal(t, "monthly", cfg.Cost.Budgets.Global.Period)
	assert.Len(t, cfg.Cost.Budgets.Global.Alerts, 3)

	// Validate alert parsing
	assert.Equal(t, 50.0, cfg.Cost.Budgets.Global.Alerts[0].Threshold)
	assert.Equal(t, AlertTypeActual, cfg.Cost.Budgets.Global.Alerts[0].Type)
	assert.Equal(t, 80.0, cfg.Cost.Budgets.Global.Alerts[1].Threshold)
	assert.Equal(t, AlertTypeActual, cfg.Cost.Budgets.Global.Alerts[1].Type)
	assert.Equal(t, 100.0, cfg.Cost.Budgets.Global.Alerts[2].Threshold)
	assert.Equal(t, AlertTypeForecasted, cfg.Cost.Budgets.Global.Alerts[2].Type)

	// Validate the parsed config
	require.NoError(t, cfg.Cost.Validate())
}

func TestBudgetConfig_YAMLRoundTrip(t *testing.T) {
	original := CostConfig{
		Budgets: &BudgetsConfig{
			Global: &ScopedBudget{
				Amount:   1500.50,
				Currency: "EUR",
				Period:   "monthly",
				Alerts: []AlertConfig{
					{Threshold: 75.0, Type: AlertTypeActual},
					{Threshold: 100.0, Type: AlertTypeForecasted},
				},
			},
		},
	}

	// Marshal to YAML
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back
	var parsed CostConfig
	err = yaml.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// Verify equality
	require.NotNil(t, parsed.Budgets)
	require.NotNil(t, parsed.Budgets.Global)
	assert.Equal(t, original.Budgets.Global.Amount, parsed.Budgets.Global.Amount)
	assert.Equal(t, original.Budgets.Global.Currency, parsed.Budgets.Global.Currency)
	assert.Equal(t, original.Budgets.Global.Period, parsed.Budgets.Global.Period)
	assert.Len(t, parsed.Budgets.Global.Alerts, 2)
	assert.Equal(t, original.Budgets.Global.Alerts[0].Threshold, parsed.Budgets.Global.Alerts[0].Threshold)
	assert.Equal(t, original.Budgets.Global.Alerts[0].Type, parsed.Budgets.Global.Alerts[0].Type)
}

func TestConfig_CostIntegration(t *testing.T) {
	// Test that cost config integrates properly with main config
	t.Run("set and get cost values", func(t *testing.T) {
		cfg := &Config{}

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
		assert.Equal(t, 1000.0, amount)

		currency, err := cfg.Get("cost.budgets.currency")
		require.NoError(t, err)
		assert.Equal(t, "USD", currency)

		period, err := cfg.Get("cost.budgets.period")
		require.NoError(t, err)
		assert.Equal(t, "monthly", period)
	})

	t.Run("get entire cost config", func(t *testing.T) {
		cfg := &Config{
			Cost: CostConfig{
				Budgets: &BudgetsConfig{
					Global: &ScopedBudget{
						Amount:   500.0,
						Currency: "EUR",
					},
				},
			},
		}

		cost, err := cfg.Get("cost")
		require.NoError(t, err)
		costConfig, ok := cost.(CostConfig)
		require.True(t, ok)
		require.NotNil(t, costConfig.Budgets)
		require.NotNil(t, costConfig.Budgets.Global)
		assert.Equal(t, 500.0, costConfig.Budgets.Global.Amount)
	})

	t.Run("get entire budgets config", func(t *testing.T) {
		cfg := &Config{
			Cost: CostConfig{
				Budgets: &BudgetsConfig{
					Global: &ScopedBudget{
						Amount:   750.0,
						Currency: "GBP",
					},
				},
			},
		}

		budgets, err := cfg.Get("cost.budgets")
		require.NoError(t, err)
		budgetsConfig, ok := budgets.(*BudgetsConfig)
		require.True(t, ok)
		require.NotNil(t, budgetsConfig.Global)
		assert.Equal(t, 750.0, budgetsConfig.Global.Amount)
	})

	t.Run("invalid set value", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.Set("cost.budgets.amount", "not-a-number")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be a number")
	})

	t.Run("unknown cost setting", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.Set("cost.unknown", "value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown cost setting")
	})

	t.Run("unknown budgets setting", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.Set("cost.budgets.unknown", "value")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown cost.budgets setting")
	})
}

func TestConfig_List_IncludesCost(t *testing.T) {
	cfg := &Config{
		Cost: CostConfig{
			Budgets: &BudgetsConfig{
				Global: &ScopedBudget{
					Amount:   1000.0,
					Currency: "USD",
				},
			},
		},
	}

	list := cfg.List()
	cost, exists := list["cost"]
	require.True(t, exists)

	costConfig, ok := cost.(CostConfig)
	require.True(t, ok)
	require.NotNil(t, costConfig.Budgets)
	require.NotNil(t, costConfig.Budgets.Global)
	assert.Equal(t, 1000.0, costConfig.Budgets.Global.Amount)
}

// T004: Unit test for ErrExitCodeOutOfRange error type.
func TestErrExitCodeOutOfRange(t *testing.T) {
	// Verify the error variable exists and has the expected message
	require.NotNil(t, ErrExitCodeOutOfRange)
	assert.Contains(t, ErrExitCodeOutOfRange.Error(), "exit code must be between 0 and 255")
}

// T005: Unit test for BudgetConfig.GetExitCode() method.
func TestBudgetConfig_GetExitCode(t *testing.T) {
	tests := []struct {
		name     string
		budget   BudgetConfig
		expected int
	}{
		{
			name:     "default exit code when not set",
			budget:   BudgetConfig{Amount: 100.0, Currency: "USD"},
			expected: 1,
		},
		{
			name:     "explicit exit code 0",
			budget:   BudgetConfig{Amount: 100.0, Currency: "USD", ExitCode: 0, ExitOnThreshold: true},
			expected: 0,
		},
		{
			name:     "explicit exit code 2",
			budget:   BudgetConfig{Amount: 100.0, Currency: "USD", ExitCode: 2},
			expected: 2,
		},
		{
			name:     "max exit code 255",
			budget:   BudgetConfig{Amount: 100.0, Currency: "USD", ExitCode: 255},
			expected: 255,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.budget.GetExitCode())
		})
	}
}

// T006: Unit test for BudgetConfig.ShouldExitOnThreshold() method.
func TestBudgetConfig_ShouldExitOnThreshold(t *testing.T) {
	tests := []struct {
		name     string
		budget   BudgetConfig
		expected bool
	}{
		{
			name:     "default is disabled",
			budget:   BudgetConfig{Amount: 100.0, Currency: "USD"},
			expected: false,
		},
		{
			name:     "explicitly enabled",
			budget:   BudgetConfig{Amount: 100.0, Currency: "USD", ExitOnThreshold: true},
			expected: true,
		},
		{
			name:     "explicitly disabled",
			budget:   BudgetConfig{Amount: 100.0, Currency: "USD", ExitOnThreshold: false},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.budget.ShouldExitOnThreshold())
		})
	}
}

// T007: Unit test for exit code validation (0-255 range).
func TestBudgetConfig_Validate_ExitCode(t *testing.T) {
	tests := []struct {
		name      string
		budget    BudgetConfig
		wantErr   bool
		errString string
	}{
		{
			name: "valid exit code 0",
			budget: BudgetConfig{
				Amount:          100.0,
				Currency:        "USD",
				ExitOnThreshold: true,
				ExitCode:        0,
			},
			wantErr: false,
		},
		{
			name: "valid exit code 1 (default)",
			budget: BudgetConfig{
				Amount:          100.0,
				Currency:        "USD",
				ExitOnThreshold: true,
				ExitCode:        1,
			},
			wantErr: false,
		},
		{
			name: "valid exit code 255 (max)",
			budget: BudgetConfig{
				Amount:          100.0,
				Currency:        "USD",
				ExitOnThreshold: true,
				ExitCode:        255,
			},
			wantErr: false,
		},
		{
			name: "invalid exit code 256 (exceeds max)",
			budget: BudgetConfig{
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
			budget: BudgetConfig{
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
			budget: BudgetConfig{
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

// Test YAML parsing with exit code fields.
func TestBudgetConfig_YAMLParsing_ExitCode(t *testing.T) {
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
		Cost CostConfig `yaml:"cost"`
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
