package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/conformance"
	"github.com/rshade/finfocus/internal/logging"
)

// Output format constants.
const (
	outputFormatTable  = "table"
	outputFormatJSON   = "json"
	outputFormatNDJSON = "ndjson"
	outputFormatJUnit  = "junit"
)

// Exit codes for conformance test results.
const (
	exitCodeFailures = 1
	exitCodeErrors   = 2
)

// NewPluginConformanceCmd creates the plugin conformance command for running
// NewPluginConformanceCmd returns a Cobra command configured to run conformance tests against a plugin binary.
// The command verifies a plugin's protocol compliance and supports the following flags:
// --mode (tcp|stdio), --verbosity (quiet|normal|verbose|debug), --output (table|json|junit), --output-file,
// --timeout, --category (repeatable: protocol, error, performance, context), and --filter (regex for test names).
func NewPluginConformanceCmd() *cobra.Command {
	var (
		mode       string
		verbosity  string
		output     string
		outputFile string
		timeout    string
		categories []string
		filter     string
	)

	cmd := &cobra.Command{
		Use:   "conformance <plugin-path>",
		Short: "Run conformance tests against a plugin binary",
		Long: `Run conformance tests against a plugin binary to verify protocol compliance.

The conformance suite validates that a plugin correctly implements the FinFocus
gRPC protocol. It tests protocol compliance, error handling, timeout behavior,
and context cancellation.`,
		Example: `  # Basic conformance check
  finfocus plugin conformance ./plugins/aws-cost

  # Verbose output with JSON
  finfocus plugin conformance --verbosity verbose --output json ./plugins/aws-cost

  # Filter to protocol tests only
  finfocus plugin conformance --category protocol ./plugins/aws-cost

  # JUnit XML for CI
  finfocus plugin conformance --output junit --output-file report.xml ./plugins/aws-cost

  # Use stdio mode
  finfocus plugin conformance --mode stdio ./plugins/aws-cost`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPluginConformanceCmd(
				cmd, args[0], mode, verbosity, output, outputFile, timeout, categories, filter,
			)
		},
	}

	cmd.Flags().StringVar(&mode, "mode", "tcp", "Communication mode: tcp, stdio")
	cmd.Flags().
		StringVar(&verbosity, "verbosity", "normal", "Output detail: quiet, normal, verbose, debug")
	cmd.Flags().StringVar(&output, "output", "table", "Output format: table, json, junit")
	cmd.Flags().StringVar(&outputFile, "output-file", "", "Write output to file (default: stdout)")
	cmd.Flags().StringVar(&timeout, "timeout", "5m", "Global suite timeout")
	cmd.Flags().StringSliceVar(
		&categories, "category", nil, "Filter by category (repeatable): protocol, error, performance, context",
	)
	cmd.Flags().StringVar(&filter, "filter", "", "Regex filter for test names")

	return cmd
}

func runPluginConformanceCmd(
	cmd *cobra.Command,
	pluginPath, mode, verbosity, output, outputFile, timeout string,
	categories []string,
	filter string,
) error {
	ctx := cmd.Context()

	// Validate inputs and create suite config
	cfg, err := buildSuiteConfig(ctx, pluginPath, mode, verbosity, timeout, categories, filter)
	if err != nil {
		return err
	}

	// Validate output format
	if output != outputFormatTable && output != outputFormatJSON && output != outputFormatJUnit {
		return fmt.Errorf("invalid output format %q: must be table, json, or junit", output)
	}

	// Create and run suite
	suite, err := conformance.NewSuite(cfg)
	if err != nil {
		return fmt.Errorf("creating conformance suite: %w", err)
	}

	report, err := suite.Run(ctx)
	if err != nil {
		return fmt.Errorf("running conformance suite: %w", err)
	}

	// Write output
	if writeErr := writeReport(cmd, report, output, outputFile); writeErr != nil {
		return writeErr
	}

	// Return exit code based on results
	return checkResults(report)
}

