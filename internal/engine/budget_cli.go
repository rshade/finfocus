package engine

import (
	"errors"
	"fmt"
	"time"

	"github.com/rshade/finfocus/internal/config"
)

// ThresholdStatusValue represents the status of a threshold evaluation.
type ThresholdStatusValue string

// Threshold status values for budget alert evaluation.
const (
	// ThresholdStatusOK indicates the threshold has not been reached.
	ThresholdStatusOK ThresholdStatusValue = "OK"
	// ThresholdStatusApproaching indicates spending is approaching the threshold (within 5%).
	ThresholdStatusApproaching ThresholdStatusValue = "APPROACHING"
	// ThresholdStatusExceeded indicates the threshold has been exceeded.
	ThresholdStatusExceeded ThresholdStatusValue = "EXCEEDED"
)

// ApproachingThresholdBuffer is the percentage buffer for "approaching" status.
// If current spend is within this percentage of a threshold, it's "approaching".
const ApproachingThresholdBuffer = 5.0

// percentFull is the percentage value representing 100% budget consumption.
const percentFull = 100

// Budget evaluation error types.
var (
	// ErrCurrencyMismatch is returned when spend currency doesn't match budget currency.
	ErrCurrencyMismatch = errors.New("currency mismatch between budget and actual spend")
	// ErrInvalidSpend is returned when the spend value is invalid.
	ErrInvalidSpend = errors.New("spend value is invalid")
	// ErrBudgetDisabled is returned when trying to evaluate a disabled budget.
	ErrBudgetDisabled = errors.New("budget is disabled (amount is 0)")
)

// Exit code constants for CLI integration.
const (
	// ExitCodeBudgetEvaluationError is returned when budget evaluation fails (FR-009).
	// This is distinct from the configured exit code for threshold violations.
	ExitCodeBudgetEvaluationError = 1
)

// ThresholdStatus represents the status of an individual alert threshold.
type ThresholdStatus struct {
	// Threshold is the configured threshold percentage (e.g., 80.0 for 80%).
	Threshold float64
	// Type is the alert type ("actual" or "forecasted").
	Type config.AlertType
	// Status is the evaluation result: OK, APPROACHING, or EXCEEDED.
	Status ThresholdStatusValue
}

// BudgetStatus represents the result of evaluating a budget against current spend.
type BudgetStatus struct {
	// Budget is the original budget configuration.
	Budget config.BudgetConfig
	// CurrentSpend is the actual spend amount.
	CurrentSpend float64
	// Percentage is the percentage of budget consumed (0-100+).
	Percentage float64
	// ForecastedSpend is the estimated total spend by end of period (0 if forecasting not applicable).
	ForecastedSpend float64
	// ForecastPercentage is the percentage of budget forecasted to be consumed.
	ForecastPercentage float64
	// Alerts contains the status of each configured alert threshold.
	Alerts []ThresholdStatus
	// Currency is the currency of the spend (validated to match budget).
	Currency string
}

// BudgetEngine defines the interface for budget evaluation operations.
type BudgetEngine interface {
	// Evaluate compares current spend against the configured budget and alerts.
	// It returns a BudgetStatus or an error if evaluation fails.
	Evaluate(budget config.BudgetConfig, currentSpend float64, currency string) (*BudgetStatus, error)
}

// DefaultBudgetEngine implements BudgetEngine with standard evaluation logic.
type DefaultBudgetEngine struct {
	// now is a function that returns the current time (injectable for testing).
	now func() time.Time
}

// NewBudgetEngine returns a DefaultBudgetEngine configured to use time.Now as the time source.
func NewBudgetEngine() *DefaultBudgetEngine {
	return &DefaultBudgetEngine{
		now: time.Now,
	}
}

// NewBudgetEngineWithTime creates a new DefaultBudgetEngine with a custom time function.
// NewBudgetEngineWithTime creates a DefaultBudgetEngine that uses nowFunc as the source of current time.
// nowFunc is called whenever the engine needs the current time and enables deterministic behavior for tests.
// If nowFunc is nil, time.Now is used as the default.
// It returns a pointer to a DefaultBudgetEngine configured to use the provided time function.
func NewBudgetEngineWithTime(nowFunc func() time.Time) *DefaultBudgetEngine {
	if nowFunc == nil {
		nowFunc = time.Now
	}
	return &DefaultBudgetEngine{
		now: nowFunc,
	}
}

