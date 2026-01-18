package registry // needs access to unexported methods

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestListLatestPlugins(t *testing.T) {
	tests := []struct {
		name              string
		setupDir          func(t *testing.T) string
		wantCount         int
		wantPlugins       []PluginInfo
		wantWarningsCount int
		wantErr           bool
	}{
		{
			name: "empty directory",
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			wantCount: 0,
		},
		{
			name: "single plugin single version",
			setupDir: func(t *testing.T) string {
				return createSinglePluginDir(t, "testplugin", "v1.0.0")
			},
			wantCount: 1,
			wantPlugins: []PluginInfo{
				{Name: "testplugin", Version: "v1.0.0"},
			},
		},
		{
			name:      "multiple plugins single version",
			setupDir:  createMultiplePluginsDir,
			wantCount: 2,
			wantPlugins: []PluginInfo{
				{Name: "aws-plugin", Version: "v1.0.0"},
				{Name: "kubecost", Version: "v2.1.0"},
			},
		},
		{
			name:      "same plugin multiple versions selects latest",
			setupDir:  createMultiVersionPluginDir,
			wantCount: 1,
			wantPlugins: []PluginInfo{
				{Name: "testplugin", Version: "v2.0.0"},
			},
		},
		{
			name: "three versions selects highest",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				versions := []string{"v1.0.0", "v1.1.0", "v2.0.0"}
				for _, v := range versions {
					if err := createPluginVersion(dir, "tri-ver", v); err != nil {
						t.Fatal(err)
					}
				}
				return dir
			},
			wantCount: 1,
			wantPlugins: []PluginInfo{
				{Name: "tri-ver", Version: "v2.0.0"},
			},
		},
		{
			name:              "edge cases: pre-release, invalid, corrupt",
			setupDir:          createEdgeCasePluginDir,
			wantCount:         2,
			wantWarningsCount: 1, // Invalid version triggers warning
			wantPlugins: []PluginInfo{
				{Name: "prerelease-test", Version: "v1.0.0"},  // Stable over pre-release
				{Name: "invalid-ver-test", Version: "v1.0.0"}, // Valid one picked
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootDir := tt.setupDir(t)
			reg := &Registry{
				root:     rootDir,
				launcher: pluginhost.NewProcessLauncher(),
			}

			plugins, warnings, err := reg.ListLatestPlugins()
			verifyListLatestPluginsResult(
				t,
				plugins,
				warnings,
				err,
				tt.wantErr,
				tt.wantCount,
				tt.wantPlugins,
				tt.wantWarningsCount,
			)
		})
	}
}

func TestListLatestPlugins_FSErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission tests on Windows")
	}

	dir := t.TempDir()
	// Create an unreadable subdirectory (noPermDir)
	noPermDir := filepath.Join(dir, "no-perm-plugin")
	err := os.MkdirAll(noPermDir, 0000)
	require.NoError(t, err, "Failed to create test directory")
	defer func() {
		_ = os.Chmod(noPermDir, 0755) // Cleanup
	}()

	reg := &Registry{
		root:     dir,
		launcher: pluginhost.NewProcessLauncher(),
	}

	plugins, _, err := reg.ListLatestPlugins()
	require.NoError(t, err)

	assert.Len(t, plugins, 0, "expected 0 plugins from unreadable dir")
}

func TestListLatestPlugins_BinaryValidation(t *testing.T) {
	dir := t.TempDir()

	// 1. Missing Binary
	err := os.MkdirAll(filepath.Join(dir, "missing-bin", "v1.0.0"), 0755)
	require.NoError(t, err, "Failed to create test directory")

	// 2. Non-executable Binary (Linux/Mac only)
	if runtime.GOOS != "windows" {
		nonExecDir := filepath.Join(dir, "non-exec", "v1.0.0")
		err = os.MkdirAll(nonExecDir, 0755)
		require.NoError(t, err, "Failed to create test directory")
		err = os.WriteFile(filepath.Join(nonExecDir, "non-exec"), []byte("data"), 0644)
		require.NoError(t, err, "Failed to create test binary")
	}

	reg := &Registry{
		root:     dir,
		launcher: pluginhost.NewProcessLauncher(),
	}

	plugins, _, err := reg.ListLatestPlugins()
	require.NoError(t, err, "unexpected error")
	assert.Empty(t, plugins, "expected 0 plugins (invalid binaries)")
}

