package proto

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestActualCostObservations_KeepsTimestampedBuckets(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	points := actualCostObservations([]*pbc.ActualCostResult{
		{
			Timestamp:   timestamppb.New(day),
			Cost:        1.5,
			FocusRecord: &pbc.FocusCostRecord{BillingCurrency: "USD"},
		},
		{
			Timestamp:   timestamppb.New(day.Add(48 * time.Hour)),
			Cost:        2,
			FocusRecord: &pbc.FocusCostRecord{BillingCurrency: "EUR"},
		},
		{Cost: 99},
	})

	require.Len(t, points, 2)
	assert.Equal(t, day, points[0].Timestamp)
	assert.InDelta(t, 1.5, points[0].Cost, 0)
	assert.Equal(t, "USD", points[0].Currency)
	assert.Equal(t, "EUR", points[1].Currency)
	assert.InDelta(t, 2, points[1].Cost, 0)

	assert.Empty(t, actualCostObservations([]*pbc.ActualCostResult{{Cost: 10}}))
}
