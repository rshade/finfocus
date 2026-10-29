package forecast

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Cadence is the spacing of spend history rows.
type Cadence int

const (
	// CadenceDaily is one row per consecutive day.
	CadenceDaily Cadence = iota
	// CadenceMonthly is one row per consecutive calendar month.
	CadenceMonthly
)

const (
	spendDateLayout = "2006-01-02"
	spendColumns    = 2
	// MaxSpendRows is ten years of daily spend. Fit time grows about linearly
	// (measured: 10 years in ~0.13s, 20 years in ~0.27s), and monthly
	// forecasts gain nothing from older data.
	MaxSpendRows = 3660
	// utf8BOM is the byte order mark some spreadsheet exports write first.
	utf8BOM = "\uFEFF"
)

// SpendHistory is past spend at one cadence, sorted by time. Daily points are
// at 00:00 UTC; monthly points are at the first of their UTC month.
type SpendHistory struct {
	Cadence Cadence
	Points  []Point
}

type spendRow struct {
	point Point
	line  int
}

// ParseSpendCSV reads `date,value` rows. A first row whose value is not a
// number is a header. Dates are 2006-01-02 or RFC3339 (kept as their UTC day).
// Every error is an InputError that names the CSV line.
func ParseSpendCSV(r io.Reader) (SpendHistory, error) {
	reader := csv.NewReader(bufio.NewReader(&bomSkipper{r: r}))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	var rows []spendRow
	seen := map[time.Time]bool{}
	for first := true; ; first = false {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if parseErr, ok := errors.AsType[*csv.ParseError](err); ok {
				return SpendHistory{}, spendError(parseErr.Line, "%v", parseErr.Err)
			}
			return SpendHistory{}, inputError(fmt.Errorf("reading spend history: %w", err))
		}
		line, _ := reader.FieldPos(0)
		if len(record) != spendColumns {
			return SpendHistory{}, spendError(line, "expected 2 columns (date,value), got %d", len(record))
		}
		rawValue := strings.TrimSpace(record[1])
		if first && isSpendHeader(strings.TrimSpace(record[0]), rawValue) {
			continue
		}
		row, err := parseSpendRow(strings.TrimSpace(record[0]), rawValue, line)
		if err != nil {
			return SpendHistory{}, err
		}
		if seen[row.point.Time] {
			return SpendHistory{}, spendError(line, "duplicate date %s", row.point.Time.Format(spendDateLayout))
		}
		if len(rows) == MaxSpendRows {
			return SpendHistory{}, spendError(line,
				"spend history has more than %d rows; keep at most ten years of daily spend", MaxSpendRows)
		}
		seen[row.point.Time] = true
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return SpendHistory{}, inputError(errors.New("spend history has no rows"))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].point.Time.Before(rows[j].point.Time) })
	return spendHistoryFrom(rows)
}

func parseSpendRow(rawDate, rawValue string, line int) (spendRow, error) {
	when, err := parseSpendDate(rawDate)
	if err != nil {
		return spendRow{}, spendError(line, "invalid date %q (use 2006-01-02 or RFC3339)", rawDate)
	}
	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return spendRow{}, spendError(line, "invalid value %q", rawValue)
	}
	if value < 0 {
		return spendRow{}, spendError(line, "value must not be negative")
	}
	return spendRow{point: Point{Time: when, Value: value}, line: line}, nil
}

func parseSpendDate(raw string) (time.Time, error) {
	if when, err := time.Parse(spendDateLayout, raw); err == nil {
		return when, nil
	}
	when, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	when = when.UTC()
	return time.Date(when.Year(), when.Month(), when.Day(), 0, 0, 0, 0, time.UTC), nil
}

// spendHistoryFrom takes the cadence from the first two rows and requires the
// rest to keep it.
func spendHistoryFrom(rows []spendRow) (SpendHistory, error) {
	cadence := CadenceDaily
	if len(rows) > 1 {
		a, b := rows[0].point.Time, rows[1].point.Time
		switch {
		case b.Equal(a.AddDate(0, 0, 1)):
		case monthIndex(b)-monthIndex(a) == 1:
			cadence = CadenceMonthly
		default:
			return SpendHistory{}, gapError(rows[1], a, "consecutive days or months")
		}
	}
	points := make([]Point, len(rows))
	for i, row := range rows {
		if i > 0 {
			prev := rows[i-1].point.Time
			if cadence == CadenceDaily && !row.point.Time.Equal(prev.AddDate(0, 0, 1)) {
				return SpendHistory{}, gapError(row, prev, "consecutive days")
			}
			if cadence == CadenceMonthly && monthIndex(row.point.Time)-monthIndex(prev) != 1 {
				return SpendHistory{}, gapError(row, prev, "consecutive months")
			}
		}
		points[i] = row.point
		if cadence == CadenceMonthly {
			points[i].Time = monthStart(row.point.Time)
		}
	}
	return SpendHistory{Cadence: cadence, Points: points}, nil
}

func gapError(row spendRow, prev time.Time, want string) error {
	return spendError(row.line, "%s does not follow %s; rows must be %s",
		row.point.Time.Format(spendDateLayout), prev.Format(spendDateLayout), want)
}

func spendError(line int, format string, args ...any) error {
	return inputError(fmt.Errorf("spend history line %d: "+format, append([]any{line}, args...)...))
}

func monthIndex(t time.Time) int { return t.Year()*12 + int(t.Month()) - 1 }

// isSpendHeader reports whether the first row names columns rather than
// holding data: neither its date nor its value parses.
func isSpendHeader(rawDate, rawValue string) bool {
	_, dateErr := parseSpendDate(rawDate)
	_, valueErr := strconv.ParseFloat(rawValue, 64)
	return dateErr != nil && valueErr != nil
}

// bomSkipper drops a leading UTF-8 byte order mark, which spreadsheet exports
// often write.
type bomSkipper struct {
	r       io.Reader
	checked bool
}

func (b *bomSkipper) Read(p []byte) (int, error) {
	if b.checked {
		return b.r.Read(p)
	}
	b.checked = true
	buf := make([]byte, len(utf8BOM))
	n, err := io.ReadFull(b.r, buf)
	if string(buf[:n]) == utf8BOM {
		n = 0
	}
	b.r = io.MultiReader(bytes.NewReader(buf[:n]), b.r)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return 0, err
	}
	return b.r.Read(p)
}
