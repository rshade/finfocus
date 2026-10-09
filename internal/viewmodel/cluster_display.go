package viewmodel

import (
	"github.com/rshade/finfocus/internal/engine"
)

// ClusterPageSize is the number of pagination units shown per page.
const ClusterPageSize = 250

// ClusterDisplayEntry pairs an overview row to render with its index in the
// source slice, whether it is an expansion child (rendered indented), and
// whether it is a pagination unit.
type ClusterDisplayEntry struct {
	Row     engine.OverviewRowResult
	RowsIdx int
	Child   bool
	Unit    bool
}

// PaginationState describes pagination over the cluster pagination units.
type PaginationState struct {
	// Enabled is true when the unit count exceeds ClusterPageSize.
	Enabled bool
	// Page is the clamped 1-based current page.
	Page int
	// TotalPages is the number of pages; meaningful when Enabled.
	TotalPages int
	// TotalUnits is the total number of pagination units.
	TotalUnits int
}

// paginationUnits returns the row indices that pagination counts. While the
// filter is inactive, an expansion child whose parent is in rows is drawn
// under that parent, so it is not a unit of its own and always lands on its
// parent's page.
func paginationUnits(rows []engine.OverviewRowResult, filterActive bool) []int {
	parentPresent := map[string]bool{}
	if !filterActive {
		for i := range rows {
			if rows[i].ParentURN == "" {
				parentPresent[rows[i].URN] = true
			}
		}
	}
	units := make([]int, 0, len(rows))
	for i := range rows {
		if filterActive || rows[i].ParentURN == "" || !parentPresent[rows[i].ParentURN] {
			units = append(units, i)
		}
	}
	return units
}

// ClusterPagination computes the pagination state for the given rows and
// requested 1-based page, clamping the page into valid bounds.
func ClusterPagination(rows []engine.OverviewRowResult, filterActive bool, page int) PaginationState {
	units := len(paginationUnits(rows, filterActive))
	state := PaginationState{Page: 1, TotalPages: 1, TotalUnits: units}
	if units > ClusterPageSize {
		state.Enabled = true
		state.TotalPages = (units + ClusterPageSize - 1) / ClusterPageSize
		state.Page = min(max(page, 1), state.TotalPages)
	}
	return state
}

// pageUnits returns the pagination units on the given page.
func pageUnits(units []int, state PaginationState) []int {
	if !state.Enabled {
		return units
	}
	start := (state.Page - 1) * ClusterPageSize
	if start >= len(units) {
		return nil
	}
	return units[start:min(start+ClusterPageSize, len(units))]
}

// FlattenClusterRows computes the rows to display for the requested page:
// the page's pagination units in order, each followed by its children when
// expanded. Children are hidden while their parent is collapsed. When the
// filter is active the list is rendered flat, preserving the filter's
// substring semantics. It also returns the (clamped) pagination state.
func FlattenClusterRows(
	rows []engine.OverviewRowResult,
	expanded map[string]bool,
	filterActive bool,
	page int,
) ([]ClusterDisplayEntry, PaginationState) {
	state := ClusterPagination(rows, filterActive, page)

	childrenByParent := map[string][]int{}
	if !filterActive {
		for i := range rows {
			if parent := rows[i].ParentURN; parent != "" {
				childrenByParent[parent] = append(childrenByParent[parent], i)
			}
		}
	}

	var entries []ClusterDisplayEntry
	for _, i := range pageUnits(paginationUnits(rows, filterActive), state) {
		r := rows[i]
		entries = append(entries, ClusterDisplayEntry{Row: r, RowsIdx: i, Child: r.ParentURN != "", Unit: true})
		if filterActive || !expanded[r.URN] {
			continue
		}
		for _, c := range childrenByParent[r.URN] {
			entries = append(entries, ClusterDisplayEntry{Row: rows[c], RowsIdx: c, Child: true})
		}
	}
	return entries, state
}