// Evaluate compares current spend against the configured budget and alerts.
// It returns a BudgetStatus with threshold evaluations or an error if validation fails.
//
// Errors are returned when:
//   - budget.Currency != currency (currency mismatch)
//   - budget.Amount is <= 0 (budget disabled or invalid)
//   - currentSpend is negative (invalid spend)
func (e *DefaultBudgetEngine) Evaluate(
	budget config.BudgetConfig,
	currentSpend float64,
	currency string,
) (*BudgetStatus, error) {
	// Validate budget is enabled
	if budget.IsDisabled() {
		return nil, ErrBudgetDisabled
	}

	// Validate currency match
	if budget.Currency != currency {
		return nil, fmt.Errorf("%w: budget is %s, spend is %s", ErrCurrencyMismatch, budget.Currency, currency)
	}

	// Validate spend is non-negative (allow zero)
	if currentSpend < 0 {
		return nil, fmt.Errorf("%w: negative spend not allowed: %.2f", ErrInvalidSpend, currentSpend)
	}

	// Calculate percentage of budget consumed
	percentage := (currentSpend / budget.Amount) * percentFull

	// Calculate forecasted spend using linear extrapolation
	forecastedSpend, forecastPercentage := e.calculateForecast(budget.Amount, currentSpend)

	// Evaluate all configured alerts
	alerts := e.evaluateAlerts(budget.Alerts, percentage, forecastPercentage)

	return &BudgetStatus{
		Budget:             budget,
		CurrentSpend:       currentSpend,
		Percentage:         percentage,
		ForecastedSpend:    forecastedSpend,
		ForecastPercentage: forecastPercentage,
		Alerts:             alerts,
		Currency:           currency,
	}, nil
}

// calculateForecast calculates the forecasted monthly spend using linear extrapolation.
// Formula: forecast = (current_spend / current_day_in_period) * total_days_in_period
// Returns the forecasted spend amount and the percentage of budget it represents.
func (e *DefaultBudgetEngine) calculateForecast(budgetAmount, currentSpend float64) (float64, float64) {
	now := e.now()
	currentDay := now.Day()
	totalDays := daysInMonth(now)

	// Avoid division by zero on day 1
	if currentDay == 0 {
		currentDay = 1
	}

	// Linear extrapolation: (spend / day) * total_days
	dailyRate := currentSpend / float64(currentDay)
	forecastedSpend := dailyRate * float64(totalDays)

	// Calculate forecast percentage of budget
	forecastPercentage := (forecastedSpend / budgetAmount) * percentFull

	return forecastedSpend, forecastPercentage
}

// evaluateAlerts evaluates all configured alert thresholds against the current percentages.
func (e *DefaultBudgetEngine) evaluateAlerts(
	alerts []config.AlertConfig,
	actualPercentage, forecastPercentage float64,
) []ThresholdStatus {
	results := make([]ThresholdStatus, 0, len(alerts))

	for _, alert := range alerts {
		var percentage float64
		if alert.Type == config.AlertTypeActual {
			percentage = actualPercentage
		} else {
			percentage = forecastPercentage
		}

		status := evaluateThreshold(alert.Threshold, percentage)
		results = append(results, ThresholdStatus{
			Threshold: alert.Threshold,
			Type:      alert.Type,
			Status:    status,
		})
	}

	return results
}

// evaluateThreshold determines the threshold status for a given percentage relative to a threshold.
// It returns ThresholdStatusExceeded when `percentage` is greater than or equal to `threshold`,
// ThresholdStatusApproaching when `percentage` is within `ApproachingThresholdBuffer` percentage points below `threshold`,
// and ThresholdStatusOK otherwise.
// For very small thresholds (at or below the buffer), the approaching check is skipped.
func evaluateThreshold(threshold, percentage float64) ThresholdStatusValue {
	if percentage >= threshold {
		return ThresholdStatusExceeded
	}

	// Skip approaching check for small thresholds to avoid false positives
	if threshold > ApproachingThresholdBuffer {
		// Check if approaching (within ApproachingThresholdBuffer percentage points)
		if percentage >= (threshold - ApproachingThresholdBuffer) {
			return ThresholdStatusApproaching
		}
	}

	return ThresholdStatusOK
}