func TestListLatestPlugins_Concurrency(t *testing.T) {
	dir := createMultiVersionPluginDir(t)
	reg := &Registry{
		root:     dir,
		launcher: pluginhost.NewProcessLauncher(),
	}

	concurrency := 10
	errCh := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			_, _, err := reg.ListLatestPlugins()
			errCh <- err
		}()
	}

	for i := 0; i < concurrency; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("concurrent ListLatestPlugins failed: %v", err)
		}
	}
}

type mockLauncher struct {
	startCalled map[string]int
}

func (m *mockLauncher) Start(
	ctx context.Context,
	path string,
	args ...string,
) (*grpc.ClientConn, func() error, error) {
	if m.startCalled == nil {
		m.startCalled = make(map[string]int)
	}
	m.startCalled[filepath.Base(path)]++
	return nil, func() error { return nil }, errors.New("mock launch failed")
}

func TestRegistry_Open_WithWarnings(t *testing.T) {
	// Create directory with valid and invalid plugins
	dir := createEdgeCasePluginDir(t)
	mock := &mockLauncher{}
	reg := &Registry{
		root:     dir,
		launcher: mock,
	}

	ctx := context.Background()
	// Open should proceed despite warnings about invalid versions
	// It will try to launch valid plugins. Mock launcher returns error,
	// so Open will log warnings and return (clients=empty, err=nil).
	clients, cleanup, err := reg.Open(ctx, "")
	if cleanup != nil {
		defer cleanup()
	}

	require.NoError(t, err, "Open failed")
	assert.Len(t, clients, 0, "Expected 0 clients (mock fails)")

	// Verify it attempted to launch valid plugins
	// Expected valid plugins: "prerelease-test", "invalid-ver-test" (names from createEdgeCasePluginDir)
	// The binary names are created by createPluginVersion as plugin Name.
	expectedAttempts := []string{"prerelease-test", "invalid-ver-test"}
	assert.Len(t, mock.startCalled, len(expectedAttempts), "Expected %d launch attempts", len(expectedAttempts))

	for _, name := range expectedAttempts {
		// On windows it adds .exe
		key := name
		if runtime.GOOS == "windows" {
			key += ".exe"
		}
		assert.Equal(t, 1, mock.startCalled[key], "launch attempts for %s", key)
	}
}

func TestListPlugins(t *testing.T) {
	tests := []struct {
		name        string
		setupDir    func(t *testing.T) string
		wantCount   int
		wantPlugins []PluginInfo
		wantErr     bool
	}{
		{
			name: "empty directory",
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			wantCount: 0,
		},
		{
			name: "no plugin directory",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				return filepath.Join(dir, "nonexistent")
			},
			wantCount: 0,
		},
		{
			name: "single plugin with version",
			setupDir: func(t *testing.T) string {
				return createSinglePluginDir(t, "testplugin", "v1.0.0")
			},
			wantCount: 1,
			wantPlugins: []PluginInfo{
				{Name: "testplugin", Version: "v1.0.0"},
			},
		},
		{
			name:      "multiple plugins with versions",
			setupDir:  createMultiplePluginsDir,
			wantCount: 2,
			wantPlugins: []PluginInfo{
				{Name: "aws-plugin", Version: "v1.0.0"},
				{Name: "kubecost", Version: "v2.1.0"},
			},
		},
		{
			name:      "multiple versions same plugin",
			setupDir:  createMultiVersionPluginDir,
			wantCount: 2,
			wantPlugins: []PluginInfo{
				{Name: "testplugin", Version: "v1.0.0"},
				{Name: "testplugin", Version: "v2.0.0"},
			},
		},
		{
			name:      "non-executable file ignored",
			setupDir:  createNonExecutablePluginDir,
			wantCount: 0,
		},
		{
			name: "files in root ignored",
			setupDir: func(t *testing.T) string {
				dir := t.TempDir()
				if err := os.WriteFile(filepath.Join(dir, "somefile"), []byte("test"), 0755); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootDir := tt.setupDir(t)
			reg := &Registry{
				root:     rootDir,
				launcher: pluginhost.NewProcessLauncher(),
			}

			plugins, err := reg.ListPlugins()
			verifyListPluginsResult(t, plugins, err, tt.wantErr, tt.wantCount, tt.wantPlugins)
		})
	}
}

