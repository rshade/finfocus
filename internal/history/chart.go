package history

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guptarohit/asciigraph"

	"github.com/rshade/finfocus/internal/resourcetype"
)

const (
	defaultChartHeight = 15
	defaultChartWidth  = 80
	wholeMoneyEpsilon  = 0.001
)

// ChartOptions selects the series rendered by RenderChart.
type ChartOptions struct {
	Stack          string
	From           time.Time
	To             time.Time
	Provider       string
	SplitProviders bool
	NoBudget       bool
	NoAnnotations  bool
	Height         int
	Width          int
	Budget         float64
}

// RenderChart draws an ASCII cost timeline. Fewer than two points does not call asciigraph.
func RenderChart(snapshots []CostSnapshot, annotations []CostAnnotation, opt ChartOptions) string {
	filtered := filterSnapshots(snapshots, opt.From, opt.To)
	if len(filtered) == 0 {
		return "No cost history snapshots in the selected range.\n"
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].Timestamp.Equal(filtered[j].Timestamp) {
			return filtered[i].Version < filtered[j].Version
		}
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})
	if len(filtered) == 1 {
		return formatSingle(opt.Stack, opt.Provider, filtered[0])
	}
	return formatPlot(filtered, annotations, opt)
}

func filterSnapshots(snapshots []CostSnapshot, from, to time.Time) []CostSnapshot {
	out := make([]CostSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if inRange(snapshot.Timestamp, from, to) {
			out = append(out, snapshot)
		}
	}
	return out
}

func formatSingle(stack, provider string, snapshot CostSnapshot) string {
	amount := formatMoneyIn(snapshot.Currency, seriesValue(snapshot, provider))
	if isDollar(snapshot.Currency) {
		amount += " " + snapshot.Currency
	}
	return fmt.Sprintf(
		"Monthly Cost (%s) — Stack: %s (1 snapshot, %s)\n%s  %s  v%d\n",
		captionCurrency(snapshot.Currency),
		stack,
		snapshot.Timestamp.UTC().Format("Jan 2006"),
		snapshot.Timestamp.UTC().Format("2006-01-02"),
		strings.TrimSpace(amount),
		snapshot.Version,
	)
}

func formatPlot(snapshots []CostSnapshot, annotations []CostAnnotation, opt ChartOptions) string {
	height := opt.Height
	if height <= 0 {
		height = defaultChartHeight
	}
	width := opt.Width
	if width <= 0 {
		width = defaultChartWidth
	}
	data, legends, colors := chartSeries(snapshots, opt)
	plot := asciigraph.PlotMany(
		data,
		asciigraph.Height(height),
		asciigraph.Width(width),
		asciigraph.Caption(chartCaption(opt.Stack, snapshots)),
		asciigraph.SeriesLegends(legends...),
		asciigraph.SeriesColors(colors...),
	)
	var b strings.Builder
	b.WriteString(plot)
	b.WriteString("\n\n  Legend: ")
	b.WriteString(strings.Join(legends, "  "))
	b.WriteByte('\n')
	if !opt.NoAnnotations {
		writeAnnotations(&b, snapshots, annotations, opt.Provider)
	}
	return b.String()
}

func chartCaption(stack string, snapshots []CostSnapshot) string {
	first := snapshots[0].Timestamp.UTC().Format("Jan 2006")
	last := snapshots[len(snapshots)-1].Timestamp.UTC().Format("Jan 2006")
	return fmt.Sprintf(
		"Monthly Cost (%s) — Stack: %s (%d snapshots, %s – %s)",
		captionCurrency(snapshots[0].Currency), stack, len(snapshots), first, last,
	)
}

func chartSeries(snapshots []CostSnapshot, opt ChartOptions) ([][]float64, []string, []asciigraph.AnsiColor) {
	totals := make([]float64, len(snapshots))
	for i, snapshot := range snapshots {
		totals[i] = seriesValue(snapshot, opt.Provider)
	}
	legend := "Total"
	if opt.Provider != "" {
		legend = strings.ToUpper(opt.Provider)
	}
	data := [][]float64{totals}
	legends := []string{legend}
	colors := []asciigraph.AnsiColor{asciigraph.Blue}
	if opt.SplitProviders && opt.Provider == "" {
		data, legends, colors = appendProviders(data, legends, colors, snapshots)
	}
	if opt.Budget > 0 && !opt.NoBudget {
		flat := make([]float64, len(snapshots))
		for i := range flat {
			flat[i] = opt.Budget
		}
		data = append(data, flat)
		legends = append(legends, "Budget")
		colors = append(colors, asciigraph.Red)
	}
	return data, legends, colors
}

func appendProviders(
	data [][]float64,
	legends []string,
	colors []asciigraph.AnsiColor,
	snapshots []CostSnapshot,
) ([][]float64, []string, []asciigraph.AnsiColor) {
	palette := []asciigraph.AnsiColor{asciigraph.Green, asciigraph.Cyan, asciigraph.Magenta, asciigraph.Yellow}
	for i, name := range providerNames(snapshots) {
		series := make([]float64, len(snapshots))
		for j, snapshot := range snapshots {
			series[j] = seriesValue(snapshot, name)
		}
		data = append(data, series)
		legends = append(legends, strings.ToUpper(name))
		colors = append(colors, palette[i%len(palette)])
	}
	return data, legends, colors
}

// ProviderNames lists the providers present in snapshots, normalized and sorted.
func ProviderNames(snapshots []CostSnapshot) []string {
	return providerNames(snapshots)
}

