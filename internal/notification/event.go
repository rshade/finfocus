package notification

import (
	"time"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/pkg/version"
)

// Budget scopes named in events.
const (
	ScopeGlobal   = "global"
	ScopeProvider = "provider"
	ScopeTag      = "tag"
	ScopeType     = "type"
)

const percentFull = 100

// BudgetAlertInput holds the evaluated facts of one exceeded threshold.
type BudgetAlertInput struct {
	// Scope is global, provider, tag, or type. Empty means global (legacy budgets).
	Scope string
	// ScopeKey is the provider name, tag selector, or type pattern; empty for global.
	ScopeKey string
	// Amount, Currency, and Period describe the budget. Empty Period means monthly.
	Amount   float64
	Currency string
	Period   string
	// Threshold and AlertType describe the exceeded alert.
	Threshold float64
	AlertType config.AlertType
	// CurrentSpend and CurrentPercent are the evaluated spend.
	CurrentSpend   float64
	CurrentPercent float64
	// ForecastedSpend and ForecastPercent are the end-of-period forecast.
	ForecastedSpend float64
	ForecastPercent float64
	// Time is the evaluation time. Zero means now.
	Time time.Time
}

// BudgetAlertEvent is the message sent to every destination of one exceeded threshold.
type BudgetAlertEvent struct {
	Scope            string
	ScopeKey         string
	Amount           float64
	Currency         string
	Period           string
	ThresholdPercent float64
	AlertType        config.AlertType
	// ThresholdValue is Amount × ThresholdPercent / 100.
	ThresholdValue float64
	// Spend and SpendPercent are the forecast for a forecasted alert, otherwise current spend.
	Spend        float64
	SpendPercent float64
	Timestamp    time.Time
	Version      string
}

// NewBudgetAlertEvent builds the event for one exceeded threshold. A forecasted
// alert reports the forecasted spend and percentage; an actual alert reports
// current spend. The timestamp is converted to UTC.
func NewBudgetAlertEvent(in BudgetAlertInput) BudgetAlertEvent {
	scope := in.Scope
	if scope == "" {
		scope = ScopeGlobal
	}
	period := in.Period
	if period == "" {
		period = config.DefaultBudgetPeriod
	}
	when := in.Time
	if when.IsZero() {
		when = time.Now()
	}
	spend, percent := in.CurrentSpend, in.CurrentPercent
	if in.AlertType == config.AlertTypeForecasted {
		spend, percent = in.ForecastedSpend, in.ForecastPercent
	}
	return BudgetAlertEvent{
		Scope:            scope,
		ScopeKey:         in.ScopeKey,
		Amount:           in.Amount,
		Currency:         in.Currency,
		Period:           period,
		ThresholdPercent: in.Threshold,
		AlertType:        in.AlertType,
		ThresholdValue:   in.Amount * in.Threshold / percentFull,
		Spend:            spend,
		SpendPercent:     percent,
		Timestamp:        when.UTC(),
		Version:          version.GetVersion(),
	}
}

// BudgetName is "global" for the global budget, otherwise "<scope>:<scope_key>".
func (e BudgetAlertEvent) BudgetName() string {
	if e.Scope == ScopeGlobal || e.ScopeKey == "" {
		return e.Scope
	}
	return e.Scope + ":" + e.ScopeKey
}
