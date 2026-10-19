package cli

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/tui/detail"
)

// loadDetailHistory reads one resource's stored cost history.
// An empty stack, the overview placeholder "unknown", a missing database, or
// an empty resource ID returns detail.ErrHistorySkipped so the detail view
// omits the section. A database that exists and does not mention the resource
// returns an empty series.
func loadDetailHistory(ctx context.Context, dir, stack, resourceID string) ([]detail.Point, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stack = strings.TrimSpace(stack)
	resourceID = strings.TrimSpace(resourceID)
	if stack == "" || resourceID == "" {
		return nil, detail.ErrHistorySkipped
	}
	path, err := historyPath(dir, stack)
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, detail.ErrHistorySkipped
		}
		return nil, statErr
	}
	db, err := openHistoryRead(dir, stack)
	if err != nil {
		return nil, err
	}
	defer func() { _ = db.Close() }()
	snapshots, err := db.Snapshots(time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	return detailPoints(history.ResourcePoints(snapshots, resourceID)), nil
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
