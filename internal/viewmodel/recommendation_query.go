package viewmodel

import (
	"crypto/sha256"
	"fmt"
	"slices"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/proto"
)

// ActionSummary is one ordered action subtotal for browser rendering.
type ActionSummary struct {
	ActionDisplay  string `json:"actionDisplay"`
	Action         string `json:"action"`
	Count          int    `json:"count"`
	SavingsDisplay string `json:"savingsDisplay"`
}

// RecommendationPage preserves the CLI summary schema and complete scorer data.
type RecommendationPage struct {
	Errors         []engine.RecommendationError `json:"errors"`
	Items          []engine.Recommendation      `json:"items"`
	Summary        engine.RecommendationSummary `json:"summary"`
	SavingsDisplay string                       `json:"savingsDisplay"`
	ItemKeys       []string                     `json:"itemKeys"`
	ItemActions    []string                     `json:"itemActions"`
	ItemSavings    []string                     `json:"itemSavings"`
	Actions        []ActionSummary              `json:"actions"`
	Scoring        *engine.ScoringSummary       `json:"scoring,omitempty"`
	Page           int                          `json:"page"`
	TotalPages     int                          `json:"totalPages"`
}

// FormatRecommendationSavings preserves the TUI dollar prefix and USD default.
func FormatRecommendationSavings(amount float64, currency string) string {
	if currency == "" {
		currency = defaultCurrency
	}
	return fmt.Sprintf("$%.2f %s", amount, currency)
}

// RecommendationSavingsDisplay formats savings using the shared TUI convention.
func RecommendationSavingsDisplay(rec engine.Recommendation) string {
	return FormatRecommendationSavings(rec.EstimatedSavings, rec.Currency)
}

// BuildRecommendationPage filters, sorts and pages independent recommendation data.
func BuildRecommendationPage(
	result *engine.RecommendationsResult,
	filter string,
	sort RecommendationSortField,
	requested int,
) RecommendationPage {
	items := FilterRecommendations(append([]engine.Recommendation(nil), result.Recommendations...), filter)
	SortRecommendations(items, sort)
	summary := engine.BuildRecommendationSummary(items)
	page := RecommendationPage{
		Summary:        summary,
		Errors:         result.Errors,
		Scoring:        result.Scoring,
		ItemSavings:    []string{},
		Actions:        []ActionSummary{},
		SavingsDisplay: FormatRecommendationSavings(summary.TotalSavings, summary.Currency),
	}
	var start, end int
	page.Page, page.TotalPages, start, end = PageBounds(len(items), requested)
	page.Items = append([]engine.Recommendation{}, items[start:end]...)
	for _, rec := range page.Items {
		page.ItemActions = append(page.ItemActions, proto.ActionTypeLabelFromString(rec.Type))
		page.ItemSavings = append(page.ItemSavings, RecommendationSavingsDisplay(rec))
		page.ItemKeys = append(page.ItemKeys, RecommendationDetailKey(rec))
	}
	actions := make([]string, 0, len(summary.CountByActionType))
	for action := range summary.CountByActionType {
		actions = append(actions, action)
	}
	slices.Sort(actions)
	for _, action := range actions {
		page.Actions = append(
			page.Actions,
			ActionSummary{
				ActionDisplay:  proto.ActionTypeLabelFromString(action),
				Action:         action,
				Count:          summary.CountByActionType[action],
				SavingsDisplay: FormatRecommendationSavings(summary.SavingsByAction[action], summary.Currency),
			},
		)
	}
	return page
}

// RecommendationDetailKey disambiguates IDless recommendations without changing IDs.
func RecommendationDetailKey(rec engine.Recommendation) string {
	if rec.ID != "" {
		return rec.ID
	}
	sum := sha256.Sum256(
		fmt.Appendf(nil, "%q/%q/%q/%q/%q", rec.ResourceID, rec.Type, rec.Description, rec.Source, rec.Category),
	)
	return fmt.Sprintf("idless-%x", sum)
}
