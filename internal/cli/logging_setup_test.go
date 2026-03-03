package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/constants"
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