func TestFindBinary(t *testing.T) {
	tests := []struct {
		name       string
		setupDir   func(t *testing.T) string
		wantExists bool
	}{
		{"executable file found", createExecutableBinaryDir, true},
		{"non-executable file ignored", createNonExecutableBinaryDir, false},
		{"directory ignored", createDirectoryInsteadOfBinary, false},
		{"empty directory", func(t *testing.T) string { return t.TempDir() }, false},
		{"multiple files, finds executable", createMixedFilesDir, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := tt.setupDir(t)
			reg := &Registry{root: "", launcher: pluginhost.NewProcessLauncher()}
			binPath := reg.findBinary(dir)
			verifyBinaryResult(t, binPath, tt.wantExists)
		})
	}
}

func TestRegistry_Open(t *testing.T) {
	tests := []struct {
		name        string
		setupDir    func(t *testing.T) string
		onlyName    string
		wantClients int
		wantErr     bool
	}{
		{"no plugins", func(t *testing.T) string { return t.TempDir() }, "", 0, false},
		{"plugin directory doesn't exist", createNonexistentPluginDir, "", 0, false},
		{"filter by specific plugin name", createMultiplePluginsDir, "aws-plugin", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootDir := tt.setupDir(t)
			reg := &Registry{root: rootDir, launcher: pluginhost.NewProcessLauncher()}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			clients, cleanup, err := reg.Open(ctx, tt.onlyName)
			verifyRegistryOpenResult(t, clients, cleanup, err, tt.wantErr, tt.wantClients)
		})
	}
}

func TestFindBinary_Legacy(t *testing.T) {
	t.Run("pulumicost-plugin- prefix supported when enabled", func(t *testing.T) {
		t.Setenv("FINFOCUS_LOG_LEGACY", "1")
		dir := t.TempDir()
		// Create a directory named after the plugin version parent
		pluginDir := filepath.Join(dir, "myplugin", "v1.0.0")
		require.NoError(t, os.MkdirAll(pluginDir, 0755))

		binPath := filepath.Join(pluginDir, "pulumicost-plugin-myplugin")
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}
		require.NoError(t, os.WriteFile(binPath, []byte("binary"), 0755))

		reg := &Registry{root: "", launcher: pluginhost.NewProcessLauncher()}
		found := reg.findBinary(pluginDir)
		assert.Equal(t, binPath, found)
	})

	t.Run("finfocus-plugin- prefix takes precedence", func(t *testing.T) {
		t.Setenv("FINFOCUS_LOG_LEGACY", "1")
		dir := t.TempDir()
		pluginDir := filepath.Join(dir, "myplugin", "v1.0.0")
		require.NoError(t, os.MkdirAll(pluginDir, 0755))

		legacyBin := filepath.Join(pluginDir, "pulumicost-plugin-myplugin")
		newBin := filepath.Join(pluginDir, "finfocus-plugin-myplugin")
		if runtime.GOOS == "windows" {
			legacyBin += ".exe"
			newBin += ".exe"
		}
		require.NoError(t, os.WriteFile(legacyBin, []byte("old"), 0755))
		require.NoError(t, os.WriteFile(newBin, []byte("new"), 0755))

		reg := &Registry{root: "", launcher: pluginhost.NewProcessLauncher()}
		found := reg.findBinary(pluginDir)
		assert.Equal(t, newBin, found)
	})
}

