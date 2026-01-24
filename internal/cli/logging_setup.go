package cli

import (
	"context"
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
