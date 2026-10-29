package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/forecast"
	"github.com/rshade/finfocus/internal/history"
)

// forecastJSON is the `cost forecast --output json` document.
type forecastJSON = forecast.Document

// forecastSettings are the validated, presentation-free forecast inputs that
// `cost forecast` takes from flags and the web dashboard takes from a query.
type forecastSettings struct {
	months     int
	growthType string
	rate       *float64
	noHistory  bool
	noBudget   bool
	now        time.Time
	cfg        *config.Config
	model      string
	spend      *forecast.SpendHistory
	confidence float64
	drivers    forecast.Drivers
}

// forecastModelInputs are the model and driver settings as text, the way the
// flags and the web query both carry them. spendCSV is nil when no spend
// history was given.
type forecastModelInputs struct {
	model      string
	spendCSV   io.Reader
	confidence *float64
	growthCap  *float64
	steps      []string
	excludes   []string
}

// parseForecastModel validates the model and drivers into settings. Every
// error is a forecast.InputError, raised before any plugin starts.
func parseForecastModel(in forecastModelInputs, settings *forecastSettings) error {
	model := strings.ToLower(strings.TrimSpace(in.model))
	if model == "" {
		model = forecast.ModelGrowth
	}
	if err := checkForecastModelInputs(model, in); err != nil {
		return err
	}
	settings.model = model
	settings.confidence = forecast.DefaultConfidence
	if in.confidence != nil {
		if err := forecast.ValidateConfidence(*in.confidence); err != nil {
			return err
		}
		settings.confidence = *in.confidence
	}
	drivers, err := parseForecastDrivers(in)
	if err != nil {
		return err
	}
	settings.drivers = drivers
	if model == forecast.ModelARIMA {
		spend, spendErr := forecast.ParseSpendCSV(in.spendCSV)
		if spendErr != nil {
			return spendErr
		}
		if spendErr = forecast.ValidateSpendHistory(spend, drivers); spendErr != nil {
			return spendErr
		}
		settings.spend = &spend
	}
	return nil
}

// checkForecastModelInputs rejects an unknown model and inputs that belong to
// the other model.
func checkForecastModelInputs(model string, in forecastModelInputs) error {
	statistical := model == forecast.ModelARIMA
	var err error
	switch {
	case model != forecast.ModelGrowth && !statistical:
		err = fmt.Errorf("invalid model %q (supported: growth, arima)", in.model)
	case statistical && in.spendCSV == nil:
		err = errors.New("--model arima requires --spend-history")
	case !statistical && in.spendCSV != nil:
		err = errors.New("--spend-history requires --model arima")
	case !statistical && in.confidence != nil:
		err = errors.New("--confidence requires --model arima")
	case !statistical && len(in.excludes) > 0:
		err = errors.New("--exclude requires --model arima")
	}
	if err != nil {
		return &forecast.InputError{Err: err}
	}
	return nil
}

func parseForecastDrivers(in forecastModelInputs) (forecast.Drivers, error) {
	var drivers forecast.Drivers
	if in.growthCap != nil {
		if err := forecast.ValidateGrowthCap(*in.growthCap); err != nil {
			return forecast.Drivers{}, err
		}
		drivers.GrowthCap = *in.growthCap
	}
	for _, raw := range in.steps {
		step, err := forecast.ParseStep(raw)
		if err != nil {
			return forecast.Drivers{}, err
		}
		drivers.Steps = append(drivers.Steps, step)
	}
	for _, raw := range in.excludes {
		excluded, err := forecast.ParseExclude(raw)
		if err != nil {
			return forecast.Drivers{}, err
		}
		drivers.Excludes = append(drivers.Excludes, excluded)
	}
	return drivers, nil
}

// forecastBuild is a projection with the history and budget beside it.
type forecastBuild struct {
	projection forecast.Projection
	history    forecast.Series
	budget     float64
}

func (b forecastBuild) document() forecast.Document {
	return forecast.NewDocument(b.projection, forecast.SeriesList(b.projection, b.history), b.budget)
}

// buildForecast projects priced costs and attaches history and the budget.
// The CLI and the web dashboard both call it after pricing, so they agree on
// every value and warning. readHistory is not called when history is off.
func buildForecast(
	costs []engine.CostResult,
	settings forecastSettings,
	readHistory func() ([]history.CostSnapshot, error),
) (forecastBuild, error) {
	resources, warnings := forecastResources(costs, settings.growthType, settings.rate)
	if settings.growthType == forecast.GrowthNone && settings.rate != nil {
		warnings = append(warnings, "growth rate ignored for growth type none")
	}
	now := settings.now
	if now.IsZero() {
		now = time.Now()
	}
	var projection forecast.Projection
	var err error
	if settings.model == forecast.ModelARIMA {
		var notes []string
		projection, notes, err = statisticalForecast(resources, settings, now)
		warnings = append(warnings, notes...)
	} else {
		projection, err = forecast.Project(resources, settings.months, now)
		warnings = append(warnings, projection.Warnings...)
	}
	if err != nil {
		return forecastBuild{}, err
	}
	warnings = append(warnings, settings.drivers.Apply(&projection)...)
	var historySeries forecast.Series
	if !settings.noHistory && readHistory != nil {
		var note string
		historySeries, note = forecastHistoryFrom(readHistory, projection.Currency)
		if note != "" {
			warnings = append(warnings, note)
		}
	}
	budget, budgetWarning := forecastBudgetFor(settings.cfg, projection.Currency, settings.noBudget)
	if budgetWarning != "" {
		warnings = append(warnings, budgetWarning)
	}
	sort.Strings(warnings)
	projection.Warnings = warnings
	return forecastBuild{projection: projection, history: historySeries, budget: budget}, nil
}

