package tui

import (
	"cmp"
	"fmt"
	"image/color"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/canvas"
	"github.com/NimbleMarkets/ntcharts/v2/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/v2/linechart/timeserieslinechart"
	"github.com/charmbracelet/x/ansi"

	"github.com/rshade/finfocus/internal/history"
)

const (
	historyMinWidth        = 24
	historyMinChartHeight  = 4
	historyMinScreenHeight = 8
	historyChartExtraRows  = 2 // Annotation markers and legend.
	historyLabelMinSpacing = 11
	historyLabelColumns    = 5
	historyLabelRows       = 4
	historyScalePadding    = 0.1
	historyTopChanges      = 3
)

const historyHelp = "←/→ Pan  +/- Zoom  s Providers  Tab Focus  d Detail  b Budget\n" +
	"1–9 Deployments  Home/End First/Last  q Quit"

type historySeries struct {
	provider string
	label    string
	color    color.Color
	focus    bool
}

// View renders the current cost window and keyboard controls.
func (m *HistoryViewModel) View() tea.View {
	view := tea.NewView(m.render())
	view.AltScreen = true
	return view
}

func (m *HistoryViewModel) render() string {
	if m.width < historyMinWidth || m.height < historyMinScreenHeight {
		return "Resize terminal or press q."
	}
	if len(m.doc.Snapshots) == 0 {
		return "No cost history snapshots in the selected range.\nq Quit"
	}
	start, end := m.visible()
	points := m.doc.Snapshots[start:end]
	selected := m.doc.Snapshots[m.cursor]
	currency := cmp.Or(selected.Currency, "USD")
	width := m.width
	if m.options.Width > 0 {
		width = min(width, max(historyMinWidth, m.options.Width))
	}
	lines := []string{
		fmt.Sprintf("Monthly Cost (%s) — Stack: %s", historyText(currency), historyText(m.doc.Stack)),
		fmt.Sprintf("%s  v%d  %s/mo", selected.Timestamp.UTC().Format(time.DateOnly), selected.Version,
			history.FormatChartMoney(selected.Currency, history.SnapshotSeriesValue(selected, m.options.Provider))),
	}
	if m.doc.CurrencyWarning != "" {
		lines = append(lines, historyText(m.doc.CurrencyWarning))
	}
	help := strings.Split(historyHelp, "\n")
	series, budgets := m.series(), m.visibleBudgets()
	legend := historyLegend(series, budgets)
	details := m.detailLines()
	available := m.height - len(lines) - len(help) - historyChartExtraRows
	details = details[:min(len(details), max(0, available-historyMinChartHeight))]
	height := min(available-len(details), max(historyMinChartHeight, m.options.Height))
	if height >= historyMinChartHeight {
		chart := historyChart(width, height, points, series, budgets)
		for line := range strings.SplitSeq(chart.View(), "\n") {
			lines = append(lines, strings.TrimRight(line, " "))
		}
		lines = append(lines, m.markerRow(&chart, start, end, width), legend)
	}
	lines = append(lines, details...)
	lines = append(lines, help...)
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}

func (m *HistoryViewModel) series() []historySeries {
	if !m.split || len(m.providers) == 0 {
		provider := m.options.Provider
		return []historySeries{{provider: provider, label: cmp.Or(strings.ToUpper(historyText(provider)), "Total"),
			color: historyProviderColor(provider, 0)}}
	}
	out := make([]historySeries, len(m.providers))
	for i, provider := range m.providers {
		out[i] = historySeries{provider: provider, label: strings.ToUpper(historyText(provider)),
			color: historyProviderColor(provider, i), focus: i == m.active}
	}
	return out
}

func historyProviderColor(provider string, i int) color.Color {
	switch provider {
	case "aws":
		return ColorWarning
	case "gcp":
		return ColorInfo
	case "azure":
		return ColorSpot
	default:
		palette := []color.Color{ColorHeader, ColorOK, ColorSpinner, ColorInfo}
		return palette[i%len(palette)]
	}
}

func (m *HistoryViewModel) visibleBudgets() []history.ChartBudget {
	if !m.budget {
		return nil
	}
	var out []history.ChartBudget
	for _, budget := range m.budgets {
		switch {
		case budget.Provider == "" && m.options.Provider == "":
			out = append(out, budget)
		case budget.Provider != "" && (m.split || budget.Provider == m.options.Provider):
			out = append(out, budget)
		}
	}
	return out
}

func historyLegend(series []historySeries, budgets []history.ChartBudget) string {
	items := make([]string, 0, len(series)+len(budgets))
	for _, item := range series {
		label := item.label
		if item.focus {
			label = "[" + label + "]"
		}
		items = append(items, lipgloss.NewStyle().Foreground(item.color).Render(label))
	}
	for _, budget := range budgets {
		label := "Budget"
		if budget.Provider != "" {
			label = strings.ToUpper(historyText(budget.Provider)) + " budget"
		}
		items = append(items, lipgloss.NewStyle().Foreground(ColorCritical).Render(label+" "+budget.Display))
	}
	return strings.Join(items, "  ")
}

