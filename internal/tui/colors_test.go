package tui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestColorConstants(t *testing.T) {
	tests := []struct {
		name     string
		color    color.Color
		expected color.Color
	}{
		{"ColorOK", ColorOK, lipgloss.Color("82")},
		{"ColorWarning", ColorWarning, lipgloss.Color("208")},
		{"ColorCritical", ColorCritical, lipgloss.Color("196")},
		{"ColorInfo", ColorInfo, lipgloss.Color("33")},
		{"ColorHeader", ColorHeader, lipgloss.Color("99")},
		{"ColorLabel", ColorLabel, lipgloss.Color("245")},
		{"ColorValue", ColorValue, lipgloss.Color("255")},
		{"ColorBorder", ColorBorder, lipgloss.Color("238")},
		{"ColorHighlight", ColorHighlight, lipgloss.Color("229")},
		{"ColorMuted", ColorMuted, lipgloss.Color("240")},
		{"ColorPriorityCritical", ColorPriorityCritical, lipgloss.Color("196")},
		{"ColorPriorityHigh", ColorPriorityHigh, lipgloss.Color("208")},
		{"ColorPriorityMedium", ColorPriorityMedium, lipgloss.Color("226")},
		{"ColorPriorityLow", ColorPriorityLow, lipgloss.Color("82")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.color)
		})
	}
}
