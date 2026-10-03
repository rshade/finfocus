package cli

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/history"
)

// NewCostHistoryDiffCmd creates `cost history diff`.
func NewCostHistoryDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare resources between two stored cost snapshots",
		Example: `  finfocus cost history diff --stack dev --from v35 --to v42
  finfocus cost history diff --stack dev --from 2025-03-15 --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDiff(cmd, "")
		},
	}
	cmd.Flags().String("from", "", "Start snapshot: version (v35) or date (YYYY-MM-DD)")
	cmd.Flags().String("to", "", "End snapshot: version or date (default: latest)")
	cmd.Flags().String("output", outputFormatTable, "Output format: table, json")
	cmd.Flags().Float64("threshold", 0, "Hide changes at or below this monthly amount")
	return cmd
}

func runDiff(cmd *cobra.Command, dir string) error {
	format, err := diffOutputFormat(cmd)
	if err != nil {
		return err
	}
	stack, err := requiredStack(cmd)
	if err != nil {
		return err
	}
	fromSpec, toSpec, threshold, err := readDiffFlags(cmd)
	if err != nil {
		return err
	}
	path, err := historyPath(dir, stack)
	if err != nil {
		return err
	}
	db, err := history.OpenCostDBRead(path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if err = diffStackMatches(db, stack); err != nil {
		return err
	}
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	if err != nil {
		return err
	}
	fromSnap, err := history.ResolveCostSnapshot(snaps, fromSpec)
	if err != nil {
		return err
	}
	toSnap, err := history.ResolveCostSnapshot(snaps, toSpec)
	if err != nil {
		return err
	}
	diff, err := history.DiffSnapshots(fromSnap, toSnap, threshold)
	if err != nil {
		return err
	}
	if format == outputFormatJSON {
		return writeJSON(cmd, newCostDiffJSON(stack, diff))
	}
	cmd.Print(history.RenderCostDiff(stack, diff))
	return nil
}

func diffOutputFormat(cmd *cobra.Command) (string, error) {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return "", err
	}
	format := resolveOutputFormat(cmd, "output", output)
	if format != outputFormatTable && format != outputFormatJSON {
		return "", fmt.Errorf("unsupported output format: %s (supported: table, json)", format)
	}
	return format, nil
}

func readDiffFlags(cmd *cobra.Command) (string, string, float64, error) {
	fromSpec, err := cmd.Flags().GetString("from")
	if err != nil {
		return "", "", 0, err
	}
	if strings.TrimSpace(fromSpec) == "" {
		return "", "", 0, errors.New("--from is required")
	}
	toSpec, err := cmd.Flags().GetString("to")
	if err != nil {
		return "", "", 0, err
	}
	threshold, err := cmd.Flags().GetFloat64("threshold")
	if err != nil {
		return "", "", 0, err
	}
	if threshold < 0 {
		return "", "", 0, errors.New("--threshold must be >= 0")
	}
	return fromSpec, toSpec, threshold, nil
}

func diffStackMatches(db *history.CostDB, stack string) error {
	stats, err := db.Stats()
	if err != nil {
		return err
	}
	if stats.Stack != "" && stats.Stack != stack {
		return fmt.Errorf("cost history database stack %q does not match %q", stats.Stack, stack)
	}
	return nil
}

type costDiffPointJSON struct {
	Version      int     `json:"version"`
	Timestamp    string  `json:"timestamp"`
	TotalMonthly float64 `json:"total_monthly"`
}

type costDiffItemJSON struct {
	URN         string  `json:"urn"`
	Type        string  `json:"type,omitempty"`
	MonthlyCost float64 `json:"monthly_cost"`
	Delta       float64 `json:"delta"`
	SKU         string  `json:"sku,omitempty"`
	Region      string  `json:"region,omitempty"`
}

type costDiffJSON struct {
	Stack          string             `json:"stack"`
	From           costDiffPointJSON  `json:"from"`
	To             costDiffPointJSON  `json:"to"`
	Delta          float64            `json:"delta"`
	Currency       string             `json:"currency"`
	Added          []costDiffItemJSON `json:"added"`
	Removed        []costDiffItemJSON `json:"removed"`
	Changed        []costDiffItemJSON `json:"changed"`
	UnchangedCount int                `json:"unchanged_count"`
	UnchangedTotal float64            `json:"unchanged_total"`
}

func newCostDiffJSON(stack string, diff history.CostDiff) costDiffJSON {
	return costDiffJSON{
		Stack:          stack,
		From:           pointJSON(diff.From),
		To:             pointJSON(diff.To),
		Delta:          diff.Delta,
		Currency:       diff.Currency,
		Added:          itemsJSON(diff.Added),
		Removed:        itemsJSON(diff.Removed),
		Changed:        itemsJSON(diff.Changed),
		UnchangedCount: diff.UnchangedCount,
		UnchangedTotal: diff.UnchangedTotal,
	}
}

func pointJSON(point history.CostDiffPoint) costDiffPointJSON {
	stamp := ""
	if !point.Timestamp.IsZero() {
		stamp = point.Timestamp.UTC().Format(time.RFC3339)
	}
	return costDiffPointJSON{Version: point.Version, Timestamp: stamp, TotalMonthly: point.TotalMonthly}
}

func itemsJSON(items []history.CostDiffItem) []costDiffItemJSON {
	out := make([]costDiffItemJSON, 0, len(items))
	for _, item := range items {
		out = append(out, costDiffItemJSON{
			URN:         item.URN,
			Type:        item.Type,
			MonthlyCost: item.MonthlyCost,
			Delta:       item.Delta,
			SKU:         item.SKU,
			Region:      item.Region,
		})
	}
	return out
}
