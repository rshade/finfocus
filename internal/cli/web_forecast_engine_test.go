package cli

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/webui"
)

const forecastPricedURN = "urn:pulumi:dev::test::aws:ec2/instance:Instance::priced"

// pricedUnchangedClient prices only the deployed, unchanged instance. Every
// resource the preview touches comes back at $0, so a forecast total of 10 can
// only come from the unchanged resource.
type pricedUnchangedClient struct{ includeDismissedClient }

func (*pricedUnchangedClient) GetProjectedCost(
	_ context.Context,
	in *proto.GetProjectedCostRequest,
	_ ...grpc.CallOption,
) (*proto.GetProjectedCostResponse, error) {
	out := &proto.GetProjectedCostResponse{}
	for _, resource := range in.Resources {
		monthly := 0.0
		if resource.ID == forecastPricedURN {
			monthly = 10
		}
		out.Results = append(out.Results, &proto.CostResult{Currency: "USD", MonthlyCost: monthly})
	}
	return out, nil
}

// The forecast prices through the session engine, so a regression to the
// plan-only costs drops the unchanged resource and the total falls to $0.
func TestWebForecast_SessionEngineKeepsUnchangedCostsAfterPreview(t *testing.T) {
	t.Parallel()
	const (
		changed = "urn:pulumi:dev::test::aws:ec2/instance:Instance::changed"
		created = "urn:pulumi:dev::test::aws:s3/bucket:Bucket::created"
	)
	state := []ingest.StackExportResource{
		{URN: changed, Type: "aws:ec2/instance:Instance", Inputs: map[string]any{"instanceType": "t3.small"}},
		{URN: forecastPricedURN, Type: "aws:ec2/instance:Instance", Inputs: map[string]any{"instanceType": "t3.micro"}},
	}
	changedStep := ingest.PulumiStep{
		Op:  "update",
		URN: changed,
		OldState: &ingest.PulumiState{
			Type: "aws:ec2/instance:Instance", Inputs: map[string]any{"instanceType": "t3.small"},
		},
		NewState: &ingest.PulumiState{
			Type: "aws:ec2/instance:Instance", Inputs: map[string]any{"instanceType": "m5.large"},
		},
	}
	createStep := ingest.PulumiStep{
		Op: "create", URN: created,
		NewState: &ingest.PulumiState{Type: "aws:s3/bucket:Bucket", Inputs: map[string]any{}},
	}
	sameStep := ingest.PulumiStep{
		Op: "same", URN: forecastPricedURN,
		OldState: &ingest.PulumiState{
			Type: "aws:ec2/instance:Instance", Inputs: map[string]any{"instanceType": "t3.micro"},
		},
		NewState: &ingest.PulumiState{
			Type: "aws:ec2/instance:Instance", Inputs: map[string]any{"instanceType": "t3.micro"},
		},
	}
	tests := []struct {
		name  string
		steps []ingest.PulumiStep
	}{
		{name: "state only"},
		{name: "after a preview", steps: []ingest.PulumiStep{changedStep, createStep}},
		{
			name:  "after a preview that lists unchanged resources",
			steps: []ingest.PulumiStep{changedStep, createStep, sameStep},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			params := overviewParams{cfg: &config.Config{}, stack: "dev"}
			data := newWebData(&cobra.Command{}, params, engine.DateRange{}, nil)
			data.eng = engine.New([]*pluginhost.Client{{Name: "fixture", API: &pricedUnchangedClient{}}}, nil)
			data.forecastNow = forecastWebNow()
			data.readHistory = func(string) ([]history.CostSnapshot, error) { return nil, nil }
			publishWebTestState(t, data, state)
			if tt.steps != nil {
				publishWebTestPlan(t.Context(), t, data, &ingest.PulumiPlan{Steps: tt.steps})
			}

			costs, err := data.sessionForecastCosts(t.Context())
			require.NoError(t, err)
			total := 0.0
			for _, cost := range costs {
				total += cost.Monthly
			}
			assert.InDelta(t, 10, total, 1e-9, "the unchanged resource stays in the forecast")

			doc, err := data.forecast(t.Context(), webui.ForecastQuery{})
			require.NoError(t, err)
			require.NotEmpty(t, doc.Series)
			require.NotEmpty(t, doc.Series[0].Points)
			assert.InDelta(t, 10, doc.Series[0].Points[0].Value, 1e-9, "the model baseline is the whole stack")
		})
	}
}
