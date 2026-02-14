// Package contracts defines the API contract for the analyzer install feature.
// This file is a design artifact, not production code.
package contracts

// InstallOptions configures analyzer installation behavior.
type InstallOptions struct {
	// Force overwrites an existing installation without prompting.
	Force bool

	// TargetDir overrides the default Pulumi plugin directory.
	// Resolution precedence when empty: $PULUMI_HOME/plugins/ > ~/.pulumi/plugins/
	TargetDir string
}

// InstallResult describes the outcome of an install or status check.
type InstallResult struct {
	// Installed indicates whether the analyzer is currently installed.
	Installed bool

	// Version is the installed analyzer version (empty if not installed).
	Version string

	// Path is the full filesystem path to the installed binary.
	Path string

	// Method is "symlink" or "copy" depending on the installation strategy.
	Method string

	// NeedsUpdate is true when the installed version differs from the current binary.
	NeedsUpdate bool

	// CurrentVersion is the version of the running finfocus binary.
	CurrentVersion string
}

// --- Function signatures for internal/analyzer/install.go ---
//
// func Install(ctx context.Context, opts InstallOptions) (*InstallResult, error)
// func Uninstall(ctx context.Context, targetDir string) error
// func IsInstalled(targetDir string) (bool, error)
// func InstalledVersion(targetDir string) (string, error)
// func NeedsUpdate(targetDir string) (bool, error)
// func ResolvePulumiPluginDir(override string) (string, error)

// --- CLI commands for internal/cli/ ---
//
// NewAnalyzerInstallCmd() *cobra.Command
//   Flags: --force (bool), --target-dir (string)
//   RunE: calls analyzer.Install()
//
// NewAnalyzerUninstallCmd() *cobra.Command
//   Flags: --target-dir (string)
//   RunE: calls analyzer.Uninstall()
