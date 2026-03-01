package tui

import (
	"testing"
)

func TestStyleDefinitions(t *testing.T) {
	// Test that styles are properly initialized with correct properties
	// Note: We test style properties rather than rendered output since
	// lipgloss may not apply ANSI codes in test environments

	// Text styles
	t.Run("HeaderStyle", func(t *testing.T) {
		if !HeaderStyle.GetBold() {
			t.Error("HeaderStyle should be bold")
		}
		if HeaderStyle.GetForeground() != ColorHeader {
			t.Error("HeaderStyle should have header color")
		}
	})

	t.Run("LabelStyle", func(t *testing.T) {
		if LabelStyle.GetForeground() != ColorLabel {
			t.Error("LabelStyle should have label color")
		}
	})

	t.Run("ValueStyle", func(t *testing.T) {
		if ValueStyle.GetForeground() != ColorValue {
			t.Error("ValueStyle should have value color")
		}
	})

	// Status styles
	t.Run("OKStyle", func(t *testing.T) {
		if !OKStyle.GetBold() {
			t.Error("OKStyle should be bold")
		}
		if OKStyle.GetForeground() != ColorOK {
			t.Error("OKStyle should have OK color")
		}
	})

	t.Run("WarningStyle", func(t *testing.T) {
		if !WarningStyle.GetBold() {
			t.Error("WarningStyle should be bold")
		}
		if WarningStyle.GetForeground() != ColorWarning {
			t.Error("WarningStyle should have warning color")
		}
	})

	t.Run("CriticalStyle", func(t *testing.T) {
		if !CriticalStyle.GetBold() {
			t.Error("CriticalStyle should be bold")
		}
		if CriticalStyle.GetForeground() != ColorCritical {
			t.Error("CriticalStyle should have critical color")
		}
	})

	t.Run("InfoStyle", func(t *testing.T) {
		if !InfoStyle.GetBold() {
			t.Error("InfoStyle should be bold")
		}
		if InfoStyle.GetForeground() != ColorInfo {
			t.Error("InfoStyle should have info color")
		}
	})

	// Container styles
	t.Run("BoxStyle", func(t *testing.T) {
		// BoxStyle should have padding (returns top, right, bottom, left)
		top, right, bottom, left := BoxStyle.GetPadding()
		if top+right+bottom+left == 0 {
			t.Error("BoxStyle should have padding")
		}
	})

	// Table styles
	t.Run("TableHeaderStyle", func(t *testing.T) {
		if !TableHeaderStyle.GetBold() {
			t.Error("TableHeaderStyle should be bold")
		}
		if TableHeaderStyle.GetForeground() != ColorHeader {
			t.Error("TableHeaderStyle should have header color")
		}
		top, right, bottom, left := TableHeaderStyle.GetPadding()
		if top != 0 || right != 1 || bottom != 0 || left != 1 {
			t.Error("TableHeaderStyle should preserve default horizontal cell padding")
		}
	})

	t.Run("TableSelectedStyle", func(t *testing.T) {
		if TableSelectedStyle.GetForeground() != ColorHighlight {
			t.Error("TableSelectedStyle should have highlight color")
		}
	})
}
