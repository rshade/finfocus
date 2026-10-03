package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/detect"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/spec"
)

type collectDeps struct {
	look      func(string) (string, error)
	run       func(context.Context, string, ...string) ([]byte, error)
	price     history.Pricer
	openPrice func(context.Context) (history.Pricer, func(), error)
	dir       string
	confirm   func(string) bool
	cfg       *config.Config
}

type collectFlags struct {
	stack       string
	from        time.Time
	versions    int
	parallel    int
	skipDestroy bool
}

// NewCostHistoryCollectCmd creates `cost history collect`.
func NewCostHistoryCollectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Store projected costs for Pulumi stack checkpoints",
		Example: `  finfocus cost history collect --stack dev
  finfocus cost history collect --stack dev --versions 20
  finfocus cost history collect --stack dev --from 2025-01-01 --skip-destroy`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCollect(cmd, collectDeps{cfg: config.GetGlobalConfig()})
		},
	}
	cmd.Flags().String("from", "", "Only collect versions on or after this date (YYYY-MM-DD)")
	cmd.Flags().Int("versions", 0, "Limit to the last N versions (0 = all)")
	cmd.Flags().Int("parallel", defaultParallelExports, "Concurrent pulumi stack exports")
	cmd.Flags().Bool("skip-destroy", false, "Skip destroy versions instead of storing them as $0")
	return cmd
}

const defaultParallelExports = 4

func runCollect(cmd *cobra.Command, deps collectDeps) error {
	flags, err := readCollectFlags(cmd)
	if err != nil {
		return err
	}
	ctx := commandContext(cmd)
	bin, err := detectStack(ctx, cmd, flags.stack, deps)
	if err != nil {
		return err
	}
	return collectInto(ctx, cmd, bin, flags, deps)
}

func readCollectFlags(cmd *cobra.Command) (collectFlags, error) {
	var flags collectFlags
	var err error
	flags.stack, err = cmd.Flags().GetString("stack")
	if err != nil {
		return flags, err
	}
	flags.stack = strings.TrimSpace(flags.stack)
	if flags.stack == "" {
		return flags, ErrStackRequired
	}
	from, err := cmd.Flags().GetString("from")
	if err != nil {
		return flags, err
	}
	flags.from, err = parseDateFlag(from, false)
	if err != nil {
		return flags, err
	}
	flags.versions, err = cmd.Flags().GetInt("versions")
	if err != nil {
		return flags, err
	}
	if flags.versions < 0 {
		return flags, errors.New("versions must be >= 0")
	}
	flags.parallel, err = cmd.Flags().GetInt("parallel")
	if err != nil {
		return flags, err
	}
	if flags.parallel < 1 {
		return flags, errors.New("parallel must be >= 1")
	}
	flags.skipDestroy, err = cmd.Flags().GetBool("skip-destroy")
	if err != nil {
		return flags, err
	}
	return flags, nil
}

var ErrStackRequired = errors.New("--stack flag required")

func detectStack(ctx context.Context, cmd *cobra.Command, stack string, deps collectDeps) (string, error) {
	run := deps.run
	if run == nil {
		run = execCommand
	}
	bin, version, err := detect.Detect(ctx, deps.look, run)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Detecting Pulumi CLI... v%s ✓\n", version)
	raw, err := run(ctx, bin, "stack", "ls", "--json")
	if err != nil {
		return "", fmt.Errorf("listing pulumi stacks: %w", err)
	}
	names, err := parseStackList(raw)
	if err != nil {
		return "", err
	}
	if !stackKnown(stack, names) {
		return "", fmt.Errorf("stack '%s' not found. Available stacks: %s", stack, joinStacks(names))
	}
	return bin, nil
}

func collectInto(ctx context.Context, cmd *cobra.Command, bin string, flags collectFlags, deps collectDeps) error {
	target, err := resolveHistoryTarget(deps.dir, flags.stack)
	if err != nil {
		return err
	}
	path := target.Path
	db, err := history.OpenCostDBFor(path, target.Project, target.Stack)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	run := deps.run
	if run == nil {
		run = execCommand
	}
	var cleanup func()
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()
	started := time.Now()
	result, err := history.Collect(
		ctx,
		db,
		pulumiExporter{bin: bin, stack: flags.stack, run: run},
		history.CollectOptions{
			From:        flags.from,
			Versions:    flags.versions,
			Parallel:    flags.parallel,
			SkipDestroy: flags.skipDestroy,
			Confirm:     collectConfirm(cmd, deps),
			Pricer:      deps.price,
			OpenPricer:  collectOpener(cmd, deps, &cleanup),
			OnPlan:      func(plan history.CollectPlan) { writeCollectPlan(cmd.OutOrStdout(), path, plan) },
			Progress:    func(p history.CollectProgress) { writeProgress(cmd.OutOrStdout(), p, started) },
		},
	)
	if err != nil {
		return err
	}
	for _, skipped := range result.Skipped {
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: skipped version %d: %v\n", skipped.Version, skipped.Err)
	}
	return autoPruneCostHistory(cmd, db, path, deps.cfg)
}

