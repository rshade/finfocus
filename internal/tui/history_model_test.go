package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
)

func TestHistoryViewNavigation(t *testing.T) {
	t.Parallel()
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	doc := history.ViewDocument{Stack: "dev"}
	for i := range 9 {
		doc.Snapshots = append(doc.Snapshots, history.CostSnapshot{
			Version: i + 1, Timestamp: now.AddDate(0, 0, i*4), Currency: "USD",
			TotalMonthly: float64(100 + i*10), ByProvider: map[string]float64{"aws": 40, "gcp": 60},
		})
	}
	doc.Annotations = []history.CostAnnotation{{Version: 4, Message: "Add database"}}
	m := NewHistoryViewModel(doc, history.ChartOptions{Budget: 150}, nil)
	require.NotNil(t, m)
	send := func(code rune, text string) {
		m.Update(tea.KeyPressMsg{Code: code, Text: text})
	}
	send(tea.KeyHome, "")
	assert.Zero(t, m.cursor)
	send(tea.KeyRight, "")
	assert.Equal(t, 1, m.cursor)
	send('+', "+")
	assert.Less(t, m.span, len(doc.Snapshots))
	send('-', "-")
	assert.Equal(t, len(doc.Snapshots), m.span)
	send('s', "s")
	assert.True(t, m.split)
	send(tea.KeyTab, "")
	assert.Equal(t, 1, m.active)
	send('b', "b")
	assert.False(t, m.budget)
	send('1', "1")
	assert.Equal(t, 3, m.cursor)
	send('d', "d")
	assert.Contains(t, m.View().Content, "Add database")
	send(tea.KeyEnd, "")
	assert.Equal(t, 8, m.cursor)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	assert.Equal(t, 120, m.width)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	require.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
}
