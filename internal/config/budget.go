package config

import (
	"errors"
	"fmt"
)

// AlertType represents the type of budget alert evaluation.
type AlertType string

// Valid alert types for budget threshold evaluation.
const (
	// AlertTypeActual triggers when actual spend exceeds threshold.
	AlertTypeActual AlertType = "actual"
	// AlertTypeForecasted triggers when forecasted spend exceeds threshold.
	AlertTypeForecasted AlertType = "forecasted"
)

// DefaultBudgetPeriod is the default period for budget tracking.
const DefaultBudgetPeriod = "monthly"

// Budget validation limits.
const (
	MaxThresholdPercent = 1000.0 // Allow alerts up to 1000% for extreme overspend detection
	MinThresholdPercent = 0.0    // Minimum threshold percentage
)

// Exit code limits (Unix standard).
const (
	MinExitCode = 0   // Minimum valid exit code
	MaxExitCode = 255 // Maximum valid exit code (Unix standard)
)

// Budget validation errors.
var (
	ErrBudgetAmountNegative   = errors.New("budget amount cannot be negative")
	ErrBudgetCurrencyRequired = errors.New(
		"currency is required when budget amount is greater than 0",
	)
	ErrUnsupportedBudgetPeriod  = errors.New("budget period must be 'monthly'")
	ErrAlertThresholdOutOfRange = errors.New("alert threshold must be between 0 and 1000")
	ErrAlertTypeInvalid         = errors.New("alert type must be 'actual' or 'forecasted'")
	ErrExitCodeOutOfRange       = errors.New("exit code must be between 0 and 255")
)

// AlertConfig defines a specific threshold that triggers a notification.
// It represents a point in the budget consumption where the user should be alerted.
type AlertConfig struct {
	// Threshold is the percentage of budget consumed that triggers this alert (e.g., 80.0 for 80%).
	Threshold float64 `yaml:"threshold" json:"threshold"`
	// Type is the evaluation type: "actual" or "forecasted".
	Type AlertType `yaml:"type" json:"type"`
}

// Validate checks if the alert configuration is valid.
func (a AlertConfig) Validate() error {
	if a.Threshold < MinThresholdPercent || a.Threshold > MaxThresholdPercent {
		return fmt.Errorf("%w: got %.2f", ErrAlertThresholdOutOfRange, a.Threshold)
	}
	if a.Type != AlertTypeActual && a.Type != AlertTypeForecasted {
		return fmt.Errorf("%w: got %q", ErrAlertTypeInvalid, a.Type)
	}
	return nil
}

// BudgetConfig represents the spending limit and associated alerts for a period.
// It defines a budget with an amount, currency, time period, and threshold alerts.
type BudgetConfig struct {
	// Amount is the total spend limit for the period. Use 0 to disable the budget.
	Amount float64 `yaml:"amount" json:"amount"`
	// Currency is the ISO 4217 currency code (e.g., "USD", "EUR").
	Currency string `yaml:"currency" json:"currency"`
	// Period is the time period for the budget (e.g., "monthly"). Defaults to "monthly".
	Period string `yaml:"period,omitempty" json:"period,omitempty"`
	// Alerts is a list of thresholds that trigger notifications.
	Alerts []AlertConfig `yaml:"alerts,omitempty" json:"alerts,omitempty"`

	// ExitOnThreshold enables non-zero exit codes when budget thresholds are exceeded.
	// When true, the CLI will exit with the configured exit code on threshold violation.
	ExitOnThreshold bool `yaml:"exit_on_threshold,omitempty" json:"exit_on_threshold,omitempty"`
	// ExitCode is the exit code to use when a threshold is exceeded.
	// Only validated when ExitOnThreshold is true. Defaults to 1 if not set.
	// Must be in range 0-255 (Unix standard).
	ExitCode int `yaml:"exit_code,omitempty" json:"exit_code,omitempty"`
}

// IsEnabled returns true if the budget is configured and enabled (Amount > 0).
func (b BudgetConfig) IsEnabled() bool {
	return b.Amount > 0
}

// IsDisabled returns true if the budget is disabled (Amount == 0).
func (b BudgetConfig) IsDisabled() bool {
	return b.Amount == 0
}

