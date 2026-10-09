package viewmodel

import (
	"sort"

	"github.com/rshade/finfocus/internal/engine"
)

// SortField represents the field to sort a resource table by.
type SortField int

const (
	// SortByCost sorts by monthly/total cost.
	SortByCost SortField = iota
	// SortByName sorts by resource ID.
	SortByName
	// SortByType sorts by resource type.
	SortByType
	// SortByDelta sorts by cost delta.
	SortByDelta
)

// RecommendationSortField represents the field to sort recommendations by.
type RecommendationSortField int

const (
	// SortBySavings sorts by estimated savings (descending).
	SortBySavings RecommendationSortField = iota
	// SortByResourceID sorts by resource ID (ascending).
	SortByResourceID
	// SortByActionType sorts by action type (ascending).
	SortByActionType
)

// OverviewRowSortCost returns the primary cost for sorting an overview row.
func OverviewRowSortCost(row engine.OverviewRowResult) float64 {
	if row.Projected != nil {
		return *row.Projected
	}
	if row.ActualMTD != nil {
		return *row.ActualMTD
	}
	return 0.0
}

// OverviewRowSortDelta returns the pre-computed delta for sorting, keeping
// display and sort order consistent.
func OverviewRowSortDelta(row engine.OverviewRowResult) float64 {
	if row.Delta == nil {
		return 0.0
	}
	return *row.Delta
}

// SortOverviewRows sorts overview rows in place by the given field.
func SortOverviewRows(rows []engine.OverviewRowResult, field SortField) {
	switch field {
	case SortByCost:
		sort.Slice(rows, func(i, j int) bool {
			return OverviewRowSortCost(rows[i]) > OverviewRowSortCost(rows[j])
		})
	case SortByName:
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].URN < rows[j].URN
		})
	case SortByType:
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Type < rows[j].Type
		})
	case SortByDelta:
		sort.Slice(rows, func(i, j int) bool {
			return OverviewRowSortDelta(rows[i]) > OverviewRowSortDelta(rows[j])
		})
	}
}

// SortCostResults sorts cost results in place by the given field. When
// isActual is true, SortByCost orders by TotalCost instead of Monthly.
func SortCostResults(results []engine.CostResult, field SortField, isActual bool) {
	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
		switch field {
		case SortByCost:
			costA, costB := a.Monthly, b.Monthly
			if isActual {
				costA, costB = a.TotalCost, b.TotalCost
			}
			return costA > costB
		case SortByName:
			return a.ResourceID < b.ResourceID
		case SortByType:
			return a.ResourceType < b.ResourceType
		case SortByDelta:
			return a.Delta > b.Delta
		default:
			return false
		}
	})
}

// SortRecommendations sorts recommendations in place by the given field.
func SortRecommendations(recs []engine.Recommendation, field RecommendationSortField) {
	sort.Slice(recs, func(i, j int) bool {
		a, b := recs[i], recs[j]
		switch field {
		case SortBySavings:
			return a.EstimatedSavings > b.EstimatedSavings
		case SortByResourceID:
			return a.ResourceID < b.ResourceID
		case SortByActionType:
			return a.Type < b.Type
		default:
			return false
		}
	})
}

// TimeAggregationSortSupported reports fields present on time aggregate rows.
// Aggregate rows have period and total; they carry neither resource type nor delta.
func TimeAggregationSortSupported(field SortField) bool {
	return field == SortByCost || field == SortByName
}

// SortTimeAggregations sorts totals descending or periods ascending, with period ties.
func SortTimeAggregations(aggregates []engine.CrossProviderAggregation, field SortField) {
	sort.Slice(aggregates, func(i, j int) bool {
		a, b := aggregates[i], aggregates[j]
		if field == SortByCost && a.Total != b.Total {
			return a.Total > b.Total
		}
		return a.Period < b.Period
	})
}
