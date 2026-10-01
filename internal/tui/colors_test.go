package tui_test

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/tui"
)

func TestColorConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		color    color.Color
		expected color.Color
	}{
		{"ColorOK", tui.ColorOK, lipgloss.Color("82")},
		{"ColorWarning", tui.ColorWarning, lipgloss.Color("208")},
		{"ColorCritical", tui.ColorCritical, lipgloss.Color("196")},
		{"ColorInfo", tui.ColorInfo, lipgloss.Color("33")},
		{"ColorHeader", tui.ColorHeader, lipgloss.Color("99")},
		{"ColorLabel", tui.ColorLabel, lipgloss.Color("245")},
		{"ColorValue", tui.ColorValue, lipgloss.Color("255")},
		{"ColorBorder", tui.ColorBorder, lipgloss.Color("238")},
		{"ColorHighlight", tui.ColorHighlight, lipgloss.Color("229")},
		{"ColorMuted", tui.ColorMuted, lipgloss.Color("240")},
		{"ColorPriorityCritical", tui.ColorPriorityCritical, lipgloss.Color("196")},
		{"ColorPriorityHigh", tui.ColorPriorityHigh, lipgloss.Color("208")},
		{"ColorPriorityMedium", tui.ColorPriorityMedium, lipgloss.Color("226")},
		{"ColorPriorityLow", tui.ColorPriorityLow, lipgloss.Color("82")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.color)
		})
	}
}
