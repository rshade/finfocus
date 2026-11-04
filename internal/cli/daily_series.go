package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

// dailySeriesTarget resolves the daily actuals file for stack the way
// `cost history collect-daily` names it: an explicit Pulumi input names the
// stack the caller asked for, a Terraform directory's identity wins next,
// then the fully qualified stack name, then the project the loaded pipeline
// identified, and a bare legacy name is the last resort. ok is false when the
// caller gave no stack to name a file with.
func dailySeriesTarget(
	ctx context.Context,
	dir, stack, loadedProject string,
	explicitPulumi bool,
) (historyTarget, bool, error) {
	if explicitPulumi {
		return explicitDailyTarget(dir, stack, loadedProject)
	}
	if target, ok, err := terraformHistoryTargetContext(ctx, dir, terraformWorkspaceFromConfig()); ok || err != nil {
		if err != nil {
			return historyTarget{}, false, err
		}
		name, nameErr := history.DailyFileNameFor(target.Project, target.Stack)
		if nameErr != nil {
			return historyTarget{}, false, nameErr
		}
		target.Path = filepath.Join(historyDir(dir), name)
		target.Label = ""
		return target, true, nil
	}
	stack = strings.TrimSpace(stack)
	if stack == "" {
		return historyTarget{}, false, nil
	}
	project, short := history.SplitStack(stack)
	if project == "" {
		project = strings.TrimSpace(loadedProject)
	}
	var (
		name string
		err  error
	)
	if project == "" {
		name, err = history.DailyFileName(stack)
	} else {
		name, err = history.DailyFileNameFor(project, short)
	}
	if err != nil {
		return historyTarget{}, false, err
	}
	return historyTarget{
		Path:    filepath.Join(historyDir(dir), name),
		Project: project,
		Stack:   short,
	}, true, nil
}

// explicitDailyTarget names the daily file of the stack an explicit Pulumi
// input asked for, the way `cost history collect-daily` stores one: the
// project comes from the stack value or the loaded pipeline's rows, and the
// file lives in the history directory.
func explicitDailyTarget(dir, stack, loadedProject string) (historyTarget, bool, error) {
	stack = strings.TrimSpace(stack)
	if stack == "" {
		return historyTarget{}, false, nil
	}
	project, short := history.SplitStack(stack)
	if project == "" {
		project = strings.TrimSpace(loadedProject)
	}
	name, err := history.DailyFileNameFor(project, short)
	if err != nil {
		return historyTarget{}, false, err
	}
	return historyTarget{
		Path:    filepath.Join(historyDir(dir), name),
		Project: project,
		Stack:   short,
	}, true, nil
}

// readDailySeries returns every stored point. A missing file is no series,
// not an error: the stack has never been collected.
func readDailySeries(target historyTarget) ([]history.DailyPoint, error) {
	if _, err := os.Stat(target.Path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("statting daily actuals: %w", err)
	}
	db, err := history.OpenDailyDBRead(target.Path, target.Project, target.Stack)
	if err != nil {
		return nil, fmt.Errorf("opening daily actuals: %w", err)
	}
	defer func() { _ = db.Close() }()
	points, err := db.Points()
	if err != nil {
		return nil, fmt.Errorf("reading daily actuals: %w", err)
	}
	return points, nil
}

// trimDailySeries keeps the points inside the requested window and drops the
// current partial day: a day is an observation only once it is complete. A
// zero window bound filters nothing on that side. The points are never
// zero-filled: a missing day stays missing.
func trimDailySeries(points []history.DailyPoint, window engine.DateRange, now time.Time) []history.DailyPoint {
	var (
		startDay time.Time
		endDay   time.Time
	)
	if !window.Start.IsZero() {
		startDay = utcDayFloor(window.Start)
	}
	if !window.End.IsZero() {
		endDay = utcDayFloor(window.End)
	}
	today := utcDayFloor(now)
	kept := make([]history.DailyPoint, 0, len(points))
	for _, point := range points {
		day := point.Day.UTC()
		if !startDay.IsZero() && day.Before(startDay) {
			continue
		}
		if !endDay.IsZero() && day.After(endDay) {
			continue
		}
		if !day.Before(today) {
			// The current partial day is not an observation.
			continue
		}
		kept = append(kept, point)
	}
	return kept
}

// loadDailyAnomalySeries reads the stack's stored daily actuals for the
// window. An unnamed stack, a stack that has never been collected, and an
// empty store all give an empty series, and the drift rows stand alone.
func loadDailyAnomalySeries(
	ctx context.Context,
	dir, stack, loadedProject string,
	window engine.DateRange,
	now time.Time,
	explicitPulumi bool,
) ([]history.DailyPoint, error) {
	target, ok, err := dailySeriesTarget(ctx, dir, stack, loadedProject, explicitPulumi)
	if err != nil || !ok {
		return nil, err
	}
	points, err := readDailySeries(target)
	if err != nil {
		return nil, err
	}
	return trimDailySeries(points, window, now), nil
}

// utcDayFloor returns t's UTC calendar day at midnight.
func utcDayFloor(t time.Time) time.Time {
	day := t.UTC()
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
}
