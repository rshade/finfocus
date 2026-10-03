package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
)

const defaultChartHeight = 15

type viewDeps struct {
	dir   string
	cfg   *config.Config
	width int
}

// NewCostHistoryViewCmd creates `cost history view`.
func NewCostHistoryViewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view",
		Short: "Chart stored projected costs for a Pulumi stack",
		Example: `  finfocus cost history view --stack dev --plain
  finfocus cost history view --stack dev --split-providers
  finfocus cost history view --stack dev --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runView(cmd, viewDeps{})
		},
	}
	cmd.Flags().String("from", "", "Chart start date (YYYY-MM-DD)")
	cmd.Flags().String("to", "", "Chart end date (YYYY-MM-DD, inclusive)")
	cmd.Flags().String("provider", "", "Chart one provider (aws, gcp, azure)")
	cmd.Flags().Bool("split-providers", false, "Show one series per provider")
	cmd.Flags().Bool("no-budget", false, "Hide the budget threshold line")
	cmd.Flags().Bool("no-annotations", false, "Hide deployment annotations")
	cmd.Flags().Int("height", defaultChartHeight, "Chart height in rows")
	cmd.Flags().Int("width", 0, "Chart width in columns (0 = terminal width)")
	cmd.Flags().String("output", historyOutputPlain, "Output format: plain, json")
	cmd.Flags().Bool("plain", false, "Force plain chart output")
	return cmd
}

func runView(cmd *cobra.Command, deps viewDeps) error {
	format, err := historyOutputFormat(cmd)
	if err != nil {
		return err
	}
	stack, err := requiredStack(cmd)
	if err != nil {
		return err
	}
	path, err := historyPath(deps.dir, stack)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return fmt.Errorf("no cost history for stack '%s'", stack)
		}
		return fmt.Errorf("reading cost history: %w", statErr)
	}
	db, err := history.OpenCostDBRead(path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	from, to, err := viewBounds(cmd)
	if err != nil {
		return err
	}
	snapshots, err := db.Snapshots(from, to)
	if err != nil {
		return err
	}
	annotations, err := db.Annotations(from, to)
	if err != nil {
		return err
	}
	if format == historyOutputJSON {
		return writeJSON(cmd, costHistoryView{Stack: stack, Snapshots: snapshots, Annotations: annotations})
	}
	opt, err := viewChartOptions(cmd, deps, stack, from, to)
	if err != nil {
		return err
	}
	cmd.Print(history.RenderChart(snapshots, annotations, opt))
	return nil
}

type costHistoryView struct {
	Stack       string                   `json:"stack"`
	Snapshots   []history.CostSnapshot   `json:"snapshots"`
	Annotations []history.CostAnnotation `json:"annotations"`
}

func requiredStack(cmd *cobra.Command) (string, error) {
	stack, err := cmd.Flags().GetString("stack")
	if err != nil {
		return "", err
	}
	stack = strings.TrimSpace(stack)
	if stack == "" {
		return "", ErrStackRequired
	}
	return stack, nil
}

func historyOutputFormat(cmd *cobra.Command) (string, error) {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		return "", err
	}
	plain, err := cmd.Flags().GetBool("plain")
	if err != nil {
		return "", err
	}
	format := resolveOutputFormat(cmd, "output", output)
	if plain {
		format = historyOutputPlain
	}
	if format != historyOutputPlain && format != historyOutputJSON {
		return "", fmt.Errorf("unsupported output format: %s (supported: plain, json)", format)
	}
	return format, nil
}

func viewBounds(cmd *cobra.Command) (time.Time, time.Time, error) {
	fromRaw, err := cmd.Flags().GetString("from")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	toRaw, err := cmd.Flags().GetString("to")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	from, err := parseDateFlag(fromRaw, false)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := parseDateFlag(toRaw, true)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !from.IsZero() && !to.IsZero() && to.Before(from) {
		return time.Time{}, time.Time{}, errors.New("invalid date range: --to is before --from")
	}
	return from, to, nil
}

func viewChartOptions(
	cmd *cobra.Command,
	deps viewDeps,
	stack string,
	from, to time.Time,
) (history.ChartOptions, error) {
	provider, err := cmd.Flags().GetString("provider")
	if err != nil {
		return history.ChartOptions{}, err
	}
	split, err := cmd.Flags().GetBool("split-providers")
	if err != nil {
		return history.ChartOptions{}, err
	}
	noBudget, err := cmd.Flags().GetBool("no-budget")
	if err != nil {
		return history.ChartOptions{}, err
	}
	noAnnotations, err := cmd.Flags().GetBool("no-annotations")
	if err != nil {
		return history.ChartOptions{}, err
	}
	height, err := cmd.Flags().GetInt("height")
	if err != nil {
		return history.ChartOptions{}, err
	}
	width, err := cmd.Flags().GetInt("width")
	if err != nil {
		return history.ChartOptions{}, err
	}
	if width <= 0 {
		width = deps.width
	}
	if width <= 0 {
		width = terminalWidth()
	}
	opt := history.ChartOptions{
		Stack:          stack,
		From:           from,
		To:             to,
		Provider:       provider,
		SplitProviders: split,
		NoBudget:       noBudget,
		NoAnnotations:  noAnnotations,
		Height:         height,
		Width:          width,
	}
	if !noBudget {
		if amount, ok := globalBudget(deps.cfg); ok {
			opt.Budget = amount
		}
	}
	return opt, nil
}
