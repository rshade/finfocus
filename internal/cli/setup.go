package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/analyzer"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/pkg/version"
)

// DefaultPlugins is the set of plugins installed by default during setup.
//
//nolint:gochecknoglobals // Intentionally global for discoverability
var DefaultPlugins = []string{
	"aws-public",
}

// StepStatus represents the outcome of a single setup step.
//
//nolint:recvcheck // UnmarshalJSON requires pointer receiver; String/MarshalJSON use value receivers.
type StepStatus int

const (
	// StepSuccess indicates the step completed successfully.
	StepSuccess StepStatus = iota
	// StepWarning indicates the step completed with a non-fatal issue.
	StepWarning
	// StepSkipped indicates the step was intentionally skipped via flag.
	StepSkipped
	// StepError indicates the step failed.
	StepError
)

// String returns the human-readable label for a StepStatus.
func (s StepStatus) String() string {
	switch s {
	case StepSuccess:
		return "success"
	case StepWarning:
		return "warning"
	case StepSkipped:
		return "skipped"
	case StepError:
		return "error"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// MarshalJSON implements json.Marshaler to output StepStatus as a string.
func (s StepStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON implements json.Unmarshaler to parse StepStatus from a string.
func (s *StepStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("parsing step status: %w", err)
	}
	switch str {
	case "success":
		*s = StepSuccess
	case "warning":
		*s = StepWarning
	case "skipped":
		*s = StepSkipped
	case "error":
		*s = StepError
	default:
		return fmt.Errorf("unknown step status: %q", str)
	}
	return nil
}

// StepResult describes the outcome of executing a single setup step.
type StepResult struct {
	Name     string     `json:"name"               yaml:"name"`
	Status   StepStatus `json:"status"             yaml:"status"`
	Message  string     `json:"message"            yaml:"message"`
	Critical bool       `json:"critical,omitempty" yaml:"critical,omitempty"`
	Err      error      `json:"-"                  yaml:"-"`
}

// SetupOptions holds the configuration for the setup command, derived from CLI flags.
type SetupOptions struct {
	SkipAnalyzer   bool `json:"skipAnalyzer,omitempty"   yaml:"skipAnalyzer,omitempty"`
	SkipPlugins    bool `json:"skipPlugins,omitempty"    yaml:"skipPlugins,omitempty"`
	NonInteractive bool `json:"nonInteractive,omitempty" yaml:"nonInteractive,omitempty"`
}

// SetupResult is the aggregate outcome of all setup steps.
type SetupResult struct {
	Steps       []StepResult `json:"steps,omitempty"       yaml:"steps,omitempty"`
	HasErrors   bool         `json:"hasErrors,omitempty"   yaml:"hasErrors,omitempty"`
	HasWarnings bool         `json:"hasWarnings,omitempty" yaml:"hasWarnings,omitempty"`
}

// AnalyzerInstaller is the interface for installing the Pulumi analyzer.
type AnalyzerInstaller interface {
	Install(ctx context.Context, opts analyzer.InstallOptions) (*analyzer.InstallResult, error)
}

// AnalyzerInstallerFunc is a function adapter for AnalyzerInstaller.
type AnalyzerInstallerFunc func(ctx context.Context, opts analyzer.InstallOptions) (*analyzer.InstallResult, error)

// Install delegates to the underlying function.
func (f AnalyzerInstallerFunc) Install(
	ctx context.Context,
	opts analyzer.InstallOptions,
) (*analyzer.InstallResult, error) {
	return f(ctx, opts)
}

// PluginInstaller is the interface for installing plugins.
type PluginInstaller interface {
	Install(specifier string, opts registry.InstallOptions, progress func(string)) (*registry.InstallResult, error)
}

// PluginInstallerFunc is a function adapter for PluginInstaller.
type PluginInstallerFunc func(specifier string, opts registry.InstallOptions, progress func(string)) (*registry.InstallResult, error)

// Install delegates to the underlying function.
func (f PluginInstallerFunc) Install(
	specifier string,
	opts registry.InstallOptions,
	progress func(string),
) (*registry.InstallResult, error) {
	return f(specifier, opts, progress)
}

// DirPermBase is the permission mode for the base and standard directories.
const DirPermBase = 0o700

// DirPermPlugins is the permission mode for the plugins directory.
const DirPermPlugins = 0o750

// pulumiVersionTimeout is the maximum time to wait for `pulumi version` to respond.
const pulumiVersionTimeout = 5 * time.Second

// SetupRunner holds injectable dependencies for setup steps that require
// external services (analyzer installation, plugin installation).
type SetupRunner struct {
	AnalyzerInstaller AnalyzerInstaller
	PluginInstaller   PluginInstaller
}

// NewSetupRunner returns a SetupRunner wired with production defaults.
func NewSetupRunner() *SetupRunner {
	return &SetupRunner{
		AnalyzerInstaller: AnalyzerInstallerFunc(analyzer.Install),
	}
}

// FormatStatus returns a status marker appropriate for the output mode.
func FormatStatus(status StepStatus, nonInteractive bool) string {
	if nonInteractive {
		switch status {
		case StepSuccess:
			return "[OK]"
		case StepWarning:
			return "[WARN]"
		case StepSkipped:
			return "[SKIP]"
		case StepError:
			return "[ERR]"
		default:
			return "[??]"
		}
	}

	switch status {
	case StepSuccess:
		return "\u2713" // ✓
	case StepWarning:
		return "!"
	case StepSkipped:
		return "-"
	case StepError:
		return "\u2717" // ✗
	default:
		return "?"
	}
}

// NewSetupCmd creates the top-level setup command that bootstraps the FinFocus environment.
func NewSetupCmd() *cobra.Command {
	return NewSetupCmdWithRunner(NewSetupRunner())
}

// NewSetupCmdWithRunner creates the setup command with the given runner for dependency injection.
func NewSetupCmdWithRunner(runner *SetupRunner) *cobra.Command {
	var opts SetupOptions

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Bootstrap the FinFocus environment",
		Long: `Sets up the FinFocus environment by creating directories, initializing
configuration, installing the Pulumi analyzer, and installing default plugins.

This command is idempotent — it is safe to run multiple times. Existing
configuration files are preserved, and already-installed components are
detected without modification.`,
		Example: `  # Full setup
  finfocus setup

  # CI/CD setup (no TTY-dependent output)
  finfocus setup --non-interactive

  # Setup without plugins (offline environments)
  finfocus setup --skip-plugins

  # Setup directories and config only
  finfocus setup --skip-analyzer --skip-plugins`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSetup(cmd, &opts, runner)
		},
	}

	cmd.Flags().BoolVar(&opts.NonInteractive, "non-interactive", false,
		"Disable TTY-dependent output (status symbols, color)")
	cmd.Flags().BoolVar(&opts.SkipAnalyzer, "skip-analyzer", false,
		"Skip Pulumi analyzer installation")
	cmd.Flags().BoolVar(&opts.SkipPlugins, "skip-plugins", false,
		"Skip default plugin installation")

	return cmd
}

