// Package tui provides a shared set of terminal user interface (TUI) components
// and utilities for consistent CLI command styling across the FinFocus codebase.
//
// This package offers:
//
//   - Output mode detection for different terminal environments
//   - Predefined color schemes and Lip Gloss styles
//   - Reusable UI components (progress bars, status indicators, etc.)
//   - Money and percentage formatting utilities
//   - TTY detection and terminal capability checking
//
// # Basic Usage
//
// Import the package and use components for consistent CLI output:
//
//	import "github.com/rshade/finfocus/internal/tui"
//
//	// Detect output mode
//	mode := tui.DetectOutputMode(forceColor, noColor, plain)
//
//	// Use components
//	status := tui.RenderStatus("ok")
//	progress := tui.DefaultProgressBar().Render(75.0)
//	cost := tui.FormatMoneyShort(1234.56)
//
//	// Create a spinner for loading states
//	spinner := tui.DefaultSpinner()
//
//	// Create a table with consistent styling
//	columns := []table.Column{{Title: "Name", Width: 20}, {Title: "Cost", Width: 10}}
//	rows := []table.Row{{"EC2 Instance", "$25.00"}, {"S3 Bucket", "$1.50"}}
//	tbl := tui.NewTable(columns, rows, 5)
//
// # Output Modes
//
// The package supports three output modes based on terminal capabilities:
//
//   - OutputModePlain: Basic text output without ANSI styling
//   - OutputModeStyled: ANSI colors and formatting for enhanced readability
//   - OutputModeInteractive: Full Bubble Tea TUI (reserved for future use)
//
// Use DetectOutputMode() to automatically determine the appropriate mode.
//
// # Color Scheme
//
// The package defines a consistent color palette:
//
//   - ColorOK: Green (#5fd700) for success states
//   - ColorWarning: Orange (#ff8700) for warnings
//   - ColorCritical: Red (#ff0000) for errors
//   - ColorInfo: Blue (#0087ff) for information
//   - Additional colors for headers, labels, values, borders, etc.
//
// # Components
//
// Reusable UI components include:
//
//   - ProgressBar: Visual progress indicators with color coding
//   - Spinner: Loading spinner with standard styling (via DefaultSpinner)
//   - Table: Data tables with standard headers and selection styles (via NewTable/DefaultTableStyles)
//   - RenderStatus(): Status messages with icons and colors
//   - RenderDelta(): Cost change indicators with directional arrows
//   - RenderPriority(): Priority level indicators
//
// # Formatting Utilities
//
// Text formatting functions:
//
//   - FormatMoney(): Full currency formatting with thousands separators
//   - FormatMoneyShort(): Currency amount without currency name
//   - FormatPercent(): Percentage display with proper rounding
//
// # Best Practices
//
// 1. Always call DetectOutputMode() early in CLI commands
// 2. Respect user preferences (NO_COLOR, --no-color, --plain flags)
// 3. Provide plain text fallbacks for all styled output
// 4. Use predefined styles and colors for consistency
// 5. Test components in different terminal environments
//
// # Thread Safety
//
// All exported functions and methods are safe for concurrent use.
// No global mutable state is used in the package.
//
// # Dependencies
//
// This package depends on:
//   - charm.land/lipgloss/v2 for styling
//   - charm.land/bubbles/v2 for UI components
//   - golang.org/x/term for terminal detection
package tui
