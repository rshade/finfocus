package history

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourcePoints(t *testing.T) {
	t.Parallel()

	web := "urn:pulumi:dev::app::aws:ec2/instance:Instance::web"
	other := "urn:pulumi:dev::app::aws:ec2/instance:Instance::db"
	early := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	late := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	snapshots := []CostSnapshot{
		{
			Timestamp: late,
			Version:   2,
			Currency:  "USD",
			Resources: []CostResource{
				{URN: web, MonthlyCost: 5},
				{URN: web, MonthlyCost: 7},
				{URN: other, MonthlyCost: 99},
			},
		},
		{
			Timestamp: early,
			Version:   1,
			Currency:  "USD",
			Resources: []CostResource{{URN: web, MonthlyCost: 4}},
		},
		{
			Timestamp: late,
			Version:   3,
			Currency:  "EUR",
			Resources: []CostResource{{URN: other, MonthlyCost: 1}},
		},
	}

	points := ResourcePoints(snapshots, web)
	require.Len(t, points, 2)
	assert.True(t, points[0].Timestamp.Equal(early))
	assert.InDelta(t, 4, points[0].Monthly, 0)
	assert.Equal(t, "USD", points[0].Currency)
	assert.True(t, points[1].Timestamp.Equal(late))
	assert.InDelta(t, 12, points[1].Monthly, 0)
	assert.Equal(t, 2, points[1].Version)

	assert.Nil(t, ResourcePoints(snapshots, ""))
	assert.Empty(t, ResourcePoints(snapshots, "urn:missing"))
}
