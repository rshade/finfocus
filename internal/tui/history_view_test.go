package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
)

func TestGolden_HistoryView(t *testing.T) {
	t.Parallel()
	stamp := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	doc := history.ViewDocument{Stack: "dev"}
	for i, amount := range []float64{90, 120, 260, 160} {
		doc.Snapshots = append(doc.Snapshots, history.CostSnapshot{
			Version: i + 1, Timestamp: stamp.AddDate(0, i, 0),
			TotalMonthly: amount, Currency: "USD",
			ByProvider: map[string]float64{"aws": amount - 30, "gcp": 30},
			Resources:  []history.CostResource{{URN: "db", Type: "aws:rds:Instance", MonthlyCost: amount - 30}},
		})
	}
	doc.Annotations = []history.CostAnnotation{
		{Version: 1, Message: "Initial deployment"},
		{Version: 3, Message: "Scale database", ResourceChanges: map[string]int{"update": 1}},
		{Version: 4, Message: "Reduce capacity"},
	}
	budgets := []history.ChartBudget{
		{Amount: 210, Display: "$210"},
		{Provider: "aws", Amount: 180, Display: "$180"},
	}
	for _, tc := range []struct {
		name          string
		width, height int
		split, detail bool
	}{
		{name: "history_80", width: 80, height: 24, detail: true},
		{name: "history_120", width: 120, height: 30, split: true, detail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := NewHistoryViewModel(doc, history.ChartOptions{Height: 15, SplitProviders: tc.split}, budgets)
			require.NotNil(t, m)
			m.Update(tea.WindowSizeMsg{Width: tc.width, Height: tc.height})
			m.key("2")
			if tc.detail {
				m.key("d")
			}
			rendered := m.View().Content
			lines := strings.Split(rendered, "\n")
			assert.LessOrEqual(t, len(lines), tc.height)
			for _, line := range lines {
				assert.LessOrEqual(t, ansi.StringWidth(line), tc.width)
			}
			assert.Regexp(t, "[⠀-⣿]", rendered)
			testGolden(t, tc.name, rendered)
		})
	}
}

func TestHistoryViewEmptySingleAndNarrow(t *testing.T) {
	t.Parallel()
	m := NewHistoryViewModel(history.ViewDocument{}, history.ChartOptions{}, nil)
	require.Nil(t, m.Init())
	for _, key := range []string{"+", "-", "left", "right", "9", "tab", "d"} {
		m.key(key)
	}
	assert.Contains(t, m.View().Content, "No cost history snapshots")
	doc := history.ViewDocument{Snapshots: []history.CostSnapshot{
		{Version: 1, TotalMonthly: 2, Timestamp: time.Unix(0, 0)},
	}}
	m = NewHistoryViewModel(doc, history.ChartOptions{Provider: "aws", NoAnnotations: true}, nil)
	m.key("s")
	m.key("d")
	assert.False(t, m.split)
	assert.Contains(t, m.View().Content, "No deployment annotations")
	assert.Contains(t, m.View().Content, "v1")
	m.Update(tea.WindowSizeMsg{Width: 21, Height: 6})
	assert.Equal(t, "Resize terminal or press q.", m.View().Content)
}

func TestHistoryViewProviderAliasAndCurrency(t *testing.T) {
	t.Parallel()
	doc := history.ViewDocument{Snapshots: []history.CostSnapshot{{
		Version: 1, Currency: "EUR", Timestamp: time.Unix(0, 0),
		ByProvider: map[string]float64{"aws-native": 17},
	}}}
	m := NewHistoryViewModel(doc, history.ChartOptions{Provider: " AWS-native "}, []history.ChartBudget{
		{Amount: 50, Display: "50 EUR"},
		{Provider: "aws", Amount: 25, Display: "25 EUR"},
	})
	rendered := ansi.Strip(m.View().Content)
	assert.Contains(t, rendered, "17 EUR/mo")
	assert.Contains(t, rendered, "AWS budget 25 EUR")
	assert.NotContains(t, rendered, "$")
	assert.NotContains(t, rendered, "50 EUR")
}