// HasProvider reports whether any snapshot has an entry for provider, compared
// the way the chart compares it, so "aws-native" counts as "aws".
func HasProvider(snapshots []CostSnapshot, provider string) bool {
	want := resourcetype.NormalizeProvider(provider)
	for _, snapshot := range snapshots {
		for name := range snapshot.ByProvider {
			if resourcetype.NormalizeProvider(name) == want {
				return true
			}
		}
	}
	return false
}

func providerNames(snapshots []CostSnapshot) []string {
	seen := map[string]struct{}{}
	for _, snapshot := range snapshots {
		for name := range snapshot.ByProvider {
			seen[resourcetype.NormalizeProvider(name)] = struct{}{}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func seriesValue(snapshot CostSnapshot, provider string) float64 {
	if provider == "" {
		return snapshot.TotalMonthly
	}
	want := resourcetype.NormalizeProvider(provider)
	total := 0.0
	for name, value := range snapshot.ByProvider {
		if resourcetype.NormalizeProvider(name) == want {
			total += value
		}
	}
	return total
}

func writeAnnotations(b *strings.Builder, snapshots []CostSnapshot, annotations []CostAnnotation, provider string) {
	listed := annotationsInSnapshots(snapshots, annotations)
	if len(listed) == 0 {
		return
	}
	b.WriteString("\nDeployment Annotations:\n")
	for _, annotation := range listed {
		current, _ := snapshotByVersion(snapshots, annotation.Version)
		prev, hasPrev := previousSnapshot(snapshots, annotation.Version)
		change := formatChange(provider, current, prev, hasPrev)
		fmt.Fprintf(
			b,
			"  v%d  %s  %q  %s\n",
			annotation.Version,
			current.Timestamp.UTC().Format("Jan 02"),
			annotation.Message,
			change,
		)
	}
}

func annotationsInSnapshots(snapshots []CostSnapshot, annotations []CostAnnotation) []CostAnnotation {
	listed := make([]CostAnnotation, 0, len(annotations))
	for _, annotation := range annotations {
		if _, ok := snapshotByVersion(snapshots, annotation.Version); ok {
			listed = append(listed, annotation)
		}
	}
	sort.Slice(listed, func(i, j int) bool { return listed[i].Version < listed[j].Version })
	return listed
}

func snapshotByVersion(snapshots []CostSnapshot, version int) (CostSnapshot, bool) {
	for _, snapshot := range snapshots {
		if snapshot.Version == version {
			return snapshot, true
		}
	}
	return CostSnapshot{}, false
}

func previousSnapshot(snapshots []CostSnapshot, version int) (CostSnapshot, bool) {
	for i, snapshot := range snapshots {
		if snapshot.Version == version && i > 0 {
			return snapshots[i-1], true
		}
	}
	return CostSnapshot{}, false
}

func formatChange(provider string, current, prev CostSnapshot, hasPrev bool) string {
	now := seriesValue(current, provider)
	nowText := formatMoneyIn(current.Currency, now) + "/mo"
	if !hasPrev {
		return nowText
	}
	before := seriesValue(prev, provider)
	return fmt.Sprintf(
		"%s → %s (%s)",
		formatMoneyIn(prev.Currency, before)+"/mo",
		nowText,
		signedMoneyIn(current.Currency, now-before),
	)
}

func signedMoney(value float64) string {
	if value > 0 {
		return "+" + formatMoney(value)
	}
	if value < 0 {
		return formatMoney(value)
	}
	return "$0"
}

// isDollar reports whether amounts in currency are written with a dollar sign.
// An empty currency is the default, USD.
func isDollar(currency string) bool {
	return currency == "" || strings.EqualFold(currency, defaultCurrency)
}

// captionCurrency names the currency in a chart caption: "$" for dollars, else the code.
func captionCurrency(currency string) string {
	if isDollar(currency) {
		return "$"
	}
	return strings.ToUpper(currency)
}

// formatMoneyIn writes value in currency: "$1,200" for dollars, "1,200 EUR" for
// any other currency, so a euro total is never printed with a dollar sign.
func formatMoneyIn(currency string, value float64) string {
	text := formatMoney(value)
	if isDollar(currency) {
		return text
	}
	sign := ""
	if strings.HasPrefix(text, "-") {
		sign = "-"
		text = text[1:]
	}
	return sign + strings.TrimPrefix(text, "$") + " " + strings.ToUpper(currency)
}

func signedMoneyIn(currency string, value float64) string {
	if isDollar(currency) {
		return signedMoney(value)
	}
	if value > 0 {
		return "+" + formatMoneyIn(currency, value)
	}
	if value < 0 {
		return formatMoneyIn(currency, value)
	}
	return "0 " + strings.ToUpper(currency)
}

func formatMoney(value float64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	if math.Abs(value-math.Round(value)) < wholeMoneyEpsilon {
		return sign + "$" + groupThousands(int64(math.Round(value)))
	}
	const centsPerDollar = 100
	whole := int64(math.Floor(value))
	frac := int64(math.Round((value - float64(whole)) * centsPerDollar))
	if frac == centsPerDollar {
		whole++
		frac = 0
	}
	return fmt.Sprintf("%s$%s.%02d", sign, groupThousands(whole), frac)
}

func groupThousands(value int64) string {
	const group = 3
	text := strconv.FormatInt(value, 10)
	if len(text) <= group {
		return text
	}
	var b strings.Builder
	pre := len(text) % group
	if pre == 0 {
		pre = group
	}
	b.WriteString(text[:pre])
	for i := pre; i < len(text); i += group {
		b.WriteByte(',')
		b.WriteString(text[i : i+3])
	}
	return b.String()
}
