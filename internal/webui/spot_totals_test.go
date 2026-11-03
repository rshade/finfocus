package webui

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestOverviewTotalsCarrySpotExposure(t *testing.T) {
	t.Parallel()

	rows := []engine.OverviewRow{
		{URN: "spot", ProjectedCost: &engine.ProjectedCostData{
			MonthlyCost: 30, Currency: "USD",
			PricingCategory: engine.PricingCategoryDynamic, SpotInterruptionRisk: 0.18,
		}},
		{URN: "std", ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 70, Currency: "USD"}},
	}
	session := NewSession(context.Background(), SessionOptions{})
	session.SetData(nil, rows, engine.DateRange{}, "dev")
	s := newRunningServer(t, Options{Session: session})

	page := decodeOverviewPage(t, overviewRequest(t, s, "POST", "/api/overview/query", `{}`))
	require.NotNil(t, page.Totals.Spot)
	assert.Equal(t, "30.00 USD (30.0% of 100.00 USD), weighted risk 0.18", page.Totals.Spot.Display)
}

func TestOverviewTotalsWithoutSpotOmitTheBlock(t *testing.T) {
	t.Parallel()

	rows := []engine.OverviewRow{
		{URN: "std", ProjectedCost: &engine.ProjectedCostData{MonthlyCost: 70, Currency: "USD"}},
	}
	session := NewSession(context.Background(), SessionOptions{})
	session.SetData(nil, rows, engine.DateRange{}, "dev")
	s := newRunningServer(t, Options{Session: session})

	page := decodeOverviewPage(t, overviewRequest(t, s, "POST", "/api/overview/query", `{}`))
	assert.Nil(t, page.Totals.Spot)
}
