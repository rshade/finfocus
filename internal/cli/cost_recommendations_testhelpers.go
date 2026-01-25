package cli

import (
	"context"
	"io"
	"sort"

	"github.com/rshade/finfocus/internal/engine"
)

// TestableRecommendation is a test-friendly version of engine.Recommendation.
// It's used to avoid circular imports in tests.
type TestableRecommendation struct {
	ResourceID       string
	Type             string
	Description      string
	EstimatedSavings float64
	Currency         string
}

// toEngineRecommendation converts TestableRecommendation to engine.Recommendation.
func toEngineRecommendation(tr TestableRecommendation) engine.Recommendation {
	return engine.Recommendation{
		ResourceID:       tr.ResourceID,
		Type:             tr.Type,
		Description:      tr.Description,
		EstimatedSavings: tr.EstimatedSavings,
		Currency:         tr.Currency,
	}
}

// toEngineRecommendations converts a slice of TestableRecommendation to engine.Recommendation.
func toEngineRecommendations(trs []TestableRecommendation) []engine.Recommendation {
	if trs == nil {
		return nil
	}
	result := make([]engine.Recommendation, len(trs))
	for i, tr := range trs {
		result[i] = toEngineRecommendation(tr)
	}
	return result
}

// toTestableRecommendations converts engine.Recommendation to TestableRecommendation.
func toTestableRecommendations(recs []engine.Recommendation) []TestableRecommendation {
	if recs == nil {
		return nil
	}
	result := make([]TestableRecommendation, len(recs))
	for i, r := range recs {
		result[i] = TestableRecommendation{
			ResourceID:       r.ResourceID,
			Type:             r.Type,
			Description:      r.Description,
			EstimatedSavings: r.EstimatedSavings,
			Currency:         r.Currency,
		}
	}
	return result
}

// RenderRecommendationsSummaryForTest is a test export for renderRecommendationsSummary.
func RenderRecommendationsSummaryForTest(w io.Writer, recs []TestableRecommendation) {
	engineRecs := toEngineRecommendations(recs)
	renderRecommendationsSummary(w, engineRecs)
}

// SortRecommendationsBySavingsForTest is a test export for sortRecommendationsBySavings.
func SortRecommendationsBySavingsForTest(recs []TestableRecommendation) []TestableRecommendation {
	engineRecs := toEngineRecommendations(recs)
	sorted := sortRecommendationsBySavings(engineRecs)
	return toTestableRecommendations(sorted)
}

// RenderRecommendationsTableVerboseForTest is a test export for renderRecommendationsTableWithVerbose.
func RenderRecommendationsTableVerboseForTest(w io.Writer, recs []TestableRecommendation, verbose bool) error {
	engineRecs := toEngineRecommendations(recs)
	result := &engine.RecommendationsResult{
		Recommendations: engineRecs,
		TotalSavings:    calculateTotalSavingsForTest(engineRecs),
		Currency:        defaultCurrency,
	}
	return renderRecommendationsTableWithVerbose(w, result, verbose)
}

// calculateTotalSavingsForTest calculates total savings for test data.
func calculateTotalSavingsForTest(recs []engine.Recommendation) float64 {
	var total float64
	for _, rec := range recs {
		total += rec.EstimatedSavings
	}
	return total
}

// ApplyActionTypeFilterForTest is a test export for applyActionTypeFilter.
func ApplyActionTypeFilterForTest(
	recs []TestableRecommendation,
	filter string,
) ([]TestableRecommendation, error) {
	engineRecs := toEngineRecommendations(recs)
	ctx := context.Background()
	filtered, err := applyActionTypeFilter(ctx, engineRecs, filter)
	if err != nil {
		return nil, err
	}
	return toTestableRecommendations(filtered), nil
}

// RenderRecommendationsJSONForTest is a test export for renderRecommendationsJSON.
func RenderRecommendationsJSONForTest(w io.Writer, recs []TestableRecommendation) error {
	engineRecs := toEngineRecommendations(recs)
	result := &engine.RecommendationsResult{
		Recommendations: engineRecs,
		TotalSavings:    calculateTotalSavingsForTest(engineRecs),
		Currency:        defaultCurrency,
	}
	return renderRecommendationsJSON(w, result, nil)
}

// RenderRecommendationsNDJSONForTest is a test export for renderRecommendationsNDJSON.
func RenderRecommendationsNDJSONForTest(w io.Writer, recs []TestableRecommendation) error {
	engineRecs := toEngineRecommendations(recs)
	result := &engine.RecommendationsResult{
		Recommendations: engineRecs,
		TotalSavings:    calculateTotalSavingsForTest(engineRecs),
		Currency:        defaultCurrency,
	}
	return renderRecommendationsNDJSON(w, result, nil)
}

// sortRecommendationsBySavings sorts recommendations by estimated savings in descending order.
func sortRecommendationsBySavings(recs []engine.Recommendation) []engine.Recommendation {
	if len(recs) == 0 {
		return recs
	}

	// Create a copy to avoid modifying the original
	sorted := make([]engine.Recommendation, len(recs))
	copy(sorted, recs)

	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].EstimatedSavings > sorted[j].EstimatedSavings
	})

	return sorted
}
