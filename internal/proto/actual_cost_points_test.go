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

func TestActualCostObservations_BlankCurrencyDoesNotBorrowAnotherBucketsCurrency(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	points := actualCostObservations([]*pbc.ActualCostResult{
		{Timestamp: timestamppb.New(day), Cost: 1},
		{
			Timestamp:   timestamppb.New(day),
			Cost:        2,
			FocusRecord: &pbc.FocusCostRecord{BillingCurrency: "EUR"},
		},
	})

	require.Len(t, points, 2)
	assert.Equal(t, "USD", points[0].Currency, "a blank currency is USD, whatever another bucket says")
	assert.Equal(t, "EUR", points[1].Currency)
}

func TestActualCostObservations_SkipsBucketsWiderThanOneUTCDay(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	period := func(start, end time.Time) *pbc.FocusCostRecord {
		return &pbc.FocusCostRecord{
			ChargePeriodStart: timestamppb.New(start),
			ChargePeriodEnd:   timestamppb.New(end),
		}
	}
	points := actualCostObservations([]*pbc.ActualCostResult{
		{Timestamp: timestamppb.New(day), Cost: 300, FocusRecord: period(day, day.AddDate(0, 0, 30))},
		{Timestamp: timestamppb.New(day), Cost: 7, FocusRecord: period(day.Add(10*time.Hour), day.Add(34*time.Hour))},
		{Timestamp: timestamppb.New(day), Cost: 1, FocusRecord: period(day, day.AddDate(0, 0, 1))},
		{Timestamp: timestamppb.New(day.AddDate(0, 0, 1)), Cost: 2},
	})

	require.Len(t, points, 2, "a 30-day bucket and a bucket straddling midnight are not daily amounts")
	assert.InDelta(t, 1, points[0].Cost, 0, "a bucket covering exactly one UTC day is kept")
	assert.InDelta(t, 2, points[1].Cost, 0, "a bucket with no charge period stays a daily amount")
}

func TestActualCostObservations_SkipsABucketStampedOnAnotherDayThanItsPeriod(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	points := actualCostObservations([]*pbc.ActualCostResult{
		{
			Timestamp: timestamppb.New(day.AddDate(0, 0, 3)),
			Cost:      9,
			FocusRecord: &pbc.FocusCostRecord{
				ChargePeriodStart: timestamppb.New(day),
				ChargePeriodEnd:   timestamppb.New(day.AddDate(0, 0, 1)),
			},
		},
		{
			Timestamp: timestamppb.New(day.Add(5 * time.Hour)),
			Cost:      4,
			FocusRecord: &pbc.FocusCostRecord{
				ChargePeriodStart: timestamppb.New(day),
				ChargePeriodEnd:   timestamppb.New(day.AddDate(0, 0, 1)),
			},
		},
	})

	require.Len(t, points, 1, "a timestamp outside the stated period would put the amount on the wrong day")
	assert.InDelta(t, 4, points[0].Cost, 0)
}
