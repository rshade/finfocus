package tui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/rshade/finfocus/internal/proto"
)

// Status text constants.
const (
	StatusOK       = "OK"
	StatusWarning  = "WARNING"
	StatusCritical = "CRITICAL"
	StatusExceeded = "EXCEEDED"
	StatusSuccess  = "SUCCESS"
)

// Priority text constants.
const (
	PriorityCritical = "CRITICAL"
	PriorityHigh     = "HIGH"
	PriorityMedium   = "MEDIUM"
	PriorityLow      = "LOW"
)

// Status icons for different states.
// These Unicode icons are used throughout the TUI for consistent visual indicators.
const (
	IconOK         = "✓" // Success/completion indicator.
	IconWarning    = "⚠" // Warning/caution indicator.
	IconCritical   = "🚨" // Critical/error indicator.
	IconPending    = "○" // Pending/inactive state.
	IconProgress   = "◉" // In-progress/active state.
	IconArrowUp    = "↑" // Increase/upward trend.
	IconArrowDown  = "↓" // Decrease/downward trend.
	IconArrowRight = "→" // Neutral/no change.
)

// centsMultiplier is used for rounding monetary values to cents (2 decimal places).
const centsMultiplier = 100

// RenderStatus renders a styled status indicator consisting of an icon and label
// corresponding to the provided status. Recognized statuses (OK, SUCCESS, WARNING,
// CRITICAL, EXCEEDED) map to predefined icons and color themes; unrecognized
// statuses are shown in a muted color with the provided text lowercased.
// The returned string is the icon and label formatted with the selected style.
func RenderStatus(status string) string {
	status = strings.ToUpper(status)

	var icon, text string
	var color lipgloss.Color

	switch status {
	case StatusOK, StatusSuccess:
		icon = IconOK
		text = StatusOK
		color = ColorOK
	case StatusWarning:
		icon = IconWarning
		text = StatusWarning
		color = ColorWarning
	case StatusCritical, StatusExceeded:
		icon = IconCritical
		text = StatusCritical
		color = ColorCritical
	default:
		icon = IconPending
		text = strings.ToLower(status)
		color = ColorMuted
	}

	style := lipgloss.NewStyle().Foreground(color).Bold(true)
	return style.Render(fmt.Sprintf("%s %s", icon, text))
}

// RenderDelta renders a styled indicator for a monetary delta.
//
// The returned string contains a sign ("+" for positive, empty for negative or
// zero), a formatted amount, and an arrow icon: up for positive, down for
// negative, and right for zero. The text is bold and colorized (warning color
// for positive, OK color for negative, muted for zero).
//
// The sign and icon are based on the rounded value (to cents) to ensure visual
// consistency between the displayed amount and the directional indicator.
func RenderDelta(delta float64) string {
	// Round to cents so sign/icon match what we display.
	rounded := math.Round(delta*centsMultiplier) / centsMultiplier

	var icon, sign string
	var color lipgloss.Color

	switch {
	case rounded > 0:
		icon = IconArrowUp
		sign = "+"
		color = ColorWarning
	case rounded < 0:
		icon = IconArrowDown
		sign = ""
		color = ColorOK
	default:
		icon = IconArrowRight
		sign = ""
		color = ColorMuted
	}

	formatted := FormatMoneyShort(rounded)
	style := lipgloss.NewStyle().Foreground(color).Bold(true)
	return style.Render(fmt.Sprintf("%s%s %s", sign, formatted, icon))
}

// RenderPriority renders a styled priority indicator with icon and color.
func RenderPriority(priority string) string {
	priority = strings.ToUpper(priority)

	var icon, text string
	var color lipgloss.Color

	switch priority {
	case PriorityCritical:
		icon = IconCritical
		text = PriorityCritical
		color = ColorPriorityCritical
	case PriorityHigh:
		icon = IconWarning
		text = PriorityHigh
		color = ColorPriorityHigh
	case PriorityMedium:
		icon = IconProgress
		text = PriorityMedium
		color = ColorPriorityMedium
	case PriorityLow:
		icon = IconOK
		text = PriorityLow
		color = ColorPriorityLow
	default:
		icon = IconPending
		text = strings.ToLower(priority)
		color = ColorMuted
	}

	style := lipgloss.NewStyle().Foreground(color).Bold(true)
	return style.Render(fmt.Sprintf("%s %s", icon, text))
}

// FormatActionType returns a human-readable label for a recommendation action type.
// This function is used in TUI displays to show friendly labels like "Purchase Commitment"
// instead of "PURCHASE_COMMITMENT".
//
// Known action types are converted to their human-readable labels. Unknown types
// are returned as-is for forward compatibility with future action types.
func FormatActionType(actionType string) string {
	return proto.ActionTypeLabelFromString(actionType)
}
