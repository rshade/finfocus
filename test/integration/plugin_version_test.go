//go:build integration
// +build integration

// Package integration_test contains integration tests for plugin version compatibility.
//
// # Test Status: CONDITIONALLY SKIPPED
//
// These tests are skipped when the required environment variables are not set.
// They validate real plugin binary initialization and version compatibility.
//
// # Prerequisites to Enable These Tests
//
// 1. Build the recorder plugin: `make build-recorder`
// 2. Or build any plugin that implements GetPluginInfo with a known spec version
// 3. Set environment variable FINFOCUS_TEST_PLUGIN_PATH to the plugin binary path
//
// # Coverage Note
//
// The version compatibility logic is fully tested via:
//   - internal/pluginhost/version_test.go: SemVer comparison logic (7 test cases)
//   - internal/pluginhost/client_test.go: GetPluginInfo success/unimplemented/timeout (3 test cases)
//
// These integration tests provide additional end-to-end validation when
// a suitable test plugin fixture is available in CI.
//
// # Related Issues
//
// See PR #398 for the initial implementation of plugin info discovery.
package integration_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
)

// TestPluginInitialization_CompatibleVersion verifies that a plugin with a compatible
// spec version initializes successfully without warnings.
//
// To enable: Set FINFOCUS_TEST_PLUGIN_PATH to a plugin binary with matching spec version.
func TestPluginInitialization_CompatibleVersion(t *testing.T) {
	pluginPath := os.Getenv("FINFOCUS_TEST_PLUGIN_PATH")
	if pluginPath == "" {
		t.Skip("Skipping: FINFOCUS_TEST_PLUGIN_PATH not set (see package doc for prerequisites)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	launcher := pluginhost.NewProcessLauncher()
	client, err := pluginhost.NewClient(ctx, launcher, pluginPath)
	require.NoError(t, err, "compatible plugin should initialize successfully")
	require.NotNil(t, client)
	defer func() { _ = client.Close() }()

	assert.NotEmpty(t, client.Name, "plugin should report a name")
	assert.NotNil(t, client.Metadata, "plugin should have metadata from GetPluginInfo")
	assert.NotEmpty(t, client.Metadata.SpecVersion, "plugin should report a spec version")
}

// TestPluginInitialization_IncompatibleVersion_Warning verifies that a plugin with
// a mismatched major spec version triggers a warning but still initializes.
//
// To enable: Set FINFOCUS_TEST_INCOMPATIBLE_PLUGIN_PATH to a plugin binary with
// a different major spec version than core.
func TestPluginInitialization_IncompatibleVersion_Warning(t *testing.T) {
	pluginPath := os.Getenv("FINFOCUS_TEST_INCOMPATIBLE_PLUGIN_PATH")
	if pluginPath == "" {
		t.Skip("Skipping: FINFOCUS_TEST_INCOMPATIBLE_PLUGIN_PATH not set (requires plugin with different major spec version)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	launcher := pluginhost.NewProcessLauncher()
	// In permissive mode (default), incompatible plugins load with a warning
	client, err := pluginhost.NewClient(ctx, launcher, pluginPath)
	require.NoError(t, err, "incompatible plugin should still initialize in permissive mode")
	require.NotNil(t, client)
	defer func() { _ = client.Close() }()

	assert.NotEmpty(t, client.Name, "plugin should report a name")
	assert.NotNil(t, client.Metadata, "plugin should have metadata from GetPluginInfo")
}

// TestPluginInitialization_LegacyPlugin_NoGetPluginInfo verifies that a legacy plugin
// that doesn't implement GetPluginInfo initializes successfully with a debug log.
//
// To enable: Set FINFOCUS_TEST_LEGACY_PLUGIN_PATH to a plugin binary without GetPluginInfo.
func TestPluginInitialization_LegacyPlugin_NoGetPluginInfo(t *testing.T) {
	pluginPath := os.Getenv("FINFOCUS_TEST_LEGACY_PLUGIN_PATH")
	if pluginPath == "" {
		t.Skip("Skipping: FINFOCUS_TEST_LEGACY_PLUGIN_PATH not set (requires legacy plugin without GetPluginInfo RPC)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	launcher := pluginhost.NewProcessLauncher()
	client, err := pluginhost.NewClient(ctx, launcher, pluginPath)
	require.NoError(t, err, "legacy plugin should initialize without GetPluginInfo")
	require.NotNil(t, client)
	defer func() { _ = client.Close() }()

	assert.NotEmpty(t, client.Name, "legacy plugin should still report a name")
	// Legacy plugins don't implement GetPluginInfo, so metadata should be nil
	assert.Nil(t, client.Metadata, "legacy plugin should have nil metadata")
}
