package cli

import (
	"context"
	"fmt"
	"io"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/tui"
)

// RenderCostOutput routes the cost results to the appropriate rendering function
// based on the detected output mode (Plain, Styled, or Interactive).
// The context parameter is reserved for future use (e.g., cancellation, tracing)
// but is currently unused to maintain API compatibility.
func RenderCostOutput(
	_ context.Context,
	cmd *cobra.Command,
	outputFormat string,
	resultWithErrors *engine.CostResultWithErrors,
) error {
	// 1. Determine and validate output format.
	fmtType := engine.OutputFormat(config.GetOutputFormat(outputFormat))

	// Validate format is supported before proceeding
	if !isValidOutputFormat(fmtType) {
		return fmt.Errorf("unsupported output format: %s", fmtType)
	}

	// 2. If output format is explicitly structured (JSON/NDJSON), bypass TUI completely.
	// This satisfies FR-004: Maintain output for --output json/ndjson.
	if fmtType == engine.OutputJSON || fmtType == engine.OutputNDJSON {
		return engine.RenderResults(cmd.OutOrStdout(), fmtType, resultWithErrors.Results)
	}

	// 2. Detect the appropriate output mode for the terminal.
	// We rely on standard detection (flags passed as false for now, as they aren't global yet).
	// Future improvement: plumb --no-color / --plain flags if added to CLI.
	mode := tui.DetectOutputMode(false, false, false)

	// 3. Route to specific renderer
	switch mode {
	case tui.OutputModeInteractive:
		return runInteractiveTUI(resultWithErrors)

	case tui.OutputModeStyled:
		return renderStyledOutput(cmd.OutOrStdout(), resultWithErrors)

	case tui.OutputModePlain:
		return renderPlainOutput(cmd.OutOrStdout(), resultWithErrors)

	default:
		return renderPlainOutput(cmd.OutOrStdout(), resultWithErrors)
	}
}

// RenderActualCostOutput routes actual cost results to the appropriate rendering function.
// The context parameter is reserved for future use (e.g., cancellation, tracing)
// but is currently unused to maintain API compatibility.
func RenderActualCostOutput(
	_ context.Context,
	cmd *cobra.Command,
	outputFormat string,
	resultWithErrors *engine.CostResultWithErrors,
	groupBy string,
	estimateConfidence bool,
) error {
	fmtType := engine.OutputFormat(config.GetOutputFormat(outputFormat))

	// Validate format is supported before proceeding
	if !isValidOutputFormat(fmtType) {
		return fmt.Errorf("unsupported output format: %s", fmtType)
	}

	if fmtType == engine.OutputJSON || fmtType == engine.OutputNDJSON {
		// Use existing logic for JSON/NDJSON (handling aggregation inside)
		return renderActualCostOutput(cmd.OutOrStdout(), fmtType, resultWithErrors.Results, groupBy, estimateConfidence)
	}

	mode := tui.DetectOutputMode(false, false, false)
	switch mode {
	case tui.OutputModeInteractive:
		return runInteractiveActualCostTUI(resultWithErrors, engine.GroupBy(groupBy))

	case tui.OutputModeStyled, tui.OutputModePlain:
		fallthrough
	default:
		if err := renderActualCostOutput(cmd.OutOrStdout(), engine.OutputTable, resultWithErrors.Results, groupBy, estimateConfidence); err != nil {
			return err
		}
		displayErrorSummary(cmd, resultWithErrors, engine.OutputTable)
		return nil
	}
}

func runInteractiveTUI(resultWithErrors *engine.CostResultWithErrors) error {
	p := tea.NewProgram(tui.NewCostViewModel(resultWithErrors.Results))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run interactive TUI: %w", err)
	}
	return nil
}

func runInteractiveActualCostTUI(resultWithErrors *engine.CostResultWithErrors, groupBy engine.GroupBy) error {
	p := tea.NewProgram(tui.NewCostViewModelFromActual(resultWithErrors.Results, groupBy))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run interactive TUI: %w", err)
	}
	return nil
}

// renderPlainOutput renders the standard table output (legacy behavior).
func renderPlainOutput(w io.Writer, resultWithErrors *engine.CostResultWithErrors) error {
	if err := engine.RenderResults(w, engine.OutputTable, resultWithErrors.Results); err != nil {
		return err
	}

	if resultWithErrors.HasErrors() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "ERRORS")
		fmt.Fprintln(w, "======")
		fmt.Fprint(w, resultWithErrors.ErrorSummary())
	}
	return nil
}

// renderStyledOutput renders the styled summary using Lip Gloss (T011).
func renderStyledOutput(w io.Writer, resultWithErrors *engine.CostResultWithErrors) error {
	summary := tui.RenderCostSummary(resultWithErrors.Results, tui.TerminalWidth())
	fmt.Fprint(w, summary)

	// Display error summary using plain text format.
	// Error styling is intentionally kept simple for readability across terminals.
	if resultWithErrors.HasErrors() {
		fmt.Fprintln(w)
		fmt.Fprint(w, resultWithErrors.ErrorSummary())
	}

	return nil
}

// isValidOutputFormat checks if the provided format is one of the supported output formats.
func isValidOutputFormat(format engine.OutputFormat) bool {
	switch format {
	case engine.OutputTable, engine.OutputJSON, engine.OutputNDJSON:
		return true
	default:
		return false
	}
}
