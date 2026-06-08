//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

// PluginManager handles plugin installation and removal for E2E tests.
type PluginManager struct {
	T          *testing.T
	BinaryPath string
}

// NewPluginManager creates a new PluginManager.
func NewPluginManager(t *testing.T) *PluginManager {
	binaryPath := findFinFocusBinaryForPlugin()
	if binaryPath == "" {
		t.Fatal("finfocus binary not found - run 'make build' first")
	}
	return &PluginManager{
		T:          t,
		BinaryPath: binaryPath,
	}
}

// findFinFocusBinaryForPlugin locates the finfocus binary for plugin operations.
func findFinFocusBinaryForPlugin() string {
	return findFinFocusBinary()
}

// InstallPlugin installs a plugin by name (e.g., "aws-public").
// Returns true if installation was successful, false otherwise.
func (pm *PluginManager) InstallPlugin(ctx context.Context, name string) error {
	pm.T.Logf("Installing plugin: %s", name)

	cmd := newCommand(ctx, pm.BinaryPath, "plugin", "install", name, "--force")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		pm.T.Logf("Plugin install stdout: %s", stdout.String())
		pm.T.Logf("Plugin install stderr: %s", stderr.String())
		return fmt.Errorf("failed to install plugin %s: %w", name, err)
	}

	pm.T.Logf("Plugin %s installed successfully", name)
	pm.T.Logf("Output: %s", stdout.String())
	return nil
}

// RemovePlugin removes a plugin by name.
func (pm *PluginManager) RemovePlugin(ctx context.Context, name string) error {
	pm.T.Logf("Removing plugin: %s", name)

	cmd := newCommand(ctx, pm.BinaryPath, "plugin", "remove", name, "-y")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		pm.T.Logf("Plugin remove stdout: %s", stdout.String())
		pm.T.Logf("Plugin remove stderr: %s", stderr.String())
		return fmt.Errorf("failed to remove plugin %s: %w", name, err)
	}

	pm.T.Logf("Plugin %s removed successfully", name)
	return nil
}

// IsPluginInstalled checks if a plugin is installed.
func (pm *PluginManager) IsPluginInstalled(ctx context.Context, name string) bool {
	cmd := newCommand(ctx, pm.BinaryPath, "plugin", "list")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		return false
	}

	return strings.Contains(stdout.String(), name)
}

// ListPlugins returns the output of plugin list command.
func (pm *PluginManager) ListPlugins(ctx context.Context) (string, error) {
	cmd := newCommand(ctx, pm.BinaryPath, "plugin", "list")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to list plugins: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// EnsurePluginInstalled ensures a plugin is installed, installing it if not present.
func (pm *PluginManager) EnsurePluginInstalled(ctx context.Context, name string) error {
	if pm.IsPluginInstalled(ctx, name) {
		pm.T.Logf("Plugin %s already installed", name)
		return nil
	}
	return pm.InstallPlugin(ctx, name)
}

// ShouldCleanupPlugins returns true if plugins should be removed after tests.
// Controlled by E2E_CLEANUP_PLUGINS environment variable (default: false).
// Set to "true" or "1" to enable plugin cleanup.
func ShouldCleanupPlugins() bool {
	val := os.Getenv("E2E_CLEANUP_PLUGINS")
	return val == "true" || val == "1"
}

// CleanupPluginIfEnabled removes a plugin only if E2E_CLEANUP_PLUGINS is set.
// This is useful for test teardown to restore the system state.
func (pm *PluginManager) CleanupPluginIfEnabled(ctx context.Context, name string) {
	if !ShouldCleanupPlugins() {
		pm.T.Logf("Plugin cleanup disabled (set E2E_CLEANUP_PLUGINS=true to enable)")
		return
	}

	if !pm.IsPluginInstalled(ctx, name) {
		pm.T.Logf("Plugin %s not installed, skipping cleanup", name)
		return
	}

	pm.T.Logf("Cleaning up plugin %s (E2E_CLEANUP_PLUGINS=true)", name)
	if err := pm.RemovePlugin(ctx, name); err != nil {
		pm.T.Logf("Warning: failed to remove plugin %s: %v", name, err)
	} else {
		pm.T.Logf("Plugin %s removed successfully", name)
	}
}

// DeferPluginCleanup returns a function suitable for defer that cleans up a plugin.
// Usage: defer pm.DeferPluginCleanup(ctx, "aws-public")()
func (pm *PluginManager) DeferPluginCleanup(ctx context.Context, name string) func() {
	return func() {
		pm.CleanupPluginIfEnabled(ctx, name)
	}
}
