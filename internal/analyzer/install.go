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
	// The version portion (e.g., "v0.3.1") is appended at runtime using normalizeVersion()
	// to ensure exactly one "v" prefix regardless of the raw version string format.
	analyzerDirPrefix = "analyzer-finfocus-"

	// goosWindows is the runtime.GOOS value for Windows.
	goosWindows = "windows"

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

	// PolicyPackDir is the path to the policy pack directory (empty if not set up).
	PolicyPackDir string `json:"policy_pack_dir,omitempty"`

	// PolicyPackMethod is "symlink" or "copy" for the policy pack binary.
	PolicyPackMethod string `json:"policy_pack_method,omitempty"`
}

// normalizeVersion ensures a version string has exactly one "v" prefix.
// Production builds embed a v-prefixed version (e.g., "v0.3.1") while dev builds may not
// (e.g., "0.1.0-dirty"). This function normalizes all forms to "v{semver}" by stripping
// all leading "v" characters and adding exactly one back:
//   - "0.3.1"   → "v0.3.1"  (dev build, no prefix)
//   - "v0.3.1"  → "v0.3.1"  (production build, correct)
//   - "vv0.3.1" → "v0.3.1"  (malformed double-v, guarded against regression)
func normalizeVersion(v string) string {
	return "v" + strings.TrimLeft(v, "v")
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
		if !strings.HasPrefix(entry.Name(), analyzerDirPrefix) {
			continue
		}
		// Use os.Stat (not entry.IsDir) so symlinks-to-directories are followed (#750).
		entryPath := filepath.Join(targetDir, entry.Name())
		info, statErr := os.Stat(entryPath)
		if statErr == nil && info.IsDir() {
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
			// Strip leading "v" to return a bare semver string for consistent comparisons.
			// Directories are always created with a "v" prefix via normalizeVersion().
			ver = strings.TrimPrefix(ver, "v")
			return ver, nil
		}
	}

	return "", nil
}

// NeedsUpdate compares the installed analyzer version against the current binary version.
// Returns true if they differ, false if they match or analyzer is not installed.
// Both versions are normalized (stripped of "v" prefix) before comparison.
func NeedsUpdate(targetDir string) (bool, error) {
	installed, err := InstalledVersion(targetDir)
	if err != nil {
		return false, err
	}

	if installed == "" {
		return false, nil
	}

	current := version.GetVersion()
	// Normalize both sides: strip "v" prefix so "v0.3.1" and "0.3.1" compare as equal.
	return strings.TrimPrefix(installed, "v") != strings.TrimPrefix(current, "v"), nil
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
		// Use normalizeVersion to construct the correct directory path (ensures "v" prefix).
		binaryPath := filepath.Join(pluginDir, analyzerDirPrefix+normalizeVersion(installedVer), analyzerBinaryName)
		action := ActionAlreadyCurrent
		// Normalize both sides before comparing: "v0.3.1" == "0.3.1" after stripping "v"
		if strings.TrimPrefix(installedVer, "v") != strings.TrimPrefix(currentVersion, "v") {
			action = ActionUpdateAvailable
		}
		result := &InstallResult{
			Installed:      true,
			Version:        installedVer,
			Path:           binaryPath,
			NeedsUpdate:    strings.TrimPrefix(installedVer, "v") != strings.TrimPrefix(currentVersion, "v"),
			CurrentVersion: currentVersion,
			Action:         action,
		}

		// Ensure policy pack is bootstrapped even on no-op installs
		execPath, execErr := os.Executable()
		if execErr == nil {
			if resolved, resolveErr := filepath.EvalSymlinks(execPath); resolveErr == nil {
				execPath = resolved
			}
			setupPolicyPackForInstall(ctx, result, execPath, false)
		}

		return result, nil
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

	// Create the versioned directory.
	// normalizeVersion ensures exactly one "v" prefix: "0.3.1" → "v0.3.1", "v0.3.1" → "v0.3.1".
	// This fixes the double-v bug where "analyzer-finfocus-v" + "v0.3.1" = "analyzer-finfocus-vv0.3.1".
	versionedDir := filepath.Join(pluginDir, analyzerDirPrefix+normalizeVersion(currentVersion))
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

	result := &InstallResult{
		Installed:      true,
		Version:        currentVersion,
		Path:           targetPath,
		Method:         method,
		NeedsUpdate:    false,
		CurrentVersion: currentVersion,
		Action:         ActionInstalled,
	}

	setupPolicyPackForInstall(ctx, result, execPath, opts.Force)

	return result, nil
}