// runSetup orchestrates all setup steps using a collect-and-continue pattern.
// Each step is executed sequentially. Failures in one step do not prevent
// subsequent steps from running. The function returns an error only if a
// critical step fails.
func runSetup(cmd *cobra.Command, opts *SetupOptions, runner *SetupRunner) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	traceID := logging.GetOrGenerateTraceID(ctx)
	ctx = logging.ContextWithTraceID(ctx, traceID)

	log := logging.FromContext(ctx)

	// Auto-detect non-interactive mode when stdin is not a TTY
	if !opts.NonInteractive && !isTerminal(os.Stdin) {
		opts.NonInteractive = true
	}

	result := &SetupResult{}

	// Step 1: Display version
	step := StepDisplayVersion()
	printStep(cmd, step, opts.NonInteractive)
	result.Steps = append(result.Steps, step)

	// Step 2: Detect Pulumi
	step = StepDetectPulumi(ctx)
	printStep(cmd, step, opts.NonInteractive)
	result.Steps = append(result.Steps, step)

	// Resolve config directory once for all steps that need it
	baseDir := config.ResolveConfigDir()

	// Step 3: Create directories
	dirSteps := StepCreateDirectories(baseDir)
	for _, s := range dirSteps {
		printStep(cmd, s, opts.NonInteractive)
		result.Steps = append(result.Steps, s)
	}

	// Step 4: Initialize config
	step = StepInitConfig(baseDir)
	printStep(cmd, step, opts.NonInteractive)
	result.Steps = append(result.Steps, step)

	// Step 5: Install analyzer
	if opts.SkipAnalyzer {
		step = StepResult{
			Name:    "Analyzer installation",
			Status:  StepSkipped,
			Message: "Skipped analyzer installation",
		}
	} else {
		step = runner.StepInstallAnalyzer(ctx)
	}
	printStep(cmd, step, opts.NonInteractive)
	result.Steps = append(result.Steps, step)

	// Step 6: Install plugins
	if opts.SkipPlugins {
		step = StepResult{
			Name:    "Plugin installation",
			Status:  StepSkipped,
			Message: "Skipped plugin installation",
		}
		printStep(cmd, step, opts.NonInteractive)
		result.Steps = append(result.Steps, step)
	} else {
		pluginSteps := runner.StepInstallPlugins(baseDir)
		for _, s := range pluginSteps {
			printStep(cmd, s, opts.NonInteractive)
			result.Steps = append(result.Steps, s)
		}
	}

	// Compute aggregate status
	computeAggregateStatus(result)

	// Print summary
	printSummary(cmd, result)

	if result.HasErrors {
		joinedErr := collectCriticalErrors(result)
		log.Error().
			Ctx(ctx).
			Str("component", "setup").
			Err(joinedErr).
			Msg("setup completed with critical errors")
		return fmt.Errorf("setup failed: one or more critical steps failed: %w", joinedErr)
	}

	return nil
}

