package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/constants"
	"github.com/rshade/finfocus/internal/logging"
)

// newTestLoggingCmd builds a minimal cobra.Command with the flags that setupLogging
// reads. The command's context is pre-set to context.Background() so that
// context.WithValue in setupLogging does not panic.
func newTestLoggingCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	cmd.PersistentFlags().Bool("debug", false, "enable debug logging")
	cmd.PersistentFlags().Bool("skip-version-check", false, "skip version check")
	cmd.SetContext(context.Background())
	return cmd
}

// minimalLoggingConfig returns a *config.Config whose Logging section has no log
// file set (empty File field), so that the default behaviour of setupLogging is
// to write to stderr (UsingFile == false).  All other fields are left at their
// zero values, which is safe for the subset of config accessed by setupLogging.
func minimalLoggingConfig() *config.Config {
	return &config.Config{
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			// File intentionally empty: default is stderr output.
		},
	}
}

// TestSetupLogging_AnalyzerModeRedirect verifies that when FINFOCUS_ANALYZER_MODE=true
// setupLogging redirects all log output to a file instead of stderr (#748).
//
// The critical scenario is that a config with no log file configured would
// normally route logs to stderr, but analyzer mode must override this and force
// file output so that JSON log lines never appear in pulumi preview's Diagnostics.
func TestSetupLogging_AnalyzerModeRedirect(t *testing.T) {
	t.Run("analyzer mode forces file output when config has no log file", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("FINFOCUS_HOME", tmpDir)
		t.Setenv(constants.EnvAnalyzerMode, "true")

		// Use a minimal config with no log file so the default would be stderr.
		config.SetGlobalConfig(minimalLoggingConfig())
		t.Cleanup(config.ResetGlobalConfigForTest)

		cmd := newTestLoggingCmd()
		result := setupLogging(cmd)
		defer func() { _ = result.Close() }()

		assert.True(t, result.UsingFile,
			"analyzer mode must redirect logs to a file, not stderr, "+
				"even when the config specifies no log file")
		assert.NotEmpty(t, result.FilePath, "file path must be set in analyzer mode")
		// The default analyzer log path must be under FINFOCUS_HOME.
		assert.Contains(t, result.FilePath, tmpDir,
			"analyzer log file must be placed under FINFOCUS_HOME")
	})

	t.Run("normal mode uses stderr when config has no log file", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("FINFOCUS_HOME", tmpDir)
		t.Setenv(constants.EnvAnalyzerMode, "")

		// Same minimal config: no log file configured.
		config.SetGlobalConfig(minimalLoggingConfig())
		t.Cleanup(config.ResetGlobalConfigForTest)

		cmd := newTestLoggingCmd()
		result := setupLogging(cmd)
		defer func() { _ = result.Close() }()

		assert.False(t, result.UsingFile,
			"without analyzer mode, a config with no log file must use stderr output")

		// Verify the analyzer log file was NOT created.
		analyzerLog := filepath.Join(tmpDir, "logs", "analyzer.log")
		require.NoError(t, result.Close())
		assert.NoFileExists(t, analyzerLog,
			"analyzer.log must not be created in normal mode")
	})
}

// TestSetupLogging_LogPathMessageSuppression verifies structured output mode can
// suppress the "Logging to:" helper line while still using file logging.
func TestSetupLogging_LogPathMessageSuppression(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)
	t.Setenv(constants.EnvAnalyzerMode, "")

	logPath := filepath.Join(tmpDir, "logs", "finfocus.log")
	config.SetGlobalConfig(&config.Config{
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			File:   logPath,
		},
	})
	t.Cleanup(config.ResetGlobalConfigForTest)

	t.Run("default emits log path helper", func(t *testing.T) {
		cmd := newTestLoggingCmd()
		var errBuf bytes.Buffer
		cmd.SetErr(&errBuf)

		result := setupLogging(cmd)
		defer func() { _ = result.Close() }()

		require.True(t, result.UsingFile)
		assert.Contains(t, errBuf.String(), "Logging to:")
	})

	t.Run("suppressed context hides log path helper", func(t *testing.T) {
		cmd := newTestLoggingCmd()
		cmd.SetContext(contextWithSuppressAuxOutput(context.Background(), true))
		var errBuf bytes.Buffer
		cmd.SetErr(&errBuf)

		result := setupLogging(cmd)
		defer func() { _ = result.Close() }()

		require.True(t, result.UsingFile)
		assert.NotContains(t, errBuf.String(), "Logging to:")
	})
}

