package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/constants"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// loggingSession owns the log destinations opened for one CLI process
// invocation: the logger, the plugin I/O log handle, and the audit logger.
//
// A normal command opens a session in the root PersistentPreRunE and closes it
// in PersistentPostRunE. An MCP server (mcp-server or --mcp) opens one session
// when the server command starts; every dispatched tools/call reuses it through
// attach instead of opening and closing its own, so file handles are neither
// leaked nor closed twice across calls.
type loggingSession struct {
	logging.LogPathResult

	logger          zerolog.Logger
	pluginLogWriter *os.File
	auditLogger     logging.AuditLogger

	closeOnce sync.Once
	closeErr  error
}

// setupLogging configures logging based on config file, environment, and CLI flags,
// attaches it to cmd's context, and returns the session that owns the opened handles.
func setupLogging(cmd *cobra.Command) *loggingSession {
	loggingCfg := config.GetLoggingConfig()

	debug, _ := cmd.Flags().GetBool("debug")
	if debug {
		loggingCfg.Level = "debug"
	}

	if envLevel := os.Getenv(pluginsdk.EnvLogLevel); envLevel != "" && !debug {
		loggingCfg.Level = envLevel
	}
	if envFormat := os.Getenv(pluginsdk.EnvLogFormat); envFormat != "" {
		loggingCfg.Format = envFormat
	}

	// When running as a Pulumi Analyzer plugin, redirect all logs to a file so
	// that JSON log lines do not appear in `pulumi preview`'s Diagnostics output
	// (#748). This applies even when no log file is configured.
	if os.Getenv(constants.EnvAnalyzerMode) == "true" {
		if loggingCfg.File == "" {
			loggingCfg.File = filepath.Join(config.ResolveConfigDir(), "logs", "analyzer.log")
		}
	}

	// Ensure log directory exists after all overrides have been applied.
	if loggingCfg.File != "" {
		if err := config.EnsureLogDir(); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Warning: could not create log directory: %v\n", err)
		}
	}

	result := logging.NewLoggerWithPath(loggingCfg.ToLoggingConfig())
	session := &loggingSession{
		LogPathResult: result,
		logger:        logging.ComponentLogger(result.Logger, "cli"),
	}

	if !suppressAuxOutputFromContext(cmd.Context()) {
		if result.UsingFile {
			logging.PrintLogPathMessage(cmd.ErrOrStderr(), result.FilePath)
		} else if result.FallbackUsed {
			logging.PrintFallbackWarning(cmd.ErrOrStderr(), result.FallbackReason)
		}
	}

	// When logging to a file, open a second append-mode handle for plugin I/O.
	// Plugin stderr/stdout will be redirected here to keep the terminal clean.
	// When no file is configured, plugins write to stderr only when it is an
	// interactive terminal; otherwise their output is discarded so orphaned
	// plugins cannot hold inherited pipes open (issue #1231).
	if result.UsingFile {
		pluginLogFile, err := os.OpenFile(result.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			session.logger.Warn().Err(err).Msg("could not open plugin log file, plugin output will go to stderr")
		} else {
			session.SetPluginLogFile(pluginLogFile)
			session.pluginLogWriter = pluginLogFile
		}
	}

	session.auditLogger = logging.NewAuditLogger(logging.AuditLoggerConfig{
		Enabled: loggingCfg.Audit.Enabled,
		File:    loggingCfg.Audit.File,
	})

	session.attach(cmd)
	return session
}

// attach decorates cmd's context with the session's logger, plugin log writer,
// and audit logger, plus the per-invocation trace ID and skip-version-check value.
func (s *loggingSession) attach(cmd *cobra.Command) {
	s.attachWithTrace(cmd, logging.GetOrGenerateTraceID(cmd.Context()))
}

// attachCall attaches the session to a dispatched MCP tools/call. The call's
// context inherits the server's trace ID, so it is ignored: each call gets a
// fresh ID unless FINFOCUS_TRACE_ID injects one externally.
func (s *loggingSession) attachCall(cmd *cobra.Command) {
	s.attachWithTrace(cmd, logging.GetOrGenerateTraceID(context.Background()))
}

func (s *loggingSession) attachWithTrace(cmd *cobra.Command, traceID string) {
	skipVersionCheck, _ := cmd.Flags().GetBool("skip-version-check")
	ctx := context.WithValue(cmd.Context(), pluginhost.SkipVersionCheckKey, skipVersionCheck)
	ctx = logging.ContextWithTraceID(ctx, traceID)
	ctx = s.logger.WithContext(ctx)

	if s.pluginLogWriter != nil {
		ctx = logging.ContextWithPluginLogWriter(ctx, s.pluginLogWriter)
		ctx = logging.ContextWithPluginLogPath(ctx, s.FilePath)
	}

	ctx = logging.ContextWithAuditLogger(ctx, s.auditLogger)
	cmd.SetContext(ctx)

	s.logger.Info().Ctx(ctx).Str("command", cmd.Name()).Msg("command started")
}

// Close releases the audit logger and log file handles. It is idempotent: only
// the first call closes anything, and every call returns the first result.
func (s *loggingSession) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		auditErr := s.auditLogger.Close()
		fileErr := s.LogPathResult.Close()
		s.closeErr = errors.Join(auditErr, fileErr)
	})
	return s.closeErr
}
