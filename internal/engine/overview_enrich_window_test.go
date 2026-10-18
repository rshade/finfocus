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
