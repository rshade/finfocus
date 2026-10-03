package history

import (
	"math"
	"strings"
)

const (
	// DefaultSparklineWidth is the Trend column width used by cost tables.
	DefaultSparklineWidth = 7
	// sparklineLevels is the number of block heights from U+2581 through U+2588.
	sparklineLevels = 8
	sparklineLow    = '\u2581'
)

// Sparkline renders values as Unicode block elements (U+2581 through U+2588).
// An empty series or a non-positive width returns an empty string.
// A flat series is the low block, repeated to width.
func Sparkline(values []float64, width int) string {
	if len(values) == 0 || width <= 0 {
		return ""
	}
	resampled := resample(values, width)
	low, high := bounds(resampled)
	span := high - low
	if span == 0 {
		return strings.Repeat(string(sparklineLow), width)
	}
	var b strings.Builder
	b.Grow(width * len(string(sparklineLow)))
	for _, value := range resampled {
		idx := int((value - low) / span * sparklineLevels)
		if idx >= sparklineLevels {
			idx = sparklineLevels - 1
		}
		if idx < 0 {
			idx = 0
		}
		b.WriteRune(sparklineLow + rune(idx))
	}
	return b.String()
}

// TableTrends builds one sparkline per resource URN and one for the snapshot totals.
// The map is always non-nil. A URN missing from a snapshot contributes 0 so every
// series has one point per snapshot. Snapshots are ordered by time, then version.
func TableTrends(snapshots []CostSnapshot) (map[string]string, string) {
	ordered := orderedSnapshots(snapshots)
	series := resourceSeries(ordered)
	trends := make(map[string]string, len(series))
	for urn, values := range series {
		trends[urn] = Sparkline(values, DefaultSparklineWidth)
	}
	totals := make([]float64, len(ordered))
	for i, snapshot := range ordered {
		totals[i] = snapshot.TotalMonthly
	}
	return trends, Sparkline(totals, DefaultSparklineWidth)
}

func resourceSeries(snapshots []CostSnapshot) map[string][]float64 {
	urns := make([]string, 0)
	seen := make(map[string]struct{})
	for _, snapshot := range snapshots {
		for _, resource := range snapshot.Resources {
			if resource.URN == "" {
				continue
			}
			if _, ok := seen[resource.URN]; ok {
				continue
			}
			seen[resource.URN] = struct{}{}
			urns = append(urns, resource.URN)
		}
	}
	series := make(map[string][]float64, len(urns))
	for _, urn := range urns {
		series[urn] = make([]float64, len(snapshots))
	}
	for i, snapshot := range snapshots {
		amounts := make(map[string]float64, len(snapshot.Resources))
		for _, resource := range snapshot.Resources {
			if resource.URN == "" {
				continue
			}
			amounts[resource.URN] += resource.MonthlyCost
		}
		for urn, values := range series {
			values[i] = amounts[urn]
		}
	}
	return series
}

func resample(values []float64, width int) []float64 {
	count := len(values)
	if count == width {
		return append([]float64(nil), values...)
	}
	out := make([]float64, width)
	if count == 1 {
		for i := range out {
			out[i] = values[0]
		}
		return out
	}
	if count > width {
		for i := range out {
			start := i * count / width
			end := min((i+1)*count/width, count)
			if end <= start {
				out[i] = values[min(start, count-1)]
				continue
			}
			var sum float64
			for _, value := range values[start:end] {
				sum += value
			}
			out[i] = sum / float64(end-start)
		}
		return out
	}
	last := count - 1
	span := float64(width - 1)
	for i := range out {
		idx := int(math.Round(float64(i) * float64(last) / span))
		out[i] = values[idx]
	}
	return out
}

func bounds(values []float64) (float64, float64) {
	low := values[0]
	high := values[0]
	for _, value := range values[1:] {
		if value < low {
			low = value
		}
		if value > high {
			high = value
		}
	}
	return low, high
}

func orderedSnapshots(snapshots []CostSnapshot) []CostSnapshot {
	ordered := append([]CostSnapshot(nil), snapshots...)
	sortSnapshots(ordered)
	return ordered
}
