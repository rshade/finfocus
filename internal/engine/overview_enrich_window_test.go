package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type windowRecordingEnricher struct {
	mockEnricher

	hourlyRate float64
	request    ActualCostRequest
}

func (w *windowRecordingEnricher) GetActualCostWithOptionsAndErrors(
	_ context.Context,
	req ActualCostRequest,
) (*CostResultWithErrors, error) {
	w.request = req
	return &CostResultWithErrors{Results: []CostResult{{
		TotalCost: w.hourlyRate * req.To.Sub(req.From).Hours(),
	}}}, nil
}

func TestEnrichActualCost_StartsAtResourceCreation(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	window := DateRange{Start: start, End: end}
	inside := end.Add(-22 * time.Hour)
	before := start.Add(-48 * time.Hour)
	after := end.Add(time.Hour)

	tests := []struct {
		name    string
		created *time.Time
		want    time.Time
	}{
		{"created before window", &before, start},
		{"created inside window", &inside, inside},
		{"created time unknown", nil, start},
		{"created after window end", &after, start},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			eng := &windowRecordingEnricher{hourlyRate: 8.392 / 730}
			row := &OverviewRow{URN: "urn:web", CreatedAt: tt.created}
			resource := ResourceDescriptor{Type: "aws:ec2/instance:Instance", ID: "i-1", Provider: "aws"}

			require.Nil(t, enrichActualCost(context.Background(), row, eng, resource, window))

			assert.Equal(t, tt.want, eng.request.From)
			assert.Equal(t, end, eng.request.To)
			require.NotNil(t, row.ActualCost)
			assert.Equal(t, window, row.ActualCost.Period)
			assert.InDelta(t, eng.hourlyRate*end.Sub(tt.want).Hours(), row.ActualCost.MTDCost, 1e-9)
		})
	}
}

func TestEnrichActualCost_ChargesOnlyHoursSinceCreation(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	created := end.Add(-22 * time.Hour)
	eng := &windowRecordingEnricher{hourlyRate: 8.392 / 730}
	row := &OverviewRow{URN: "urn:web", CreatedAt: &created}

	require.Nil(t, enrichActualCost(context.Background(), row, eng,
		ResourceDescriptor{Type: "aws:ec2/instance:Instance", ID: "i-1", Provider: "aws"},
		DateRange{Start: start, End: end}))

	require.NotNil(t, row.ActualCost)
	assert.InDelta(t, 0.253, row.ActualCost.MTDCost, 0.001)
}

func TestEnrichActualCost_ImportedResourceKeepsRequestedWindow(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	imported := end.Add(-22 * time.Hour)
	eng := &windowRecordingEnricher{hourlyRate: 8.392 / 730}
	row := &OverviewRow{URN: "urn:web", CreatedAt: &imported, External: true}

	require.Nil(t, enrichActualCost(context.Background(), row, eng,
		ResourceDescriptor{Type: "aws:ec2/instance:Instance", ID: "i-1", Provider: "aws"},
		DateRange{Start: start, End: end}))

	assert.Equal(t, start, eng.request.From, "import time is not the cloud creation time")
}

func TestStateResourceToRow_CarriesExternal(t *testing.T) {
	t.Parallel()

	row := stateResourceToRow(StateResource{URN: "urn:web", External: true})

	assert.True(t, row.External)
}

func TestForceExtrapolateActual_UsesClampedBillingInterval(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	created := end.Add(-22 * time.Hour)
	eng := &windowRecordingEnricher{hourlyRate: 8.392 / 730}
	row := OverviewRow{URN: "urn:web", Status: StatusDeleting, CreatedAt: &created}

	require.Nil(t, enrichActualCost(context.Background(), &row, eng,
		ResourceDescriptor{Type: "aws:ec2/instance:Instance", ID: "i-1", Provider: "aws"},
		DateRange{Start: start, End: end}))

	wantMonthly := eng.hourlyRate * 22 * (defaultDaysPerMonth * 24 / 22)
	assert.InDelta(t, wantMonthly, ForceExtrapolateActual(row, 18), 0.001)
	delta, ok := CalculateRowDelta(row, 18)
	require.True(t, ok)
	assert.InDelta(t, -wantMonthly, delta, 0.001)
}
