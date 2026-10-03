package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/rshade/ax-go"
	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
)

// NewCostHistoryPruneCmd creates `cost history prune`.
func NewCostHistoryPruneCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Delete old projected-cost snapshots for one stack",
		Example: `  finfocus cost history prune --stack dev --older-than 365d --dry-run
  finfocus cost history prune --stack dev --keep 100 --force`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPrune(cmd, "")
		},
	}
	cmd.Flags().Int("keep", 0, "Keep the N most recent snapshots (0 = no limit)")
	cmd.Flags().String("older-than", "", "Remove snapshots older than a duration (365d, 6m, 2y)")
	cmd.Flags().Bool("force", false, "Skip the confirmation prompt")
	return cmd
}

func runPrune(cmd *cobra.Command, dir string) error {
	stack, err := requiredStack(cmd)
	if err != nil {
		return err
	}
	policy, err := readPrunePolicy(cmd)
	if err != nil {
		return err
	}
	path, err := historyPath(dir, stack)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return fmt.Errorf("no cost history for stack %q", stack)
		}
		return fmt.Errorf("statting cost history: %w", statErr)
	}
	db, err := history.OpenCostDB(path, stack)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	plan, err := db.ApplyPrune(policy, true)
	if err != nil {
		return err
	}
	if plan.Pruned == 0 {
		cmd.Printf("Stack: %s\nNothing to prune.\n", stack)
		return nil
	}
	if historyDryRun(cmd) {
		writePruneDryRun(cmd, stack, plan, policy)
		return nil
	}
	if !flagYes(cmd) && !flagForce(cmd) {
		prompt := fmt.Sprintf("Prune %d snapshots on %s? [y/N] ", plan.Pruned, stack)
		if !confirmWithReader(cmd, prompt) {
			return errors.New("prune cancelled")
		}
	}
	return executePrune(cmd, db, path, stack, policy)
}

func readPrunePolicy(cmd *cobra.Command) (history.CostPrunePolicy, error) {
	keep, err := cmd.Flags().GetInt("keep")
	if err != nil {
		return history.CostPrunePolicy{}, err
	}
	if keep < 0 {
		return history.CostPrunePolicy{}, errors.New("--keep must be >= 0")
	}
	spec, err := cmd.Flags().GetString("older-than")
	if err != nil {
		return history.CostPrunePolicy{}, err
	}
	cutoff, err := history.ParseOlderThan(spec, time.Now())
	if err != nil {
		return history.CostPrunePolicy{}, err
	}
	policy := history.CostPrunePolicy{Keep: keep, Cutoff: cutoff}
	if !policy.Active() {
		return history.CostPrunePolicy{}, errors.New("set --keep or --older-than")
	}
	return policy, nil
}

func flagForce(cmd *cobra.Command) bool {
	if cmd == nil || cmd.Flags().Lookup("force") == nil {
		return false
	}
	value, err := cmd.Flags().GetBool("force")
	return err == nil && value
}

func historyDryRun(cmd *cobra.Command) bool {
	if cmd != nil && ax.DryRunFromContext(commandContext(cmd)) {
		return true
	}
	if cmd == nil || cmd.Flags().Lookup("dry-run") == nil {
		return false
	}
	value, err := cmd.Flags().GetBool("dry-run")
	return err == nil && value
}

func writePruneDryRun(
	cmd *cobra.Command,
	stack string,
	plan history.CostPruneResult,
	policy history.CostPrunePolicy,
) {
	cmd.Printf("Stack: %s\n", stack)
	cmd.Printf("Total snapshots: %d\n", plan.Total)
	cmd.Printf("Snapshots to prune: %d (%s)\n", plan.Pruned, pruneReason(policy))
	cmd.Printf("Snapshots to keep: %d\n", plan.Kept)
	freed := formatSize(estimatedFreed(plan.SizeBefore, plan.Pruned, plan.Total))
	cmd.Printf("Estimated space freed: ~%s\n", freed)
	cmd.Println("Run without --dry-run to execute.")
}

func executePrune(
	cmd *cobra.Command,
	db *history.CostDB,
	path, stack string,
	policy history.CostPrunePolicy,
) error {
	plan, err := db.ApplyPrune(policy, false)
	if err != nil {
		return err
	}
	if err = db.Close(); err != nil {
		return err
	}
	before, after, err := history.CompactCostFile(path)
	if err != nil {
		return err
	}
	cmd.Printf("Stack: %s\n", stack)
	cmd.Printf("Pruning %d snapshots %s...\n", plan.Pruned, pruneReason(policy))
	cmd.Printf("Removed %d snapshots and %d annotations.\n", plan.Pruned, plan.Pruned)
	writeCompactLine(cmd.OutOrStdout(), before, after)
	cmd.Println("Done.")
	return nil
}

func pruneReason(policy history.CostPrunePolicy) string {
	const reasonParts = 2
	parts := make([]string, 0, reasonParts)
	if !policy.Cutoff.IsZero() {
		parts = append(parts, "older than "+policy.Cutoff.UTC().Format("2006-01-02"))
	}
	if policy.Keep > 0 {
		parts = append(parts, fmt.Sprintf("beyond the %d most recent", policy.Keep))
	}
	return strings.Join(parts, "; ")
}

func estimatedFreed(size int64, pruned, total int) int64 {
	if size <= 0 || pruned <= 0 || total <= 0 {
		return 0
	}
	return size * int64(pruned) / int64(total)
}

func writeCompactLine(w io.Writer, before, after int64) {
	fmt.Fprintf(w, "Compacted database: %s → %s\n", formatSize(before), formatSize(after))
}

func autoPruneCostHistory(cmd *cobra.Command, db *history.CostDB, path string, cfg *config.Config) error {
	policy, ok := retentionPolicy(cfg, time.Now())
	if !ok {
		return nil
	}
	result, err := db.ApplyPrune(policy, false)
	if err != nil {
		return err
	}
	if result.Pruned == 0 {
		return nil
	}
	if err = db.Close(); err != nil {
		return err
	}
	before, after, err := history.CompactCostFile(path)
	if err != nil {
		return err
	}
	cmd.Printf("Pruned %d snapshots (retention policy).\n", result.Pruned)
	writeCompactLine(cmd.OutOrStdout(), before, after)
	return nil
}

func retentionPolicy(cfg *config.Config, now time.Time) (history.CostPrunePolicy, bool) {
	if cfg == nil || !cfg.Cost.History.Retention.AutoPrune {
		return history.CostPrunePolicy{}, false
	}
	ret := cfg.Cost.History.Retention
	policy := history.CostPrunePolicy{Keep: ret.MaxSnapshots}
	if ret.MaxAgeDays > 0 {
		policy.Cutoff = now.UTC().AddDate(0, 0, -ret.MaxAgeDays)
	}
	return policy, policy.Active()
}