// GetPeriod returns the budget period, defaulting to "monthly" if not set.
func (b BudgetConfig) GetPeriod() string {
	if b.Period == "" {
		return DefaultBudgetPeriod
	}
	return b.Period
}

// ShouldExitOnThreshold returns true if the CLI should exit with a non-zero
// code when budget thresholds are exceeded.
func (b BudgetConfig) ShouldExitOnThreshold() bool {
	return b.ExitOnThreshold
}

// GetExitCode returns the configured exit code, defaulting to 1 if not set.
// This method provides the raw exit code defined in the configuration. Callers
// should typically check ShouldExitOnThreshold() (or ExitOnThreshold) to determine
// if they should act on this exit code.
//
// Note: Exit code 0 is valid and explicitly allowed (for warning-only mode).
// When ExitOnThreshold is true and GetExitCode() returns 0, the CLI should log
// a warning instead of terminating with a non-zero error.
func (b BudgetConfig) GetExitCode() int {
	// When ExitOnThreshold is true and ExitCode is 0, return 0 (warning-only mode)
	if b.ExitOnThreshold && b.ExitCode == 0 {
		return 0
	}
	// When ExitCode is explicitly set, return it
	if b.ExitCode != 0 {
		return b.ExitCode
	}
	// Default to 1 when not set
	return 1
}

// Validate checks if the budget configuration is valid.
// Returns nil if the budget is disabled (Amount == 0) or if all validations pass.
func (b BudgetConfig) Validate() error {
	// Negative amounts are never allowed
	if b.Amount < 0 {
		return ErrBudgetAmountNegative
	}

	// If budget is disabled (Amount == 0), no further validation needed
	if b.IsDisabled() {
		return nil
	}

	// Period validation: only "monthly" is supported
	// Check period before other validations so invalid periods fail fast
	if b.Period != "" && b.Period != DefaultBudgetPeriod {
		return fmt.Errorf("%w: got %q", ErrUnsupportedBudgetPeriod, b.Period)
	}

	// Currency is required for enabled budgets
	if b.Currency == "" {
		return ErrBudgetCurrencyRequired
	}

	// Validate all alert configurations
	for i, alert := range b.Alerts {
		if err := alert.Validate(); err != nil {
			return fmt.Errorf("alert[%d]: %w", i, err)
		}
	}

	// Validate exit code only when exit on threshold is enabled
	if b.ExitOnThreshold {
		if b.ExitCode < MinExitCode || b.ExitCode > MaxExitCode {
			return fmt.Errorf("%w: got %d", ErrExitCodeOutOfRange, b.ExitCode)
		}
	}

	return nil
}

