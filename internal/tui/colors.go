package tui

import "charm.land/lipgloss/v2"

// Status colors.
//
//nolint:gochecknoglobals // Immutable color lookup values; var required because lipgloss.Color() returns color.Color interface.
var (
	ColorOK       = lipgloss.Color("82")  // #5fd700 - Green
	ColorWarning  = lipgloss.Color("208") // #ff8700 - Orange
	ColorCritical = lipgloss.Color("196") // #ff0000 - Red
	ColorInfo     = lipgloss.Color("33")  // #0087ff - Blue
)

// UI element colors.
//
//nolint:gochecknoglobals // Immutable color lookup values; var required because lipgloss.Color() returns color.Color interface.
var (
	ColorHeader     = lipgloss.Color("99")  // #875fff - Purple
	ColorLabel      = lipgloss.Color("245") // #8a8a8a - Gray
	ColorValue      = lipgloss.Color("255") // #eeeeee - White
	ColorBorder     = lipgloss.Color("238") // #444444 - Dark gray
	ColorHighlight  = lipgloss.Color("229") // #ffffaf - Yellow
	ColorMuted      = lipgloss.Color("240") // #585858 - Dim gray
	ColorSelectedBg = lipgloss.Color("237") // #3a3a3a - Selected row background
	ColorSpinner    = lipgloss.Color("205") // #ff5faf - Pink (loading indicator)
	ColorSubtle     = lipgloss.Color("243") // #767676 - Subtle gray for secondary info
)

// Priority colors.
//
//nolint:gochecknoglobals // Immutable color lookup values; var required because lipgloss.Color() returns color.Color interface.
var (
	ColorPriorityCritical = ColorCritical
	ColorPriorityHigh     = ColorWarning
	ColorPriorityMedium   = lipgloss.Color("226") // #ffff00 - Yellow
	ColorPriorityLow      = ColorOK
)