// Helper functions to reduce cognitive complexity

func createSinglePluginDir(t *testing.T, name, version string) string {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, name, version)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(pluginDir, name)
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	if err := os.WriteFile(binPath, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func createMultiplePluginsDir(t *testing.T) string {
	dir := t.TempDir()
	plugins := []struct{ name, version string }{
		{"aws-plugin", "v1.0.0"},
		{"kubecost", "v2.1.0"},
	}
	for _, p := range plugins {
		pluginDir := filepath.Join(dir, p.name, p.version)
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatal(err)
		}
		binPath := filepath.Join(pluginDir, p.name)
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}
		if err := os.WriteFile(binPath, []byte("#!/bin/bash\necho "+p.name+"\nexit 1"), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func createMultiVersionPluginDir(t *testing.T) string {
	dir := t.TempDir()
	versions := []string{"v1.0.0", "v2.0.0"}
	for _, version := range versions {
		pluginDir := filepath.Join(dir, "testplugin", version)
		if err := os.MkdirAll(pluginDir, 0755); err != nil {
			t.Fatal(err)
		}
		binPath := filepath.Join(pluginDir, "testplugin")
		if runtime.GOOS == "windows" {
			binPath += ".exe"
		}
		if err := os.WriteFile(binPath, []byte("#!/bin/bash\necho "+version), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func createNonExecutablePluginDir(t *testing.T) string {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "testplugin", "v1.0.0")
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(pluginDir, "testplugin")
	if err := os.WriteFile(binPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func verifyListPluginsResult(
	t *testing.T,
	plugins []PluginInfo,
	err error,
	wantErr bool,
	wantCount int,
	wantPlugins []PluginInfo,
) {
	if wantErr && err == nil {
		t.Error("expected error but got none")
		return
	}
	if !wantErr && err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if len(plugins) != wantCount {
		t.Errorf("expected %d plugins, got %d", wantCount, len(plugins))
		return
	}
	if wantPlugins != nil {
		verifyExpectedPlugins(t, plugins, wantPlugins)
	}
}

func verifyExpectedPlugins(t *testing.T, plugins []PluginInfo, wantPlugins []PluginInfo) {
	for _, wantPlugin := range wantPlugins {
		found := false
		for _, plugin := range plugins {
			if plugin.Name == wantPlugin.Name && plugin.Version == wantPlugin.Version {
				found = true
				if plugin.Path == "" {
					t.Errorf("plugin %s v%s has empty path", plugin.Name, plugin.Version)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected plugin %s v%s not found", wantPlugin.Name, wantPlugin.Version)
		}
	}
}

func TestNewDefault(t *testing.T) {
	reg := NewDefault()

	if reg == nil {
		t.Fatal("NewDefault returned nil")
	}

	if reg.root == "" {
		t.Error("registry root is empty")
	}

	if reg.launcher == nil {
		t.Error("registry launcher is nil")
	}

	// Verify the root path structure
	expectedSuffix := filepath.Join(".finfocus", "plugins")
	if !filepath.IsAbs(reg.root) {
		t.Error("registry root is not absolute path")
	}

	if !strings.HasSuffix(reg.root, expectedSuffix) {
		t.Errorf("registry root doesn't end with expected path: %s", expectedSuffix)
	}
}

// Additional helper functions for TestFindBinary and TestRegistry_Open

func createExecutableBinaryDir(t *testing.T) string {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "plugin")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	if err := os.WriteFile(binPath, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func createNonExecutableBinaryDir(t *testing.T) string {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "plugin")
	if err := os.WriteFile(binPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func createDirectoryInsteadOfBinary(t *testing.T) string {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "plugin")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func createMixedFilesDir(t *testing.T) string {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("readme"), 0644); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(dir, "plugin")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	if err := os.WriteFile(binPath, []byte("#!/bin/bash\necho test"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func createNonexistentPluginDir(t *testing.T) string {
	dir := t.TempDir()
	return filepath.Join(dir, "nonexistent")
}

func verifyBinaryResult(t *testing.T, binPath string, wantExists bool) {
	if wantExists && binPath == "" {
		t.Error("expected to find binary but got empty path")
	}
	if !wantExists && binPath != "" {
		t.Errorf("expected no binary but got path: %s", binPath)
	}
	if binPath != "" {
		verifyBinaryExecutable(t, binPath)
	}
}

func verifyBinaryExecutable(t *testing.T, binPath string) {
	info, err := os.Stat(binPath)
	if err != nil {
		t.Errorf("binary path invalid: %v", err)
		return
	}
	if runtime.GOOS != "windows" {
		if info.Mode()&0111 == 0 {
			t.Error("found binary is not executable")
		}
	} else {
		if filepath.Ext(binPath) != ".exe" {
			t.Error("found binary on Windows doesn't have .exe extension")
		}
	}
}

func createEdgeCasePluginDir(t *testing.T) string {
	dir := t.TempDir()

	// 1. Pre-release vs Stable: v1.0.0-alpha vs v1.0.0
	// Should select v1.0.0
	if err := createPluginVersion(dir, "prerelease-test", "v1.0.0-alpha"); err != nil {
		t.Fatal(err)
	}
	if err := createPluginVersion(dir, "prerelease-test", "v1.0.0"); err != nil {
		t.Fatal(err)
	}

	// 2. Invalid Version: invalid-version
	// Should be skipped/warned
	if err := createPluginVersion(dir, "invalid-ver-test", "invalid-version"); err != nil {
		t.Fatal(err)
	}
	// Add a valid one to ensure it's picked up
	if err := createPluginVersion(dir, "invalid-ver-test", "v1.0.0"); err != nil {
		t.Fatal(err)
	}

	// 3. Corrupted Directory (file instead of dir)
	// Should be skipped/warned
	corruptPath := filepath.Join(dir, "corrupt-plugin")
	if err := os.WriteFile(corruptPath, []byte("file-not-dir"), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func createPluginVersion(rootDir, name, version string) error {
	pluginDir := filepath.Join(rootDir, name, version)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return err
	}
	binPath := filepath.Join(pluginDir, name)
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	return os.WriteFile(binPath, []byte("#!/bin/bash\necho "+version), 0755)
}

func verifyListLatestPluginsResult(
	t *testing.T,
	plugins []PluginInfo,
	warnings []string,
	err error,
	wantErr bool,
	wantCount int,
	wantPlugins []PluginInfo,
	wantWarningsCount int,
) {
	if wantErr && err == nil {
		require.Error(t, err, "expected error but got none")
		return
	}
	if !wantErr && err != nil {
		require.NoError(t, err, "unexpected error")
		return
	}
	require.Len(t, plugins, wantCount, "expected %d plugins", wantCount)
	require.Len(t, warnings, wantWarningsCount, "expected %d warnings. Warnings: %v", wantWarningsCount, warnings)
	if wantPlugins != nil {
		verifyExpectedPlugins(t, plugins, wantPlugins)
	}
}

func verifyRegistryOpenResult(
	t *testing.T,
	clients []*pluginhost.Client,
	cleanup func(),
	err error,
	wantErr bool,
	wantClients int,
) {
	if cleanup != nil {
		defer cleanup()
	}
	if wantErr && err == nil {
		t.Error("expected error but got none")
		return
	}
	if !wantErr && err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	if len(clients) != wantClients {
		t.Errorf("expected %d clients, got %d", wantClients, len(clients))
	}
	if cleanup != nil {
		cleanup() // Should not panic
	}
}
