package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/tui/detail"
)

// loadDetailHistory reads one resource's stored cost history.
// An empty identity, or a missing database, returns detail.ErrHistorySkipped
// so the detail view omits the section. A database that exists and does not
// mention the resource returns an empty series.
//
// A Pulumi URN selects the project and stack written by cost history collect.
// The fallback stack is the short name stored in the database, including when
// the caller passed org/project/stack. A display label such as a state-file
// name is not used once the URN names a stack.
func loadDetailHistory(ctx context.Context, dir, stack, resourceID string) ([]detail.Point, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stack = strings.TrimSpace(stack)
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return nil, detail.ErrHistorySkipped
	}
	target, err := detailHistoryTarget(ctx, dir, stack, resourceID)
	if err != nil {
		return nil, err
	}
	if target.Path == "" {
		return nil, detail.ErrHistorySkipped
	}
	if _, statErr := os.Stat(target.Path); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, detail.ErrHistorySkipped
		}
		return nil, statErr
	}
	db, err := history.OpenCostDBRead(target.Path)
	if err != nil {
		return nil, err
	}
	if err = diffStackMatches(db, target.Stack); err != nil {
		_ = db.Close()
		return nil, err
	}
	defer func() { _ = db.Close() }()
	snapshots, err := db.Snapshots(time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	return detailPoints(history.ResourcePoints(snapshots, resourceID)), nil
}

// detailHistoryTarget picks the cost-history database for one resource.
// URN identity wins over fallbackStack. fallbackStack is resolved the same way
// collect resolves --stack, and the returned Stack is the short name stored
// in the database.
func detailHistoryTarget(ctx context.Context, dir, fallbackStack, resourceID string) (historyTarget, error) {
	project := history.ProjectFromURN(resourceID)
	urnStack := history.StackFromURN(resourceID)
	if project != "" && urnStack != "" {
		name, err := history.CostFileNameFor(project, urnStack)
		if err != nil {
			return historyTarget{}, err
		}
		return historyTarget{
			Path:    filepath.Join(historyDir(dir), name),
			Project: project,
			Stack:   urnStack,
		}, nil
	}
	if fallbackStack == "" {
		return historyTarget{}, nil
	}
	return resolveHistoryTargetContext(ctx, dir, fallbackStack)
}

func detailPoints(points []history.ResourcePoint) []detail.Point {
	if len(points) == 0 {
		return nil
	}
	out := make([]detail.Point, len(points))
	for i, point := range points {
		out[i] = detail.Point{
			Timestamp: point.Timestamp,
			Amount:    point.Monthly,
			Currency:  point.Currency,
		}
	}
	return out
}

func costActualHistoryLoader(stack string) detail.Loader {
	return func(ctx context.Context, resourceID string) ([]detail.Point, error) {
		return loadDetailHistory(ctx, "", stack, resourceID)
	}
}

func overviewHistoryLoader() func(context.Context, string, string) ([]detail.Point, error) {
	return func(ctx context.Context, stack, resourceID string) ([]detail.Point, error) {
		return loadDetailHistory(ctx, "", stack, resourceID)
	}
}
