package tui

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

// TestStyleDefinitions verifies that styles are properly initialized with
// correct properties. We test style properties rather than rendered output
// since lipgloss may not apply ANSI codes in test environments.
func TestStyleDefinitions(t *testing.T) {
	tests := []struct {
		name      string
		style     lipgloss.Style
		wantBold  bool
		wantColor color.Color
	}{
		{name: "HeaderStyle", style: HeaderStyle, wantBold: true, wantColor: ColorHeader},
		{name: "LabelStyle", style: LabelStyle, wantColor: ColorLabel},
		{name: "ValueStyle", style: ValueStyle, wantColor: ColorValue},
		{name: "OKStyle", style: OKStyle, wantBold: true, wantColor: ColorOK},
		{name: "WarningStyle", style: WarningStyle, wantBold: true, wantColor: ColorWarning},
		{name: "CriticalStyle", style: CriticalStyle, wantBold: true, wantColor: ColorCritical},
		{name: "InfoStyle", style: InfoStyle, wantBold: true, wantColor: ColorInfo},
		{name: "TableHeaderStyle", style: TableHeaderStyle, wantBold: true, wantColor: ColorHeader},
		{name: "TableSelectedStyle", style: TableSelectedStyle, wantColor: ColorHighlight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantBold, tt.style.GetBold(), "bold")
			assert.Equal(t, tt.wantColor, tt.style.GetForeground(), "foreground color")
		})
	}
}

func TestStyleDefinitions_Padding(t *testing.T) {
	t.Run("BoxStyle", func(t *testing.T) {
		// BoxStyle should have padding (returns top, right, bottom, left).
		top, right, bottom, left := BoxStyle.GetPadding()
		assert.NotZero(t, top+right+bottom+left, "BoxStyle should have padding")
	})

	t.Run("TableHeaderStyle", func(t *testing.T) {
		top, right, bottom, left := TableHeaderStyle.GetPadding()
		assert.Equal(t, 0, top, "TableHeaderStyle top padding")
		assert.Equal(t, 1, right, "TableHeaderStyle right padding")
		assert.Equal(t, 0, bottom, "TableHeaderStyle bottom padding")
		assert.Equal(t, 1, left, "TableHeaderStyle left padding")
	})
}
