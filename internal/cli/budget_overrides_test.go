// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package cli

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
)

func TestLegacyBudgetConfigPrecedence(t *testing.T) {
	t.Parallel()

	off := false
	on := true
	fileCode := 3
	parentCode := 4
	cliCode := 9

	tests := []struct {
		name      string
		budgets   *config.BudgetsConfig
		overrides BudgetFlagOverrides
		wantExit  bool
		wantCode  int
	}{
		{
			name: "cli beats scoped config",
			budgets: &config.BudgetsConfig{
				Global: &config.ScopedBudget{
					Amount:          100,
					Currency:        "USD",
					ExitOnThreshold: &off,
					ExitCode:        &fileCode,
				},
			},
			overrides: BudgetFlagOverrides{ExitOnThreshold: &on, ExitCode: &cliCode},
			wantExit:  true,
			wantCode:  9,
		},
		{
			name: "scoped config beats parent",
			budgets: &config.BudgetsConfig{
				ExitOnThreshold: true,
				ExitCode:        &parentCode,
				Global: &config.ScopedBudget{
					ExitOnThreshold: &off,
					ExitCode:        &fileCode,
				},
			},
			wantExit: false,
			wantCode: 3,
		},
		{
			name: "parent beats zero value",
			budgets: &config.BudgetsConfig{
				ExitOnThreshold: true,
				ExitCode:        &parentCode,
				Global:          &config.ScopedBudget{Amount: 10},
			},
			wantExit: true,
			wantCode: 4,
		},
		{
			name:     "zero value when nothing is set",
			budgets:  &config.BudgetsConfig{Global: &config.ScopedBudget{}},
			wantExit: false,
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := legacyBudgetConfig(tt.budgets, tt.overrides)
			assert.Equal(t, tt.wantExit, got.ExitOnThreshold)
			assert.Equal(t, tt.wantCode, got.ExitCode)
		})
	}
}

//nolint:paralleltest // SetGlobalConfig replaces the process-wide global config singleton
func TestEvaluateBudgetStatus_FlagOverrideBeatsConfig(t *testing.T) {
	off := false
	fileCode := 5
	cfg := &config.Config{
		Cost: config.CostConfig{
			Budgets: &config.BudgetsConfig{
				Global: &config.ScopedBudget{
					Amount:          100,
					Currency:        "USD",
					ExitOnThreshold: &off,
					ExitCode:        &fileCode,
					Alerts: []config.AlertConfig{{
						Threshold: 80,
						Type:      config.AlertTypeActual,
					}},
				},
			},
		},
	}
	prev := config.GetGlobalConfig()
	t.Cleanup(func() { config.SetGlobalConfig(prev) })
	config.SetGlobalConfig(cfg)

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetContext(context.Background())

	on := true
	cliCode := 9
	err := evaluateBudgetStatus(cmd, []engine.CostResult{{
		Monthly:  200,
		Currency: "USD",
	}}, 200, BudgetFlagOverrides{
		ExitOnThreshold: &on,
		ExitCode:        &cliCode,
	})

	require.Error(t, err)
	var budgetErr *BudgetExitError
	require.ErrorAs(t, err, &budgetErr)
	assert.Equal(t, 9, budgetErr.ExitCode)
	assert.False(t, *cfg.Cost.Budgets.Global.ExitOnThreshold)
	assert.Equal(t, 5, *cfg.Cost.Budgets.Global.ExitCode)
}

//nolint:paralleltest // SetGlobalConfig replaces the process-wide global config singleton
func TestCostCmd_InvalidExitCodeRejected(t *testing.T) {
	prev := config.GetGlobalConfig()
	t.Cleanup(func() { config.SetGlobalConfig(prev) })
	config.SetGlobalConfig(&config.Config{
		Cost: config.CostConfig{
			Budgets: &config.BudgetsConfig{
				Global: &config.ScopedBudget{Amount: 100, Currency: "USD"},
			},
		},
	})

	cmd := newCostCmd()
	err := cmd.ParseFlags([]string{"--exit-on-threshold", "--exit-code=999"})
	require.NoError(t, err)
	err = cmd.PersistentPreRunE(cmd, []string{})
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid budget configuration")
	assert.Nil(t, config.GetGlobalConfig().Cost.Budgets.Global.ExitOnThreshold)
	assert.Nil(t, config.GetGlobalConfig().Cost.Budgets.Global.ExitCode)
}
