package cli

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

type interactiveEstimatePlugin struct {
	proto.CostSourceClient

	rate float64
}

func (p *interactiveEstimatePlugin) GetPricingSpec(
	context.Context,
	*pbc.GetPricingSpecRequest,
	...grpc.CallOption,
) (*pbc.GetPricingSpecResponse, error) {
	return &pbc.GetPricingSpecResponse{
		Spec: &pbc.PricingSpec{BillingMode: "hourly", Currency: "USD", RatePerUnit: p.rate},
	}, nil
}

func (p *interactiveEstimatePlugin) EstimateCost(
	_ context.Context,
	request *pbc.EstimateCostRequest,
	_ ...grpc.CallOption,
) (*pbc.EstimateCostResponse, error) {
	return &pbc.EstimateCostResponse{
		Currency:    "USD",
		CostMonthly: p.rate * request.GetAttributes().GetFields()["size"].GetNumberValue(),
	}, nil
}

func TestInteractiveEstimateEngineIntegration(t *testing.T) {
	t.Parallel()
	resource := &engine.ResourceDescriptor{
		ID:         "instance",
		Type:       "aws:ec2:Instance",
		Provider:   "aws",
		Properties: map[string]any{"size": float64(2)},
	}
	eng := engine.New(
		[]*pluginhost.Client{
			{Name: "first", API: &interactiveEstimatePlugin{rate: 10}},
			{Name: "second", API: &interactiveEstimatePlugin{rate: 30}},
		},
		nil,
	)
	model := newInteractiveEstimateModel(context.Background(), eng, resource)
	require.NotNil(t, model)
	assert.InDelta(t, 20, model.GetResult().Baseline.Monthly, 1e-9)
	assert.InDelta(t, 20, model.GetResult().Modified.Monthly, 1e-9)
	// Ordinary edits before discovery retain automatic provider selection.
	model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model.Update(tea.KeyPressMsg{Text: "3"})
	_, edit := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, edit)
	model.Update(edit())
	assert.InDelta(t, 30, model.GetResult().Modified.Monthly, 1e-9)
	assert.InDelta(t, 10, model.GetResult().TotalChange, 1e-9)
	// Discovery and mode selection use the same engine callback with current edits.
	discovery := model.Init()
	require.NotNil(t, discovery)
	_, initialMode := model.Update(discovery())
	require.NotNil(t, initialMode)
	model.Update(initialMode())
	_, selected := model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	require.NotNil(t, selected)
	model.Update(selected())
	assert.InDelta(t, 60, model.GetResult().Baseline.Monthly, 1e-9)
	assert.InDelta(t, 90, model.GetResult().Modified.Monthly, 1e-9)
	assert.InDelta(t, 30, model.GetResult().TotalChange, 1e-9)
	// Reverting to the original value must use EstimateBaseline, not fail the
	// public EstimateCost empty-overrides validation.
	model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	model.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	model.Update(tea.KeyPressMsg{Text: "2"})
	_, reverted := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, reverted)
	model.Update(reverted())
	assert.Empty(t, model.GetOverrides())
	assert.InDelta(t, 60, model.GetResult().Baseline.Monthly, 1e-9)
	assert.InDelta(t, 60, model.GetResult().Modified.Monthly, 1e-9)
	assert.Zero(t, model.GetResult().TotalChange)
	assert.NotContains(t, model.View().Content, "Error:")
}

func TestInteractiveEstimateInitialFailureRemainsEditable(t *testing.T) {
	t.Parallel()
	model := newInteractiveEstimateModel(
		context.Background(),
		engine.New(nil, nil),
		&engine.ResourceDescriptor{ID: "invalid", Properties: map[string]any{"size": float64(2)}},
	)
	require.NotNil(t, model)
	assert.Zero(t, model.GetResult().Baseline.Monthly)
	assert.Contains(t, model.View().Content, "size")
	assert.NotContains(t, model.View().Content, "Error:")
}