// TestSetupLogging_DebugPreservesFileOutput verifies that --debug raises log
// level without forcing logs off the configured file sink.
func TestSetupLogging_DebugPreservesFileOutput(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)
	t.Setenv("FINFOCUS_ANALYZER_MODE", "")
	logPath := filepath.Join(tmpDir, "logs", "finfocus.log")
	config.SetGlobalConfig(&config.Config{
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			File:   logPath,
		},
	})
	t.Cleanup(config.ResetGlobalConfigForTest)

	cmd := newTestLoggingCmd()
	require.NoError(t, cmd.PersistentFlags().Set("debug", "true"))
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)

	result := setupLogging(cmd)
	defer func() { _ = result.Close() }()

	require.True(t, result.UsingFile)
	assert.Equal(t, logPath, result.FilePath)
	assert.Contains(t, errBuf.String(), "Logging to:")
}

// TestLoggingSession_CloseIsIdempotent verifies a session that owns open log and
// audit files can be closed repeatedly. An MCP server used to close the same
// handle twice and fail with "file already closed".
func TestLoggingSession_CloseIsIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)
	t.Setenv(constants.EnvAnalyzerMode, "")
	config.SetGlobalConfig(&config.Config{
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			File:   filepath.Join(tmpDir, "logs", "finfocus.log"),
			Audit:  config.AuditConfig{Enabled: true, File: filepath.Join(tmpDir, "logs", "audit.log")},
		},
	})
	t.Cleanup(config.ResetGlobalConfigForTest)

	session := setupLogging(newTestLoggingCmd())
	require.True(t, session.UsingFile)

	require.NoError(t, session.Close())
	require.NoError(t, session.Close())
}

// TestLoggingSession_AttachReusesHandles verifies attach decorates another
// command's context with the same session without opening new files.
func TestLoggingSession_AttachReusesHandles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", tmpDir)
	t.Setenv(constants.EnvAnalyzerMode, "")
	config.SetGlobalConfig(&config.Config{
		Logging: config.LoggingConfig{
			Level:  "info",
			Format: "json",
			File:   filepath.Join(tmpDir, "logs", "finfocus.log"),
		},
	})
	t.Cleanup(config.ResetGlobalConfigForTest)

	session := setupLogging(newTestLoggingCmd())
	t.Cleanup(func() { _ = session.Close() })

	call := newTestLoggingCmd()
	var errBuf bytes.Buffer
	call.SetErr(&errBuf)
	session.attach(call)

	assert.Empty(t, errBuf.String(), "attach must not print the log path again")
	assert.Equal(t, session.FilePath, logging.PluginLogPathFromContext(call.Context()))
	assert.NotEmpty(t, logging.TraceIDFromContext(call.Context()))
}

// TestLoggingSession_AttachCallTraceIDs verifies sequential dispatched MCP calls
// get distinct trace IDs instead of inheriting the server's, while an external
// FINFOCUS_TRACE_ID still applies to every call.
func TestLoggingSession_AttachCallTraceIDs(t *testing.T) {
	tests := []struct {
		name          string
		externalTrace string
	}{
		{name: "fresh ID per call", externalTrace: ""},
		{name: "external ID preserved", externalTrace: "external-trace-123"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FINFOCUS_HOME", t.TempDir())
			t.Setenv(constants.EnvAnalyzerMode, "")
			t.Setenv(pluginsdk.EnvTraceID, tc.externalTrace)
			config.SetGlobalConfig(minimalLoggingConfig())
			t.Cleanup(config.ResetGlobalConfigForTest)

			server := newTestLoggingCmd()
			session := setupLogging(server)
			t.Cleanup(func() { _ = session.Close() })
			serverTrace := logging.TraceIDFromContext(server.Context())

			call := newTestLoggingCmd()
			call.SetContext(server.Context())
			seen := map[string]bool{}
			for range 3 {
				session.attachCall(call)
				seen[logging.TraceIDFromContext(call.Context())] = true
			}

			if tc.externalTrace != "" {
				assert.Equal(t, map[string]bool{tc.externalTrace: true}, seen)
				return
			}
			assert.Len(t, seen, 3, "each dispatched call must get its own trace ID")
			assert.NotContains(t, seen, serverTrace, "calls must not inherit the server's trace ID")
		})
	}
}