func historyChart(
	width, height int, points []history.CostSnapshot, series []historySeries, budgets []history.ChartBudget,
) timeserieslinechart.Model {
	first, last := historyTimeRange(points)
	low, high := historyChartRange(points, series, budgets)
	chart := timeserieslinechart.New(
		width,
		height,
		timeserieslinechart.WithTimeRange(first, last),
		timeserieslinechart.WithYRange(low, high),
		timeserieslinechart.WithXYSteps(
			max(historyLabelMinSpacing, width/historyLabelColumns),
			max(1, height/historyLabelRows),
		),
		timeserieslinechart.WithXLabelFormatter(func(_ int, value float64) string {
			stamp := time.UnixMilli(int64(value * float64(time.Second/time.Millisecond))).UTC()
			if last.Sub(first) < 24*time.Hour {
				return stamp.Format("15:04:05")
			}
			return stamp.Format("Jan 02")
		}),
		timeserieslinechart.WithYLabelFormatter(func(_ int, value float64) string {
			return strconv.FormatFloat(value, 'g', 4, 64)
		}),
	)
	names := make([]string, 0, len(series))
	for _, item := range series {
		style := lipgloss.NewStyle().Foreground(item.color).Bold(item.focus)
		chart.SetDataSetStyle(item.provider, style)
		for _, point := range points {
			chart.PushDataSet(item.provider, timeserieslinechart.TimePoint{
				Time: point.Timestamp, Value: history.SnapshotSeriesValue(point, item.provider),
			})
		}
		if item.focus {
			continue
		}
		names = append(names, item.provider)
	}
	for _, item := range series {
		if item.focus {
			names = append(names, item.provider)
		}
	}
	chart.DrawBrailleDataSets(names)
	style := lipgloss.NewStyle().Foreground(ColorCritical)
	for _, budget := range budgets {
		chart.DrawLineWithStyle(
			canvas.Float64Point{X: chart.ViewMinX(), Y: budget.Amount},
			canvas.Float64Point{X: chart.ViewMaxX(), Y: budget.Amount},
			runes.ThinLineStyle, style,
		)
	}
	return chart
}

func historyChartRange(
	points []history.CostSnapshot, series []historySeries, budgets []history.ChartBudget,
) (float64, float64) {
	low, high := math.Inf(1), math.Inf(-1)
	for _, item := range series {
		for _, point := range points {
			value := history.SnapshotSeriesValue(point, item.provider)
			low, high = min(low, value), max(high, value)
		}
	}
	for _, budget := range budgets {
		low, high = min(low, budget.Amount), max(high, budget.Amount)
	}
	padding := (high - low) * historyScalePadding
	if padding == 0 {
		padding = max(1, math.Abs(high)*historyScalePadding)
	}
	return low - padding, high + padding
}

func (m *HistoryViewModel) markerRow(chart *timeserieslinechart.Model, start, end, width int) string {
	row := []rune(strings.Repeat(" ", width))
	place := func(i int, symbol rune) {
		stamp := float64(m.doc.Snapshots[i].Timestamp.UnixMilli()) / float64(time.Second/time.Millisecond)
		fraction := (stamp - chart.ViewMinX()) / (chart.ViewMaxX() - chart.ViewMinX())
		x := chart.Origin().X + 1 + int(math.Round(fraction*float64(chart.GraphWidth()-1)))
		row[min(width-1, max(0, x))] = symbol
	}
	place(m.cursor, '│')
	for _, i := range m.markers {
		if i >= start && i < end {
			place(i, '▲')
		}
	}
	return strings.TrimRight(string(row), " ")
}

func (m *HistoryViewModel) detailLines() []string {
	if !m.detail {
		return nil
	}
	i := m.nearestMarker()
	if i < 0 {
		return []string{"No deployment annotations in the selected history."}
	}
	point := m.doc.Snapshots[i]
	note := m.notes[point.Version]
	lines := []string{fmt.Sprintf("Deployment v%d — %s", point.Version, point.Timestamp.UTC().Format(time.DateOnly))}
	if note.Message != "" {
		lines = append(lines, historyText(note.Message))
	}
	var changes []string
	for kind, count := range note.ResourceChanges {
		changes = append(changes, fmt.Sprintf("%d %s", count, historyText(kind)))
	}
	slices.Sort(changes)
	if len(changes) > 0 {
		lines = append(lines, "Resources  "+strings.Join(changes, ", "))
	}
	if i == 0 {
		return append(lines, "Cost  "+history.FormatChartMoney(point.Currency, point.TotalMonthly)+"/mo")
	}
	lines = append(lines, historyDiffLines(m.doc.Snapshots[i-1], point)...)
	return lines
}

func historyDiffLines(before, after history.CostSnapshot) []string {
	diff, err := history.DiffSnapshots(before, after, 0)
	if err != nil {
		return nil
	}
	lines := []string{
		fmt.Sprintf(
			"Cost  %s → %s/mo (%s)",
			history.FormatChartMoney(diff.Currency, diff.From.TotalMonthly),
			history.FormatChartMoney(
				diff.Currency,
				diff.To.TotalMonthly,
			),
			history.FormatChartDelta(diff.Currency, diff.Delta),
		),
	}
	items := slices.Concat(diff.Added, diff.Removed, diff.Changed)
	slices.SortStableFunc(items, func(a, b history.CostDiffItem) int {
		return cmp.Compare(math.Abs(b.Delta), math.Abs(a.Delta))
	})
	for _, item := range items[:min(historyTopChanges, len(items))] {
		line := fmt.Sprintf("  %s  %s/mo  %s", historyText(item.Type),
			history.FormatChartDelta(diff.Currency, item.Delta), historyText(item.SKU))
		lines = append(lines, strings.TrimRight(line, " "))
	}
	return lines
}

func historyText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}
