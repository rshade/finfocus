package tui

import "github.com/charmbracelet/lipgloss"

// Text styles provide consistent formatting for different text elements.
// These styles automatically adapt to terminal capabilities and NO_COLOR settings.
//
//nolint:gochecknoglobals // Global styles are the standard pattern for lipgloss.
var (
	// HeaderStyle formats headings and titles with bold text and header color.
	// Use for section headers, command names, and important labels.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorHeader)

	// LabelStyle formats field labels and secondary text with muted color.
	// Use for form labels, metadata keys, and descriptive text.
	LabelStyle = lipgloss.NewStyle().
			Foreground(ColorLabel)

	// ValueStyle formats data values and primary content with bright color.
	// Use for numbers, results, and important data display.
	ValueStyle = lipgloss.NewStyle().
			Foreground(ColorValue)

	// SubtleStyle formats secondary/supplementary information with muted styling.
	// Use for equivalencies, hints, and supporting details.
	SubtleStyle = lipgloss.NewStyle().
			Foreground(ColorSubtle).
			Italic(true)
)

// Status styles provide visual indicators for different operational states.
// All status styles include bold formatting for emphasis.
//
//nolint:gochecknoglobals // Global styles are the standard pattern for lipgloss.
var (
	// OKStyle formats success messages and positive states with green color.
	// Use for confirmations, successful operations, and good results.
	OKStyle = lipgloss.NewStyle().
		Foreground(ColorOK).
		Bold(true)

	// WarningStyle formats caution messages and warnings with orange color.
	// Use for non-critical issues, recommendations, and alerts.
	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorWarning).
			Bold(true)

	// CriticalStyle formats error messages and critical states with red color.
	// Use for failures, errors, and urgent issues requiring attention.
	CriticalStyle = lipgloss.NewStyle().
			Foreground(ColorCritical).
			Bold(true)

	// InfoStyle formats informational messages with blue color.
	// Use for neutral information, hints, and general notices.
	InfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo).
			Bold(true)
)

// Container styles provide layout and grouping for content blocks.
//
//nolint:gochecknoglobals // Global styles are the standard pattern for lipgloss.
var (
	// BoxStyle creates bordered containers with padding and rounded corners.
	// Use for grouping related content, creating panels, and visual separation.
	BoxStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorder).
		Padding(0, 1)
)

// Table styles provide formatting for tabular data display.
//
//nolint:gochecknoglobals // Global styles are the standard pattern for lipgloss.
var (
	// TableHeaderStyle formats table headers with bold text and bottom border.
	// Use for column headers in data tables and lists.
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorHeader).
				BorderStyle(lipgloss.NormalBorder()).
				BorderBottom(true)

	// TableSelectedStyle highlights selected table rows with background color.
	// Use for indicating the currently selected or active row in interactive tables.
	TableSelectedStyle = lipgloss.NewStyle().
				Background(ColorSelectedBg).
				Foreground(ColorHighlight)
)
