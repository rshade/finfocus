package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func TestRecommendationWebDisplayParity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		amount         float64
		currency, want string
	}{{"default", 12, "", "$12.00 USD"}, {"explicit", 12, "USD", "$12.00 USD"}, {"eur", 12, "EUR", "$12.00 EUR"}, {"negative", -12, "", "$-12.00 USD"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := engine.Recommendation{EstimatedSavings: tc.amount, Currency: tc.currency}
			row := NewRecommendationRow(rec)
			assert.Equal(t, tc.want, row.Savings)
			assert.Equal(t, row.Savings, viewmodel.RecommendationSavingsDisplay(rec))
		})
	}
}