// printStep outputs a single step's status line.
func printStep(cmd *cobra.Command, step StepResult, nonInteractive bool) {
	marker := FormatStatus(step.Status, nonInteractive)
	cmd.Printf("%s %s\n", marker, step.Message)
}

// printSummary outputs the final completion message.
func printSummary(cmd *cobra.Command, result *SetupResult) {
	cmd.Println()
	if result.HasErrors {
		cmd.Println("Setup completed with errors. Review the messages above for remediation steps.")
	} else {
		cmd.Println("Setup complete! Run 'finfocus cost projected --pulumi-json plan.json' to get started.")
	}
}

// computeAggregateStatus updates the HasErrors and HasWarnings flags on the result.
func computeAggregateStatus(result *SetupResult) {
	for _, s := range result.Steps {
		if s.Status == StepError && s.Critical {
			result.HasErrors = true
		}
		if s.Status == StepWarning {
			result.HasWarnings = true
		}
	}
}

// collectCriticalErrors returns a joined error from all critical step failures.
func collectCriticalErrors(result *SetupResult) error {
	var stepErrs []error
	for _, s := range result.Steps {
		if s.Status == StepError && s.Critical && s.Err != nil {
			stepErrs = append(stepErrs, s.Err)
		}
	}
	return errors.Join(stepErrs...)
}

// pluginHasVersionDir checks if a plugin directory contains at least one
// plausible version subdirectory (a directory starting with "v" and not ".").
func pluginHasVersionDir(pluginDir string) bool {
	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") && strings.HasPrefix(entry.Name(), "v") {
			return true
		}
	}
	return false
}

