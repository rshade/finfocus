package pluginhost

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartPlugin_ParentEnvWinsAndPortStays(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script plugin is a shell script")
	}
	t.Setenv("FINFOCUS_RECORDER_MOCK_RESPONSE", "from-parent")

	dir := t.TempDir()
	out := filepath.Join(dir, "env.txt")
	script := filepath.Join(dir, "plugin.sh")
	body := "#!/bin/sh\n" +
		"printf '%s\\n' \"$FINFOCUS_RECORDER_MOCK_RESPONSE\" > '" + out + "'\n" +
		"printf '%s\\n' \"$FINFOCUS_PLUGIN_PORT\" >> '" + out + "'\n" +
		"printf '%s\\n' \"$FINFOCUS_PLUGIN_RECORDER_OUTPUT_DIR\" >> '" + out + "'\n"
	require.NoError(t, os.WriteFile(script, []byte(body), 0o755))

	launcher := NewProcessLauncher()
	ctx := WithDeliveredEnv(context.Background(), Delivered{Pairs: []string{
		"FINFOCUS_RECORDER_MOCK_RESPONSE=from-file",
		"FINFOCUS_PLUGIN_PORT=1",
		"FINFOCUS_PLUGIN_RECORDER_OUTPUT_DIR=/tmp/recorded",
	}})
	cmd, _, err := launcher.startPlugin(ctx, script, 4321, nil)
	require.NoError(t, err)
	require.NoError(t, cmd.Wait())

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "from-parent", lines[0])
	assert.Equal(t, "4321", lines[1])
	assert.Equal(t, "/tmp/recorded", lines[2])
}

func TestStartPlugin_ESCOverridesParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script plugin is a shell script")
	}
	t.Setenv("X_TOKEN", "shell")

	dir := t.TempDir()
	out := filepath.Join(dir, "env.txt")
	script := filepath.Join(dir, "plugin.sh")
	body := "#!/bin/sh\n" +
		"printf '%s\\n' \"$X_TOKEN\" > '" + out + "'\n" +
		"printf '%s\\n' \"$FINFOCUS_PLUGIN_PORT\" >> '" + out + "'\n" +
		"env | grep -c '^X_TOKEN=' >> '" + out + "' || true\n"
	require.NoError(t, os.WriteFile(script, []byte(body), 0o755))

	launcher := NewProcessLauncher()
	ctx := WithDeliveredEnv(context.Background(), Delivered{
		Pairs:     []string{"X_TOKEN=esc", "FINFOCUS_PLUGIN_PORT=1"},
		Overrides: map[string]bool{"X_TOKEN": true, "FINFOCUS_PLUGIN_PORT": true},
	})
	cmd, _, err := launcher.startPlugin(ctx, script, 4321, nil)
	require.NoError(t, err)
	require.NoError(t, cmd.Wait())

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "esc", lines[0])
	assert.Equal(t, "4321", lines[1])
	assert.Equal(t, "1", lines[2])
}
