package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/analyzer"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/pkg/version"
)

// defaultPlugins is the set of plugins installed by default during setup.
//
//nolint:gochecknoglobals // Intentionally global for discoverability
var defaultPlugins = []string{
	"aws-public",
}

// StepStatus represents the outcome of a single setup step.
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

// StepResult describes the outcome of executing a single setup step.
type StepResult struct {
	Name     string
	Status   StepStatus
	Message  string
	Critical bool
	Err      error
}

// SetupOptions holds the configuration for the setup command, derived from CLI flags.
type SetupOptions struct {
	SkipAnalyzer   bool
	SkipPlugins    bool
	NonInteractive bool
}

// SetupResult is the aggregate outcome of all setup steps.
type SetupResult struct {
	Steps       []StepResult
	HasErrors   bool
	HasWarnings bool
}

// dirPermBase is the permission mode for the base and standard directories.
const dirPermBase = 0o700

// dirPermPlugins is the permission mode for the plugins directory.
const dirPermPlugins = 0o750

// formatStatus returns a status marker appropriate for the output mode.
func formatStatus(status StepStatus, nonInteractive bool) string {
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
			return runSetup(cmd, &opts)
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
func runSetup(cmd *cobra.Command, opts *SetupOptions) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	log := logging.FromContext(ctx)

	// Auto-detect non-interactive mode when stdin is not a TTY
	if !opts.NonInteractive && !isTerminal(os.Stdin) {
		opts.NonInteractive = true
	}

	result := &SetupResult{}

	// Step 1: Display version
	step := stepDisplayVersion()
	printStep(cmd, step, opts.NonInteractive)
	result.Steps = append(result.Steps, step)

	// Step 2: Detect Pulumi
	step = stepDetectPulumi(ctx)
	printStep(cmd, step, opts.NonInteractive)
	result.Steps = append(result.Steps, step)

	// Step 3: Create directories
	dirSteps := stepCreateDirectories()
	for _, s := range dirSteps {
		printStep(cmd, s, opts.NonInteractive)
		result.Steps = append(result.Steps, s)
	}

	// Step 4: Initialize config
	step = stepInitConfig()
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
		step = stepInstallAnalyzer(ctx)
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
		pluginSteps := stepInstallPlugins(cmd)
		for _, s := range pluginSteps {
			printStep(cmd, s, opts.NonInteractive)
			result.Steps = append(result.Steps, s)
		}
	}

	// Compute aggregate status
	for _, s := range result.Steps {
		if s.Status == StepError && s.Critical {
			result.HasErrors = true
		}
		if s.Status == StepWarning {
			result.HasWarnings = true
		}
	}

	// Print summary
	printSummary(cmd, result)

	if result.HasErrors {
		log.Error().
			Ctx(ctx).
			Str("component", "setup").
			Msg("setup completed with critical errors")
		return errors.New("setup failed: one or more critical steps failed")
	}

	return nil
}

// printStep outputs a single step's status line.
func printStep(cmd *cobra.Command, step StepResult, nonInteractive bool) {
	marker := formatStatus(step.Status, nonInteractive)
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

// stepDisplayVersion prints the FinFocus version and Go runtime info.
func stepDisplayVersion() StepResult {
	ver := version.GetVersion()
	goVer := runtime.Version()
	msg := fmt.Sprintf("FinFocus v%s (%s)", ver, goVer)
	return StepResult{
		Name:    "Version display",
		Status:  StepSuccess,
		Message: msg,
	}
}

// stepDetectPulumi checks if the pulumi CLI is on PATH and reports its version.
func stepDetectPulumi(ctx context.Context) StepResult {
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
	out, runErr := exec.CommandContext(ctx, "pulumi", "version").Output()
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

// stepCreateDirectories creates the required FinFocus directories.
// Returns one StepResult per directory.
func stepCreateDirectories() []StepResult {
	baseDir := config.ResolveConfigDir()

	dirs := []struct {
		path string
		perm os.FileMode
		name string
	}{
		{baseDir, dirPermBase, baseDir},
		{filepath.Join(baseDir, "plugins"), dirPermPlugins, filepath.Join(baseDir, "plugins")},
		{filepath.Join(baseDir, "cache"), dirPermBase, filepath.Join(baseDir, "cache")},
		{filepath.Join(baseDir, "logs"), dirPermBase, filepath.Join(baseDir, "logs")},
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

// stepInitConfig initializes the default config file if one does not exist.
func stepInitConfig() StepResult {
	baseDir := config.ResolveConfigDir()
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

// stepInstallAnalyzer installs the Pulumi analyzer using the existing analyzer.Install() function.
func stepInstallAnalyzer(ctx context.Context) StepResult {
	result, err := analyzer.Install(ctx, analyzer.InstallOptions{})
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

// stepInstallPlugins installs the default plugin set.
// Returns one StepResult per plugin in the default set.
func stepInstallPlugins(cmd *cobra.Command) []StepResult {
	cfg := config.New()
	installer := registry.NewInstaller(cfg.PluginDir)

	var results []StepResult
	for _, pluginName := range defaultPlugins {
		// Check if already installed by scanning the plugin directory
		pluginDir := filepath.Join(cfg.PluginDir, pluginName)
		if info, statErr := os.Stat(pluginDir); statErr == nil && info.IsDir() {
			// Check if there's at least one version directory
			entries, readErr := os.ReadDir(pluginDir)
			if readErr == nil && len(entries) > 0 {
				results = append(results, StepResult{
					Name:    "Plugin installation",
					Status:  StepSuccess,
					Message: fmt.Sprintf("Plugin already installed: %s", pluginName),
				})
				continue
			}
		}

		var progressBuf bytes.Buffer
		installResult, err := installer.Install(pluginName, registry.InstallOptions{
			FallbackToLatest: true,
		}, func(msg string) {
			progressBuf.WriteString(msg)
		})
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

		_ = cmd // used for potential future progress output

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
