package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/tui/detail"
	"github.com/rshade/finfocus/internal/webui"
)

// historyView answers the dashboard's `cost history view` for the session
// stack through readHistoryView, the function runView uses, plus the global
// budget the CLI chart draws. Like runView, it opens the database before it
// parses the dates, so a stack with no history answers that first.
func (d *webData) historyView(ctx context.Context, query webui.HistoryViewQuery) (webui.HistoryViewResult, error) {
	db, stack, err := d.openSessionHistory(ctx)
	if err != nil {
		return webui.HistoryViewResult{}, err
	}
	defer func() { _ = db.Close() }()
	from, to, err := historyViewBounds(query.From, query.To)
	if err != nil {
		return webui.HistoryViewResult{}, err
	}
	doc, err := readHistoryView(db, stack, from, to, history.CurrencyChoice{
		Currency: query.Currency,
		Strict:   query.Strict,
	})
	if err != nil {
		return webui.HistoryViewResult{}, err
	}
	result := webui.HistoryViewResult{Document: doc}
	if cfg := d.historyConfig(); cfg != nil {
		if amount, ok := globalBudget(cfg); ok {
			result.Budget = &amount
			result.BudgetCurrency = cfg.Cost.Budgets.Global.Currency
		}
	}
	return result, nil
}

// historyDiff answers the dashboard's `cost history diff` for the session
// stack through readHistoryDiff, the function runDiff uses.
func (d *webData) historyDiff(ctx context.Context, query webui.HistoryDiffQuery) (history.DiffDocument, error) {
	diffQuery := historyDiffQuery{From: query.From, To: query.To, Threshold: query.Threshold}
	if err := checkDiffInputs(diffQuery); err != nil {
		return history.DiffDocument{}, err
	}
	db, stack, err := d.openSessionHistory(ctx)
	if err != nil {
		return history.DiffDocument{}, err
	}
	defer func() { _ = db.Close() }()
	diff, err := readHistoryDiff(db, diffQuery)
	if err != nil {
		return history.DiffDocument{}, err
	}
	return history.NewDiffDocument(stack, diff), nil
}

// historyResource loads one resource's series through loadDetailHistory, the
// function behind the overview and cost TUI detail views.
func (d *webData) historyResource(ctx context.Context, id string) ([]webui.HistoryPoint, error) {
	stack, err := d.historyStack(ctx)
	if err != nil {
		return nil, err
	}
	points, err := loadDetailHistory(ctx, d.historyDirOverride, stack, id)
	if errors.Is(err, detail.ErrHistorySkipped) {
		return nil, fmt.Errorf("%w: %w", history.ErrNoCostHistory, err)
	}
	if err != nil {
		return nil, err
	}
	out := make([]webui.HistoryPoint, len(points))
	for i, point := range points {
		out[i] = webui.HistoryPoint{Timestamp: point.Timestamp, Amount: point.Amount, Currency: point.Currency}
	}
	return out, nil
}

// historyStack reads the session stack under the query gate and releases the
// gate before any history file is opened.
func (d *webData) historyStack(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	unlock, err := d.lock(ctx)
	if err != nil {
		return "", err
	}
	defer unlock()
	return d.forecastStack(), nil
}

// openSessionHistory resolves the session stack's database the way
// resolveHistoryCommandTarget does, with the merged config's workspace
// setting in place of the flag, and opens it read-only.
func (d *webData) openSessionHistory(ctx context.Context) (*history.CostDB, string, error) {
	stack, err := d.historyStack(ctx)
	if err != nil {
		return nil, "", err
	}
	cfg := d.historyConfig()
	includeWorkspace := cfg != nil && cfg.Cost.Terraform.IncludeWorkspace
	target, ok, err := terraformHistoryTargetContext(ctx, d.historyDirOverride, includeWorkspace)
	if err != nil {
		return nil, "", err
	}
	if !ok {
		if strings.TrimSpace(stack) == "" {
			return nil, "", fmt.Errorf("%w: the session has no stack", history.ErrNoCostHistory)
		}
		if target, err = resolveHistoryTargetFor(d.historyDirOverride, stack, currentProjectName()); err != nil {
			return nil, "", err
		}
	}
	db, err := openHistoryTargetReadOnly(target)
	if err != nil {
		return nil, "", err
	}
	return db, target.Stack, nil
}

// withMergedConfig records the project-merged config the root command built.
func (d *webData) withMergedConfig(cfg *config.Config) *webData {
	d.mergedCfg = cfg
	return d
}

// historyConfig is the config the CLI history commands read: the merged project
// and global config when the root command initialized it, else the session's.
func (d *webData) historyConfig() *config.Config {
	if d.mergedCfg != nil {
		return d.mergedCfg
	}
	return d.params.cfg
}
