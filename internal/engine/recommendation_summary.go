package engine

// RecommendationSummary is the shared JSON and NDJSON recommendation summary.
type RecommendationSummary struct {
	TotalCount        int                `json:"total_count"`
	TotalSavings      float64            `json:"total_savings"`
	Currency          string             `json:"currency"`
	CountByActionType map[string]int     `json:"count_by_action_type"`
	SavingsByAction   map[string]float64 `json:"savings_by_action_type"`
}

// BuildRecommendationSummary preserves the CLI summary counts, savings and currency.
func BuildRecommendationSummary(recommendations []Recommendation) RecommendationSummary {
	summary := RecommendationSummary{
		TotalCount:        len(recommendations),
		Currency:          defaultCurrency,
		CountByActionType: map[string]int{},
		SavingsByAction:   map[string]float64{},
	}
	for _, rec := range recommendations {
		summary.CountByActionType[rec.Type]++
		summary.SavingsByAction[rec.Type] += rec.EstimatedSavings
		summary.TotalSavings += rec.EstimatedSavings
		if rec.Currency != "" {
			summary.Currency = rec.Currency
		}
	}
	return summary
}