func collectConfirm(cmd *cobra.Command, deps collectDeps) func(string) bool {
	if deps.confirm != nil {
		return deps.confirm
	}
	return func(warning string) bool {
		if flagYes(cmd) {
			return true
		}
		return confirmWithReader(cmd, warning+" ")
	}
}

func collectOpener(
	cmd *cobra.Command,
	deps collectDeps,
	cleanup *func(),
) func(context.Context) (history.Pricer, error) {
	return func(ctx context.Context) (history.Pricer, error) {
		if deps.openPrice != nil {
			price, closeFn, err := deps.openPrice(ctx)
			*cleanup = closeFn
			return price, err
		}
		price, closeFn, label, err := openHistoryPricer(cmd)
		if err != nil {
			return nil, err
		}
		*cleanup = closeFn
		fmt.Fprintf(cmd.OutOrStdout(), "Starting plugins... %s\n", label)
		return price, nil
	}
}

type pulumiExporter struct {
	bin   string
	stack string
	run   func(context.Context, string, ...string) ([]byte, error)
}

func (p pulumiExporter) History(ctx context.Context) ([]byte, error) {
	return p.run(ctx, p.bin, "stack", "history", "--json", "--stack", p.stack)
}

func (p pulumiExporter) Export(ctx context.Context, version int) ([]byte, error) {
	return p.run(ctx, p.bin, "stack", "export", "--stack", p.stack, "--version", strconv.Itoa(version))
}

func openHistoryPricer(cmd *cobra.Command) (history.Pricer, func(), string, error) {
	ctx := commandContext(cmd)
	clients, cleanup, err := openPlugins(ctx, "", nil)
	if err != nil {
		return nil, nil, "", err
	}
	cfg := config.GetGlobalConfig()
	if cfg == nil {
		cfg = config.New()
	}
	eng, _, cacheCleanup := newEngineWithCache(ctx, cmd, clients, spec.NewLoader(cfg.SpecDir), cfg)
	price := func(ctx context.Context, resources []history.PriceResource) ([]history.PriceQuote, error) {
		described := make([]engine.ResourceDescriptor, len(resources))
		for i, resource := range resources {
			described[i] = engine.ResourceDescriptor{
				Type:       resource.Type,
				ID:         resource.ID,
				Provider:   resource.Provider,
				Properties: resource.Properties,
			}
		}
		priced, priceErr := eng.GetProjectedCostWithErrors(ctx, described)
		if priceErr != nil {
			return nil, priceErr
		}
		if priced == nil {
			return nil, nil
		}
		quotes := make([]history.PriceQuote, 0, len(priced.Results))
		for _, result := range priced.Results {
			quotes = append(quotes, history.PriceQuote{
				ResourceID:   result.ResourceID,
				ResourceType: result.ResourceType,
				Adapter:      result.Adapter,
				Currency:     result.Currency,
				Monthly:      result.Monthly,
				Notes:        result.Notes,
				Failed:       result.Error != nil,
			})
		}
		return quotes, nil
	}
	closer := func() {
		cacheCleanup()
		cleanup()
	}
	return price, closer, formatPluginList(clients), nil
}

func formatPluginList(clients []*pluginhost.Client) string {
	parts := make([]string, 0, len(clients))
	for _, client := range clients {
		if client == nil || client.Name == "" {
			continue
		}
		label := client.Name
		if client.Metadata != nil && client.Metadata.Version != "" {
			label = fmt.Sprintf("%s (%s)", client.Name, client.Metadata.Version)
		}
		parts = append(parts, label+" ✓")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func writeCollectPlan(w io.Writer, path string, plan history.CollectPlan) {
	fmt.Fprintf(w, "Fetching stack history... %d versions found\n", plan.TotalHistory)
	label := displayPath(path)
	if plan.Fresh {
		fmt.Fprintf(w, "Database: %s (new)\n", label)
	} else {
		fmt.Fprintf(w, "Database: %s\n", label)
	}
	if plan.LastVersion > 0 {
		collected := "-"
		if !plan.CollectedAt.IsZero() {
			collected = plan.CollectedAt.UTC().Format("2006-01-02")
		}
		fmt.Fprintf(w, "Last collected: v%d (%s)\n", plan.LastVersion, collected)
	}
	if plan.Reset {
		fmt.Fprintln(w, "History reset.")
	}
	if plan.New == 0 {
		fmt.Fprintln(w, "No new versions to collect.")
		return
	}
	fmt.Fprintf(w, "New versions: %d (v%d–v%d)\n", plan.New, plan.FirstNew, plan.LastNew)
}

func writeProgress(w io.Writer, p history.CollectProgress, started time.Time) {
	filled := 0
	if p.Total > 0 {
		filled = min(p.Done*historyProgressWidth/p.Total, historyProgressWidth)
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", historyProgressWidth-filled)
	when := "-"
	if !p.When.IsZero() {
		when = p.When.UTC().Format("2006-01-02")
	}
	fmt.Fprintf(w, "%s  %d/%d  [v%d — %s]\n", bar, p.Done, p.Total, p.Version, when)
	if p.Done > 0 && p.Done < p.Total {
		remain := time.Since(started) / time.Duration(p.Done) * time.Duration(p.Total-p.Done)
		fmt.Fprintf(w, "Estimated time remaining: ~%s\n", formatRemain(remain))
	}
}
