package detail

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const (
	maxHistoryRows = 12
	historyHeader  = "COST HISTORY"
)

// View renders the cost-history subsection. Idle, including a skipped source,
// renders nothing.
func (m *Model) View() tea.View {
	return tea.NewView(m.section())
}

func (m *Model) section() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch m.loadState {
	case LoadStateIdle:
		return ""
	case LoadStateLoading:
		return m.viewLoading()
	case LoadStateError:
		return m.viewError()
	case LoadStateLoaded:
		return m.viewLoaded()
	default:
		return ""
	}
}

// ViewError renders the inline failure and the r retry hint.
// It is empty when the model is not in the error state.
func (m *Model) ViewError() string {
	if m == nil {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.loadState != LoadStateError {
		return ""
	}
	return m.viewError()
}

func (m *Model) viewLoading() string {
	body := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Render("Loading cost history...")
	return historyHeader + "\n" + body
}

func (m *Model) viewError() string {
	var b strings.Builder
	b.WriteString(historyHeader)
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true).
		Render("❌ Error: " + m.errorMessage))
	b.WriteString("\n\n")
	if m.retryCount > 1 {
		fmt.Fprintf(&b, "Retries: %d\n\n", m.retryCount)
	}
	b.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Render("[Press 'r' to retry]"))
	return b.String()
}

func (m *Model) viewLoaded() string {
	var b strings.Builder
	b.WriteString(historyHeader)
	b.WriteString("\n")
	if len(m.points) == 0 {
		b.WriteString("No cost history.")
		return b.String()
	}
	points := m.points
	hidden := 0
	if len(points) > maxHistoryRows {
		hidden = len(points) - maxHistoryRows
		points = points[len(points)-maxHistoryRows:]
	}
	for _, point := range points {
		fmt.Fprintf(&b, "  %s  %s\n", point.Timestamp.UTC().Format(time.DateOnly), formatAmount(point))
	}
	if hidden > 0 {
		fmt.Fprintf(&b, "Showing latest %d of %d.\n", maxHistoryRows, len(m.points))
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatAmount(point Point) string {
	if point.Currency == "" {
		return fmt.Sprintf("$%.2f", point.Amount)
	}
	return fmt.Sprintf("$%.2f %s", point.Amount, point.Currency)
}
