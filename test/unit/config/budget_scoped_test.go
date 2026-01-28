package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/rshade/finfocus/internal/config"
)

// fixturesDir returns the path to the test fixtures directory.
func fixturesDir() string {
	return filepath.Join("..", "..", "fixtures", "budgets")
}

// loadFixture loads and parses a test fixture file.
func loadFixture(t *testing.T, filename string) *testConfig {
	t.Helper()
	path := filepath.Join(fixturesDir(), filename)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "failed to read fixture %s", filename)

	var cfg testConfig
	err = yaml.Unmarshal(data, &cfg)
	require.NoError(t, err, "failed to parse fixture %s", filename)

	return &cfg
}

// testConfig mirrors the config structure for test parsing.
type testConfig struct {
	Cost struct {
		Budgets config.BudgetsConfig `yaml:"budgets"`
	} `yaml:"cost"`
}

// TestScopedBudget_Validation tests ScopedBudget validation logic.
func TestScopedBudget_Validation(t *testing.T) {
	tests := []struct {
		name           string
		budget         *config.ScopedBudget
		globalCurrency string
		wantErr        bool
		errContains    string
	}{
		{
			name:    "nil budget is valid",
			budget:  nil,
			wantErr: false,
		},
		{
			name: "valid budget with all fields",
			budget: &config.ScopedBudget{
				Amount:   1000.0,
				Currency: "USD",
				Period:   "monthly",
				Alerts: []config.AlertConfig{
					{Threshold: 80.0, Type: config.AlertTypeActual},
				},
			},
			wantErr: false,
		},
		{
			name: "disabled budget (zero amount) is valid",
			budget: &config.ScopedBudget{
				Amount: 0,
			},
			wantErr: false,
		},
		{
			name: "negative amount is invalid",
			budget: &config.ScopedBudget{
				Amount: -100.0,
			},
			wantErr:     true,
			errContains: "negative",
		},
		{
			name: "invalid period is rejected",
			budget: &config.ScopedBudget{
				Amount:   1000.0,
				Currency: "USD",
				Period:   "weekly",
			},
			wantErr:     true,
			errContains: "monthly",
		},
		{
			name: "currency mismatch with global",
			budget: &config.ScopedBudget{
				Amount:   1000.0,
				Currency: "EUR",
			},
			globalCurrency: "USD",
			wantErr:        true,
			errContains:    "currency",
		},
		{
			name: "currency inheritance (empty) is valid",
			budget: &config.ScopedBudget{
				Amount:   1000.0,
				Currency: "",
			},
			globalCurrency: "USD",
			wantErr:        false,
		},
		{
			name: "invalid currency format (lowercase)",
			budget: &config.ScopedBudget{
				Amount:   1000.0,
				Currency: "usd",
			},
			wantErr:     true,
			errContains: "uppercase",
		},
		{
			name: "invalid currency format (wrong length)",
			budget: &config.ScopedBudget{
				Amount:   1000.0,
				Currency: "US",
			},
			wantErr:     true,
			errContains: "3 letters",
		},
		{
			name: "invalid exit code (> 255)",
			budget: &config.ScopedBudget{
				Amount:   1000.0,
				Currency: "USD",
				ExitCode: intPtr(256),
			},
			wantErr:     true,
			errContains: "exit code",
		},
		{
			name: "invalid exit code (< 0)",
			budget: &config.ScopedBudget{
				Amount:   1000.0,
				Currency: "USD",
				ExitCode: intPtr(-1),
			},
			wantErr:     true,
			errContains: "exit code",
		},
		{
			name: "valid exit code 0 (warning mode)",
			budget: &config.ScopedBudget{
				Amount:   1000.0,
				Currency: "USD",
				ExitCode: intPtr(0),
			},
			wantErr: false,
		},
		{
			name: "invalid alert threshold",
			budget: &config.ScopedBudget{
				Amount:   1000.0,
				Currency: "USD",
				Alerts: []config.AlertConfig{
					{Threshold: -5.0, Type: config.AlertTypeActual},
				},
			},
			wantErr:     true,
			errContains: "alert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.budget.Validate(tt.globalCurrency)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestTagBudget_Validation tests TagBudget validation including selector parsing.
func TestTagBudget_Validation(t *testing.T) {
	tests := []struct {
		name           string
		tagBudget      *config.TagBudget
		globalCurrency string
		wantErr        bool
		errContains    string
	}{
		{
			name:      "nil tag budget is valid",
			tagBudget: nil,
			wantErr:   false,
		},
		{
			name: "valid tag budget with key:value selector",
			tagBudget: &config.TagBudget{
				Selector: "team:platform",
				Priority: 100,
				ScopedBudget: config.ScopedBudget{
					Amount:   1000.0,
					Currency: "USD",
				},
			},
			wantErr: false,
		},
		{
			name: "valid tag budget with wildcard selector",
			tagBudget: &config.TagBudget{
				Selector: "cost-center:*",
				Priority: 10,
				ScopedBudget: config.ScopedBudget{
					Amount:   500.0,
					Currency: "USD",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid selector format (spaces)",
			tagBudget: &config.TagBudget{
				Selector: "team platform",
				ScopedBudget: config.ScopedBudget{
					Amount:   1000.0,
					Currency: "USD",
				},
			},
			wantErr:     true,
			errContains: "selector",
		},
		{
			name: "invalid selector format (no colon)",
			tagBudget: &config.TagBudget{
				Selector: "team",
				ScopedBudget: config.ScopedBudget{
					Amount:   1000.0,
					Currency: "USD",
				},
			},
			wantErr:     true,
			errContains: "selector",
		},
		{
			name: "invalid selector format (empty key)",
			tagBudget: &config.TagBudget{
				Selector: ":value",
				ScopedBudget: config.ScopedBudget{
					Amount:   1000.0,
					Currency: "USD",
				},
			},
			wantErr:     true,
			errContains: "selector",
		},
		{
			name: "selector with underscores and hyphens is valid",
			tagBudget: &config.TagBudget{
				Selector: "cost_center-id:value_123",
				ScopedBudget: config.ScopedBudget{
					Amount:   1000.0,
					Currency: "USD",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tagBudget.Validate(tt.globalCurrency)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestParseTagSelector tests tag selector parsing.
func TestParseTagSelector(t *testing.T) {
	tests := []struct {
		name       string
		selector   string
		wantKey    string
		wantValue  string
		wantWild   bool
		wantErr    bool
		errContain string
	}{
		{
			name:      "simple key:value",
			selector:  "team:platform",
			wantKey:   "team",
			wantValue: "platform",
			wantWild:  false,
		},
		{
			name:      "wildcard selector",
			selector:  "env:*",
			wantKey:   "env",
			wantValue: "*",
			wantWild:  true,
		},
		{
			name:      "key with underscores",
			selector:  "cost_center:finance",
			wantKey:   "cost_center",
			wantValue: "finance",
			wantWild:  false,
		},
		{
			name:      "key with hyphens",
			selector:  "cost-center:ops-team",
			wantKey:   "cost-center",
			wantValue: "ops-team",
			wantWild:  false,
		},
		{
			name:       "invalid - spaces",
			selector:   "team platform",
			wantErr:    true,
			errContain: "selector",
		},
		{
			name:       "invalid - no colon",
			selector:   "teamplatform",
			wantErr:    true,
			errContain: "selector",
		},
		{
			name:       "invalid - empty string",
			selector:   "",
			wantErr:    true,
			errContain: "selector",
		},
		{
			name:       "invalid - special characters",
			selector:   "team@home:value!",
			wantErr:    true,
			errContain: "selector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := config.ParseTagSelector(tt.selector)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
			} else {
				require.NoError(t, err)
				require.NotNil(t, parsed)
				assert.Equal(t, tt.wantKey, parsed.Key)
				assert.Equal(t, tt.wantValue, parsed.Value)
				assert.Equal(t, tt.wantWild, parsed.IsWildcard)
			}
		})
	}
}

// TestParsedTagSelector_Matches tests tag matching logic.
func TestParsedTagSelector_Matches(t *testing.T) {
	tests := []struct {
		name     string
		selector string
		tags     map[string]string
		want     bool
	}{
		{
			name:     "exact match",
			selector: "team:platform",
			tags:     map[string]string{"team": "platform"},
			want:     true,
		},
		{
			name:     "no match - wrong value",
			selector: "team:platform",
			tags:     map[string]string{"team": "backend"},
			want:     false,
		},
		{
			name:     "no match - key not present",
			selector: "team:platform",
			tags:     map[string]string{"env": "prod"},
			want:     false,
		},
		{
			name:     "wildcard matches any value",
			selector: "team:*",
			tags:     map[string]string{"team": "anything"},
			want:     true,
		},
		{
			name:     "wildcard no match - key not present",
			selector: "team:*",
			tags:     map[string]string{"env": "prod"},
			want:     false,
		},
		{
			name:     "nil tags returns false",
			selector: "team:platform",
			tags:     nil,
			want:     false,
		},
		{
			name:     "empty tags returns false",
			selector: "team:platform",
			tags:     map[string]string{},
			want:     false,
		},
		{
			name:     "multiple tags - correct match",
			selector: "team:platform",
			tags:     map[string]string{"team": "platform", "env": "prod"},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := config.ParseTagSelector(tt.selector)
			require.NoError(t, err)

			got := parsed.Matches(tt.tags)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestBudgetsConfig_Validation tests BudgetsConfig validation.
func TestBudgetsConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.BudgetsConfig
		wantErr     bool
		errContains string
		wantWarns   int
	}{
		{
			name:    "nil config is valid",
			config:  nil,
			wantErr: false,
		},
		{
			name: "global only is valid",
			config: &config.BudgetsConfig{
				Global: &config.ScopedBudget{
					Amount:   5000.0,
					Currency: "USD",
				},
			},
			wantErr: false,
		},
		{
			name: "scoped budgets require global",
			config: &config.BudgetsConfig{
				Providers: map[string]*config.ScopedBudget{
					"aws": {Amount: 3000.0, Currency: "USD"},
				},
			},
			wantErr:     true,
			errContains: "global budget is required",
		},
		{
			name: "provider currency must match global",
			config: &config.BudgetsConfig{
				Global: &config.ScopedBudget{
					Amount:   5000.0,
					Currency: "USD",
				},
				Providers: map[string]*config.ScopedBudget{
					"aws": {Amount: 3000.0, Currency: "EUR"},
				},
			},
			wantErr:     true,
			errContains: "currency",
		},
		{
			name: "tag currency must match global",
			config: &config.BudgetsConfig{
				Global: &config.ScopedBudget{
					Amount:   5000.0,
					Currency: "USD",
				},
				Tags: []config.TagBudget{
					{
						Selector: "team:platform",
						ScopedBudget: config.ScopedBudget{
							Amount:   1000.0,
							Currency: "EUR",
						},
					},
				},
			},
			wantErr:     true,
			errContains: "currency",
		},
		{
			name: "duplicate tag priorities generate warning",
			config: &config.BudgetsConfig{
				Global: &config.ScopedBudget{
					Amount:   5000.0,
					Currency: "USD",
				},
				Tags: []config.TagBudget{
					{
						Selector: "team:platform",
						Priority: 100,
						ScopedBudget: config.ScopedBudget{
							Amount: 1000.0,
						},
					},
					{
						Selector: "team:backend",
						Priority: 100,
						ScopedBudget: config.ScopedBudget{
							Amount: 1000.0,
						},
					},
				},
			},
			wantErr:   false,
			wantWarns: 1,
		},
		{
			name: "invalid tag selector is rejected",
			config: &config.BudgetsConfig{
				Global: &config.ScopedBudget{
					Amount:   5000.0,
					Currency: "USD",
				},
				Tags: []config.TagBudget{
					{
						Selector: "invalid selector",
						ScopedBudget: config.ScopedBudget{
							Amount: 1000.0,
						},
					},
				},
			},
			wantErr:     true,
			errContains: "selector",
		},
		{
			name: "type budget currency must match global",
			config: &config.BudgetsConfig{
				Global: &config.ScopedBudget{
					Amount:   5000.0,
					Currency: "USD",
				},
				Types: map[string]*config.ScopedBudget{
					"aws:ec2/instance": {Amount: 1000.0, Currency: "EUR"},
				},
			},
			wantErr:     true,
			errContains: "currency",
		},
		{
			name: "full valid configuration",
			config: &config.BudgetsConfig{
				Global: &config.ScopedBudget{
					Amount:   10000.0,
					Currency: "USD",
				},
				Providers: map[string]*config.ScopedBudget{
					"aws": {Amount: 5000.0},
					"gcp": {Amount: 3000.0},
				},
				Tags: []config.TagBudget{
					{Selector: "team:platform", Priority: 100, ScopedBudget: config.ScopedBudget{Amount: 2000.0}},
					{Selector: "env:prod", Priority: 50, ScopedBudget: config.ScopedBudget{Amount: 5000.0}},
				},
				Types: map[string]*config.ScopedBudget{
					"aws:ec2/instance": {Amount: 1000.0},
				},
				ExitOnThreshold: true,
				ExitCode:        1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings, err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
			assert.Len(t, warnings, tt.wantWarns)
		})
	}
}

// TestBudgetsConfig_Helpers tests helper methods on BudgetsConfig.
func TestBudgetsConfig_Helpers(t *testing.T) {
	t.Run("HasScopedBudgets", func(t *testing.T) {
		tests := []struct {
			name   string
			config *config.BudgetsConfig
			want   bool
		}{
			{"nil config", nil, false},
			{"empty config", &config.BudgetsConfig{}, false},
			{"global only", &config.BudgetsConfig{Global: &config.ScopedBudget{Amount: 1000}}, false},
			{
				"with providers",
				&config.BudgetsConfig{Providers: map[string]*config.ScopedBudget{"aws": {Amount: 1000}}},
				true,
			},
			{"with tags", &config.BudgetsConfig{Tags: []config.TagBudget{{Selector: "a:b"}}}, true},
			{
				"with types",
				&config.BudgetsConfig{Types: map[string]*config.ScopedBudget{"aws:ec2": {Amount: 1000}}},
				true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.want, tt.config.HasScopedBudgets())
			})
		}
	})

	t.Run("HasGlobalBudget", func(t *testing.T) {
		tests := []struct {
			name   string
			config *config.BudgetsConfig
			want   bool
		}{
			{"nil config", nil, false},
			{"empty config", &config.BudgetsConfig{}, false},
			{"nil global", &config.BudgetsConfig{Global: nil}, false},
			{"zero amount", &config.BudgetsConfig{Global: &config.ScopedBudget{Amount: 0}}, false},
			{"enabled global", &config.BudgetsConfig{Global: &config.ScopedBudget{Amount: 1000}}, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.want, tt.config.HasGlobalBudget())
			})
		}
	})

	t.Run("IsEnabled", func(t *testing.T) {
		tests := []struct {
			name   string
			config *config.BudgetsConfig
			want   bool
		}{
			{"nil config", nil, false},
			{"empty config", &config.BudgetsConfig{}, false},
			{"global enabled", &config.BudgetsConfig{Global: &config.ScopedBudget{Amount: 1000}}, true},
			{
				"provider enabled",
				&config.BudgetsConfig{Providers: map[string]*config.ScopedBudget{"aws": {Amount: 1000}}},
				true,
			},
			{
				"tag enabled",
				&config.BudgetsConfig{Tags: []config.TagBudget{{ScopedBudget: config.ScopedBudget{Amount: 1000}}}},
				true,
			},
			{
				"type enabled",
				&config.BudgetsConfig{Types: map[string]*config.ScopedBudget{"aws:ec2": {Amount: 1000}}},
				true,
			},
			{"all disabled", &config.BudgetsConfig{Global: &config.ScopedBudget{Amount: 0}}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.want, tt.config.IsEnabled())
			})
		}
	})

	t.Run("GetEffectiveExitOnThreshold", func(t *testing.T) {
		cfg := &config.BudgetsConfig{ExitOnThreshold: true}
		assert.True(t, cfg.GetEffectiveExitOnThreshold(nil))
		assert.False(t, cfg.GetEffectiveExitOnThreshold(boolPtr(false)))
		assert.True(t, cfg.GetEffectiveExitOnThreshold(boolPtr(true)))

		cfgFalse := &config.BudgetsConfig{ExitOnThreshold: false}
		assert.False(t, cfgFalse.GetEffectiveExitOnThreshold(nil))
	})

	t.Run("GetEffectiveExitCode", func(t *testing.T) {
		cfg := &config.BudgetsConfig{ExitCode: 5}
		assert.Equal(t, 5, cfg.GetEffectiveExitCode(nil))
		assert.Equal(t, 10, cfg.GetEffectiveExitCode(intPtr(10)))

		cfgDefault := &config.BudgetsConfig{ExitCode: 0}
		assert.Equal(t, 1, cfgDefault.GetEffectiveExitCode(nil)) // Default is 1

		var nilCfg *config.BudgetsConfig
		assert.Equal(t, 1, nilCfg.GetEffectiveExitCode(nil)) // Default is 1
	})
}

// TestBudgetsConfig_LoadFromFixtures tests loading and validating fixture files.
func TestBudgetsConfig_LoadFromFixtures(t *testing.T) {
	t.Run("global_only.yaml", func(t *testing.T) {
		cfg := loadFixture(t, "global_only.yaml")
		warnings, err := cfg.Cost.Budgets.Validate()
		require.NoError(t, err)
		assert.Empty(t, warnings)

		assert.True(t, cfg.Cost.Budgets.HasGlobalBudget())
		assert.False(t, cfg.Cost.Budgets.HasScopedBudgets())
		assert.Equal(t, 5000.0, cfg.Cost.Budgets.Global.Amount)
		assert.Equal(t, "USD", cfg.Cost.Budgets.Global.Currency)
		assert.Len(t, cfg.Cost.Budgets.Global.Alerts, 3)
	})

	t.Run("multi_provider.yaml", func(t *testing.T) {
		cfg := loadFixture(t, "multi_provider.yaml")
		warnings, err := cfg.Cost.Budgets.Validate()
		require.NoError(t, err)
		assert.Empty(t, warnings)

		assert.Len(t, cfg.Cost.Budgets.Providers, 3)
		assert.Equal(t, 5000.0, cfg.Cost.Budgets.Providers["aws"].Amount)
		assert.Equal(t, 3000.0, cfg.Cost.Budgets.Providers["gcp"].Amount)
		assert.Equal(t, 2000.0, cfg.Cost.Budgets.Providers["azure"].Amount)
	})

	t.Run("tag_budgets.yaml", func(t *testing.T) {
		cfg := loadFixture(t, "tag_budgets.yaml")
		warnings, err := cfg.Cost.Budgets.Validate()
		require.NoError(t, err)
		// Should have warnings about duplicate priorities (team:platform and team:backend both priority 100)
		assert.Len(t, warnings, 2) // One for priority 100, one for priority 50

		assert.Len(t, cfg.Cost.Budgets.Tags, 5)
	})

	t.Run("resource_types.yaml", func(t *testing.T) {
		cfg := loadFixture(t, "resource_types.yaml")
		warnings, err := cfg.Cost.Budgets.Validate()
		require.NoError(t, err)
		assert.Empty(t, warnings)

		assert.Len(t, cfg.Cost.Budgets.Types, 4)
		assert.Equal(t, 3000.0, cfg.Cost.Budgets.Types["aws:ec2/instance"].Amount)
	})

	t.Run("full_scoped.yaml", func(t *testing.T) {
		cfg := loadFixture(t, "full_scoped.yaml")
		warnings, err := cfg.Cost.Budgets.Validate()
		require.NoError(t, err)
		assert.Empty(t, warnings)

		assert.True(t, cfg.Cost.Budgets.HasGlobalBudget())
		assert.True(t, cfg.Cost.Budgets.HasScopedBudgets())
		assert.Len(t, cfg.Cost.Budgets.Providers, 3)
		assert.Len(t, cfg.Cost.Budgets.Tags, 3)
		assert.Len(t, cfg.Cost.Budgets.Types, 2)
	})

	t.Run("invalid_currency_mismatch.yaml", func(t *testing.T) {
		cfg := loadFixture(t, "invalid_currency_mismatch.yaml")
		_, err := cfg.Cost.Budgets.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "currency")
	})

	t.Run("invalid_missing_global.yaml", func(t *testing.T) {
		cfg := loadFixture(t, "invalid_missing_global.yaml")
		_, err := cfg.Cost.Budgets.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "global budget is required")
	})

	t.Run("invalid_tag_selector.yaml", func(t *testing.T) {
		cfg := loadFixture(t, "invalid_tag_selector.yaml")
		_, err := cfg.Cost.Budgets.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "selector")
	})
}

