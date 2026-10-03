package tui

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
)

func TestBuildOverviewTable_WarnColumn(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	rows := []engine.OverviewRow{
		{
			URN: "my-instance", Type: "aws:ec2/instance:Instance", Status: engine.StatusActive,
			Warnings: []engine.OverviewWarning{engine.WarnDrift, engine.WarnError},
		},
		{
			URN: "my-bucket", Type: "aws:s3/bucket:Bucket", Status: engine.StatusCreating,
			Warnings: []engine.OverviewWarning{engine.WarnNew},
		},
	}
	model, _ := NewOverviewModel(ctx, rows, len(rows), nil, nil)
	model.width = 140
	view := model.buildOverviewTable().View()
	t.Logf("overview table:\n%s", view)
	assert.Contains(t, view, "Warn")
	assert.Contains(t, view, "drift,error")
	assert.Contains(t, view, "new")
	assert.NotContains(t, view, "estimate")
	assert.NotContains(t, view, "stale")
}
