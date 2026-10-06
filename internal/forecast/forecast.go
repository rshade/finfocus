// Package forecast projects monthly cost from a plugin growth model.
// The series is timestamped so a later interactive chart can plot it
// without recomputing growth. The math is finfocus-spec's pricing.ApplyGrowth.
package forecast

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/rshade/finfocus-spec/sdk/go/pricing"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/resourcetype"
)

const (
	// MinMonths is the shortest forecast, one month ahead of the current month.
	MinMonths = 1
	// MaxMonths is the longest forecast. It matches the SDK's long-exponential
	// warning boundary, so a longer range is refused instead of warned.
	MaxMonths = 36

	// SeriesForecast is the total projected series.
	SeriesForecast = "forecast"
	// SeriesHistory is stored cost-history snapshots, when the command has them.
	SeriesHistory = "history"
	// LineWarning is the NDJSON kind for a warning line.
	LineWarning = "warning"

	// GrowthNone keeps the current monthly cost.
	GrowthNone = "none"
	// GrowthLinear applies base * (1 + rate * month).
	GrowthLinear = "linear"
	// GrowthExponential applies base * (1 + rate) ^ month.
	GrowthExponential = "exponential"
	// DefaultMonths is the horizon when --months is omitted.
	DefaultMonths = 12

	defaultCurrency = "USD"
	emptyProvider   = "<none>"
)

var (
	// ErrInvalidMonths is returned when the horizon is outside MinMonths..MaxMonths.
	ErrInvalidMonths = errors.New("forecast months must be from 1 to 36")
	// ErrNoCostData is returned when every resource was excluded.
	ErrNoCostData = errors.New("no cost data available")
	// ErrMixedCurrencies is returned when priced resources use two currencies.
	ErrMixedCurrencies = errors.New("forecast resources use more than one currency")
	// ErrMissingGrowthRate is returned when the caller set linear or exponential
	// growth and did not supply a rate.
	ErrMissingGrowthRate = errors.New("growth rate is required for linear or exponential growth")
	// ErrInvalidGrowthRate is returned when the rate is below -1.
	ErrInvalidGrowthRate = errors.New("growth rate must be >= -1")
)

// Resource is one priced resource to project.
type Resource struct {
	ID         string
	Provider   string
	Monthly    float64
	Currency   string
	GrowthType string
	GrowthRate *float64
}

// Point is the cost at one UTC month start.
type Point struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

// Series is a named list of points. Names are forecast, history, or a provider.
type Series struct {
	Name   string  `json:"name"`
	Points []Point `json:"points"`
}

