package forecast

import (
	"fmt"
	"strings"

	"github.com/guptarohit/asciigraph"
)

const (
	defaultChartHeight = 15
	defaultChartWidth  = 80
)

// ChartOptions selects how Render draws a projection.
type ChartOptions struct {
	Height         int
	Width          int
	SplitProviders bool
	Budget         float64
	NoColor        bool
}

// Render draws the forecast series. Fewer than two points prints one line.
// Provider series and the budget line share the forecast's month index.
// History is not drawn here: its timestamps are not the same index.
func Render(p Projection, opt ChartOptions) string {
	if len(p.Forecast.Points) == 0 {
		return "No forecast points.\n"
	}
	if len(p.Forecast.Points) == 1 {
		return formatSingle(p)
	}
	return formatPlot(p, opt)
}

func formatSingle(p Projection) string {
	point := p.Forecast.Points[0]
	return fmt.Sprintf(
		"Forecast monthly cost (%s) — %s\n%s  %.2f\n",
		p.Currency,
		point.Time.UTC().Format("Jan 2006"),
		point.Time.UTC().Format("2006-01-02"),
		point.Value,
	)
}

func formatPlot(p Projection, opt ChartOptions) string {
	height := opt.Height
	if height <= 0 {
		height = defaultChartHeight
	}
	width := opt.Width
	if width <= 0 {
		width = defaultChartWidth
	}
	data, legends, colors := chartSeries(p, opt)
	options := []asciigraph.Option{
		asciigraph.Height(height),
		asciigraph.Width(width),
		asciigraph.Caption(chartCaption(p)),
		asciigraph.SeriesLegends(legends...),
	}
	if !opt.NoColor && len(colors) == len(data) {
		options = append(options, asciigraph.SeriesColors(colors...))
	}
	var b strings.Builder
	b.WriteString(asciigraph.PlotMany(data, options...))
	b.WriteString("\n\n  Legend: ")
	b.WriteString(strings.Join(legends, "  "))
	b.WriteByte('\n')
	if opt.NoColor {
		return stripANSI(b.String())
	}
	return b.String()
}

// stripANSI removes SGR sequences. asciigraph paints its legend even when no
// series color is requested, and --no-color / NO_COLOR must be plain text.
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b || i+1 >= len(s) || s[i+1] != '[' {
			b.WriteByte(s[i])
			continue
		}
		i += 2
		for i < len(s) && ((s[i] >= '0' && s[i] <= '9') || s[i] == ';') {
			i++
		}
	}
	return b.String()
}

func chartSeries(p Projection, opt ChartOptions) ([][]float64, []string, []asciigraph.AnsiColor) {
	data := [][]float64{valuesOf(p.Forecast.Points)}
	legends := []string{"Forecast"}
	colors := []asciigraph.AnsiColor{asciigraph.Blue}
	if opt.SplitProviders {
		palette := []asciigraph.AnsiColor{asciigraph.Green, asciigraph.Cyan, asciigraph.Magenta, asciigraph.Yellow}
		for i, series := range p.ByProvider {
			data = append(data, valuesOf(series.Points))
			legends = append(legends, strings.ToUpper(series.Name))
			colors = append(colors, palette[i%len(palette)])
		}
	}
	if opt.Budget > 0 {
		flat := make([]float64, len(p.Forecast.Points))
		for i := range flat {
			flat[i] = opt.Budget
		}
		data = append(data, flat)
		legends = append(legends, "Budget")
		colors = append(colors, asciigraph.Red)
	}
	return data, legends, colors
}

func chartCaption(p Projection) string {
	first := p.Forecast.Points[0].Time.UTC()
	last := p.Forecast.Points[len(p.Forecast.Points)-1].Time.UTC()
	return fmt.Sprintf(
		"Forecast monthly cost (%s) — %s – %s",
		p.Currency,
		first.Format("Jan 2006"),
		last.Format("Jan 2006"),
	)
}

func valuesOf(points []Point) []float64 {
	values := make([]float64, len(points))
	for i, point := range points {
		values[i] = point.Value
	}
	return values
}
