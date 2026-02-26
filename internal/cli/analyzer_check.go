package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/analyzer"
)

// runAnalyzerChecks is overridable in tests to isolate CLI rendering behavior.
//
//nolint:gochecknoglobals // Test seam for command behavior verification.
var runAnalyzerChecks = analyzer.RunChecks

// NewAnalyzerCheckCmd creates the analyzer check command.
func NewAnalyzerCheckCmd() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Verify analyzer setup health",
		Long: `Runs diagnostics for the local analyzer installation.

Checks include:
  - policy pack directory presence
  - PulumiPolicy.yaml validity
  - analyzer binary availability in PATH
  - gRPC startup and handshake`,
		Example: `  # Run checks with table output
  finfocus analyzer check

  # Machine-readable output
  finfocus analyzer check --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAnalyzerCheck(cmd.Context(), cmd, outputFormat)
		},
	}

	cmd.Flags().StringVar(&outputFormat, "output", "table", "Output format: table or json")

	return cmd
}

func runAnalyzerCheck(ctx context.Context, cmd *cobra.Command, outputFormat string) error {
	report, err := runAnalyzerChecks(ctx)
	if err != nil {
		return fmt.Errorf("running analyzer checks: %w", err)
	}

	switch outputFormat {
	case "json":
		if renderErr := renderCheckJSON(cmd, report); renderErr != nil {
			return renderErr
		}
	case "table":
		renderCheckTable(cmd, report)
	default:
		return fmt.Errorf("unsupported output format: %s (supported: table, json)", outputFormat)
	}

	if !report.AllPass {
		return errors.New("one or more analyzer checks failed")
	}

	return nil
}

func renderCheckJSON(cmd *cobra.Command, report *analyzer.CheckReport) error {
	data, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshaling check report: %w", err)
	}
	cmd.Println(string(data))
	return nil
}

func renderCheckTable(cmd *cobra.Command, report *analyzer.CheckReport) {
	cmd.Println("Analyzer check results:")
	for _, check := range report.Checks {
		status := strings.ToUpper(check.Status)
		cmd.Printf("  %-24s %-6s %s\n", check.DisplayName, status, check.Message)
		if check.Remediation != "" {
			cmd.Printf("    Remediation: %s\n", check.Remediation)
		}
	}

	if report.AllPass {
		cmd.Println("All checks passed.")
		return
	}

	cmd.Println("One or more checks failed.")
}
