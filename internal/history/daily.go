package history

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	dailyDBSuffix      = ".daily.db"
	dailySchemaVersion = uint64(1)
	metaKeyDailySchema = "daily_schema"
	dailyBucketPoints  = "daily_points"
	dailyHoursPerDay   = 24
	dailyKeyParts      = 3
	blankDailyCurrency = "USD"
)

// ErrDailyFileAsTimeline is returned when a projected-timeline open is pointed
// at a daily actuals file. The timeline schema is left untouched.
var ErrDailyFileAsTimeline = errors.New(
	"refusing to open a daily actuals file as a projected cost timeline",
)

// ErrTimelineAsDaily is returned when a daily actuals open is pointed at a
// projected cost timeline.
var ErrTimelineAsDaily = errors.New(
	"refusing to open a projected cost timeline as daily actuals",
)

// DailyPoint is one resource's actual cost for one UTC day and one currency.
type DailyPoint struct {
	ResourceID string
	Day        time.Time
	Currency   string
	Amount     float64
	WrittenAt  time.Time
}

// DailyFileName maps a stack name to `<sanitized>.daily.db`.
// Slashes become dashes. Names containing `..` are rejected.
func DailyFileName(stack string) (string, error) {
	name, err := CostFileName(stack)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(name, costDBSuffix) + dailyDBSuffix, nil
}

// DailyFileNameFor names the daily actuals file for one stack of one project.
// With no project it returns the legacy `<stack>.daily.db` name.
func DailyFileNameFor(project, stack string) (string, error) {
	if project == "" {
		return DailyFileName(stack)
	}
	name, err := CostFileNameFor(project, stack)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(name, costDBSuffix) + dailyDBSuffix, nil
}

// IsDailyActualsFile reports whether name is a per-stack daily actuals file.
func IsDailyActualsFile(name string) bool {
	return strings.HasSuffix(name, dailyDBSuffix) && name != dailyDBSuffix
}

// ObservedCost is one plugin-reported bucket before it is folded onto a UTC day.
type ObservedCost struct {
	At       time.Time
	Amount   float64
	Currency string
}

// PointsFromObservations folds timestamped buckets into one point per UTC day
// and currency. Buckets on the same day and currency are summed. A bucket with
// no timestamp is ignored. An empty result is a skip: the caller must not
// invent a zero for a missing day. A non-finite amount rejects the resource.
func PointsFromObservations(resourceID string, observed []ObservedCost, written time.Time) ([]DailyPoint, error) {
	if err := validateDailyIdentity(resourceID, ""); err != nil {
		return nil, err
	}
	type dayKey struct {
		day      string
		currency string
	}
	sums := make(map[dayKey]float64)
	order := make([]dayKey, 0, len(observed))
	for _, item := range observed {
		if item.At.IsZero() {
			continue
		}
		if math.IsNaN(item.Amount) || math.IsInf(item.Amount, 0) {
			return nil, fmt.Errorf("resource %s: daily cost is not a finite amount", resourceID)
		}
		if strings.Contains(item.Currency, "\x00") {
			return nil, errors.New("daily point currency contains NUL")
		}
		at := item.At.UTC()
		key := dayKey{
			day:      time.Date(at.Year(), at.Month(), at.Day(), 0, 0, 0, 0, time.UTC).Format(time.DateOnly),
			currency: normalizeDailyCurrency(item.Currency),
		}
		if _, ok := sums[key]; !ok {
			order = append(order, key)
		}
		sums[key] += item.Amount
	}
	if len(order) == 0 {
		return nil, nil
	}
	points := make([]DailyPoint, 0, len(order))
	for _, key := range order {
		day, err := time.Parse(time.DateOnly, key.day)
		if err != nil {
			return nil, fmt.Errorf("parsing daily point day: %w", err)
		}
		points = append(points, DailyPoint{
			ResourceID: resourceID,
			Day:        day.UTC(),
			Currency:   key.currency,
			Amount:     sums[key],
			WrittenAt:  written.UTC(),
		})
	}
	return points, nil
}

// PointsForSeries maps one daily series into points.
// An empty series returns no points and no error: the caller treats that as
// "the plugin sent nothing" and must not invent zeros.
// Index 0 is the UTC day of start. The length must equal the whole-day count
// from start up to end, the same count windowDailyCosts uses. A mismatch
// returns an error and no points.
func PointsForSeries(
	resourceID, currency string,
	start, end time.Time,
	daily []float64,
	written time.Time,
) ([]DailyPoint, error) {
	if len(daily) == 0 {
		return nil, nil
	}
	if err := validateDailyIdentity(resourceID, currency); err != nil {
		return nil, err
	}
	days, err := dailySlotCount(start, end)
	if err != nil {
		return nil, fmt.Errorf("resource %s: %w", resourceID, err)
	}
	if len(daily) != days {
		return nil, fmt.Errorf(
			"resource %s: daily costs have %d points for a %d-day window",
			resourceID, len(daily), days,
		)
	}
	currency = normalizeDailyCurrency(currency)
	points := make([]DailyPoint, 0, len(daily))
	for i, amount := range daily {
		if math.IsNaN(amount) || math.IsInf(amount, 0) {
			return nil, fmt.Errorf("resource %s: daily cost %d is not a finite amount", resourceID, i)
		}
		points = append(points, DailyPoint{
			ResourceID: resourceID,
			Day:        dailyDay(start, i),
			Currency:   currency,
			Amount:     amount,
			WrittenAt:  written.UTC(),
		})
	}
	return points, nil
}

func validateDailyIdentity(resourceID, currency string) error {
	if resourceID == "" || strings.Contains(resourceID, "\x00") {
		return errors.New("daily point resource id is empty or contains NUL")
	}
	if strings.Contains(currency, "\x00") {
		return errors.New("daily point currency contains NUL")
	}
	return nil
}

func normalizeDailyCurrency(currency string) string {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return blankDailyCurrency
	}
	return currency
}

// dailySlotCount is the number of whole UTC days in [start, end).
// An end that falls on a midnight is exclusive, matching windowDailyCosts.
func dailySlotCount(start, end time.Time) (int, error) {
	if start.IsZero() || end.IsZero() {
		return 0, errors.New("daily costs are missing a start or end date")
	}
	if !end.After(start) {
		return 0, errors.New("daily cost end is not after the start")
	}
	days := int(end.UTC().Sub(start.UTC()).Hours() / dailyHoursPerDay)
	if days <= 0 {
		return 0, errors.New("daily cost window is shorter than one day")
	}
	return days, nil
}

func dailyDay(start time.Time, index int) time.Time {
	start = start.UTC()
	midnight := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	return midnight.AddDate(0, 0, index)
}

func dailyPointKey(point DailyPoint) ([]byte, error) {
	if err := validateDailyIdentity(point.ResourceID, point.Currency); err != nil {
		return nil, err
	}
	day := point.Day.UTC()
	if day.IsZero() {
		return nil, errors.New("daily point day is required")
	}
	encoded := point.ResourceID + "\x00" + day.Format(time.DateOnly) + "\x00" + point.Currency
	return []byte(encoded), nil
}
