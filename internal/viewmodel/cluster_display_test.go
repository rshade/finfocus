package viewmodel

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func clusterRows() []engine.OverviewRowResult {
	return []engine.OverviewRowResult{
		{URN: "urn:cluster-a", Type: "kw:Cluster", ChildURNs: []string{"urn:child-1", "urn:child-2"}},
		{URN: "urn:child-1", ParentURN: "urn:cluster-a"},
		{URN: "urn:standalone", Type: "aws:s3:Bucket"},
		{URN: "urn:child-2", ParentURN: "urn:cluster-a"},
		{URN: "urn:orphan-child", ParentURN: "urn:not-present"},
	}
}

func entryURNs(entries []ClusterDisplayEntry) []string {
	urns := make([]string, len(entries))
	for i, e := range entries {
		urns[i] = e.Row.URN
	}
	return urns
}

func TestFlattenClusterRowsCollapsed(t *testing.T) {
	t.Parallel()

	entries, state := FlattenClusterRows(clusterRows(), nil, false, 1)

	// Children with a present parent are not pagination units; an orphan child is.
	assert.Equal(t,
		[]string{"urn:cluster-a", "urn:standalone", "urn:orphan-child"},
		entryURNs(entries))
	assert.False(t, state.Enabled)
	assert.Equal(t, 1, state.Page)
	assert.Equal(t, 3, state.TotalUnits)

	// The orphan child is its own unit but still flagged as a child row.
	for _, e := range entries {
		assert.Equal(t, e.Row.ParentURN != "", e.Child)
	}
}

func TestFlattenClusterRowsExpanded(t *testing.T) {
	t.Parallel()

	expanded := map[string]bool{"urn:cluster-a": true}
	entries, state := FlattenClusterRows(clusterRows(), expanded, false, 1)

	// Parent followed by its children in row order; children do not consume
	// pagination units.
	assert.Equal(t,
		[]string{"urn:cluster-a", "urn:child-1", "urn:child-2", "urn:standalone", "urn:orphan-child"},
		entryURNs(entries))
	assert.Equal(t, 3, state.TotalUnits)

	childByURN := map[string]bool{}
	units := 0
	for _, e := range entries {
		childByURN[e.Row.URN] = e.Child
		if e.Unit {
			units++
		}
	}
	assert.Equal(t, state.TotalUnits, units)
	assert.False(t, childByURN["urn:cluster-a"])
	assert.True(t, childByURN["urn:child-1"])
	assert.True(t, childByURN["urn:child-2"])
	assert.False(t, childByURN["urn:standalone"])
}

func TestFlattenClusterRowsFilterActive(t *testing.T) {
	t.Parallel()

	expanded := map[string]bool{"urn:cluster-a": true}
	entries, state := FlattenClusterRows(clusterRows(), expanded, true, 1)

	// Filter active: flat list in row order, every row is a unit, no children
	// appended after an expanded parent.
	assert.Equal(t,
		[]string{"urn:cluster-a", "urn:child-1", "urn:standalone", "urn:child-2", "urn:orphan-child"},
		entryURNs(entries))
	assert.Equal(t, 5, state.TotalUnits)
	assert.False(t, state.Enabled)
}

func TestFlattenClusterRowsPagination(t *testing.T) {
	t.Parallel()

	rows := make([]engine.OverviewRowResult, 0, ClusterPageSize+50)
	for i := range ClusterPageSize + 50 {
		rows = append(rows, engine.OverviewRowResult{URN: fmt.Sprintf("urn:%03d", i)})
	}

	t.Run("page one holds a full page", func(t *testing.T) {
		t.Parallel()

		entries, state := FlattenClusterRows(rows, nil, false, 1)
		require.Len(t, entries, ClusterPageSize)
		assert.True(t, state.Enabled)
		assert.Equal(t, 2, state.TotalPages)
		assert.Equal(t, 1, state.Page)
		assert.Equal(t, "urn:000", entries[0].Row.URN)
		assert.Equal(t, ClusterPageSize+50, state.TotalUnits)
	})

	t.Run("page two holds the remainder", func(t *testing.T) {
		t.Parallel()

		entries, state := FlattenClusterRows(rows, nil, false, 2)
		require.Len(t, entries, 50)
		assert.Equal(t, "urn:250", entries[0].Row.URN)
		assert.Equal(t, 2, state.Page)
	})

	t.Run("page beyond range clamps to last page", func(t *testing.T) {
		t.Parallel()

		entries, state := FlattenClusterRows(rows, nil, false, 99)
		require.Len(t, entries, 50)
		assert.Equal(t, 2, state.Page)
	})

	t.Run("page below one clamps to first page", func(t *testing.T) {
		t.Parallel()

		entries, state := FlattenClusterRows(rows, nil, false, 0)
		require.Len(t, entries, ClusterPageSize)
		assert.Equal(t, 1, state.Page)
	})

	t.Run("exactly one page of rows disables pagination", func(t *testing.T) {
		t.Parallel()

		exact := rows[:ClusterPageSize]
		entries, state := FlattenClusterRows(exact, nil, false, 1)
		require.Len(t, entries, ClusterPageSize)
		assert.False(t, state.Enabled)
	})
}

func TestFlattenClusterRowsExpandedChildrenSpanPages(t *testing.T) {
	t.Parallel()

	// A parent with children is one unit; its expanded children are drawn on
	// the parent's page without counting toward the page size.
	rows := make([]engine.OverviewRowResult, 0, ClusterPageSize+1)
	rows = append(rows, engine.OverviewRowResult{URN: "urn:cluster", ChildURNs: []string{"urn:c1", "urn:c2"}})
	rows = append(rows,
		engine.OverviewRowResult{URN: "urn:c1", ParentURN: "urn:cluster"},
		engine.OverviewRowResult{URN: "urn:c2", ParentURN: "urn:cluster"},
	)
	for i := range ClusterPageSize - 1 {
		rows = append(rows, engine.OverviewRowResult{URN: fmt.Sprintf("urn:flat:%03d", i)})
	}

	expanded := map[string]bool{"urn:cluster": true}
	entries, state := FlattenClusterRows(rows, expanded, false, 1)
	assert.False(t, state.Enabled)
	require.Len(t, entries, ClusterPageSize+2)
	assert.Equal(t, "urn:cluster", entries[0].Row.URN)
	assert.Equal(t, "urn:c1", entries[1].Row.URN)
	assert.Equal(t, "urn:c2", entries[2].Row.URN)
}

func TestClusterPagination(t *testing.T) {
	t.Parallel()

	t.Run("clamps and enables like the TUI", func(t *testing.T) {
		t.Parallel()

		rows := make([]engine.OverviewRowResult, ClusterPageSize+1)
		for i := range rows {
			rows[i].URN = fmt.Sprintf("urn:%d", i)
		}
		state := ClusterPagination(rows, false, 7)
		assert.True(t, state.Enabled)
		assert.Equal(t, 2, state.TotalPages)
		assert.Equal(t, 2, state.Page)
	})

	t.Run("disabled below threshold resets to page one", func(t *testing.T) {
		t.Parallel()

		state := ClusterPagination(clusterRows(), false, 3)
		assert.False(t, state.Enabled)
		assert.Equal(t, 1, state.Page)
	})

	t.Run("filter active counts every row", func(t *testing.T) {
		t.Parallel()

		state := ClusterPagination(clusterRows(), true, 1)
		assert.Equal(t, 5, state.TotalUnits)
	})
}
