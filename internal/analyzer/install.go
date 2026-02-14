package analyzer

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/pkg/version"
)

const (
	// analyzerDirPrefix is the directory name prefix for Pulumi plugin versioned directories.
	analyzerDirPrefix = "analyzer-finfocus-v"

	// analyzerBinaryName is the binary name Pulumi expects inside the plugin directory.
	analyzerBinaryName = "pulumi-analyzer-finfocus"
)

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

// ResolvePulumiPluginDir resolves the Pulumi plugin directory with the following precedence:
//  1. override (--target-dir flag) if non-empty
//  2. $PULUMI_HOME/plugins/ if PULUMI_HOME is set
// ResolvePulumiPluginDir determines the filesystem path to use for Pulumi plugins.
// It returns the provided override if non-empty, otherwise uses PULUMI_HOME/plugins if PULUMI_HOME is set, and falls back to $HOME/.pulumi/plugins; an error is returned only if the user's home directory cannot be resolved.
func ResolvePulumiPluginDir(override string) (string, error) {
	if override != "" {
		return override, nil
	}

	if pulumiHome := os.Getenv("PULUMI_HOME"); pulumiHome != "" {
		return filepath.Join(pulumiHome, "plugins"), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}

	return filepath.Join(homeDir, ".pulumi", "plugins"), nil
}

// IsInstalled checks whether any analyzer-finfocus-v* directory exists in the plugin directory.
func IsInstalled(targetDir string) (bool, error) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), analyzerDirPrefix) {
			return true, nil
		}
	}

	return false, nil
}

// InstalledVersion returns the version string parsed from the first analyzer-finfocus-v{version}
// InstalledVersion returns the version string from the first analyzer plugin directory
// found inside targetDir. It looks for a subdirectory whose name begins with
// analyzerDirPrefix and returns the suffix after that prefix as the version.
//
// If no analyzer directory is present, it returns an empty string and a nil error.
// If targetDir does not exist, it returns an empty string and a nil error.
// If reading the directory fails for any other reason, it returns a non-nil error
// describing the failure.
func InstalledVersion(targetDir string) (string, error) {
	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("reading plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), analyzerDirPrefix) {
			ver := strings.TrimPrefix(entry.Name(), analyzerDirPrefix)
			return ver, nil
		}
	}

	return "", nil
}

// NeedsUpdate compares the installed analyzer version against the current binary version.
// NeedsUpdate reports whether the installed analyzer version under the provided
// targetDir differs from the version of the currently running finfocus binary.
// If no analyzer installation is found in targetDir it returns false and a nil
// error. An error is returned if the installed version cannot be determined.
func NeedsUpdate(targetDir string) (bool, error) {
	installed, err := InstalledVersion(targetDir)
	if err != nil {
		return false, err
	}

	if installed == "" {
		return false, nil
	}

	current := version.GetVersion()
	return installed != current, nil
}

