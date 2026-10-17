package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestRemoveVersionKeepsConfiguredRelease(t *testing.T) {
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	config.ResetGlobalConfigForTest()
	t.Cleanup(config.ResetGlobalConfigForTest)

	dir := filepath.Join(home, "plugins")
	installer := NewInstaller(dir)
	older := filepath.Join(dir, "demo", "v1.0.0")
	newer := filepath.Join(dir, "demo", "v2.0.0")
	require.NoError(t, os.MkdirAll(older, 0o755))
	require.NoError(t, os.MkdirAll(newer, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(older, "demo"), []byte("old"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(newer, "demo"), []byte("new"), 0o755))
	require.NoError(t, config.AddInstalledPlugin(config.InstalledPlugin{Name: "demo", Version: "v2.0.0"}))

	var messages []string
	err := installer.Remove("demo", RemoveOptions{Version: "v1.0.0"}, func(msg string) {
		messages = append(messages, msg)
	})
	require.NoError(t, err)
	_, err = os.Stat(older)
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = os.Stat(newer)
	require.NoError(t, err)
	got, err := config.GetInstalledPlugin("demo")
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", got.Version)
	assert.Contains(t, messages, "Removing demo@v1.0.0...")

	err = installer.Remove("demo", RemoveOptions{Version: "v9.9.9"}, nil)
	require.Error(t, err)
	require.ErrorContains(t, err, "not installed")
	_, err = os.Stat(newer)
	assert.NoError(t, err)
}

func TestLockAvailable(t *testing.T) {
	t.Parallel()
	installer := NewInstaller(t.TempDir())
	require.NoError(t, installer.LockAvailable("aws-public"))
	unlock, err := installer.acquireLock("aws-public")
	require.NoError(t, err)
	t.Cleanup(unlock)
	err = installer.LockAvailable("aws-public")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPluginBusy)
}