// daysInMonth returns the total number of days in the month of the given time.
func daysInMonth(t time.Time) int {
	// Get the first day of next month, then go back one day
	year, month, _ := t.Date()
	firstOfNextMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, t.Location())
	lastOfMonth := firstOfNextMonth.AddDate(0, 0, -1)
	return lastOfMonth.Day()
}

// HasExceededAlerts returns true if any alert has EXCEEDED status.
func (s *BudgetStatus) HasExceededAlerts() bool {
	for _, alert := range s.Alerts {
		if alert.Status == ThresholdStatusExceeded {
			return true
		}
	}
	return false
}

// HasApproachingAlerts returns true if any alert has APPROACHING status.
func (s *BudgetStatus) HasApproachingAlerts() bool {
	for _, alert := range s.Alerts {
		if alert.Status == ThresholdStatusApproaching {
			return true
		}
	}
	return false
}

// GetExceededAlerts returns all alerts with EXCEEDED status.
func (s *BudgetStatus) GetExceededAlerts() []ThresholdStatus {
	var exceeded []ThresholdStatus
	for _, alert := range s.Alerts {
		if alert.Status == ThresholdStatusExceeded {
			exceeded = append(exceeded, alert)
		}
	}
	return exceeded
}

// GetHighestExceededThreshold returns the highest threshold that has been exceeded.
// Returns 0 if no thresholds are exceeded.
func (s *BudgetStatus) GetHighestExceededThreshold() float64 {
	highest := 0.0
	for _, alert := range s.Alerts {
		if alert.Status == ThresholdStatusExceeded && alert.Threshold > highest {
			highest = alert.Threshold
		}
	}
	return highest
}

// CappedPercentage returns the percentage capped at 100 for display purposes.
// The actual Percentage field may exceed 100 for over-budget scenarios.
func (s *BudgetStatus) CappedPercentage() float64 {
	if s.Percentage > percentFull {
		return percentFull
	}
	return s.Percentage
}

// IsOverBudget returns true if current spend exceeds the budget amount.
func (s *BudgetStatus) IsOverBudget() bool {
	return s.Percentage > percentFull
}

// IsForecastOverBudget returns true if forecasted spend exceeds the budget amount.
func (s *BudgetStatus) IsForecastOverBudget() bool {
	return s.ForecastPercentage > percentFull
}

// ShouldExit returns true if the CLI should "exit" due to an exceeded budget threshold.
// It returns true when ExitOnThreshold is enabled in the budget config and any
// threshold has been exceeded, regardless of whether the configured exit code
// is zero (warning-only).
func (s *BudgetStatus) ShouldExit() bool {
	if !s.Budget.ShouldExitOnThreshold() {
		return false
	}
	return s.HasExceededAlerts()
}

// GetExitCode returns the appropriate exit code based on budget evaluation.
// Returns 0 if no exit should occur (ShouldExit() is false).
// Returns the configured exit code if a threshold is exceeded and ExitOnThreshold is enabled.
// Note: A zero exit code representing a warning-only condition is explicitly allowed.
func (s *BudgetStatus) GetExitCode() int {
	if !s.ShouldExit() {
		return 0
	}
	return s.Budget.GetExitCode()
}

// ExitReason returns a human-readable reason for the exit code.
// Returns an empty string if no exit should occur.
// A zero exit code represents a warning-only condition.
// Used for debug logging and user feedback.
func (s *BudgetStatus) ExitReason() string {
	if !s.ShouldExit() {
		return ""
	}

	highest := s.GetHighestExceededThreshold()
	if s.Budget.GetExitCode() == 0 {
		return fmt.Sprintf("budget threshold exceeded (%.0f%%) - warning only, exit code 0", highest)
	}
	return fmt.Sprintf("budget threshold exceeded (%.0f%%) - exiting with code %d",
		highest, s.Budget.GetExitCode())
}
