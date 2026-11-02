package engine

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func spotItem(cost, risk float64) SpotExposureItem {
	return SpotExposureItem{Cost: cost, Currency: "USD", Category: PricingCategoryDynamic, Risk: risk}
}

func standardItem(cost float64) SpotExposureItem {
	return SpotExposureItem{Cost: cost, Currency: "USD", Category: PricingCategoryStandard}
}

func TestComputeSpotExposure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		items           []SpotExposureItem
		wantNil         bool
		wantSpotCost    float64
		wantTotal       float64
		wantShare       *float64
		wantRisk        *float64
		wantSpot        int
		wantUnknown     int
		wantDisplay     string
		wantDisplayPart string
	}{
		{
			name:    "no items",
			wantNil: true,
		},
		{
			name:    "no spot rows",
			items:   []SpotExposureItem{standardItem(100), {Cost: 50, Currency: "USD"}},
			wantNil: true,
		},
		{
			name:         "mixed spot and standard",
			items:        []SpotExposureItem{spotItem(300, 0.2), spotItem(100, 0.12), standardItem(600)},
			wantSpotCost: 400,
			wantTotal:    1000,
			wantShare:    new(0.4),
			wantRisk:     new(0.18),
			wantSpot:     2,
			wantDisplay:  "400.00 USD (40.0% of 1000.00 USD), weighted risk 0.18",
		},
		{
			name:         "all spot",
			items:        []SpotExposureItem{spotItem(100, 0.5), spotItem(100, 0.1)},
			wantSpotCost: 200,
			wantTotal:    200,
			wantShare:    new(1.0),
			wantRisk:     new(0.3),
			wantSpot:     2,
			wantDisplay:  "200.00 USD (100.0% of 200.00 USD), weighted risk 0.30",
		},
		{
			name:         "all unknown risk is not zero risk",
			items:        []SpotExposureItem{spotItem(100, 0), spotItem(50, 0), standardItem(50)},
			wantSpotCost: 150,
			wantTotal:    200,
			wantShare:    new(0.75),
			wantSpot:     2,
			wantUnknown:  2,
			wantDisplay: "150.00 USD (75.0% of 200.00 USD), weighted risk unknown " +
				"(no spot resource has a risk score)",
		},
		{
			name:         "some unknown risk is excluded from the weighted risk",
			items:        []SpotExposureItem{spotItem(100, 0.4), spotItem(900, 0), standardItem(1000)},
			wantSpotCost: 1000,
			wantTotal:    2000,
			wantShare:    new(0.5),
			wantRisk:     new(0.4),
			wantSpot:     2,
			wantUnknown:  1,
			wantDisplay: "1000.00 USD (50.0% of 2000.00 USD), weighted risk 0.40 " +
				"(1 of 2 spot resources have no risk score and are excluded)",
		},
		{
			name:         "zero cost has no share and no weighted risk",
			items:        []SpotExposureItem{spotItem(0, 0.3)},
			wantSpotCost: 0,
			wantTotal:    0,
			wantSpot:     1,
			wantDisplay:  "0.00 USD, weighted risk unknown",
		},
		{
			name:         "known risk on a zero-cost row still names the excluded row",
			items:        []SpotExposureItem{spotItem(0, 0.3), spotItem(50, 0)},
			wantSpotCost: 50,
			wantTotal:    50,
			wantShare:    new(1.0),
			wantSpot:     2,
			wantUnknown:  1,
			wantDisplay: "50.00 USD (100.0% of 50.00 USD), weighted risk unknown " +
				"(1 of 2 spot resources have no risk score and are excluded)",
		},
		{
			name:         "invalid risk counts as unknown",
			items:        []SpotExposureItem{spotItem(100, math.NaN()), spotItem(100, 1.5), spotItem(100, 0.2)},
			wantSpotCost: 300,
			wantTotal:    300,
			wantShare:    new(1.0),
			wantRisk:     new(0.2),
			wantSpot:     3,
			wantUnknown:  2,
			wantDisplayPart: "weighted risk 0.20 " +
				"(2 of 3 spot resources have no risk score and are excluded)",
		},
		{
			name:    "mixed currencies are not summed",
			items:   []SpotExposureItem{spotItem(100, 0.1), {Cost: 50, Currency: "EUR"}},
			wantNil: true,
		},
		{
			name:         "empty currency joins the common one",
			items:        []SpotExposureItem{spotItem(100, 0.1), {Cost: 100}},
			wantSpotCost: 100,
			wantTotal:    200,
			wantShare:    new(0.5),
			wantRisk:     new(0.1),
			wantSpot:     1,
			wantDisplay:  "100.00 USD (50.0% of 200.00 USD), weighted risk 0.10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ComputeSpotExposure(tt.items)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.InDelta(t, tt.wantSpotCost, got.SpotCost, 1e-9)
			assert.InDelta(t, tt.wantTotal, got.TotalCost, 1e-9)
			assert.Equal(t, tt.wantSpot, got.SpotResources)
			assert.Equal(t, tt.wantUnknown, got.RiskUnknownResources)
			assertOptionalFloat(t, tt.wantShare, got.SpotShare)
			assertOptionalFloat(t, tt.wantRisk, got.WeightedRisk)
			if tt.wantDisplay != "" {
				assert.Equal(t, tt.wantDisplay, got.Display)
			}
			if tt.wantDisplayPart != "" {
				assert.Contains(t, got.Display, tt.wantDisplayPart)
			}
		})
	}
}