// StepDisplayVersion prints the FinFocus version and Go runtime info.
func StepDisplayVersion() StepResult {
	ver := version.GetVersion()
	goVer := runtime.Version()
	msg := fmt.Sprintf("FinFocus v%s (%s)", ver, goVer)
	return StepResult{
		Name:    "Version display",
		Status:  StepSuccess,
		Message: msg,
	}
}

// StepDetectPulumi checks if the pulumi CLI is on PATH and reports its version.
func StepDetectPulumi(ctx context.Context) StepResult {
	log := logging.FromContext(ctx)

	_, err := exec.LookPath("pulumi")
	if err != nil {
		log.Debug().
			Ctx(ctx).
			Str("component", "setup").
			Msg("pulumi CLI not found on PATH")
		return StepResult{
			Name:    "Pulumi detection",
			Status:  StepWarning,
			Message: "Pulumi CLI not found on PATH. Install from https://www.pulumi.com/docs/install/",
			Err:     err,
		}
	}

	// Get Pulumi version
	timeoutCtx, cancel := context.WithTimeout(ctx, pulumiVersionTimeout)
	defer cancel()
	out, runErr := exec.CommandContext(timeoutCtx, "pulumi", "version").Output()
	if runErr != nil {
		log.Debug().
			Ctx(ctx).
			Str("component", "setup").
			Err(runErr).
			Msg("failed to get pulumi version")
		return StepResult{
			Name:    "Pulumi detection",
			Status:  StepWarning,
			Message: "Pulumi CLI found but could not determine version",
			Err:     runErr,
		}
	}

	pulumiVer := strings.TrimSpace(string(out))
	return StepResult{
		Name:    "Pulumi detection",
		Status:  StepSuccess,
		Message: fmt.Sprintf("Pulumi CLI detected (%s)", pulumiVer),
	}
}

// StepCreateDirectories creates the required FinFocus directories.
// Returns one StepResult per directory.
func StepCreateDirectories(baseDir string) []StepResult {
	dirs := []struct {
		path string
		perm os.FileMode
		name string
	}{
		{baseDir, DirPermBase, baseDir},
		{filepath.Join(baseDir, "plugins"), DirPermPlugins, filepath.Join(baseDir, "plugins")},
		{filepath.Join(baseDir, "cache"), DirPermBase, filepath.Join(baseDir, "cache")},
		{filepath.Join(baseDir, "logs"), DirPermBase, filepath.Join(baseDir, "logs")},
	}

	var results []StepResult
	for _, d := range dirs {
		info, err := os.Stat(d.path)
		if err == nil && info.IsDir() {
			results = append(results, StepResult{
				Name:     "Directory creation",
				Status:   StepSuccess,
				Message:  fmt.Sprintf("Directory exists: %s", d.name),
				Critical: true,
			})
			continue
		}

		if mkErr := os.MkdirAll(d.path, d.perm); mkErr != nil {
			results = append(results, StepResult{
				Name:   "Directory creation",
				Status: StepError,
				Message: fmt.Sprintf(
					"Failed to create %s: %v\n  Try: export FINFOCUS_HOME=/path/to/writable/directory",
					d.name,
					mkErr,
				),
				Critical: true,
				Err:      mkErr,
			})
			continue
		}

		results = append(results, StepResult{
			Name:     "Directory creation",
			Status:   StepSuccess,
			Message:  fmt.Sprintf("Created %s", d.name),
			Critical: true,
		})
	}

	return results
}

// StepInitConfig initializes the default config file if one does not exist.
func StepInitConfig(baseDir string) StepResult {
	configPath := filepath.Join(baseDir, "config.yaml")

	if _, err := os.Stat(configPath); err == nil {
		return StepResult{
			Name:     "Config initialization",
			Status:   StepSuccess,
			Message:  fmt.Sprintf("Config already exists (%s)", configPath),
			Critical: true,
		}
	}

	cfg := config.New()
	if err := cfg.Save(); err != nil {
		return StepResult{
			Name:     "Config initialization",
			Status:   StepError,
			Message:  fmt.Sprintf("Failed to initialize config: %v", err),
			Critical: true,
			Err:      err,
		}
	}

	return StepResult{
		Name:     "Config initialization",
		Status:   StepSuccess,
		Message:  fmt.Sprintf("Initialized config (%s)", configPath),
		Critical: true,
	}
}