// TestScopedBudget_HelperMethods tests ScopedBudget helper methods.
func TestScopedBudget_HelperMethods(t *testing.T) {
	t.Run("IsEnabled", func(t *testing.T) {
		var nilBudget *config.ScopedBudget
		assert.False(t, nilBudget.IsEnabled())
		assert.True(t, (&config.ScopedBudget{Amount: 100}).IsEnabled())
		assert.False(t, (&config.ScopedBudget{Amount: 0}).IsEnabled())
	})

	t.Run("IsDisabled", func(t *testing.T) {
		var nilBudget *config.ScopedBudget
		assert.True(t, nilBudget.IsDisabled())
		assert.False(t, (&config.ScopedBudget{Amount: 100}).IsDisabled())
		assert.True(t, (&config.ScopedBudget{Amount: 0}).IsDisabled())
	})

	t.Run("GetPeriod", func(t *testing.T) {
		var nilBudget *config.ScopedBudget
		assert.Equal(t, "monthly", nilBudget.GetPeriod())
		assert.Equal(t, "monthly", (&config.ScopedBudget{}).GetPeriod())
		assert.Equal(t, "monthly", (&config.ScopedBudget{Period: "monthly"}).GetPeriod())
	})

	t.Run("GetCurrency", func(t *testing.T) {
		var nilBudget *config.ScopedBudget
		assert.Equal(t, "", nilBudget.GetCurrency())
		assert.Equal(t, "", (&config.ScopedBudget{}).GetCurrency())
		assert.Equal(t, "USD", (&config.ScopedBudget{Currency: "USD"}).GetCurrency())
	})

	t.Run("ShouldExitOnThreshold", func(t *testing.T) {
		var nilBudget *config.ScopedBudget
		assert.Nil(t, nilBudget.ShouldExitOnThreshold())
		assert.Nil(t, (&config.ScopedBudget{}).ShouldExitOnThreshold())

		trueVal := true
		assert.True(t, *(&config.ScopedBudget{ExitOnThreshold: &trueVal}).ShouldExitOnThreshold())
	})

	t.Run("GetExitCode", func(t *testing.T) {
		var nilBudget *config.ScopedBudget
		assert.Nil(t, nilBudget.GetExitCode())
		assert.Nil(t, (&config.ScopedBudget{}).GetExitCode())

		code := 5
		assert.Equal(t, 5, *(&config.ScopedBudget{ExitCode: &code}).GetExitCode())
	})
}

// Helper functions.
func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}
