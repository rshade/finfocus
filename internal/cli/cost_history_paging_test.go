package cli

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
)

func historyRows(count int) []map[string]any {
	rows := make([]map[string]any, 0, count)
	for version := count; version >= 1; version-- {
		rows = append(rows, histRow(version, "2025-01-01T00:00:00Z", "v"+strconv.Itoa(version)))
	}
	return rows
}

// pagedHistoryRun answers `pulumi stack history` the way a paged CLI does:
// newest first, pageSize rows per page, and an empty page past the end.
func pagedHistoryRun(
	t *testing.T,
	total int,
	calls *[][]string,
) func(context.Context, string, ...string) ([]byte, error) {
	t.Helper()
	rows := historyRows(total)
	return func(_ context.Context, _ string, args ...string) ([]byte, error) {
		*calls = append(*calls, slices.Clone(args))
		size, page := flagInt(args, "--page-size"), flagInt(args, "--page")
		if size == 0 {
			size, page = 10, 1
		}
		start := min((page-1)*size, len(rows))
		end := min(start+size, len(rows))
		return mustJSON(t, rows[start:end]), nil
	}
}

func flagInt(args []string, name string) int {
	for i, arg := range args {
		if arg == name && i+1 < len(args) {
			value, _ := strconv.Atoi(args[i+1])
			return value
		}
	}
	return 0
}

func TestPulumiExporter_HistoryReadsEveryPage(t *testing.T) {
	t.Parallel()

	var calls [][]string
	exporter := pulumiExporter{bin: "pulumi", stack: "dev", run: pagedHistoryRun(t, 250, &calls)}

	body, err := exporter.History(context.Background())
	require.NoError(t, err)

	updates, err := history.ParseStackHistory(body)
	require.NoError(t, err)
	assert.Len(t, updates, 250)
	versions := make(map[int]bool, len(updates))
	for _, update := range updates {
		versions[update.Version] = true
	}
	assert.Len(t, versions, 250, "no version is repeated")
	assert.Len(t, calls, 3, "two full pages and a short one")
}

func TestPulumiExporter_HistoryStopsWhenPagingIsIgnored(t *testing.T) {
	t.Parallel()

	rows := historyRows(10)
	calls := 0
	exporter := pulumiExporter{
		bin: "pulumi", stack: "dev",
		run: func(context.Context, string, ...string) ([]byte, error) {
			calls++
			return mustJSON(t, rows), nil
		},
	}

	body, err := exporter.History(context.Background())
	require.NoError(t, err)
	updates, err := history.ParseStackHistory(body)
	require.NoError(t, err)
	assert.Len(t, updates, 10)
	assert.LessOrEqual(t, calls, 2)
}

func TestPulumiExporter_HistoryFallsBackWhenPagingFlagsAreRejected(t *testing.T) {
	t.Parallel()

	rows := historyRows(5)
	var calls [][]string
	exporter := pulumiExporter{
		bin: "pulumi", stack: "dev",
		run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, slices.Clone(args))
			if slices.Contains(args, "--page-size") {
				return nil, errors.New("unknown flag: --page-size")
			}
			return mustJSON(t, rows), nil
		},
	}

	body, err := exporter.History(context.Background())
	require.NoError(t, err)
	updates, err := history.ParseStackHistory(body)
	require.NoError(t, err)
	assert.Len(t, updates, 5)
	require.Len(t, calls, 2)
	assert.False(t, slices.Contains(calls[1], "--page-size"))
}

func TestPulumiExporter_HistoryReportsARealFailure(t *testing.T) {
	t.Parallel()

	exporter := pulumiExporter{
		bin: "pulumi", stack: "dev",
		run: func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("pulumi: no stack named 'dev'")
		},
	}

	_, err := exporter.History(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no stack named")
}
