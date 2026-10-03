package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
)

const (
	historyOutputPlain   = "plain"
	historyOutputJSON    = "json"
	historyProgressWidth = 32
	defaultTermWidth     = 80
)

// NewCostHistoryCmd creates the `cost history` command group.
func NewCostHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Collect and view projected cost history for a Pulumi stack",
		Long: `Stores a projected-cost snapshot for each successful Pulumi update and charts
that timeline offline. collect needs the Pulumi CLI and cost plugins. view,
list, diff, and prune read the per-stack database only.`,
	}
	cmd.AddCommand(
		NewCostHistoryCollectCmd(),
		NewCostHistoryViewCmd(),
		NewCostHistoryListCmd(),
		NewCostHistoryDiffCmd(),
		NewCostHistoryPruneCmd(),
	)
	return cmd
}

func commandContext(cmd *cobra.Command) context.Context {
	if cmd != nil && cmd.Context() != nil {
		return cmd.Context()
	}
	return context.Background()
}

func flagYes(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Flags().Lookup("yes") == nil {
		return false
	}
	value, err := cmd.Flags().GetBool("yes")
	return err == nil && value
}

func parseDateFlag(value string, endOfDay bool) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.UTC(), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD)", value)
	}
	parsed = parsed.UTC()
	if endOfDay {
		parsed = parsed.Add(24*time.Hour - time.Nanosecond)
	}
	return parsed, nil
}

func historyDir(dir string) string {
	if dir != "" {
		return dir
	}
	return filepath.Join(config.ResolveConfigDir(), "history")
}

func historyPath(dir, stack string) (string, error) {
	name, err := history.CostFileName(stack)
	if err != nil {
		return "", err
	}
	return filepath.Join(historyDir(dir), name), nil
}

func displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	rel, err := filepath.Rel(home, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return "~/" + filepath.ToSlash(rel)
}

func stackKnown(name string, listed []string) bool {
	for _, item := range listed {
		if item == name || strings.HasSuffix(item, "/"+name) {
			return true
		}
	}
	return false
}

func parseStackList(data []byte) ([]string, error) {
	var entries []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parsing pulumi stack ls: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Name != "" {
			names = append(names, entry.Name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func joinStacks(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func execCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		if ok {
			return nil, fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

func terminalWidth() int {
	fd := int(os.Stdout.Fd())
	if !term.IsTerminal(fd) {
		return defaultTermWidth
	}
	width, _, err := term.GetSize(fd)
	if err != nil || width <= 0 {
		return defaultTermWidth
	}
	return fitTerminalChartWidth(width, true)
}

// fitTerminalChartWidth leaves the non-terminal default unchanged and reserves
// asciigraph's three-column axis when the measured width is a real terminal.
func fitTerminalChartWidth(columns int, isTerminal bool) int {
	const axisOffset = 3
	if !isTerminal || columns <= axisOffset {
		return columns
	}
	return columns - axisOffset
}

func globalBudget(cfg *config.Config) (float64, bool) {
	if cfg == nil {
		cfg = config.GetGlobalConfig()
	}
	if cfg == nil || cfg.Cost.Budgets == nil || !cfg.Cost.Budgets.HasGlobalBudget() {
		return 0, false
	}
	return cfg.Cost.Budgets.Global.Amount, true
}

func formatRemain(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	minutes := int(d / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}