// buildSuiteConfig validates inputs and creates a SuiteConfig.
func buildSuiteConfig(
	ctx context.Context,
	pluginPath, mode, verbosity, timeout string,
	categories []string,
	filter string,
) (conformance.SuiteConfig, error) {
	// Validate plugin path exists
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return conformance.SuiteConfig{}, fmt.Errorf("plugin not found: %s", pluginPath)
	}

	// Parse timeout
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return conformance.SuiteConfig{}, fmt.Errorf("invalid timeout: %w", err)
	}

	// Validate mode
	if !conformance.IsValidCommMode(mode) {
		return conformance.SuiteConfig{}, fmt.Errorf("invalid mode %q: must be tcp or stdio", mode)
	}

	// Validate verbosity
	if !conformance.IsValidVerbosity(verbosity) {
		return conformance.SuiteConfig{}, fmt.Errorf(
			"invalid verbosity %q: must be quiet, normal, verbose, or debug", verbosity,
		)
	}

	// Validate categories
	categoryList, err := parseCategories(categories)
	if err != nil {
		return conformance.SuiteConfig{}, err
	}

	// Get logger from context
	logger := logging.FromContext(ctx)

	return conformance.SuiteConfig{
		PluginPath: pluginPath,
		CommMode:   conformance.CommMode(mode),
		Verbosity:  conformance.Verbosity(verbosity),
		Timeout:    timeoutDuration,
		Categories: categoryList,
		TestFilter: filter,
		Logger:     *logger,
	}, nil
}

// parseCategories validates the provided category strings and converts them to
// conformance.Category values. It returns a slice of converted categories or an
// error if any input is not one of: "protocol", "error", "performance", or
// "context".
//
// The returned slice preserves the order of the input slice.
func parseCategories(categories []string) ([]conformance.Category, error) {
	var categoryList []conformance.Category
	for _, cat := range categories {
		if !conformance.IsValidCategory(cat) {
			return nil, fmt.Errorf(
				"invalid category %q: must be protocol, error, performance, or context",
				cat,
			)
		}
		categoryList = append(categoryList, conformance.Category(cat))
	}
	return categoryList, nil
}

// writeReport writes the conformance SuiteReport to the specified destination using the requested format.
// It selects an output writer based on outputFile (writes to stdout when empty) and writes the report
// in the given output format (`"table"`, `"json"`, or `"junit"`). The cmd parameter is used to obtain
// the command's stdout when no output file is provided.
//
// Parameters:
//   - cmd: the Cobra command used to determine standard output when outputFile is empty.
//   - report: the conformance.SuiteReport to be written.
//   - output: the desired output format; recognized values are "table", "json", and "junit".
//   - outputFile: optional path to a file to write the output; if empty, stdout is used.
//
// Returns an error if acquiring the output writer fails or if writing the report in the chosen format fails.
func writeReport(
	cmd *cobra.Command,
	report *conformance.SuiteReport,
	output, outputFile string,
) error {
	writer, cleanup, err := getOutputWriter(cmd, outputFile)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	switch output {
	case outputFormatJSON:
		if writeErr := report.WriteJSON(writer); writeErr != nil {
			return fmt.Errorf("writing JSON output: %w", writeErr)
		}
	case outputFormatJUnit:
		if writeErr := report.WriteJUnit(writer); writeErr != nil {
			return fmt.Errorf("writing JUnit output: %w", writeErr)
		}
	default:
		if writeErr := report.WriteTable(writer); writeErr != nil {
			return fmt.Errorf("writing table output: %w", writeErr)
		}
	}
	return nil
}

// getOutputWriter returns the writer to use for command output and a cleanup function that closes
// any created file.
//
// If outputFile is empty, the command's standard output writer is returned and the cleanup is nil.
// If outputFile is provided, the file is created (or truncated) and a cleanup function that closes
// the file is returned. An error is returned if the file cannot be created.
func getOutputWriter(cmd *cobra.Command, outputFile string) (io.Writer, func(), error) {
	if outputFile == "" {
		return cmd.OutOrStdout(), nil, nil
	}

	f, err := os.Create(outputFile)
	if err != nil {
		return nil, nil, fmt.Errorf("creating output file: %w", err)
	}
	return f, func() {
		if closeErr := f.Close(); closeErr != nil {
			// Log but don't fail - we've already written output
			_, _ = fmt.Fprintf(
				cmd.ErrOrStderr(),
				"warning: failed to close output file: %v\n",
				closeErr,
			)
		}
	}, nil
}

// checkResults returns an error if there are failures or errors for exit code handling.
func checkResults(report *conformance.SuiteReport) error {
	if report.Summary.Failed > 0 {
		return &exitError{code: exitCodeFailures, message: "conformance tests failed"}
	}
	if report.Summary.Errors > 0 {
		return &exitError{code: exitCodeErrors, message: "conformance tests encountered errors"}
	}
	return nil
}

// exitError represents an error that should result in a specific exit code.
type exitError struct {
	code    int
	message string
}

func (e *exitError) Error() string {
	return e.message
}

// ExitCode returns the exit code for this error.
func (e *exitError) ExitCode() int {
	return e.code
}
