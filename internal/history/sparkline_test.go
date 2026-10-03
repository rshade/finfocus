package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSparkline(t *testing.T) {
	t.Parallel()

	assert.Empty(t, Sparkline(nil, DefaultSparklineWidth))
	assert.Empty(t, Sparkline([]float64{1, 2}, 0))
	assert.Empty(t, Sparkline([]float64{1}, -1))
	assert.Equal(t, "▁▁▁", Sparkline([]float64{5, 5, 5}, 3))
	assert.Equal(t, "▁▁▁▁", Sparkline([]float64{5}, 4))

	rising := []rune(Sparkline([]float64{1, 2, 3, 4, 5, 6, 7}, DefaultSparklineWidth))
	require.Len(t, rising, DefaultSparklineWidth)
	assert.Equal(t, '▁', rising[0])
	assert.Equal(t, '█', rising[len(rising)-1])
	for _, glyph := range rising {
		assert.GreaterOrEqual(t, glyph, '▁')
		assert.LessOrEqual(t, glyph, '█')
	}
}

func TestTableTrends_MissingURNIsZero(t *testing.T) {
	t.Parallel()

	first := CostSnapshot{
		Timestamp:    time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
		Version:      1,
		TotalMonthly: 4,
		Resources:    []CostResource{{URN: "urn:web", MonthlyCost: 4}},
	}
	second := CostSnapshot{
		Timestamp:    time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC),
		Version:      2,
		TotalMonthly: 24,
		Resources: []CostResource{
			{URN: "urn:web", MonthlyCost: 20},
			{URN: "urn:db", MonthlyCost: 4},
		},
	}
	// Later version is listed first to prove ordering is by time.
	trends, total := TableTrends([]CostSnapshot{second, first})
	require.Contains(t, trends, "urn:web")
	require.Contains(t, trends, "urn:db")
	assert.NotEmpty(t, trends["urn:web"])
	assert.NotEmpty(t, trends["urn:db"])
	assert.NotEmpty(t, total)
	assert.Equal(t, '▁', []rune(trends["urn:db"])[0])
	assert.Equal(t, '█', []rune(trends["urn:db"])[len([]rune(trends["urn:db"]))-1])

	empty, emptyTotal := TableTrends(nil)
	require.NotNil(t, empty)
	assert.Empty(t, empty)
	assert.Empty(t, emptyTotal)
}
