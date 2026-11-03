package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/tui"
	"github.com/rshade/finfocus/internal/weblog"
)

const (
	logsDefaultTail  = 200
	logsLevelWarn    = "warn"
	logsLevelWarning = "warning"
	logsLevelError   = "error"
	logsCommandName  = "logs"
)

type logsParams struct {
	file    string
	tail    int
	sources []string
	level   string
	grep    string
	output  string
}

// newLogsCmd creates `finfocus logs`, a snapshot of the log file. It shares
// parsing, redaction, filtering, and display strings with the dashboard Logs
// view through the weblog package and starts no plugin.
func newLogsCmd() *cobra.Command {
	var params logsParams

	cmd := &cobra.Command{
		Use:   logsCommandName,
		Short: "Show the last entries of the finfocus log file",
		Long: "Print a snapshot of the finfocus log file, oldest entry first, using the " +
			"same parsing, redaction, filters, and display text as the dashboard Logs view. " +
			"The file is only read, never modified, and no plugin is started. " +
			"--grep is a case-insensitive substring match over the message and field values.",
		Example: `  # Last 200 entries of the configured log file
  finfocus logs

  # Everything from one plugin, as JSON
  finfocus logs --tail 0 --source aws-public --output json

  # Warnings and errors that mention a region
  finfocus logs --level warn --grep us-east-1

  # Read another file
  finfocus logs --file ./finfocus.log --output ndjson`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			params.output = resolveOutputFormat(cmd, "output", params.output)
			return runLogs(cmd, params)
		},
	}

	cmd.Flags().StringVar(&params.file, "file", "",
		"Log file to read (default: the configured logging.file)")
	cmd.Flags().IntVar(&params.tail, "tail", logsDefaultTail,
		"Number of most recent matching entries to show (0 for all)")
	cmd.Flags().StringArrayVar(&params.sources, "source", nil,
		"Only entries from this source, such as core or a plugin name (repeatable)")
	cmd.Flags().StringVar(&params.level, "level", "",
		"Minimum level: trace, debug, info, warn, error, fatal, panic")
	cmd.Flags().StringVar(&params.grep, "grep", "",
		"Only entries whose message or field values contain this text (case-insensitive)")
	cmd.Flags().StringVar(&params.output, "output", outputFormatTable,
		"Output format (table, json, ndjson)")
	addAccessibilityFlags(cmd)

	return cmd
}

func runLogs(cmd *cobra.Command, params logsParams) error {
	ctx := cmd.Context()
	if params.output != outputFormatTable && params.output != outputFormatJSON && params.output != outputFormatNDJSON {
		return unsupportedOutputError(ctx, params.output, "table, json, ndjson")
	}
	if params.tail < 0 {
		return toValidationError(ctx, fmt.Errorf("--tail must be 0 or greater, got %d", params.tail))
	}
	filter, err := logsFilter(params)
	if err != nil {
		return toValidationError(ctx, err)
	}

	path, explicit := logsPath(params.file)
	f, err := os.Open(path)
	if err != nil {
		return logsOpenError(ctx, path, explicit, err)
	}
	defer f.Close()

	sink := newLogsSink(cmd, params.output)
	entries := weblog.Decode(bufio.NewReader(f), "")
	if params.tail == 0 {
		return streamLogs(entries, filter, sink)
	}
	return tailLogs(entries, filter, params.tail, sink)
}

func logsFilter(params logsParams) (weblog.Filter, error) {
	level := strings.ToLower(strings.TrimSpace(params.level))
	if level == logsLevelWarning {
		level = logsLevelWarn
	}
	levels := []string{"trace", "debug", "info", logsLevelWarn, logsLevelError, "fatal", "panic"}
	if level != "" && !slices.Contains(levels, level) {
		return weblog.Filter{}, fmt.Errorf(
			"invalid --level %q (use trace, debug, info, warn, error, fatal, or panic)", params.level)
	}
	return weblog.Filter{Sources: params.sources, MinLevel: level, Text: params.grep}, nil
}

// logsPath returns the file to read and whether the caller named it. The
// default is the configured logging.file, the same file the logger writes.
func logsPath(flagValue string) (string, bool) {
	if flagValue != "" {
		return flagValue, true
	}
	cfg := config.InitializedGlobalConfig()
	if cfg == nil {
		cfg = config.New()
	}
	if cfg.Logging.File != "" {
		return cfg.Logging.File, false
	}
	return filepath.Join(config.ResolveConfigDir(), "logs", "finfocus.log"), false
}

func logsOpenError(ctx context.Context, path string, explicit bool, err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		if explicit {
			return toValidationError(ctx, fmt.Errorf("log file not found: %s", path))
		}
		return fmt.Errorf("log file not found: %s (pass --file to read another file)", path)
	}
	if explicit {
		return toValidationError(ctx, fmt.Errorf("cannot read log file %s: %w", path, err))
	}
	return fmt.Errorf("cannot read log file %s: %w", path, err)
}

