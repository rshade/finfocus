package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/router"
	"github.com/rshade/finfocus/internal/tui"
)

const configOutputText = "text"

// errConfigurationInvalid is returned after a validation report has been written.
var errConfigurationInvalid = errors.New("configuration validation failed")

// NewConfigValidateCmd returns a Cobra command that validates the application's configuration.
// The command checks syntax, budget rules, unknown fields, and routing semantics.
// --file selects a document, --output json emits the report for CI, and --verbose prints loaded settings.
func NewConfigValidateCmd() *cobra.Command {
	var verbose bool
	var file string
	var output string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration file",
		Long: `Validates the active ~/.finfocus/config.hujson file, or the file given by --file.

The report includes:
- Syntax errors, with line numbers
- Budget amount, currency, period, alert, and exit-code checks
- Unknown fields, with a "Did you mean?" suggestion when a known field is close
- Routing configuration, when plugins can be loaded`,
		Example: `  # Validate the active configuration
  finfocus config validate

  # Validate a file and emit JSON for CI
  finfocus config validate --file ./config.hujson --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			output = resolveOutputFormat(cmd, "output", output)
			if output != configOutputText && output != outputFormatJSON {
				return fmt.Errorf("invalid --output %q (must be text or json)", output)
			}
			return runConfigValidate(cmd, file, output, verbose)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show detailed validation information")
	cmd.Flags().StringVar(&file, "file", "", "configuration file to validate (default: active config)")
	cmd.Flags().StringVar(&output, "output", configOutputText, "output format: text or json")
	return cmd
}

// runConfigValidate reads one configuration file and writes a validation report.
func runConfigValidate(cmd *cobra.Command, file, output string, verbose bool) error {
	path, data, missingDefault, err := readConfigForValidate(file)
	if err != nil {
		return err
	}
	if missingDefault {
		result := config.ValidateConfig(config.New())
		return finishConfigValidate(cmd, result, config.New(), output, verbose)
	}
	result, cfg := validateConfigDocument(path, data)
	return finishConfigValidate(cmd, result, cfg, output, verbose)
}

// validateConfigDocument validates data, applying the project config rules when
// path is the resolved project's config.hujson.
func validateConfigDocument(path string, data []byte) (config.ValidationResult, *config.Config) {
	if isProjectConfigFile(path) {
		return config.ValidateProjectConfigSource(path, data)
	}
	return config.ValidateConfigSource(path, data)
}

// isProjectConfigFile reports whether path is the config.hujson of the project
// directory resolved for this run.
func isProjectConfigFile(path string) bool {
	dir := config.GetResolvedProjectDir()
	if dir == "" || path == "" {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return absPath == filepath.Join(dir, "config.hujson")
}

func readConfigForValidate(file string) (string, []byte, bool, error) {
	path := file
	if path == "" {
		path = config.New().ConfigPath()
	}
	data, err := os.ReadFile(path)
	if err == nil {
		return path, data, false, nil
	}
	if file == "" && errors.Is(err, fs.ErrNotExist) {
		return path, nil, true, nil
	}
	return "", nil, false, fmt.Errorf("reading configuration: %w", err)
}

func finishConfigValidate(
	cmd *cobra.Command,
	result config.ValidationResult,
	cfg *config.Config,
	output string,
	verbose bool,
) error {
	if cfg != nil && len(result.Errors) == 0 {
		mergeRoutingIssues(cmd, cfg, &result)
	}
	result.Valid = len(result.Errors) == 0
	if err := writeValidationReport(cmd, result, output, cmd.OutOrStdout()); err != nil {
		return err
	}
	if verbose && result.Valid && output == configOutputText && cfg != nil {
		printVerboseDetails(cmd, cfg)
	}
	if !result.Valid {
		return errConfigurationInvalid
	}
	return nil
}

func mergeRoutingIssues(cmd *cobra.Command, cfg *config.Config, result *config.ValidationResult) {
	warnings, problems := collectRoutingIssues(cmd, cfg)
	for _, problem := range problems {
		result.Errors = append(result.Errors, config.ValidationError{
			Path:    "routing",
			Message: problem,
			Hint:    "Check plugin names, patterns, and features.",
		})
	}
	for _, warning := range warnings {
		result.Warnings = append(result.Warnings, config.ValidationWarning{
			Path:    "routing",
			Message: warning,
		})
	}
}

func writeValidationReport(cmd *cobra.Command, result config.ValidationResult, output string, dest io.Writer) error {
	if output == outputFormatJSON || machineOutputRequested(cmd) {
		return writeJSON(cmd, result)
	}
	text := tui.RenderValidationReport(validationView(result), validationPlain(dest))
	if _, err := io.WriteString(dest, text); err != nil {
		return fmt.Errorf("writing validation report: %w", err)
	}
	return nil
}

func validationView(result config.ValidationResult) tui.ValidationView {
	view := tui.ValidationView{
		File:     result.File,
		Valid:    result.Valid,
		Errors:   make([]tui.ValidationViewItem, 0, len(result.Errors)),
		Warnings: make([]tui.ValidationViewItem, 0, len(result.Warnings)),
	}
	for _, item := range result.Errors {
		view.Errors = append(view.Errors, tui.ValidationViewItem{
			Line: item.Line, Path: item.Path, Message: item.Message, Hint: item.Hint, Example: item.Example,
		})
	}
	for _, item := range result.Warnings {
		view.Warnings = append(view.Warnings, tui.ValidationViewItem{
			Line: item.Line, Path: item.Path, Message: item.Message, Suggestion: item.Suggestion,
		})
	}
	return view
}

func validationPlain(dest io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	file, ok := dest.(*os.File)
	if !ok {
		return true
	}
	return !isTerminal(file)
}

// validateCostConfig reports configuration errors for the cost commands.
// Missing files are ignored. Warnings do not fail the command.
func validateCostConfig(cmd *cobra.Command, paths []string) error {
	seen := map[string]struct{}{}
	failed := false
	for _, path := range paths {
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		data, err := os.ReadFile(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("reading configuration: %w", err)
		}
		result, _ := validateConfigDocument(path, data)
		if result.Valid {
			continue
		}
		failed = true
		output := configOutputText
		if machineOutputRequested(cmd) {
			output = outputFormatJSON
		}
		if writeErr := writeValidationReport(cmd, result, output, cmd.ErrOrStderr()); writeErr != nil {
			return writeErr
		}
	}
	if failed {
		return errConfigurationInvalid
	}
	return nil
}

func costConfigPaths(cfg *config.Config) []string {
	if cfg == nil {
		return nil
	}
	paths := []string{cfg.ConfigPath()}
	if dir := config.GetResolvedProjectDir(); dir != "" {
		projectFile := filepath.Join(dir, "config.hujson")
		if projectFile != cfg.ConfigPath() {
			paths = append(paths, projectFile)
		}
	}
	return paths
}

// collectRoutingIssues checks routing rules against installed plugins.
// A plugin-load failure is printed and validation continues with the configured names.
func collectRoutingIssues(cmd *cobra.Command, cfg *config.Config) ([]string, []string) {
	if cfg == nil || cfg.Routing == nil {
		return nil, nil
	}

	reg := registry.NewDefault()
	clients, cleanup, err := reg.Open(cmd.Context(), "")
	if err != nil {
		cmd.PrintErrln("Warning: Could not load plugins for validation:", err)
		clients = make([]*pluginhost.Client, 0, len(cfg.Routing.Plugins))
		for _, p := range cfg.Routing.Plugins {
			clients = append(clients, &pluginhost.Client{Name: p.Name})
		}
	} else {
		defer cleanup()
	}

	result := router.ValidateRoutingConfig(cfg.Routing, clients)
	var problems []string
	if result.HasErrors() {
		problems = make([]string, 0, len(result.Errors))
		for _, issue := range result.Errors {
			problems = append(problems, issue.Error())
		}
	}
	if result.HasWarnings() {
		return result.WarningMessages(), problems
	}
	return nil, problems
}

// printVerboseDetails prints detailed configuration information to the command's output.
func printVerboseDetails(cmd *cobra.Command, cfg *config.Config) {
	cmd.Println()
	cmd.Println("Configuration details:")
	cmd.Printf("  Output format: %s\n", cfg.Output.DefaultFormat)
	cmd.Printf("  Output precision: %d\n", cfg.Output.Precision)
	cmd.Printf("  Logging level: %s\n", cfg.Logging.Level)
	cmd.Printf("  Log file: %s\n", cfg.Logging.File)

	printPluginDetails(cmd, cfg)
	printRoutingDetails(cmd, cfg)
}

// printPluginDetails writes a summary of configured plugins to the command output.
func printPluginDetails(cmd *cobra.Command, cfg *config.Config) {
	if len(cfg.Plugins) > 0 {
		cmd.Printf("  Configured plugins: %d\n", len(cfg.Plugins))
		names := make([]string, 0, len(cfg.Plugins))
		for name := range cfg.Plugins {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, pluginName := range names {
			cmd.Printf("    - %s\n", pluginName)
		}
	} else {
		cmd.Println("  No plugins configured")
	}
}

// printRoutingDetails writes a summary of the routing configuration to the command's output.
func printRoutingDetails(cmd *cobra.Command, cfg *config.Config) {
	if cfg.Routing == nil || len(cfg.Routing.Plugins) == 0 {
		cmd.Println("  No routing rules configured (automatic routing)")
		return
	}

	cmd.Printf("  Routing rules: %d\n", len(cfg.Routing.Plugins))
	for _, p := range cfg.Routing.Plugins {
		cmd.Printf("    - %s (priority: %d", p.Name, p.Priority)
		if len(p.Features) > 0 {
			cmd.Printf(", features: %v", p.Features)
		}
		if len(p.Patterns) > 0 {
			cmd.Printf(", patterns: %d", len(p.Patterns))
		}
		cmd.Println(")")
	}
}
