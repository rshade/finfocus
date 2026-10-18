package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

func TestRunBackgroundPreviewCapturesPlan(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var got int
	params := overviewParams{
		cfg:        &config.Config{},
		pulumiJSON: "../../test/fixtures/estimate/preview-urn.json",
		capturePlan: func(plan *ingest.PulumiPlan) {
			got = len(plan.GetResources())
		},
	}
	msg, err := runBackgroundPreviewWithError(context.Background(), params, "", "", nil)
	require.NoError(t, err)
	assert.Equal(t, 1, got)
	assert.Contains(t, msg.StatusByURN, "urn:pulumi:dev::proj::aws:ec2/instance:Instance::demo-instance")
}

func TestAppendPreviewRowsAddsPlanOnlyCreate(t *testing.T) {
	t.Parallel()

	plan, err := ingest.LoadPulumiPlanWithContext(
		context.Background(), "../../test/fixtures/estimate/preview-urn.json",
	)
	require.NoError(t, err)
	data := &webData{gate: make(chan struct{}, 1)}
	data.capturePlan(plan)

	existing := "urn:pulumi:dev::proj::aws:s3/bucket:Bucket::logs"
	rows, err := data.appendPreviewRows(context.Background(), []engine.OverviewRow{{
		URN: existing, Type: "aws:s3/bucket:Bucket", Status: engine.StatusActive,
	}})
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, existing, rows[0].URN)
	assert.Equal(t, "urn:pulumi:dev::proj::aws:ec2/instance:Instance::demo-instance", rows[1].URN)
	assert.Equal(t, engine.StatusCreating, rows[1].Status)
	assert.Equal(t, "t3.micro", rows[1].Properties["instanceType"])
}

func TestAppendPreviewRowsKeepsExistingURN(t *testing.T) {
	t.Parallel()

	plan, err := ingest.LoadPulumiPlanWithContext(
		context.Background(), "../../test/fixtures/estimate/preview-urn.json",
	)
	require.NoError(t, err)
	data := &webData{gate: make(chan struct{}, 1)}
	data.capturePlan(plan)
	urn := "urn:pulumi:dev::proj::aws:ec2/instance:Instance::demo-instance"
	rows, err := data.appendPreviewRows(context.Background(), []engine.OverviewRow{{
		URN: urn, Type: "aws:ec2/instance:Instance", Status: engine.StatusUpdating,
	}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, engine.StatusUpdating, rows[0].Status)
}
