package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/conformance"
	"github.com/rshade/finfocus/internal/logging"
)

// Certification timeout and output constants.
const (
	certificationTimeout     = 10 * time.Minute
	certificationOutputFile  = "certification.md"
	certificationExitFailure = 1
)

// NewPluginCertifyCmd creates the plugin certify command for running certification
// NewPluginCertifyCmd returns a Cobra command that runs the conformance suite against a plugin binary and generates a Markdown certification report.
// The command accepts a single plugin path argument and provides flags to control output and execution:
//   - --output, -o: path to write the certification report (default: stdout)
//   - --mode: communication mode, either "tcp" or "stdio" (default: "tcp")
//   - --timeout: global certification timeout as a duration string (default: "10m")
//
// The command prints progress and a summary; it exits with a non-zero code when certification is not achieved.
func NewPluginCertifyCmd() *cobra.Command {
	var (
		outputFile string
		mode       string
		timeout    string
	)

	cmd := &cobra.Command{
		Use:   "certify <plugin-path>",
		Short: "Run certification tests for a plugin",
		Long: `Runs the full conformance suite and generates a certification report.

Certification requires 100% of conformance tests to pass. The command outputs
a markdown certification report that can be distributed with your plugin.

A plugin is certified if:
- All protocol tests pass (Name, GetProjectedCost, GetActualCost)
- All error handling tests pass
- All context/timeout tests pass
- All performance tests pass`,
		Example: `  # Basic certification
  finfocus plugin certify ./plugins/aws-cost

  # Save report to custom file
  finfocus plugin certify --output certification-report.md ./plugins/aws-cost

  # Use stdio mode
  finfocus plugin certify --mode stdio ./plugins/aws-cost`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginCertifyCmd(cmd, args[0], outputFile, mode, timeout)
		},
	}

	cmd.Flags().StringVarP(
		&outputFile, "output", "o", "",
		"Output file for certification report (default: stdout)",
	)
	cmd.Flags().StringVar(&mode, "mode", "tcp", "Communication mode: tcp, stdio")
	cmd.Flags().StringVar(&timeout, "timeout", "10m", "Global certification timeout")

	return cmd
}

// runPluginCertifyCmd runs the conformance test suite against the plugin at pluginPath,
// produces a certification report in Markdown, and writes the report to outputFile or stdout.
//
// The cmd parameter supplies the command context and I/O helpers used for printing.
// pluginPath is the filesystem path to the plugin to certify.
// outputFile, if non-empty, is the path to write the generated Markdown report; if empty,
// the report is printed to stdout.
// mode selects the communication mode with the plugin ("tcp" or "stdio").
// timeout is a duration string (e.g., "10m") that sets the global timeout for the suite.
//
// Returns nil on successful certification (all tests passed).
// Returns a non-nil error if the plugin path does not exist, the timeout cannot be parsed,
// the communication mode is invalid, suite creation or execution fails, or writing the report fails.
// If the suite runs but the plugin is not certified, an exitError with code 1 and message
// "certification failed" is returned.
func runPluginCertifyCmd(
	cmd *cobra.Command,
	pluginPath, outputFile, mode, timeout string,
) error {
	ctx := cmd.Context()

	// Validate plugin path exists
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return fmt.Errorf("plugin not found: %s", pluginPath)
	}

	// Parse timeout
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout: %w", err)
	}

	// Validate mode
	if !conformance.IsValidCommMode(mode) {
		return fmt.Errorf("invalid mode %q: must be tcp or stdio", mode)
	}

	cmd.Printf("🔍 Certifying plugin at %s...\n", pluginPath)

	// Get logger from context
	logger := logging.FromContext(ctx)

	// Create suite config - run ALL tests for certification
	cfg := conformance.SuiteConfig{
		PluginPath: pluginPath,
		CommMode:   conformance.CommMode(mode),
		Verbosity:  conformance.VerbosityNormal,
		Timeout:    timeoutDuration,
		Categories: nil, // Run all categories
		TestFilter: "",  // No filter - run all tests
		Logger:     *logger,
	}

	// Create and run suite
	suite, err := conformance.NewSuite(cfg)
	if err != nil {
		return fmt.Errorf("creating conformance suite: %w", err)
	}

	cmd.Println("Running conformance tests...")

	report, err := suite.Run(ctx)
	if err != nil {
		return fmt.Errorf("running conformance suite: %w", err)
	}

	// Generate certification report
	certReport := conformance.Certify(report, report.Plugin.Name, report.Plugin.Version)

	// Generate markdown
	markdown := certReport.GenerateMarkdown()

	// Write output with relaxed permissions for shareable reports
	if outputFile != "" {
		//nolint:gosec // G306: Intentionally using 0o644 for shareable certification reports
		if writeErr := os.WriteFile(outputFile, []byte(markdown), 0o644); writeErr != nil {
			return fmt.Errorf("writing certification report: %w", writeErr)
		}
		cmd.Printf("📄 Certification report written to %s\n", outputFile)
	} else {
		cmd.Println()
		cmd.Println(markdown)
	}

	// Print summary
	if certReport.Certified {
		cmd.Println("✅ CERTIFIED - Plugin passed all conformance tests")
		return nil
	}

	cmd.Println("❌ NOT CERTIFIED - Plugin failed conformance tests")
	cmd.Printf("   Failed: %d, Errors: %d\n", report.Summary.Failed, report.Summary.Errors)

	return &exitError{
		code:    certificationExitFailure,
		message: "certification failed",
	}
}
