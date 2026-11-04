package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/constants"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/weblog"
)

func webLogsTestCmd(t *testing.T, name string, web bool) (*cobra.Command, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv(constants.EnvAnalyzerMode, "")
	t.Setenv(config.EnvWebLogs, "1")

	logPath := filepath.Join(home, "logs", "finfocus.log")
	config.SetGlobalConfig(&config.Config{
		Logging: config.LoggingConfig{Level: "info", Format: "json", File: logPath},
	})
	t.Cleanup(config.ResetGlobalConfigForTest)

	cmd := newTestLoggingCmd()
	cmd.Use = name
	cmd.Flags().Bool(webFlag, false, "serve the web UI")
	if web {
		require.NoError(t, cmd.Flags().Set(webFlag, "true"))
	}
	return cmd, logPath
}

//nolint:paralleltest // t.Setenv and global config
func TestSetupLogging_WebDashboardGetsHub(t *testing.T) {
	cmd, logPath := webLogsTestCmd(t, "finfocus", true)

	session, err := setupLoggingChecked(cmd)
	require.NoError(t, err)
	defer func() { _ = session.Close() }()

	ctx := cmd.Context()
	hub := weblog.HubFromContext(ctx)
	require.NotNil(t, hub)

	logging.FromContext(ctx).Info().Str("component", "probe").Msg("hub probe line")

	snap, _, cancel := hub.Subscribe(0)
	defer cancel()
	var messages []string
	for _, entry := range snap.Entries {
		messages = append(messages, entry.Message)
	}
	assert.Contains(t, messages, "hub probe line")

	data, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(data), "hub probe line")

	tap := logging.PluginLogTapFromContext(ctx)
	require.NotNil(t, tap)
	w := tap("aws-public")
	_, writeErr := w.Write([]byte("plugin says hi\n"))
	require.NoError(t, writeErr)
	require.NoError(t, w.Close())

	snap2, _, cancel2 := hub.Subscribe(0)
	defer cancel2()
	var sources []string
	for _, entry := range snap2.Entries {
		sources = append(sources, entry.Source)
	}
	assert.Contains(t, sources, "aws-public")
}

//nolint:paralleltest // t.Setenv and global config
func TestSetupLogging_OverviewGetsHub(t *testing.T) {
	cmd, _ := webLogsTestCmd(t, "overview", false)

	session, err := setupLoggingChecked(cmd)
	require.NoError(t, err)
	defer func() { _ = session.Close() }()

	assert.NotNil(t, weblog.HubFromContext(cmd.Context()))
}

//nolint:paralleltest // t.Setenv and global config
func TestSetupLogging_NonDashboardCommandGetsNoHub(t *testing.T) {
	cmd, _ := webLogsTestCmd(t, "projected", false)

	session, err := setupLoggingChecked(cmd)
	require.NoError(t, err)
	defer func() { _ = session.Close() }()

	assert.Nil(t, weblog.HubFromContext(cmd.Context()))
	assert.Nil(t, logging.PluginLogTapFromContext(cmd.Context()))
}

// Not parallel: t.Setenv and global config.
func TestSetupLogging_WebLogsDisabledGetsNoHub(t *testing.T) {
	cmd, _ := webLogsTestCmd(t, "finfocus", true)
	t.Setenv(config.EnvWebLogs, "0")

	session, err := setupLoggingChecked(cmd)
	require.NoError(t, err)
	defer func() { _ = session.Close() }()

	assert.Nil(t, weblog.HubFromContext(cmd.Context()))
}

// Not parallel: t.Setenv and global config.
func TestSetupLogging_WebLogsConfigEnables(t *testing.T) {
	cmd, logPath := webLogsTestCmd(t, "finfocus", true)
	t.Setenv(config.EnvWebLogs, "")
	enabled := true
	config.SetGlobalConfig(&config.Config{
		Logging: config.LoggingConfig{Level: "info", Format: "json", File: logPath},
		Web:     &config.WebConfig{Logs: config.WebLogsConfig{Enabled: &enabled}},
	})

	session, err := setupLoggingChecked(cmd)
	require.NoError(t, err)
	defer func() { _ = session.Close() }()

	assert.NotNil(t, weblog.HubFromContext(cmd.Context()))
}

// Not parallel: t.Setenv and global config.
func TestStartLogging_BadWebLogsValueIsValidationError(t *testing.T) {
	cmd, _ := webLogsTestCmd(t, "finfocus", true)
	t.Setenv(config.EnvWebLogs, "maybe")

	var lifecycle commandLifecycle
	err := lifecycle.startLogging(cmd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), config.EnvWebLogs)
	assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err))
}

//nolint:paralleltest // t.Setenv and global config
func TestStartLogging_ServedLogsCallLeavesTheLogFileAlone(t *testing.T) {
	serverCmd, logPath := webLogsTestCmd(t, "finfocus", false)
	session := setupLogging(serverCmd)
	t.Cleanup(func() { _ = session.Close() })
	before, err := os.ReadFile(logPath)
	require.NoError(t, err)

	root := &cobra.Command{Use: "finfocus"}
	call := &cobra.Command{Use: logsCommandName, RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	root.AddCommand(call)
	call.SetContext(context.Background())

	lifecycle := commandLifecycle{serving: true, session: session}
	require.NoError(t, lifecycle.startLogging(call))

	after, err := os.ReadFile(logPath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "a logs call must not append to the file it reads")
	assert.Same(t, session, lifecycle.session, "the running server's session must be kept")
	require.NoError(t, lifecycle.stopLogging())
}

// Not parallel: t.Setenv and global config.
func TestStartLogging_BadWebLogsValueIgnoredWithoutDashboard(t *testing.T) {
	cmd, _ := webLogsTestCmd(t, "projected", false)
	t.Setenv(config.EnvWebLogs, "maybe")

	var lifecycle commandLifecycle
	require.NoError(t, lifecycle.startLogging(cmd))
	require.NoError(t, lifecycle.stopLogging())
}
