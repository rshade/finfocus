package tui_test

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/tui"
)

// TestStyleDefinitions verifies that styles are properly initialized with
// correct properties. We test style properties rather than rendered output
// since lipgloss may not apply ANSI codes in test environments.
func TestStyleDefinitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		style     lipgloss.Style
		wantBold  bool
		wantColor color.Color
	}{
		{name: "HeaderStyle", style: tui.HeaderStyle, wantBold: true, wantColor: tui.ColorHeader},
		{name: "LabelStyle", style: tui.LabelStyle, wantColor: tui.ColorLabel},
		{name: "ValueStyle", style: tui.ValueStyle, wantColor: tui.ColorValue},
		{name: "OKStyle", style: tui.OKStyle, wantBold: true, wantColor: tui.ColorOK},
		{name: "WarningStyle", style: tui.WarningStyle, wantBold: true, wantColor: tui.ColorWarning},
		{name: "CriticalStyle", style: tui.CriticalStyle, wantBold: true, wantColor: tui.ColorCritical},
		{name: "InfoStyle", style: tui.InfoStyle, wantBold: true, wantColor: tui.ColorInfo},
		{name: "TableHeaderStyle", style: tui.TableHeaderStyle, wantBold: true, wantColor: tui.ColorHeader},
		{name: "TableSelectedStyle", style: tui.TableSelectedStyle, wantColor: tui.ColorHighlight},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantBold, tt.style.GetBold(), "bold")
			assert.Equal(t, tt.wantColor, tt.style.GetForeground(), "foreground color")
		})
	}
}

func TestStyleDefinitions_Padding(t *testing.T) {
	t.Parallel()

	t.Run("BoxStyle", func(t *testing.T) {
		t.Parallel()
		// BoxStyle should have padding (returns top, right, bottom, left).
		top, right, bottom, left := tui.BoxStyle.GetPadding()
		assert.NotZero(t, top+right+bottom+left, "BoxStyle should have padding")
	})

	t.Run("TableHeaderStyle", func(t *testing.T) {
		t.Parallel()
		top, right, bottom, left := tui.TableHeaderStyle.GetPadding()
		assert.Equal(t, 0, top, "TableHeaderStyle top padding")
		assert.Equal(t, 1, right, "TableHeaderStyle right padding")
		assert.Equal(t, 0, bottom, "TableHeaderStyle bottom padding")
		assert.Equal(t, 1, left, "TableHeaderStyle left padding")
	})
}
