package cli

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/history"
)

// NewCostHistoryListCmd creates `cost history list`.
func NewCostHistoryListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List stored Pulumi stack cost histories",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runList(cmd, "")
		},
	}
}

func runList(cmd *cobra.Command, dir string) error {
	stats, err := history.ListCostDBs(historyDir(dir))
	if err != nil {
		return err
	}
	if machineOutputRequested(cmd) {
		return writeJSON(cmd, historyListItems(stats))
	}
	if len(stats) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No cost history databases.")
		return nil
	}
	writeHistoryTable(cmd.OutOrStdout(), stats, time.Now())
	return nil
}

type historyListItem struct {
	Stack       string `json:"stack"`
	Path        string `json:"path"`
	Snapshots   int    `json:"snapshots"`
	First       string `json:"first,omitempty"`
	Last        string `json:"last,omitempty"`
	CollectedAt string `json:"collected_at,omitempty"`
	Size        int64  `json:"size"`
	LastVersion uint64 `json:"last_version"`
}

func historyListItems(stats []history.CostDBStats) []historyListItem {
	items := make([]historyListItem, 0, len(stats))
	for _, stat := range stats {
		item := historyListItem{
			Stack:       stat.Stack,
			Path:        stat.Path,
			Snapshots:   stat.Snapshots,
			Size:        stat.Size,
			LastVersion: stat.LastVersion,
		}
		if !stat.First.IsZero() {
			item.First = stat.First.UTC().Format(time.RFC3339)
		}
		if !stat.Last.IsZero() {
			item.Last = stat.Last.UTC().Format(time.RFC3339)
		}
		if !stat.CollectedAt.IsZero() {
			item.CollectedAt = stat.CollectedAt.UTC().Format(time.RFC3339)
		}
		items = append(items, item)
	}
	return items
}

func writeHistoryTable(w io.Writer, stats []history.CostDBStats, now time.Time) {
	const pad = 2
	tw := tabwriter.NewWriter(w, 0, 0, pad, ' ', 0)
	fmt.Fprintln(tw, "Stack\tSnapshots\tDate Range\tLast Collected\tSize")
	for _, stat := range stats {
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\n",
			stat.Stack,
			stat.Snapshots,
			formatHistoryRange(stat),
			formatCollected(stat.CollectedAt, now),
			formatSize(stat.Size),
		)
	}
	_ = tw.Flush()
}

func formatHistoryRange(stat history.CostDBStats) string {
	if stat.Snapshots == 0 || stat.First.IsZero() {
		return "-"
	}
	return stat.First.UTC().Format("Jan 2006") + " – " + stat.Last.UTC().Format("Jan 2006")
}

func formatCollected(when, now time.Time) string {
	if when.IsZero() {
		return "-"
	}
	const hoursPerDay = 24
	d := now.Sub(when)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < hoursPerDay*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days ago", int(d.Hours()/hoursPerDay))
	}
}

func formatSize(size int64) string {
	const (
		kb = 1024
		mb = 1024 * 1024
	)
	switch {
	case size >= mb:
		return fmt.Sprintf("%.1f MB", float64(size)/mb)
	case size >= kb:
		return fmt.Sprintf("%.0f KB", float64(size)/kb)
	default:
		return fmt.Sprintf("%d B", size)
	}
}
