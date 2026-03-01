package tui

import (
	"charm.land/lipgloss/v2"
	figure "github.com/common-nighthawk/go-figure"
)

const bannerMinWidth = 65

// RenderBanner returns the centered FinFocus splash banner.
// Falls back to plain styled text on narrow terminals (< 65 columns).
func RenderBanner(width int) string {
	var title string
	if width >= bannerMinWidth {
		fig := figure.NewFigure("FinFocus", "banner", true)
		title = HeaderStyle.Render(fig.String())
	} else {
		title = HeaderStyle.Render("FinFocus")
	}

	motif := OKStyle.Render("$ $ $ $ $ $ $ $ $ $ $ $ $ $ $ $ $ $ $ $")

	return lipgloss.JoinVertical(lipgloss.Center,
		"",
		motif,
		title,
		motif,
		"",
	)
}