// Install installs the finfocus binary as a Pulumi analyzer plugin.
// It resolves the current binary path via os.Executable, creates a versioned directory
// in the Pulumi plugin directory, and creates a symlink (Unix) or copy (Windows) of
// Install installs the current finfocus binary into the Pulumi plugins directory as a versioned
// analyzer plugin (analyzer-finfocus-v{version}/pulumi-analyzer-finfocus).
//
// If opts.TargetDir is non-empty it is used as the Pulumi plugin directory; otherwise the
// directory is resolved via PULUMI_HOME or the default $HOME/.pulumi/plugins. If an existing
// installation is present and opts.Force is false, the function returns the existing install
// status without modifying the filesystem. If opts.Force is true and an existing version is
// present, older analyzer directories are removed before installing the current binary.
//
// The function attempts to create a symlink to the running executable inside the versioned
// plugin directory; if symlinks are not possible it falls back to copying the binary. On
// success it returns an InstallResult describing the installed path, version, method
// ("symlink" or "copy"), and whether an update was needed, otherwise it returns an error.
//
// Parameters:
//   - ctx: context used for logging and cancellation.
//   - opts: installation options; Force causes overwrite of existing installations, and
//     TargetDir overrides the resolved Pulumi plugin directory.
//
// Returns:
//   - *InstallResult describing the installation state when successful.
//   - error if any filesystem or resolution step fails.
func Install(ctx context.Context, opts InstallOptions) (*InstallResult, error) {
	log := logging.FromContext(ctx)

	currentVersion := version.GetVersion()

	// Resolve the Pulumi plugin directory
	pluginDir, err := ResolvePulumiPluginDir(opts.TargetDir)
	if err != nil {
		return nil, fmt.Errorf("resolving plugin directory: %w", err)
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "analyzer").
		Str("plugin_dir", pluginDir).
		Str("version", currentVersion).
		Msg("installing analyzer")

	// Check if already installed
	installedVer, err := InstalledVersion(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("checking installed version: %w", err)
	}

	if installedVer != "" && !opts.Force {
		// Already installed - return status
		binaryPath := filepath.Join(pluginDir, analyzerDirPrefix+installedVer, analyzerBinaryName)
		return &InstallResult{
			Installed:      true,
			Version:        installedVer,
			Path:           binaryPath,
			NeedsUpdate:    installedVer != currentVersion,
			CurrentVersion: currentVersion,
		}, nil
	}

	// Resolve the current binary path
	execPath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolving executable path: %w", err)
	}

	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return nil, fmt.Errorf("resolving symlinks for executable: %w", err)
	}

	// If force and already installed, remove old version(s)
	if opts.Force && installedVer != "" {
		if removeErr := removeAnalyzerDirs(pluginDir); removeErr != nil {
			return nil, fmt.Errorf("removing old installation: %w", removeErr)
		}
	}

	// Create the versioned directory
	versionedDir := filepath.Join(pluginDir, analyzerDirPrefix+currentVersion)
	if mkErr := os.MkdirAll(versionedDir, 0o750); mkErr != nil {
		return nil, fmt.Errorf("creating plugin directory %s: %w", versionedDir, mkErr)
	}

	// Create symlink or copy
	targetPath := filepath.Join(versionedDir, analyzerBinaryName)
	method, err := linkOrCopy(execPath, targetPath)
	if err != nil {
		return nil, fmt.Errorf("installing analyzer binary: %w", err)
	}

	log.Info().
		Ctx(ctx).
		Str("component", "analyzer").
		Str("path", targetPath).
		Str("method", method).
		Str("version", currentVersion).
		Msg("analyzer installed")

	return &InstallResult{
		Installed:      true,
		Version:        currentVersion,
		Path:           targetPath,
		Method:         method,
		NeedsUpdate:    false,
		CurrentVersion: currentVersion,
	}, nil
}

// Uninstall removes all analyzer-finfocus-v* directories from the Pulumi plugins directory.
// 
// If targetDir is non-empty it is used as the Pulumi plugins directory; otherwise the resolver
// follows the standard precedence (PULUMI_HOME/plugins or $HOME/.pulumi/plugins).
// If no analyzer installation is found, Uninstall returns nil.
// Returns an error if resolving the plugin directory, checking installation state, or removing
// the analyzer directories fails.
func Uninstall(ctx context.Context, targetDir string) error {
	log := logging.FromContext(ctx)

	pluginDir, err := ResolvePulumiPluginDir(targetDir)
	if err != nil {
		return fmt.Errorf("resolving plugin directory: %w", err)
	}

	installed, err := IsInstalled(pluginDir)
	if err != nil {
		return fmt.Errorf("checking installation: %w", err)
	}

	if !installed {
		return nil
	}

	if removeErr := removeAnalyzerDirs(pluginDir); removeErr != nil {
		return fmt.Errorf("removing analyzer: %w", removeErr)
	}

	log.Info().
		Ctx(ctx).
		Str("component", "analyzer").
		Str("plugin_dir", pluginDir).
		Msg("analyzer uninstalled")

	return nil
}

// removeAnalyzerDirs removes all directories matching the analyzer prefix from the given directory.
func removeAnalyzerDirs(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), analyzerDirPrefix) {
			fullPath := filepath.Join(dir, entry.Name())
			if removeErr := os.RemoveAll(fullPath); removeErr != nil {
				return fmt.Errorf("removing %s: %w", fullPath, removeErr)
			}
		}
	}

	return nil
}

// linkOrCopy creates a symlink from src to dst on Unix, or copies the file on Windows.
// On Unix, if the symlink fails (e.g., cross-device), it falls back to a copy.
// or "copy") or an error if neither operation succeeds.
func linkOrCopy(src, dst string) (string, error) {
	if runtime.GOOS == "windows" {
		if err := copyFile(src, dst); err != nil {
			return "", err
		}
		return "copy", nil
	}

	// Try symlink first on Unix
	if err := os.Symlink(src, dst); err == nil {
		return "symlink", nil
	}

	// Fallback to copy (e.g., cross-device)
	if err := copyFile(src, dst); err != nil {
		return "", err
	}
	return "copy", nil
}

// copyFile copies the file at src to dst, preserving file mode (including executable bits).
// The destination file is created or truncated with the same permissions as the source.
// It returns an error if the source cannot be opened or stat'ed, the destination cannot be created,
// or if copying the file contents fails.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat source: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("creating destination: %w", err)
	}
	defer dstFile.Close()

	if _, copyErr := io.Copy(dstFile, srcFile); copyErr != nil {
		return fmt.Errorf("copying file: %w", copyErr)
	}

	return nil
}