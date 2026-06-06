package tui

import (
	"os"

	"golang.org/x/term"
)

// Terminal defaults.
const (
	// DefaultTerminalWidth is the fallback width when terminal size cannot be determined.
	DefaultTerminalWidth = 80
	// MinTerminalWidth is the minimum width required for interactive TUI.
	MinTerminalWidth = 60
)

// OutputMode represents the rendering mode for CLI output based on
// terminal capabilities and user preferences.
type OutputMode int

const (
	// OutputModePlain provides basic text output with no ANSI styling.
	// Used when colors are disabled or terminal doesn't support them.
	OutputModePlain OutputMode = iota

	// OutputModeStyled applies Lip Gloss styling for enhanced readability.
	// Suitable for CI environments and terminals that support colors.
	OutputModeStyled

	// OutputModeInteractive enables full Bubble Tea TUI with user interaction.
	// Requires a capable terminal with TTY support.
	OutputModeInteractive
)

// DetectOutputMode determines the appropriate output mode based on terminal
// capabilities, environment variables, and explicit flags.
//
// Priority order (highest to lowest):
// 1. Explicit flags (--plain, --no-color)
// 2. NO_COLOR environment variable
// 3. Terminal/TTY detection
// 4. TERM environment variable
// 5. CI environment detection
//
// Usage:
//
//	mode := DetectOutputMode(forceColorFlag, noColorFlag, plainFlag)
//	switch mode {
//	case OutputModePlain:
//	    // Plain text output
//	case OutputModeStyled:
//	    // ANSI styled output
//	case OutputModeInteractive:
//	    // Full TUI
//	}
func DetectOutputMode(forceColor, noColor, plain bool) OutputMode {
	// Explicit plain/noColor flags take highest precedence.
	if plain || noColor {
		return OutputModePlain
	}

	// Respect NO_COLOR standard (https://no-color.org/).
	if os.Getenv("NO_COLOR") != "" {
		return OutputModePlain
	}

	// forceColor flag enables styled output even without TTY.
	if forceColor {
		return OutputModeStyled
	}

	// Check if stdout is connected to a terminal.
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return OutputModePlain
	}

	// Check for dumb terminals that don't support advanced features.
	if os.Getenv("TERM") == "dumb" {
		return OutputModePlain
	}

	// CI environments typically support colors but not interactivity.
	if os.Getenv("CI") != "" {
		return OutputModeStyled
	}

	// Fall back to plain text if terminal is too narrow for TUI.
	if TerminalWidth() < MinTerminalWidth {
		return OutputModePlain
	}

	// Default to interactive mode for capable terminals.
	return OutputModeInteractive
}

// IsTTY returns true if stdout is connected to a terminal (TTY).
// This is useful for determining whether interactive features can be used.
//
// Usage:
//
//	if IsTTY() {
//	    // Safe to use interactive features
//	} else {
//	    // Output is being redirected, use plain text
//	}
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// TerminalWidth returns the current terminal width in characters.
// Falls back to DefaultTerminalWidth (80) if the width cannot be determined.
//
// Usage:
//
//	width := TerminalWidth()
//	// Use width for responsive layout calculations
//
// Common terminal widths:
//   - 80: Traditional terminal width
//   - 120-160: Modern wide terminals
func TerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return DefaultTerminalWidth
	}
	return width
}