// GetActualAlerts returns only alerts of type "actual".
func (b BudgetConfig) GetActualAlerts() []AlertConfig {
	var alerts []AlertConfig
	for _, a := range b.Alerts {
		if a.Type == AlertTypeActual {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// GetForecastedAlerts returns only alerts of type "forecasted".
func (b BudgetConfig) GetForecastedAlerts() []AlertConfig {
	var alerts []AlertConfig
	for _, a := range b.Alerts {
		if a.Type == AlertTypeForecasted {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// History configuration defaults.
const (
	HistoryDefaultRetentionDays = 90
	HistoryDefaultEnabled       = true
	AllocationDefaultEnabled    = false
)

// HistoryConfig defines resource history tracking behavior.
type HistoryConfig struct {
	// Enabled controls whether history tracking is active (default: true).
	// Pointer type distinguishes explicit false from omitted (nil = use default).
	Enabled *bool `yaml:"enabled" json:"enabled"`

	// RetentionDays is the number of days before stale entries are removed (default: 90).
	RetentionDays int `yaml:"retention_days" json:"retention_days"`

	// Directory overrides the history directory path (default: ~/.finfocus/history).
	Directory string `yaml:"directory,omitempty" json:"directory,omitempty"`
}

// IsEnabled returns whether history is enabled, respecting explicit false vs omitted.
func (h HistoryConfig) IsEnabled() bool {
	if h.Enabled == nil {
		return HistoryDefaultEnabled
	}
	return *h.Enabled
}

// Validate checks if the history configuration is valid.
// Returns an error for non-positive RetentionDays when explicitly set.
func (h HistoryConfig) Validate() error {
	if h.RetentionDays < 0 {
		return fmt.Errorf("history retention_days must be non-negative, got %d", h.RetentionDays)
	}
	return nil
}

// Allocation validation limits.
const (
	MaxAllocationTagKeyLength = 128
)

// AllocationConfig defines tag-based cost attribution settings.
type AllocationConfig struct {
	// Enabled controls whether tag-based fallback is active (default: false).
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Tags lists the tag keys to use for billing API queries.
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`
}

// Validate checks if the allocation configuration is valid.
// When disabled, tag validation is skipped.
// When enabled with empty tags, validation passes (caller may log a warning).
// Tag keys must be non-empty and at most 128 characters.
func (a AllocationConfig) Validate() error {
	if !a.Enabled {
		return nil
	}

	for i, tag := range a.Tags {
		if tag == "" {
			return fmt.Errorf("tag key at index %d is empty", i)
		}
		if len(tag) > MaxAllocationTagKeyLength {
			return fmt.Errorf("tag key at index %d exceeds maximum length of %d characters: got %d",
				i, MaxAllocationTagKeyLength, len(tag))
		}
	}

	return nil
}

// CostConfig holds cost-related configuration settings including budgets and caching.
// It groups all cost management features under a single configuration section.
type CostConfig struct {
	// Budgets contains the hierarchical budget configuration with
	// global, provider, tag, and resource type scopes.
	Budgets *BudgetsConfig `yaml:"budgets,omitempty" json:"budgets,omitempty"`

	// Cache contains the cache configuration for query result caching.
	Cache CacheConfig `yaml:"cache,omitempty" json:"cache,omitempty"`

	// History contains the resource history tracking configuration.
	History HistoryConfig `yaml:"history" json:"history"`

	// Allocation contains the tag-based cost attribution configuration.
	Allocation AllocationConfig `yaml:"allocation" json:"allocation"`
}

// CacheConfig defines caching behavior for query results.
type CacheConfig struct {
	// Enabled controls whether caching is enabled (default: true).
	Enabled bool `yaml:"enabled" json:"enabled"`

	// TTLSeconds is the time-to-live for cached entries in seconds (default: 3600 = 1 hour).
	TTLSeconds int `yaml:"ttl_seconds" json:"ttl_seconds"`

	// Directory is the cache directory path (default: ~/.finfocus/cache).
	Directory string `yaml:"directory,omitempty" json:"directory,omitempty"`

	// MaxSizeMB is the maximum cache size in megabytes (default: 100, 0 = unlimited).
	MaxSizeMB int `yaml:"max_size_mb" json:"max_size_mb"`
}

// Validate validates the cost configuration.
// Returns an error for fatal validation issues. Non-fatal warnings (like duplicate
// tag budget priorities) can be retrieved via GetBudgetsWarnings().
func (c CostConfig) Validate() error {
	// Validate budgets if set
	if c.Budgets != nil {
		_, err := c.Budgets.Validate()
		if err != nil {
			return fmt.Errorf("budgets: %w", err)
		}
		// Warnings are non-fatal and available via GetBudgetsWarnings()
	}

	if err := c.History.Validate(); err != nil {
		return fmt.Errorf("history: %w", err)
	}

	if err := c.Allocation.Validate(); err != nil {
		return fmt.Errorf("allocation: %w", err)
	}

	return nil
}

// HasBudget returns true if a budget is configured and enabled.
func (c CostConfig) HasBudget() bool {
	return c.Budgets != nil && c.Budgets.IsEnabled()
}

// GetBudgetsWarnings validates the budgets and returns any warnings.
// Returns nil if no budgets are configured or no warnings exist.
// Validation errors are included as warnings to avoid silent failures.
func (c CostConfig) GetBudgetsWarnings() []string {
	if c.Budgets == nil {
		return nil
	}

	warnings, err := c.Budgets.Validate()
	if err != nil {
		// Include validation error as a warning to avoid silent failure
		return append(warnings, fmt.Sprintf("validation error: %v", err))
	}
	return warnings
}