func streamLogs(entries func(func(weblog.Entry, error) bool), filter weblog.Filter, sink *logsSink) error {
	var failure error
	for e, err := range entries {
		if err != nil {
			failure = err
			break
		}
		if !filter.Match(e) {
			continue
		}
		if writeErr := sink.write(e); writeErr != nil {
			return writeErr
		}
	}
	if failure != nil {
		return fmt.Errorf("reading log file: %w", failure)
	}
	return sink.close()
}

// tailLogs keeps only the last n matching entries in a ring, so memory is
// bounded by n and not by the file.
func tailLogs(entries func(func(weblog.Entry, error) bool), filter weblog.Filter, n int, sink *logsSink) error {
	ring := make([]weblog.Entry, n)
	var count int
	for e, err := range entries {
		if err != nil {
			return fmt.Errorf("reading log file: %w", err)
		}
		if !filter.Match(e) {
			continue
		}
		ring[count%n] = e
		count++
	}
	start, total := 0, count
	if count > n {
		start, total = count%n, n
	}
	for i := range total {
		if err := sink.write(ring[(start+i)%n]); err != nil {
			return err
		}
	}
	return sink.close()
}

type logsSink struct {
	w      io.Writer
	format string
	styled bool
	wrote  int
}

func newLogsSink(cmd *cobra.Command, format string) *logsSink {
	out := cmd.OutOrStdout()
	styled := format == outputFormatTable &&
		tui.DetectResolvedOutputMode(out, accessibilityFromCmd(cmd)) != tui.OutputModePlain
	return &logsSink{w: out, format: format, styled: styled}
}

func (s *logsSink) write(e weblog.Entry) error {
	defer func() { s.wrote++ }()
	switch s.format {
	case outputFormatJSON:
		data, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("encoding log entry: %w", err)
		}
		prefix := ",\n"
		if s.wrote == 0 {
			prefix = "[\n"
		}
		_, err = fmt.Fprintf(s.w, "%s  %s", prefix, data)
		return err
	case outputFormatNDJSON:
		data, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("encoding log entry: %w", err)
		}
		_, err = fmt.Fprintf(s.w, "%s\n", data)
		return err
	default:
		_, err := fmt.Fprintln(s.w, formatLogLine(e, s.styled))
		return err
	}
}

func (s *logsSink) close() error {
	if s.format != outputFormatJSON {
		return nil
	}
	if s.wrote == 0 {
		_, err := io.WriteString(s.w, "[]\n")
		return err
	}
	_, err := io.WriteString(s.w, "\n]\n")
	return err
}

// formatLogLine renders "TIME LEVEL [source] message key=value ...". The level
// label is text first; color only reinforces it.
func formatLogLine(e weblog.Entry, styled bool) string {
	display := e.Display
	if display == (weblog.DisplayStrings{}) {
		display = weblog.Display(e)
	}
	timeText, level, source := display.Time, display.Level, "["+display.Source+"]"
	if styled {
		timeText = tui.LabelStyle.Render(timeText)
		level = logLevelStyle(e.Level).Render(level)
		source = tui.LabelStyle.Render(source)
	}
	var b strings.Builder
	b.WriteString(timeText + " " + level + " " + source + " " + e.Message)
	for _, field := range formatLogFields(e.Fields, e.Source) {
		b.WriteString(" " + field)
	}
	return b.String()
}

func logLevelStyle(level string) lipgloss.Style {
	switch level {
	case logsLevelWarn:
		return tui.WarningStyle
	case logsLevelError, "fatal", "panic":
		return tui.CriticalStyle
	case "info":
		return tui.InfoStyle
	default:
		return tui.SubtleStyle
	}
}

// formatLogFields lists key=value pairs sorted by key. plugin_name and
// component are left out when they only repeat the source label.
func formatLogFields(fields map[string]any, source string) []string {
	keys := make([]string, 0, len(fields))
	for key, value := range fields {
		if (key == "plugin_name" || key == "component") && value == source {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+formatLogFieldValue(fields[key]))
	}
	return out
}

// readsLogFile reports whether cmd is `finfocus logs`. That command opens no
// logging session: the session would create the log file it is about to read
// and append its own "command started" line to it.
func readsLogFile(cmd *cobra.Command) bool {
	return cmd.Name() == logsCommandName && cmd.Parent() != nil && cmd.Parent().Parent() == nil
}

func formatLogFieldValue(value any) string {
	if text, ok := value.(string); ok {
		if text == "" || strings.ContainsAny(text, " \t\"=") {
			return strconv.Quote(text)
		}
		return text
	}
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}
