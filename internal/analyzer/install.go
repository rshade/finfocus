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

	// ActionInstalled indicates the analyzer was freshly installed.
	ActionInstalled = "installed"

	// ActionAlreadyCurrent indicates the installed version matches the current binary.
	ActionAlreadyCurrent = "already_current"

	// ActionUpdateAvailable indicates a newer version is available.
	ActionUpdateAvailable = "update_available"
)

// InstallOptions configures analyzer installation behavior.
type InstallOptions struct {
	// Force overwrites an existing installation without prompting.
	Force bool `json:"force"`

	// TargetDir overrides the default Pulumi plugin directory.
	// Resolution precedence when empty: $PULUMI_HOME/plugins/ > ~/.pulumi/plugins/
	TargetDir string `json:"target_dir,omitempty"`
}

// InstallResult describes the outcome of an install or status check.
type InstallResult struct {
	// Installed indicates whether the analyzer is currently installed.
	Installed bool `json:"installed"`

	// Version is the installed analyzer version (empty if not installed).
	Version string `json:"version,omitempty"`

	// Path is the full filesystem path to the installed binary.
	Path string `json:"path,omitempty"`

	// Method is "symlink" or "copy" depending on the installation strategy.
	Method string `json:"method,omitempty"`

	// NeedsUpdate is true when the installed version differs from the current binary.
	NeedsUpdate bool `json:"needs_update"`

	// CurrentVersion is the version of the running finfocus binary.
	CurrentVersion string `json:"current_version,omitempty"`

	// Action describes what happened: "installed", "already_current", or "update_available".
	Action string `json:"action"`
}

// ResolvePulumiPluginDir resolves the Pulumi plugin directory with the following precedence:
//  1. override (--target-dir flag) if non-empty
//  2. $PULUMI_HOME/plugins/ if PULUMI_HOME is set
//  3. $HOME/.pulumi/plugins/ (default)
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
// directory found in the plugin directory. Returns empty string if not installed.
// Note: os.ReadDir returns entries in lexicographic order, so when multiple versions exist
// the first match wins. The --force flag removes old directories, keeping only one version.
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
// Returns true if they differ, false if they match or analyzer is not installed.
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
// the binary with the expected analyzer name.
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
		Str("operation", "install").
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
		action := ActionAlreadyCurrent
		if installedVer != currentVersion {
			action = ActionUpdateAvailable
		}
		return &InstallResult{
			Installed:      true,
			Version:        installedVer,
			Path:           binaryPath,
			NeedsUpdate:    installedVer != currentVersion,
			CurrentVersion: currentVersion,
			Action:         action,
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
	method, err := linkOrCopy(ctx, execPath, targetPath)
	if err != nil {
		_ = os.RemoveAll(versionedDir)
		return nil, fmt.Errorf("installing analyzer binary: %w", err)
	}

	log.Info().
		Ctx(ctx).
		Str("component", "analyzer").
		Str("operation", "install").
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
		Action:         ActionInstalled,
	}, nil
}

// Uninstall removes all analyzer-finfocus-v* directories from the plugin directory.
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
		Str("operation", "uninstall").
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
// Returns the method used ("symlink" or "copy").
func linkOrCopy(ctx context.Context, src, dst string) (string, error) {
	if runtime.GOOS == "windows" {
		if err := copyFile(src, dst); err != nil {
			return "", err
		}
		return "copy", nil
	}

	// Try symlink first on Unix
	if symlinkErr := os.Symlink(src, dst); symlinkErr != nil {
		log := logging.FromContext(ctx)
		log.Debug().
			Ctx(ctx).
			Str("component", "analyzer").
			Str("operation", "install").
			Err(symlinkErr).
			Str("src", src).
			Str("dst", dst).
			Msg("symlink failed, falling back to copy")

		// Fallback to copy (e.g., cross-device)
		if err := copyFile(src, dst); err != nil {
			return "", err
		}
		return "copy", nil
	}

	return "symlink", nil
}

// copyFile copies a file from src to dst, preserving executable permissions.
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

	if _, copyErr := io.Copy(dstFile, srcFile); copyErr != nil {
		_ = dstFile.Close()
		return fmt.Errorf("copying file: %w", copyErr)
	}

	if syncErr := dstFile.Sync(); syncErr != nil {
		_ = dstFile.Close()
		return fmt.Errorf("syncing destination: %w", syncErr)
	}

	if closeErr := dstFile.Close(); closeErr != nil {
		return fmt.Errorf("closing destination: %w", closeErr)
	}

	return nil
}
