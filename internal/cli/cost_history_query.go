package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/history"
)

// historyDiffQuery holds the `cost history diff` selectors and threshold.
type historyDiffQuery struct {
	From      string
	To        string
	Threshold float64
}

// queryError marks err as a cost history request the caller got wrong.
func queryError(err error) error {
	if err == nil {
		return nil
	}
	return &history.QueryError{Err: err}
}

// historyViewBounds parses the view's date range. An empty value is open.
func historyViewBounds(fromRaw, toRaw string) (time.Time, time.Time, error) {
	from, err := parseDateFlag(fromRaw, false)
	if err != nil {
		return time.Time{}, time.Time{}, queryError(err)
	}
	to, err := parseDateFlag(toRaw, true)
	if err != nil {
		return time.Time{}, time.Time{}, queryError(err)
	}
	if !from.IsZero() && !to.IsZero() && to.Before(from) {
		return time.Time{}, time.Time{}, queryError(errors.New("invalid date range: --to is before --from"))
	}
	return from, to, nil
}

// selectViewCurrency keeps one currency and the annotations of the kept
// snapshots, and returns the mixed-currency warning.
func selectViewCurrency(
	snapshots []history.CostSnapshot,
	annotations []history.CostAnnotation,
	choice history.CurrencyChoice,
) ([]history.CostSnapshot, []history.CostAnnotation, string, error) {
	selected, err := history.SelectCurrency(snapshots, choice)
	if err != nil {
		return nil, nil, "", queryError(err)
	}
	return selected.Snapshots, annotationsForSnapshots(selected.Snapshots, annotations), selected.Warning, nil
}

// readHistoryView reads the view document for stack between from and to.
func readHistoryView(
	db *history.CostDB,
	stack string,
	from, to time.Time,
	choice history.CurrencyChoice,
) (history.ViewDocument, error) {
	snapshots, err := db.Snapshots(from, to)
	if err != nil {
		return history.ViewDocument{}, err
	}
	annotations, err := db.Annotations(from, to)
	if err != nil {
		return history.ViewDocument{}, err
	}
	snapshots, annotations, warning, err := selectViewCurrency(snapshots, annotations, choice)
	if err != nil {
		return history.ViewDocument{}, err
	}
	return history.ViewDocument{
		Stack:           stack,
		Snapshots:       snapshots,
		Annotations:     annotations,
		CurrencyWarning: warning,
	}, nil
}

// checkDiffInputs rejects a missing start selector and a negative threshold.
func checkDiffInputs(query historyDiffQuery) error {
	if strings.TrimSpace(query.From) == "" {
		return queryError(errors.New("--from is required"))
	}
	if query.Threshold < 0 {
		return queryError(errors.New("--threshold must be >= 0"))
	}
	return nil
}

// readHistoryDiff resolves both selectors against every stored snapshot and
// diffs them. A database with no snapshots returns history.ErrNoSnapshots.
func readHistoryDiff(db *history.CostDB, query historyDiffQuery) (history.CostDiff, error) {
	if err := checkDiffInputs(query); err != nil {
		return history.CostDiff{}, err
	}
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	if err != nil {
		return history.CostDiff{}, err
	}
	fromSnap, err := resolveDiffSnapshot(snaps, query.From)
	if err != nil {
		return history.CostDiff{}, err
	}
	toSnap, err := resolveDiffSnapshot(snaps, query.To)
	if err != nil {
		return history.CostDiff{}, err
	}
	diff, err := history.DiffSnapshots(fromSnap, toSnap, query.Threshold)
	if err != nil {
		return history.CostDiff{}, queryError(err)
	}
	return diff, nil
}

func resolveDiffSnapshot(snaps []history.CostSnapshot, spec string) (history.CostSnapshot, error) {
	snap, err := history.ResolveCostSnapshot(snaps, spec)
	if err != nil && !history.IsNoHistory(err) {
		return history.CostSnapshot{}, queryError(err)
	}
	return snap, err
}

// requireHistoryFile reports a missing database as history.ErrNoCostHistory,
// with the message the history commands have always printed.
func requireHistoryFile(target historyTarget) error {
	if target.Path == "" {
		return fmt.Errorf("%w for stack '%s'", history.ErrNoCostHistory, target.Stack)
	}
	if _, err := os.Stat(target.Path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w for stack '%s'", history.ErrNoCostHistory, target.Stack)
		}
		return fmt.Errorf("reading cost history: %w", err)
	}
	return nil
}

// openHistoryTargetReadOnly opens an existing database without writing to it
// and checks the stack name it was recorded for. Unlike
// openHistoryTargetRead, it never stamps a Terraform project label.
func openHistoryTargetReadOnly(target historyTarget) (*history.CostDB, error) {
	if err := requireHistoryFile(target); err != nil {
		return nil, err
	}
	db, err := history.OpenCostDBRead(target.Path)
	if err != nil {
		return nil, err
	}
	if err = diffStackMatches(db, target.Stack); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