// statisticalForecast fits the spend history and attaches the plan's growth
// projection as the baseline. A plan with nothing to project leaves the
// baseline out with a warning; the forecast does not need it.
func statisticalForecast(
	resources []forecast.Resource,
	settings forecastSettings,
	now time.Time,
) (forecast.Projection, []string, error) {
	var notes []string
	baseline, err := forecast.Project(resources, settings.months, now)
	switch {
	case err == nil:
		notes = append(notes, baseline.Warnings...)
	case forecast.IsUnprojectable(err):
		notes = append(notes, "baseline: "+err.Error())
	default:
		return forecast.Projection{}, nil, err
	}
	projection, err := forecast.ProjectStatistical(
		*settings.spend, settings.drivers, settings.months, now, settings.confidence, baseline.Currency)
	if err != nil {
		return forecast.Projection{}, nil, err
	}
	notes = append(notes, projection.Warnings...)
	if len(baseline.Forecast.Points) > 0 {
		projection.Baseline = forecast.Series{Name: forecast.SeriesBaseline, Points: baseline.Forecast.Points}
	}
	return projection, notes, nil
}

// forecastHistoryFrom turns stored snapshots into the history series. A read
// error or a currency mismatch omits the series and returns a warning.
func forecastHistoryFrom(
	readHistory func() ([]history.CostSnapshot, error),
	currency string,
) (forecast.Series, string) {
	snapshots, err := readHistory()
	if err != nil {
		return forecast.Series{}, "history: " + err.Error()
	}
	if len(snapshots) == 0 {
		return forecast.Series{}, ""
	}
	seen := map[string]struct{}{}
	for _, snapshot := range snapshots {
		code := strings.ToUpper(strings.TrimSpace(snapshot.Currency))
		if code == "" {
			code = "(unset)"
		}
		seen[code] = struct{}{}
	}
	if len(seen) != 1 {
		return forecast.Series{}, "history contains more than one currency"
	}
	for code := range seen {
		if code != strings.ToUpper(currency) {
			return forecast.Series{}, fmt.Sprintf(
				"history currency %s does not match forecast currency %s",
				code,
				currency,
			)
		}
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Timestamp.Before(snapshots[j].Timestamp)
	})
	points := make([]forecast.Point, len(snapshots))
	for i, snapshot := range snapshots {
		points[i] = forecast.Point{Time: snapshot.Timestamp.UTC(), Value: snapshot.TotalMonthly}
	}
	return forecast.Series{Name: forecast.SeriesHistory, Points: points}, ""
}

// readForecastHistory reads every cost-history snapshot for stack. An empty
// stack or a missing database is no history, not an error.
func readForecastHistory(stack string) ([]history.CostSnapshot, error) {
	if stack == "" {
		return nil, nil
	}
	path, err := historyPath("", stack)
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, nil
		}
		return nil, statErr
	}
	db, err := history.OpenCostDBRead(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	return db.Snapshots(time.Time{}, time.Time{})
}

// forecastBudgetFor returns the global budget for the forecast currency. A
// budget in another currency is omitted with a warning.
func forecastBudgetFor(cfg *config.Config, forecastCurrency string, noBudget bool) (float64, string) {
	if noBudget {
		return 0, ""
	}
	if cfg == nil {
		cfg = config.GetGlobalConfig()
	}
	if cfg == nil || cfg.Cost.Budgets == nil || !cfg.Cost.Budgets.HasGlobalBudget() {
		return 0, ""
	}
	amount := cfg.Cost.Budgets.Global.Amount
	budgetCurrency := strings.ToUpper(strings.TrimSpace(cfg.Cost.Budgets.Global.Currency))
	if budgetCurrency == "" {
		budgetCurrency = defaultCurrency
	}
	spend := strings.ToUpper(strings.TrimSpace(forecastCurrency))
	if spend == "" {
		spend = defaultCurrency
	}
	if budgetCurrency != spend {
		return 0, fmt.Sprintf(
			"budget currency %s does not match forecast currency %s",
			budgetCurrency, spend,
		)
	}
	return amount, ""
}
