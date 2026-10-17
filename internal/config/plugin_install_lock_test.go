package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddInstalledPluginKeepsCommentsUnderTheConfigLock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	ResetGlobalConfigForTest()
	t.Cleanup(ResetGlobalConfigForTest)

	path := filepath.Join(home, "config.hujson")
	original := []byte("{\n  // keep me\n  \"plugins\": { \"aws\": { \"region\": \"us-east-1\" } }\n}\n")
	require.NoError(t, os.WriteFile(path, original, 0o600))

	require.NoError(t, AddInstalledPlugin(InstalledPlugin{Name: "recorder", Version: "0.1.0"}))

	saved, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(saved), "// keep me")
	require.Contains(t, string(saved), "us-east-1")
	got, err := GetInstalledPlugin("recorder")
	require.NoError(t, err)
	require.Equal(t, "0.1.0", got.Version)
}
