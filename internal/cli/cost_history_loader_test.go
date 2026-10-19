package cli

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/tui/detail"
)

func TestLoadDetailHistory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	urn := "urn:pulumi:dev::app::aws:ec2/instance:Instance::web"
	seedHistory(t, dir, "dev", history.CostSnapshot{
		Timestamp:    time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		Version:      3,
		TotalMonthly: 12,
		Currency:     "USD",
		Resources: []history.CostResource{{
			URN:         urn,
			Type:        "aws:ec2/instance:Instance",
			Provider:    "aws",
			MonthlyCost: 12,
		}},
	})

	points, err := loadDetailHistory(context.Background(), dir, "dev", urn)
	require.NoError(t, err)
	require.Len(t, points, 1)
	assert.InDelta(t, 12, points[0].Amount, 0)
	assert.Equal(t, "USD", points[0].Currency)

	missing, err := loadDetailHistory(context.Background(), dir, "dev", "urn:missing")
	require.NoError(t, err)
	assert.Empty(t, missing)

	_, err = loadDetailHistory(context.Background(), dir, "", urn)
	require.ErrorIs(t, err, detail.ErrHistorySkipped)

	_, err = loadDetailHistory(context.Background(), dir, "unknown", urn)
	require.ErrorIs(t, err, detail.ErrHistorySkipped)

	_, err = loadDetailHistory(context.Background(), dir, "other", urn)
	require.ErrorIs(t, err, detail.ErrHistorySkipped)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = loadDetailHistory(ctx, dir, "dev", urn)
	require.ErrorIs(t, err, context.Canceled)
}
