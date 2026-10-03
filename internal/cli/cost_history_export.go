package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/history"
)

const (
	exportFormatJSON   = "json"
	exportFormatCSV    = "csv"
	exportFormatNDJSON = "ndjson"
)

// NewCostHistoryExportCmd creates `cost history export`.
func NewCostHistoryExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export stored cost history as JSON, CSV, or NDJSON",
		Long: `Reads the per-stack cost history database and writes a time series.

--format is the root flag. json writes one document, csv writes a header
and one row per snapshot, and ndjson writes one snapshot per line.
--provider keeps every snapshot in the date range and reports that
provider's monthly cost.`,
		Example: `  finfocus cost history export --stack dev --format json
  finfocus cost history export --stack dev --format csv --from 2025-01-01 --to 2025-06-01
  finfocus cost history export --stack dev --format ndjson --provider aws`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runExport(cmd, "")
		},
	}
	cmd.Flags().String("from", "", "Export start date (YYYY-MM-DD)")
	cmd.Flags().String("to", "", "Export end date (YYYY-MM-DD, inclusive)")
	cmd.Flags().String("provider", "", "Report one provider's monthly cost")
	return cmd
}

func runExport(cmd *cobra.Command, dir string) error {
	format, err := exportFormat(cmd)
	if err != nil {
		return err
	}
	from, to, err := viewBounds(cmd)
	if err != nil {
		return err
	}
	stack, err := requiredStack(cmd)
	if err != nil {
		return err
	}
	provider, err := cmd.Flags().GetString("provider")
	if err != nil {
		return err
	}
	db, err := openHistoryRead(dir, stack)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	snapshots, err := db.Snapshots(from, to)
	if err != nil {
		return err
	}
	annotations, err := db.Annotations(from, to)
	if err != nil {
		return err
	}
	selected, err := history.SelectCurrency(snapshots, history.CurrencyChoice{})
	if err != nil {
		return err
	}
	if selected.Warning != "" {
		fmt.Fprintln(cmd.ErrOrStderr(), selected.Warning)
	}
	doc := history.BuildExport(
		stack,
		selected.Currency,
		provider,
		selected.Snapshots,
		annotationsForSnapshots(selected.Snapshots, annotations),
		time.Now().UTC(),
	)
	return history.WriteExport(cmd.OutOrStdout(), format, doc)
}

func exportFormat(cmd *cobra.Command) (string, error) {
	if cmd == nil || cmd.Flags().Lookup("format") == nil {
		return "", errExportFormat
	}
	raw, err := cmd.Flags().GetString("format")
	if err != nil {
		return "", err
	}
	format := strings.ToLower(strings.TrimSpace(raw))
	switch format {
	case exportFormatJSON, exportFormatCSV, exportFormatNDJSON:
		return format, nil
	case "", "human":
		return "", errExportFormat
	default:
		return "", fmt.Errorf("unsupported export format: %s (supported: json, csv, ndjson)", raw)
	}
}

var errExportFormat = errors.New("--format is required (json, csv, or ndjson)")

func openHistoryRead(dir, stack string) (*history.CostDB, error) {
	path, err := historyPath(dir, stack)
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, fmt.Errorf("no cost history for stack '%s'", stack)
		}
		return nil, fmt.Errorf("reading cost history: %w", statErr)
	}
	db, err := history.OpenCostDBRead(path)
	if err != nil {
		return nil, err
	}
	if err = diffStackMatches(db, stack); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// costTableTrends loads sparklines for the cost command's --stack.
// A missing stack, a missing file, or a read error returns nil so the cost
// command keeps its table and omits the Trend column.
func costTableTrends(cmd *cobra.Command) (map[string]string, string) {
	if cmd == nil {
		return nil, ""
	}
	stack, err := cmd.Flags().GetString("stack")
	if err != nil || strings.TrimSpace(stack) == "" {
		return nil, ""
	}
	db, err := openHistoryRead("", stack)
	if err != nil {
		return nil, ""
	}
	defer func() { _ = db.Close() }()
	snapshots, err := db.Snapshots(time.Time{}, time.Time{})
	if err != nil {
		return nil, ""
	}
	selected, err := history.SelectCurrency(snapshots, history.CurrencyChoice{})
	if err != nil {
		return nil, ""
	}
	return history.TableTrends(selected.Snapshots)
}
