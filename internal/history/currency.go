package history

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// CurrencyChoice selects how SelectCurrency handles mixed snapshot currencies.
type CurrencyChoice struct {
	// Currency keeps snapshots in this code. Empty keeps the dominant currency.
	Currency string
	// Strict fails instead of dropping the other currencies.
	Strict bool
}

// CurrencyResult is the snapshots SelectCurrency kept and any warning for the caller to print.
type CurrencyResult struct {
	Snapshots []CostSnapshot
	Warning   string
	Currency  string
}

type currencyCount struct {
	code   string
	label  string
	count  int
	latest time.Time
}

// SelectCurrency keeps one currency from snapshots.
// An explicit choice filters without a warning. The default keeps the currency with the
// most snapshots and warns. Strict returns ErrMixedCurrencies instead of dropping rows.
func SelectCurrency(snapshots []CostSnapshot, choice CurrencyChoice) (CurrencyResult, error) {
	if strings.TrimSpace(choice.Currency) != "" {
		code := normalizeCurrency(choice.Currency)
		return CurrencyResult{
			Snapshots: filterCurrency(snapshots, code),
			Currency:  currencyLabel(code),
		}, nil
	}
	counts := tallyCurrencies(snapshots)
	if len(counts) <= 1 {
		return singleCurrency(snapshots, counts), nil
	}
	if choice.Strict {
		return CurrencyResult{}, mixedCurrencyError(counts)
	}
	chosen := dominantCurrency(counts)
	return CurrencyResult{
		Snapshots: filterCurrency(snapshots, chosen.code),
		Warning:   mixedCurrencyWarning(counts, chosen),
		Currency:  chosen.label,
	}, nil
}

func singleCurrency(snapshots []CostSnapshot, counts []currencyCount) CurrencyResult {
	currency := ""
	if len(counts) == 1 {
		currency = counts[0].label
	}
	return CurrencyResult{Snapshots: snapshots, Currency: currency}
}

func normalizeCurrency(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func currencyLabel(code string) string {
	if code == "" {
		return "(unset)"
	}
	return code
}

func tallyCurrencies(snapshots []CostSnapshot) []currencyCount {
	index := make(map[string]int, len(snapshots))
	counts := make([]currencyCount, 0)
	for _, snapshot := range snapshots {
		code := normalizeCurrency(snapshot.Currency)
		pos, ok := index[code]
		if !ok {
			pos = len(counts)
			index[code] = pos
			counts = append(counts, currencyCount{code: code, label: currencyLabel(code)})
		}
		counts[pos].count++
		if snapshot.Timestamp.After(counts[pos].latest) {
			counts[pos].latest = snapshot.Timestamp
		}
	}
	return counts
}

func filterCurrency(snapshots []CostSnapshot, code string) []CostSnapshot {
	kept := make([]CostSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if normalizeCurrency(snapshot.Currency) == code {
			kept = append(kept, snapshot)
		}
	}
	return kept
}

func dominantCurrency(counts []currencyCount) currencyCount {
	best := counts[0]
	for _, item := range counts[1:] {
		if beatsCurrency(item, best) {
			best = item
		}
	}
	return best
}

func beatsCurrency(item, best currencyCount) bool {
	if item.count != best.count {
		return item.count > best.count
	}
	if !item.latest.Equal(best.latest) {
		return item.latest.After(best.latest)
	}
	return item.code < best.code
}

type strictCurrencyError struct {
	labels string
}

func (e *strictCurrencyError) Error() string {
	return "Mixed currencies detected across snapshots (" + e.labels + ").\n" +
		"Use --currency to filter or --no-strict to show dominant currency"
}

func (e *strictCurrencyError) Unwrap() error {
	return ErrMixedCurrencies
}

func mixedCurrencyError(counts []currencyCount) error {
	return &strictCurrencyError{labels: strings.Join(currencyLabels(counts), ", ")}
}

func mixedCurrencyWarning(counts []currencyCount, chosen currencyCount) string {
	ordered := orderByDominance(counts)
	parts := make([]string, len(ordered))
	others := make([]string, 0, len(ordered))
	for i, item := range ordered {
		parts[i] = item.label + ": " + snapshotCount(item.count)
		if item.code != chosen.code {
			others = append(others, item.label)
		}
	}
	head := fmt.Sprintf(
		"Warning: Mixed currencies detected (%s).",
		strings.Join(parts, ", "),
	)
	hint := fmt.Sprintf(
		"Showing %s snapshots only. Use --currency %s to view %s.",
		chosen.label,
		joinChoices(others),
		viewTarget(others),
	)
	return head + "\n" + hint
}

func snapshotCount(n int) string {
	noun := "snapshots"
	if n == 1 {
		noun = "snapshot"
	}
	return strconv.Itoa(n) + " " + noun
}

func viewTarget(others []string) string {
	if len(others) == 1 {
		return others[0] + " snapshots"
	}
	return "those snapshots"
}

func joinChoices(labels []string) string {
	last := len(labels) - 1
	if last <= 0 {
		if len(labels) == 0 {
			return ""
		}
		return labels[0]
	}
	if last == 1 {
		return labels[0] + " or " + labels[1]
	}
	return strings.Join(labels[:last], ", ") + ", or " + labels[last]
}

func currencyLabels(counts []currencyCount) []string {
	ordered := orderByDominance(counts)
	labels := make([]string, len(ordered))
	for i, item := range ordered {
		labels[i] = item.label
	}
	return labels
}

func orderByDominance(counts []currencyCount) []currencyCount {
	ordered := append([]currencyCount(nil), counts...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].count != ordered[j].count {
			return ordered[i].count > ordered[j].count
		}
		return ordered[i].label < ordered[j].label
	})
	return ordered
}
