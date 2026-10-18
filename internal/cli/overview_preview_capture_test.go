package cli

import (
	"context"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestRunBackgroundPreviewCapturesPlan(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	var got int
	params := overviewParams{
		cfg:        &config.Config{},
		pulumiJSON: "../../test/fixtures/estimate/preview-urn.json",
		capturePlan: func(_ context.Context, plan *ingest.PulumiPlan) {
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
	data.capturePlan(context.Background(), plan)

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
	data.capturePlan(context.Background(), plan)
	urn := "urn:pulumi:dev::proj::aws:ec2/instance:Instance::demo-instance"
	rows, err := data.appendPreviewRows(context.Background(), []engine.OverviewRow{{
		URN: urn, Type: "aws:ec2/instance:Instance", Status: engine.StatusUpdating,
	}})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, engine.StatusUpdating, rows[0].Status)
}

func TestAppendPreviewRowsAppendsOnlyPlanCreates(t *testing.T) {
	t.Parallel()

	step := func(op, urn, typ string) ingest.PulumiStep {
		state := &ingest.PulumiState{URN: urn, Type: typ, Inputs: map[string]any{"instanceType": "t3.micro"}}
		s := ingest.PulumiStep{Op: op, URN: urn}
		switch op {
		case "delete":
			s.OldState = state
		default:
			s.NewState = state
		}
		return s
	}
	const prefix = "urn:pulumi:dev::proj::"
	plan := &ingest.PulumiPlan{Steps: []ingest.PulumiStep{
		step("create", prefix+"aws:ec2/instance:Instance::new", "aws:ec2/instance:Instance"),
		step("create", prefix+"aws:ec2/instance:Instance::dup", "aws:ec2/instance:Instance"),
		step("update", prefix+"aws:ec2/instance:Instance::upd", "aws:ec2/instance:Instance"),
		step("same", prefix+"aws:ec2/instance:Instance::same", "aws:ec2/instance:Instance"),
		step("delete", prefix+"aws:ec2/instance:Instance::gone", "aws:ec2/instance:Instance"),
		step("same", prefix+"pulumi:pulumi:Stack::proj-dev", "pulumi:pulumi:Stack"),
		step("same", prefix+"pulumi:providers:aws::default", "pulumi:providers:aws"),
	}}

	tests := []struct {
		name     string
		existing []engine.OverviewRow
		want     []string
	}{
		{
			name: "state-only rows get only the create",
			want: []string{prefix + "aws:ec2/instance:Instance::new", prefix + "aws:ec2/instance:Instance::dup"},
		},
		{
			name:     "create with an existing row is not duplicated",
			existing: []engine.OverviewRow{{URN: prefix + "aws:ec2/instance:Instance::dup"}},
			want:     []string{prefix + "aws:ec2/instance:Instance::dup", prefix + "aws:ec2/instance:Instance::new"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			data := &webData{gate: make(chan struct{}, 1)}
			data.capturePlan(context.Background(), plan)
			rows, err := data.appendPreviewRows(context.Background(), tt.existing)
			require.NoError(t, err)
			got := make([]string, 0, len(rows))
			for _, row := range rows {
				got = append(got, row.URN)
			}
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}

type previewPricingClient struct{ includeDismissedClient }

func (c *previewPricingClient) GetProjectedCost(
	_ context.Context,
	in *proto.GetProjectedCostRequest,
	_ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	out := &proto.GetProjectedCostResponse{}
	for range in.Resources {
		out.Results = append(out.Results, &proto.CostResult{Currency: "USD", MonthlyCost: 42})
	}
	return out, nil
}

func TestAppendPreviewRowsPricesPlanOnlyCreatesWithZeroDeployedResources(t *testing.T) {
	t.Parallel()

	const urn = "urn:pulumi:dev::proj::aws:ec2/instance:Instance::fresh"
	inputs := map[string]any{"instanceType": "t3.micro"}
	plan := &ingest.PulumiPlan{Steps: []ingest.PulumiStep{{
		Op:       "create",
		URN:      urn,
		NewState: &ingest.PulumiState{URN: urn, Type: "aws:ec2/instance:Instance", Inputs: inputs},
	}}}

	data := newWebData(&cobra.Command{}, overviewParams{cfg: &config.Config{}}, engine.DateRange{}, nil)
	data.eng = engine.New([]*pluginhost.Client{{Name: "fixture", API: &previewPricingClient{}}}, nil)
	data.captureState(nil)
	data.capturePlan(context.Background(), plan)

	rows, err := data.appendPreviewRows(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, urn, rows[0].URN)
	assert.Equal(t, "aws:ec2/instance:Instance", rows[0].Type)
	assert.Equal(t, engine.StatusCreating, rows[0].Status)
	require.NotNil(t, rows[0].ProjectedCost)
	assert.InDelta(t, 42, rows[0].ProjectedCost.MonthlyCost, 0.001)

	rows[0].Properties["instanceType"] = "changed"
	assert.Equal(t, "t3.micro", inputs["instanceType"])
	assert.Equal(t, "t3.micro", data.planned[0].Properties["instanceType"])
}

func TestCapturePlanHonorsCancellationWhileGateHeld(t *testing.T) {
	t.Parallel()

	data := &webData{gate: make(chan struct{}, 1)}
	data.gate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		data.capturePlan(ctx, &ingest.PulumiPlan{Steps: []ingest.PulumiStep{
			{URN: "a", Type: "aws:s3/bucket:Bucket", Op: "create"},
		}})
	}()
	cancel()
	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 5*time.Second, time.Millisecond)
	assert.Empty(t, data.planned)
	assert.Empty(t, data.planSteps)
	assert.Len(t, data.gate, 1)
}

type gatedPricingClient struct {
	includeDismissedClient

	entered chan struct{}
	release chan struct{}
}

func (c *gatedPricingClient) GetProjectedCost(
	_ context.Context,
	in *proto.GetProjectedCostRequest,
	_ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	close(c.entered)
	<-c.release
	out := &proto.GetProjectedCostResponse{}
	for range in.Resources {
		out.Results = append(out.Results, &proto.CostResult{Currency: "USD", MonthlyCost: 1})
	}
	return out, nil
}

func TestAppendPreviewRowsHoldsQueryGateWhilePricing(t *testing.T) {
	t.Parallel()

	const urn = "urn:pulumi:dev::proj::aws:ec2/instance:Instance::fresh"
	plan := &ingest.PulumiPlan{Steps: []ingest.PulumiStep{{
		Op:  "create",
		URN: urn,
		NewState: &ingest.PulumiState{
			URN: urn, Type: "aws:ec2/instance:Instance", Inputs: map[string]any{"instanceType": "t3.micro"},
		},
	}}}
	client := &gatedPricingClient{entered: make(chan struct{}), release: make(chan struct{})}
	data := newWebData(&cobra.Command{}, overviewParams{cfg: &config.Config{}}, engine.DateRange{}, nil)
	data.eng = engine.New([]*pluginhost.Client{{Name: "fixture", API: client}}, nil)
	data.captureState(nil)
	data.capturePlan(context.Background(), plan)

	done := make(chan error, 1)
	go func() {
		_, err := data.appendPreviewRows(context.Background(), nil)
		done <- err
	}()
	<-client.entered

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	unlock, err := data.lock(ctx)
	if err == nil {
		unlock()
	}
	require.Error(t, err, "query gate must stay held while plan-only rows are priced")

	close(client.release)
	require.NoError(t, <-done)
}

func TestAppendPreviewRowsAppliesLaunchFilters(t *testing.T) {
	t.Parallel()

	const prefix = "urn:pulumi:dev::proj::"
	create := func(urn, typ string) ingest.PulumiStep {
		return ingest.PulumiStep{
			Op: "create", URN: urn,
			NewState: &ingest.PulumiState{URN: urn, Type: typ, Inputs: map[string]any{}},
		}
	}
	plan := &ingest.PulumiPlan{Steps: []ingest.PulumiStep{
		create(prefix+"aws:ec2/instance:Instance::a", "aws:ec2/instance:Instance"),
		create(prefix+"gcp:compute/instance:Instance::g", "gcp:compute/instance:Instance"),
	}}
	data := newWebData(&cobra.Command{}, overviewParams{
		cfg: &config.Config{}, filter: []string{"provider=aws"},
	}, engine.DateRange{}, nil)
	data.capturePlan(context.Background(), plan)

	rows, err := data.appendPreviewRows(context.Background(), nil)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, prefix+"aws:ec2/instance:Instance::a", rows[0].URN)
}