func assertOptionalFloat(t *testing.T, want, got *float64) {
	t.Helper()
	if want == nil {
		assert.Nil(t, got)
		return
	}
	require.NotNil(t, got)
	assert.InDelta(t, *want, *got, 1e-9)
}

func TestSpotExposureFromCosts(t *testing.T) {
	t.Parallel()

	results := []CostResult{
		{Monthly: 100, Currency: "USD", PricingCategory: PricingCategoryDynamic, SpotInterruptionRisk: 0.2},
		{Monthly: 100, Currency: "USD", PricingCategory: PricingCategoryStandard},
		{Currency: "USD", Notes: "ERROR: boom", PricingCategory: PricingCategoryDynamic},
		{Currency: "USD", Error: &StructuredError{Code: ErrCodePluginError}, PricingCategory: PricingCategoryDynamic},
	}

	got := SpotExposureFromCosts(results)
	require.NotNil(t, got)
	assert.Equal(t, 1, got.SpotResources)
	assert.InDelta(t, 100.0, got.SpotCost, 1e-9)
	assert.InDelta(t, 200.0, got.TotalCost, 1e-9)
}

func TestAggregateResultsCarriesSpotExposure(t *testing.T) {
	t.Parallel()

	assert.Nil(t, AggregateResults([]CostResult{{Monthly: 10, Currency: "USD"}}).Summary.Spot)

	got := AggregateResults([]CostResult{
		{Monthly: 10, Currency: "USD", PricingCategory: PricingCategoryDynamic, SpotInterruptionRisk: 0.1},
	})
	require.NotNil(t, got.Summary.Spot)
	assert.InDelta(t, 10.0, got.Summary.Spot.SpotCost, 1e-9)
}

func diffEntry(op string, before, after *CostResult) DiffEntry {
	return DiffEntry{Operation: op, Before: before, After: after}
}

func spotCost(monthly, risk float64) *CostResult {
	return &CostResult{
		Monthly: monthly, Currency: "USD",
		PricingCategory: PricingCategoryDynamic, SpotInterruptionRisk: risk,
	}
}

func TestComputeSpotExposureChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		entries     []DiffEntry
		wantNil     bool
		wantBefore  float64
		wantAfter   float64
		wantDelta   float64
		wantDisplay string
	}{
		{
			name: "no spot on either side",
			entries: []DiffEntry{diffEntry(DiffOperationUpdate,
				&CostResult{Monthly: 10, Currency: "USD"}, &CostResult{Monthly: 20, Currency: "USD"})},
			wantNil: true,
		},
		{
			name: "create adds spot",
			entries: []DiffEntry{
				diffEntry(DiffOperationCreate, &CostResult{Currency: "USD"}, spotCost(300, 0.18)),
			},
			wantAfter:   300,
			wantDelta:   300,
			wantDisplay: "+300.00 USD (before 0.00, after 300.00)",
		},
		{
			name: "replacement is one entry and counts once",
			entries: []DiffEntry{
				diffEntry(DiffOperationUpdate, spotCost(100, 0.1), spotCost(150, 0.1)),
			},
			wantBefore:  100,
			wantAfter:   150,
			wantDelta:   50,
			wantDisplay: "+50.00 USD (before 100.00, after 150.00)",
		},
		{
			name: "delete removes spot",
			entries: []DiffEntry{
				diffEntry(DiffOperationDelete, spotCost(100, 0.1), &CostResult{Currency: "USD"}),
			},
			wantBefore:  100,
			wantDelta:   -100,
			wantDisplay: "-100.00 USD (before 100.00, after 0.00)",
		},
		{
			name: "unchanged spot is the same on both sides",
			entries: []DiffEntry{
				diffEntry(DiffOperationSame, spotCost(100, 0.1), spotCost(100, 0.1)),
			},
			wantBefore:  100,
			wantAfter:   100,
			wantDelta:   0,
			wantDisplay: "+0.00 USD (before 100.00, after 100.00)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ComputeSpotExposureChange(&DiffResult{Entries: tt.entries})
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.InDelta(t, tt.wantBefore, got.BeforeSpotCost, 1e-9)
			assert.InDelta(t, tt.wantAfter, got.AfterSpotCost, 1e-9)
			assert.InDelta(t, tt.wantDelta, got.DeltaSpotCost, 1e-9)
			assert.Equal(t, tt.wantDisplay, got.Display)
		})
	}

	assert.Nil(t, ComputeSpotExposureChange(nil))
}