// StepInstallAnalyzer installs the Pulumi analyzer using the runner's AnalyzerInstaller.
func (r *SetupRunner) StepInstallAnalyzer(ctx context.Context) StepResult {
	result, err := r.AnalyzerInstaller.Install(ctx, analyzer.InstallOptions{})
	if err != nil {
		return StepResult{
			Name:   "Analyzer installation",
			Status: StepWarning,
			Message: fmt.Sprintf(
				"Failed to install analyzer: %v\n  Try: finfocus analyzer install",
				err,
			),
			Err: err,
		}
	}

	switch result.Action {
	case analyzer.ActionInstalled:
		return StepResult{
			Name:   "Analyzer installation",
			Status: StepSuccess,
			Message: fmt.Sprintf(
				"Installed Pulumi analyzer (v%s, %s)",
				result.Version,
				result.Method,
			),
		}
	case analyzer.ActionAlreadyCurrent:
		return StepResult{
			Name:    "Analyzer installation",
			Status:  StepSuccess,
			Message: fmt.Sprintf("Pulumi analyzer already current (v%s)", result.Version),
		}
	case analyzer.ActionUpdateAvailable:
		return StepResult{
			Name:   "Analyzer installation",
			Status: StepWarning,
			Message: fmt.Sprintf(
				"Pulumi analyzer installed at v%s, update available (v%s). Use: finfocus analyzer install --force",
				result.Version,
				result.CurrentVersion,
			),
		}
	default:
		return StepResult{
			Name:    "Analyzer installation",
			Status:  StepSuccess,
			Message: fmt.Sprintf("Pulumi analyzer (v%s)", result.Version),
		}
	}
}

// StepInstallPlugins installs the default plugin set.
// Returns one StepResult per plugin in the default set.
// baseDir is the resolved FinFocus home directory (e.g., ~/.finfocus).
func (r *SetupRunner) StepInstallPlugins(baseDir string) []StepResult {
	pluginDir := filepath.Join(baseDir, "plugins")

	var installer PluginInstaller
	if r.PluginInstaller != nil {
		installer = r.PluginInstaller
	} else {
		installer = registry.NewInstaller(pluginDir)
	}

	var results []StepResult
	for _, pluginName := range DefaultPlugins {
		// Check if already installed by scanning the plugin directory
		pluginPath := filepath.Join(pluginDir, pluginName)
		if info, statErr := os.Stat(pluginPath); statErr == nil && info.IsDir() && pluginHasVersionDir(pluginPath) {
			results = append(results, StepResult{
				Name:    "Plugin installation",
				Status:  StepSuccess,
				Message: fmt.Sprintf("Plugin already installed: %s", pluginName),
			})
			continue
		}

		installResult, err := installer.Install(pluginName, registry.InstallOptions{
			FallbackToLatest: true,
		}, nil)
		if err != nil {
			results = append(results, StepResult{
				Name:   "Plugin installation",
				Status: StepWarning,
				Message: fmt.Sprintf(
					"Failed to install plugin %s: %v\n  Try later: finfocus plugin install %s",
					pluginName,
					err,
					pluginName,
				),
				Err: err,
			})
			continue
		}

		verInfo := installResult.Version
		if verInfo == "" {
			verInfo = "latest"
		}
		results = append(results, StepResult{
			Name:    "Plugin installation",
			Status:  StepSuccess,
			Message: fmt.Sprintf("Installed plugin: %s (%s)", installResult.Name, verInfo),
		})
	}

	return results
}
