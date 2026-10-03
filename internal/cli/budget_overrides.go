// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
)

type budgetFlagOverrideKey struct{}

// BudgetFlagOverrides carries CLI budget-exit flags. A nil field means that
// flag was not set, so evaluation keeps the env, config-file, or default value.
type BudgetFlagOverrides struct {
	ExitOnThreshold *bool
	ExitCode        *int
}

func budgetFlagOverridesFromCmd(cmd *cobra.Command) BudgetFlagOverrides {
	if cmd == nil {
		return BudgetFlagOverrides{}
	}
	var overrides BudgetFlagOverrides
	if cmd.Flags().Changed("exit-on-threshold") {
		value, err := cmd.Flags().GetBool("exit-on-threshold")
		if err == nil {
			overrides.ExitOnThreshold = &value
		}
	}
	if cmd.Flags().Changed("exit-code") {
		value, err := cmd.Flags().GetInt("exit-code")
		if err == nil {
			overrides.ExitCode = &value
		}
	}
	return overrides
}

func storeBudgetFlagOverrides(cmd *cobra.Command, overrides BudgetFlagOverrides) {
	if cmd == nil {
		return
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, budgetFlagOverrideKey{}, overrides))
}

func storedBudgetFlagOverrides(cmd *cobra.Command) BudgetFlagOverrides {
	if cmd == nil || cmd.Context() == nil {
		return BudgetFlagOverrides{}
	}
	overrides, _ := cmd.Context().Value(budgetFlagOverrideKey{}).(BudgetFlagOverrides)
	return overrides
}

// legacyBudgetConfig resolves the global budget's exit settings.
// Precedence is CLI override, then the scoped budget field, then the parent
// budgets config, then the zero value.
func legacyBudgetConfig(budgets *config.BudgetsConfig, overrides BudgetFlagOverrides) config.BudgetConfig {
	budget := config.BudgetConfig{}
	if budgets != nil && budgets.Global != nil {
		global := budgets.Global
		budget.Amount = global.Amount
		budget.Currency = global.Currency
		budget.Period = global.Period
		budget.Alerts = global.Alerts
		if global.ExitOnThreshold != nil {
			budget.ExitOnThreshold = *global.ExitOnThreshold
		} else {
			budget.ExitOnThreshold = budgets.ExitOnThreshold
		}
		if global.ExitCode != nil {
			budget.ExitCode = *global.ExitCode
		} else if budgets.ExitCode != nil {
			budget.ExitCode = *budgets.ExitCode
		}
	}
	return applyBudgetFlagOverrides(budget, overrides)
}

func applyBudgetFlagOverrides(budget config.BudgetConfig, overrides BudgetFlagOverrides) config.BudgetConfig {
	if overrides.ExitOnThreshold != nil {
		budget.ExitOnThreshold = *overrides.ExitOnThreshold
	}
	if overrides.ExitCode != nil {
		budget.ExitCode = *overrides.ExitCode
	}
	return budget
}

func withBudgetFlagOverrides(budget *config.ScopedBudget, overrides BudgetFlagOverrides) *config.ScopedBudget {
	if budget == nil || (overrides.ExitOnThreshold == nil && overrides.ExitCode == nil) {
		return budget
	}
	copied := *budget
	if overrides.ExitOnThreshold != nil {
		copied.ExitOnThreshold = overrides.ExitOnThreshold
	}
	if overrides.ExitCode != nil {
		copied.ExitCode = overrides.ExitCode
	}
	return &copied
}

func validateBudgetFlagOverrides(cfg *config.Config, overrides BudgetFlagOverrides) error {
	if cfg == nil || !budgetExitEnabled(cfg, overrides) {
		return nil
	}
	global := scopedBudgetForValidation(cfg, overrides)
	if err := global.Validate(""); err != nil {
		return fmt.Errorf("invalid budget configuration: %w", err)
	}
	return nil
}

func budgetExitEnabled(cfg *config.Config, overrides BudgetFlagOverrides) bool {
	if overrides.ExitOnThreshold != nil {
		return *overrides.ExitOnThreshold
	}
	if cfg == nil || cfg.Cost.Budgets == nil || cfg.Cost.Budgets.Global == nil {
		return false
	}
	if cfg.Cost.Budgets.Global.ExitOnThreshold == nil {
		return false
	}
	return *cfg.Cost.Budgets.Global.ExitOnThreshold
}

func scopedBudgetForValidation(cfg *config.Config, overrides BudgetFlagOverrides) config.ScopedBudget {
	var global config.ScopedBudget
	if cfg != nil && cfg.Cost.Budgets != nil && cfg.Cost.Budgets.Global != nil {
		global = *cfg.Cost.Budgets.Global
	}
	if overrides.ExitOnThreshold != nil {
		global.ExitOnThreshold = overrides.ExitOnThreshold
	}
	if overrides.ExitCode != nil {
		global.ExitCode = overrides.ExitCode
	}
	return global
}