// Projection is the forecast for a set of resources.
type Projection struct {
	Currency   string   `json:"currency"`
	Months     int      `json:"months"`
	Forecast   Series   `json:"forecast"`
	ByProvider []Series `json:"byProvider,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
}

// Project sums each resource's growth from start through months ahead.
// start is normalized to 00:00:00Z on the first day of its UTC month.
// Month 0 is that month. The returned series has months+1 points.
func Project(resources []Resource, months int, start time.Time) (Projection, error) {
	if months < MinMonths || months > MaxMonths {
		return Projection{}, ErrInvalidMonths
	}
	priced, currency, err := pricedResources(resources)
	if err != nil {
		return Projection{}, err
	}
	if len(priced) == 0 {
		return Projection{}, ErrNoCostData
	}

	origin := monthStart(start)
	points := months + 1
	totals := make([]float64, points)
	byProvider := map[string][]float64{}
	var warnings []string
	for _, resource := range priced {
		values, resourceWarnings := projectResource(resource, points)
		warnings = append(warnings, resourceWarnings...)
		provider := providerName(resource.Provider)
		series := byProvider[provider]
		if series == nil {
			series = make([]float64, points)
		}
		for i, value := range values {
			totals[i] += value
			series[i] += value
		}
		byProvider[provider] = series
	}
	sort.Strings(warnings)

	out := Projection{
		Currency: currency,
		Months:   months,
		Forecast: Series{Name: SeriesForecast, Points: stamp(origin, totals)},
		Warnings: warnings,
	}
	names := make([]string, 0, len(byProvider))
	for name := range byProvider {
		names = append(names, name)
	}
	sort.Strings(names)
	out.ByProvider = make([]Series, 0, len(names))
	for _, name := range names {
		out.ByProvider = append(out.ByProvider, Series{Name: name, Points: stamp(origin, byProvider[name])})
	}
	return out, nil
}

func pricedResources(resources []Resource) ([]Resource, string, error) {
	// A blank currency adopts the single explicit currency, or USD when every
	// resource left it blank. Two explicit currencies do not collapse.
	explicit := map[string]struct{}{}
	for _, resource := range resources {
		if strings.TrimSpace(resource.Currency) == "" {
			continue
		}
		explicit[normalizeCurrency(resource.Currency)] = struct{}{}
	}
	if len(explicit) > 1 {
		return nil, "", fmt.Errorf("%w: %s", ErrMixedCurrencies, joinCurrencies(explicit))
	}
	currency := defaultCurrency
	for code := range explicit {
		currency = code
	}
	priced := make([]Resource, 0, len(resources))
	for _, resource := range resources {
		resource.Currency = currency
		priced = append(priced, resource)
	}
	return priced, currency, nil
}

func projectResource(resource Resource, points int) ([]float64, []string) {
	growth, rate, warnings := effectiveGrowth(resource, points-1)
	values := make([]float64, points)
	clamped := false
	for period := range points {
		raw := pricing.ApplyGrowth(resource.Monthly, growth, rate, period)
		value, ok := finiteNonNegative(raw)
		if !ok {
			clamped = true
		}
		values[period] = value
	}
	if clamped {
		warnings = append(warnings, fmt.Sprintf("%s: projected cost clamped at zero", resourceLabel(resource)))
	}
	return values, warnings
}

func effectiveGrowth(resource Resource, horizon int) (pbc.GrowthType, *float64, []string) {
	growth, known := growthTypeOf(resource.GrowthType)
	label := resourceLabel(resource)
	var warnings []string
	if !known {
		warnings = append(
			warnings,
			fmt.Sprintf("%s: unknown growth type %q; cost stays flat", label, resource.GrowthType),
		)
		growth = pbc.GrowthType_GROWTH_TYPE_NONE
	}
	if !growthNeedsRate(growth) {
		return growth, nil, warnings
	}
	if resource.GrowthRate == nil {
		warnings = append(
			warnings,
			fmt.Sprintf("%s: growth type %s has no rate; cost stays flat", label, growthLabel(growth)),
		)
		return pbc.GrowthType_GROWTH_TYPE_NONE, nil, warnings
	}
	if err := pricing.ValidateGrowthParams(growth, resource.GrowthRate); err != nil {
		warnings = append(warnings, fmt.Sprintf("%s: %s; cost stays flat", label, err.Error()))
		return pbc.GrowthType_GROWTH_TYPE_NONE, nil, warnings
	}
	for _, warning := range pricing.CheckGrowthWarningsWithCost(resource.Monthly, growth, resource.GrowthRate, horizon) {
		warnings = append(warnings, fmt.Sprintf("%s: %s", label, warning.Message))
	}
	return growth, resource.GrowthRate, warnings
}

func growthNeedsRate(growth pbc.GrowthType) bool {
	return growth == pbc.GrowthType_GROWTH_TYPE_LINEAR || growth == pbc.GrowthType_GROWTH_TYPE_EXPONENTIAL
}

func growthTypeOf(label string) (pbc.GrowthType, bool) {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "", "unspecified", GrowthNone:
		return pbc.GrowthType_GROWTH_TYPE_NONE, true
	case GrowthLinear:
		return pbc.GrowthType_GROWTH_TYPE_LINEAR, true
	case GrowthExponential:
		return pbc.GrowthType_GROWTH_TYPE_EXPONENTIAL, true
	default:
		return pbc.GrowthType_GROWTH_TYPE_UNSPECIFIED, false
	}
}

func growthLabel(growth pbc.GrowthType) string {
	switch growth {
	case pbc.GrowthType_GROWTH_TYPE_LINEAR:
		return GrowthLinear
	case pbc.GrowthType_GROWTH_TYPE_EXPONENTIAL:
		return GrowthExponential
	case pbc.GrowthType_GROWTH_TYPE_NONE, pbc.GrowthType_GROWTH_TYPE_UNSPECIFIED:
		return GrowthNone
	default:
		return GrowthNone
	}
}

func finiteNonNegative(value float64) (float64, bool) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0, false
	}
	return value, true
}

func stamp(origin time.Time, values []float64) []Point {
	points := make([]Point, len(values))
	for i, value := range values {
		points[i] = Point{Time: origin.AddDate(0, i, 0), Value: value}
	}
	return points
}

func monthStart(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func providerName(provider string) string {
	name := resourcetype.NormalizeProvider(provider)
	if name == "" {
		return emptyProvider
	}
	return name
}

func normalizeCurrency(currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		return defaultCurrency
	}
	return currency
}

func joinCurrencies(codes map[string]struct{}) string {
	names := make([]string, 0, len(codes))
	for code := range codes {
		names = append(names, code)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func resourceLabel(resource Resource) string {
	if resource.ID != "" {
		return resource.ID
	}
	if resource.Provider != "" {
		return resource.Provider
	}
	return "resource"
}
