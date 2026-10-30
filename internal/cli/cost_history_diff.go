package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/history"
)

// NewCostHistoryDiffCmd creates `cost history diff`.
func NewCostHistoryDiffCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare resources between two stored cost snapshots",
		Long:  historyReadHelp,
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
	target, err := resolveHistoryCommandTarget(cmd, dir)
	if err != nil {
		return err
	}
	query, err := readDiffFlags(cmd)
	if err != nil {
		return err
	}
	stack := target.Stack
	db, err := openHistoryTargetReadExisting(target)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	diff, err := readHistoryDiff(db, query)
	if err != nil {
		return err
	}
	if format == outputFormatJSON {
		return writeJSON(cmd, history.NewDiffDocument(stack, diff))
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

func readDiffFlags(cmd *cobra.Command) (historyDiffQuery, error) {
	fromSpec, err := cmd.Flags().GetString("from")
	if err != nil {
		return historyDiffQuery{}, err
	}
	toSpec, err := cmd.Flags().GetString("to")
	if err != nil {
		return historyDiffQuery{}, err
	}
	threshold, err := cmd.Flags().GetFloat64("threshold")
	if err != nil {
		return historyDiffQuery{}, err
	}
	query := historyDiffQuery{From: fromSpec, To: toSpec, Threshold: threshold}
	if err = checkDiffInputs(query); err != nil {
		return historyDiffQuery{}, err
	}
	return query, nil
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
