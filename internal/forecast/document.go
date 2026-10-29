package forecast

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Document is the forecast as `cost forecast --output json` prints it and as
// the web dashboard receives it.
type Document struct {
	Currency   string   `json:"currency"`
	Months     int      `json:"months"`
	Model      string   `json:"model,omitempty"`
	ModelOrder string   `json:"modelOrder,omitempty"`
	Confidence *float64 `json:"confidence,omitempty"`
	Budget     *float64 `json:"budget,omitempty"`
	Series     []Series `json:"series"`
	Warnings   []string `json:"warnings,omitempty"`
}

// InputError marks a forecast setting the caller got wrong. Its message is the
// wrapped error's, so the CLI and the web report the same text.
type InputError struct {
	Err error
}

func (e *InputError) Error() string { return e.Err.Error() }

func (e *InputError) Unwrap() error { return e.Err }

// IsInputError reports whether err is a forecast setting error.
func IsInputError(err error) bool {
	var input *InputError
	return errors.As(err, &input)
}

func inputError(err error) error { return &InputError{Err: err} }

// IsUnprojectable reports whether err means the priced resources cannot be
// projected: none was priced, or they use more than one currency. The CLI
// fails with the same error; it is not a plugin outage.
func IsUnprojectable(err error) bool {
	return errors.Is(err, ErrNoCostData) || errors.Is(err, ErrMixedCurrencies)
}

// ValidateMonths rejects a horizon outside MinMonths..MaxMonths.
func ValidateMonths(months int) error {
	if months < MinMonths || months > MaxMonths {
		return inputError(ErrInvalidMonths)
	}
	return nil
}

// ParseGrowth validates a growth override and its rate, given as the text of
// the --growth-type and --growth-rate flags. An empty kind keeps each
// plugin's growth model.
func ParseGrowth(kind, rawRate string) (string, *float64, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	switch kind {
	case "", GrowthNone, GrowthLinear, GrowthExponential:
	default:
		return "", nil, inputError(fmt.Errorf(
			"invalid growth type %q (supported: none, linear, exponential)",
			kind,
		))
	}
	rawRate = strings.TrimSpace(rawRate)
	var rate *float64
	if rawRate != "" {
		parsed, err := strconv.ParseFloat(rawRate, 64)
		if err != nil {
			return "", nil, inputError(fmt.Errorf("invalid growth rate %q", rawRate))
		}
		if parsed < -1 {
			return "", nil, inputError(ErrInvalidGrowthRate)
		}
		rate = &parsed
	}
	if (kind == GrowthLinear || kind == GrowthExponential) && rate == nil {
		return "", nil, inputError(ErrMissingGrowthRate)
	}
	return kind, rate, nil
}

// SeriesList orders the document series: history when present, the total
// forecast, the statistical bounds, baseline, and spend when present, then
// one series per provider.
func SeriesList(projection Projection, history Series) []Series {
	optional := []Series{projection.Lower, projection.Upper, projection.Baseline, projection.Spend}
	series := make([]Series, 0, 2+len(optional)+len(projection.ByProvider))
	if len(history.Points) > 0 {
		series = append(series, history)
	}
	series = append(series, projection.Forecast)
	for _, extra := range optional {
		if len(extra.Points) > 0 {
			series = append(series, extra)
		}
	}
	series = append(series, projection.ByProvider...)
	return series
}

// NewDocument builds the forecast document. A budget of zero or less is
// omitted.
func NewDocument(projection Projection, series []Series, budget float64) Document {
	doc := Document{
		Currency:   projection.Currency,
		Months:     projection.Months,
		Model:      projection.Model,
		ModelOrder: projection.ModelOrder,
		Series:     series,
		Warnings:   projection.Warnings,
	}
	if projection.Confidence > 0 {
		confidence := projection.Confidence
		doc.Confidence = &confidence
	}
	if budget > 0 {
		doc.Budget = &budget
	}
	return doc
}

const percent = 100

// ModelLabel names a statistical model and its bound level, the same in the
// CLI and the dashboard: "SARIMA(1,1,1)(0,1,1)[7], bounds at 95% confidence".
func ModelLabel(order string, confidence float64) string {
	return fmt.Sprintf("%s, bounds at %g%% confidence", order, confidence*percent)
}