// setupPolicyPackForInstall sets up the policy pack directory and optionally syncs
// the binary on force installs. Results are stored in the InstallResult.
func setupPolicyPackForInstall(ctx context.Context, result *InstallResult, execPath string, force bool) {
	log := logging.FromContext(ctx)

	ppDir, ppMethod, ppErr := SetupPolicyPack(ctx, execPath)
	if ppErr != nil {
		log.Warn().
			Ctx(ctx).
			Str("component", "analyzer").
			Str("operation", "install").
			Err(ppErr).
			Msg("failed to set up policy pack directory")
	} else {
		result.PolicyPackDir = ppDir
		result.PolicyPackMethod = ppMethod
	}

	if force {
		maybeSyncPolicyPack(ctx, execPath)
	}
}

// maybeSyncPolicyPack syncs the policy pack binary if the directory exists.
// Errors are logged as warnings but do not fail the overall install.
func maybeSyncPolicyPack(ctx context.Context, execPath string) {
	log := logging.FromContext(ctx)

	ppDir, resolveErr := ResolvePolicyPackDir()
	if resolveErr != nil {
		return
	}

	if _, statErr := os.Stat(ppDir); statErr != nil {
		return
	}

	if syncErr := syncPolicyPackBinary(ctx, execPath, ppDir); syncErr != nil {
		log.Warn().
			Ctx(ctx).
			Str("component", "analyzer").
			Str("operation", "install").
			Err(syncErr).
			Msg("failed to sync policy pack binary")
	}
}

// syncPolicyPackBinary removes the old binary reference in the policy pack directory
// and creates a new one pointing to the given executable path. This keeps the policy
// pack binary in sync with the Pulumi plugin binary after a --force reinstall.
func syncPolicyPackBinary(ctx context.Context, execPath, policyPackDir string) error {
	binaryPath := filepath.Join(policyPackDir, policyPackBinaryName)

	// Remove existing binary reference
	if _, statErr := os.Lstat(binaryPath); statErr == nil {
		if removeErr := os.Remove(binaryPath); removeErr != nil {
			return fmt.Errorf("removing old policy pack binary: %w", removeErr)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("lstat %s: %w", binaryPath, statErr)
	}

	_, linkErr := linkOrCopy(ctx, execPath, binaryPath)
	if linkErr != nil {
		return fmt.Errorf("creating policy pack binary reference: %w", linkErr)
	}

	return nil
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
		if !strings.HasPrefix(entry.Name(), analyzerDirPrefix) {
			continue
		}
		fullPath := filepath.Join(dir, entry.Name())
		// Use os.Stat (not entry.IsDir) so symlinked analyzer directories are removed (#750).
		info, statErr := os.Stat(fullPath)
		if statErr != nil || !info.IsDir() {
			continue
		}
		if removeErr := os.RemoveAll(fullPath); removeErr != nil {
			return fmt.Errorf("removing %s: %w", fullPath, removeErr)
		}
	}

	return nil
}

// linkOrCopy creates a symlink from src to dst on Unix, or copies the file on Windows.
// On Unix, if the symlink fails (e.g., cross-device), it falls back to a copy.
// Returns the method used ("symlink" or "copy").
func linkOrCopy(ctx context.Context, src, dst string) (string, error) {
	if runtime.GOOS == goosWindows {
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
