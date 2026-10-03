package tui

import (
	"image/color"
	"os"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// Environment variables that fill accessibility flags the command line left unset.
const (
	envPlain        = "FINFOCUS_PLAIN"
	envForceColor   = "FORCE_COLOR"
	envHighContrast = "FINFOCUS_HIGH_CONTRAST"
)

// Accessibility is the resolved plain-text and color mode for one command.
type Accessibility struct {
	NoColor      bool
	Plain        bool
	ForceColor   bool
	HighContrast bool
}

// PaletteColors holds the status colors for one rendering pass.
type PaletteColors struct {
	OK       color.Color
	Warning  color.Color
	Critical color.Color
	Header   color.Color
}

// ResolveAccessibility fills flags the caller did not set from the environment.
// explicit reports which fields were set on the command line. A set flag wins
// over the environment. An explicit --plain or --no-color wins over --color,
// --force-color, and --high-contrast. FINFOCUS_PLAIN and a non-empty NO_COLOR
// win over FORCE_COLOR and FINFOCUS_HIGH_CONTRAST when those were not set by
// a flag. Empty values and values that are not valid bools leave the flag
// unchanged. NO_COLOR follows https://no-color.org/: any non-empty value counts.
func ResolveAccessibility(flags, explicit Accessibility) Accessibility {
	out := applyAccessibilityEnv(flags, explicit)
	if explicitPlainOrNoColor(out, explicit) {
		out.ForceColor = false
		out.HighContrast = false
		return out
	}
	if explicitColorMode(out, explicit) {
		if !explicit.Plain {
			out.Plain = false
		}
		if !explicit.NoColor {
			out.NoColor = false
		}
		return out
	}
	if out.Plain || out.NoColor {
		out.ForceColor = false
		out.HighContrast = false
	}
	return out
}

func applyAccessibilityEnv(flags, explicit Accessibility) Accessibility {
	out := flags
	if !explicit.Plain && !out.Plain {
		out.Plain = envBool(envPlain)
	}
	if !explicit.NoColor && !out.NoColor {
		out.NoColor = os.Getenv("NO_COLOR") != ""
	}
	if !explicit.ForceColor && !out.ForceColor {
		out.ForceColor = envForceColorOn()
	}
	if !explicit.HighContrast && !out.HighContrast {
		out.HighContrast = envBool(envHighContrast)
	}
	return out
}

func explicitPlainOrNoColor(out, explicit Accessibility) bool {
	return (out.Plain && explicit.Plain) || (out.NoColor && explicit.NoColor)
}

func explicitColorMode(out, explicit Accessibility) bool {
	return (out.ForceColor && explicit.ForceColor) || (out.HighContrast && explicit.HighContrast)
}

// envForceColorOn reads FORCE_COLOR. The convention (force-color.org) is that a
// variable that is present and not an empty string forces color regardless of
// its value, so any such value turns it on. The one exception is 0 and false:
// the convention is silent on them, but the supports-color package that defined
// the 0 to 3 color levels treats them as "do not force", and scripts rely on it.
// The levels themselves are not distinguished here.
func envForceColorOn() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(envForceColor))) {
	case "", "0", "false":
		return false
	default:
		return true
	}
}

func envBool(key string) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return false
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return parsed
}

// Palette returns the default status colors, or the brighter high-contrast set.
// High contrast uses ANSI 46 (OK), 226 (warning), 196 (critical), and 231 (header).
// The package color variables stay unchanged.
func Palette(highContrast bool) PaletteColors {
	if highContrast {
		return PaletteColors{
			OK:       lipgloss.Color("46"),
			Warning:  lipgloss.Color("226"),
			Critical: lipgloss.Color("196"),
			Header:   lipgloss.Color("231"),
		}
	}
	return PaletteColors{
		OK:       ColorOK,
		Warning:  ColorWarning,
		Critical: ColorCritical,
		Header:   ColorHeader,
	}
}

// StatusIndicator returns a status label that does not rely on color alone.
// noColor is the bracket form ("[OK] ✓"). Color mode is the icon form ("✓ OK").
// Recognized labels are OK, WARNING, CRITICAL, ERROR, and PENDING.
func StatusIndicator(status string, noColor bool) string {
	label, icon := statusParts(status)
	if noColor {
		return "[" + label + "] " + icon
	}
	return icon + " " + label
}

func statusParts(status string) (string, string) {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case StatusOK, StatusSuccess:
		return StatusOK, IconOK
	case StatusWarning:
		return StatusWarning, IconWarning
	case StatusCritical, StatusExceeded:
		return StatusCritical, IconCritical
	case "ERROR":
		return "ERROR", IconCritical
	case "PENDING", "":
		return "PENDING", IconPending
	default:
		return strings.ToUpper(strings.TrimSpace(status)), IconPending
	}
}
