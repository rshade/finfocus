package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// setupLogging configures logging based on config file, environment, and CLI flags.
func setupLogging(cmd *cobra.Command) logging.LogPathResult {
	loggingCfg := config.GetLoggingConfig()

	debug, _ := cmd.Flags().GetBool("debug")
	if debug {
		loggingCfg.Level = "debug"
		loggingCfg.Format = "console"
		loggingCfg.File = ""
	}

	if envLevel := os.Getenv(pluginsdk.EnvLogLevel); envLevel != "" && !debug {
		loggingCfg.Level = envLevel
	}
	if envFormat := os.Getenv(pluginsdk.EnvLogFormat); envFormat != "" {
		loggingCfg.Format = envFormat
	}

	// Ensure log directory exists after all overrides have been applied.
	if loggingCfg.File != "" {
		if err := config.EnsureLogDir(); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not create log directory: %v\n", err)
		}
	}

	result := logging.NewLoggerWithPath(loggingCfg.ToLoggingConfig())
	logger = logging.ComponentLogger(result.Logger, "cli")

	if result.UsingFile {
		logging.PrintLogPathMessage(cmd.ErrOrStderr(), result.FilePath)
	} else if result.FallbackUsed {
		logging.PrintFallbackWarning(cmd.ErrOrStderr(), result.FallbackReason)
	}

	skipVersionCheck, _ := cmd.Flags().GetBool("skip-version-check")
	ctx := context.WithValue(cmd.Context(), pluginhost.SkipVersionCheckKey, skipVersionCheck)
	traceID := logging.GetOrGenerateTraceID(ctx)
	ctx = logging.ContextWithTraceID(ctx, traceID)
	ctx = logger.WithContext(ctx)

	// When logging to a file, open a second append-mode handle for plugin I/O.
	// Plugin stderr/stdout will be redirected here to keep the terminal clean.
	// In debug mode (no file), plugins continue writing to stderr for visibility.
	if result.UsingFile {
		pluginLogFile, err := os.OpenFile(result.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			logger.Warn().Err(err).Msg("could not open plugin log file, plugin output will go to stderr")
		} else {
			result.SetPluginLogFile(pluginLogFile)
			ctx = logging.ContextWithPluginLogWriter(ctx, pluginLogFile)
			ctx = logging.ContextWithPluginLogPath(ctx, result.FilePath)
		}
	}

	auditLogger := logging.NewAuditLogger(logging.AuditLoggerConfig{
		Enabled: loggingCfg.Audit.Enabled,
		File:    loggingCfg.Audit.File,
	})
	ctx = logging.ContextWithAuditLogger(ctx, auditLogger)
	cmd.SetContext(ctx)

	logger.Info().Ctx(ctx).Str("command", cmd.Name()).Msg("command started")

	return result
}

// cleanupLogging closes audit logger and log file handles.
func cleanupLogging(cmd *cobra.Command, logResult *logging.LogPathResult) error {
	ctx := cmd.Context()
	if err := logging.AuditLoggerFromContext(ctx).Close(); err != nil {
		return err
	}
	if logResult != nil {
		return logResult.Close()
	}
	return nil
}
