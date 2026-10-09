package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func TestEstimateModel_PricingSpecDiscovery(t *testing.T) {
	t.Parallel()

	resource := &engine.ResourceDescriptor{
		Provider:   "aws",
		Type:       "aws:s3:Bucket",
		ID:         "logs",
		Properties: map[string]any{"sizeGb": "20"},
	}
	discovery := engine.PricingDiscovery{
		ResourceType: resource.Type,
		Modes: []engine.PricingMode{
			{
				BillingMode: "per_gb_month",
				RatePerUnit: 0.023,
				Unit:        "GB-month",
				Currency:    "USD",
				Plugin:      "aws-plugin",
				Assumptions: []string{"Standard storage class"},
				MetricHints: []engine.PricingMetricHint{{Metric: "storage_gb", Unit: "GB"}},
				Tiers: []engine.PricingTierOption{
					{MinQuantity: 0, MaxQuantity: 100, RatePerUnit: 0.023, Description: "first 100 GB"},
				},
			},
			{
				BillingMode: "on_demand",
				RatePerUnit: 0.01,
				Unit:        "hour",
				Currency:    "USD",
				Plugin:      "other-plugin",
				Assumptions: []string{"No reserved discount"},
			},
		},
	}

	t.Run("init loads modes tiers assumptions and hints", func(t *testing.T) {
		t.Parallel()
		calls := 0
		model := NewEstimateModel(context.Background(), resource, nil).WithPricingDiscovery(
			func(_ context.Context, got *engine.ResourceDescriptor) engine.PricingDiscovery {
				calls++
				assert.Equal(t, resource.Type, got.Type)
				return discovery
			},
		)
		cmd := model.Init()
		require.NotNil(t, cmd)
		assert.Contains(t, model.View().Content, "Pricing spec: loading...")
		assert.Contains(t, model.View().Content, "sizeGb")

		updated, next := model.Update(cmd())
		require.Nil(t, next)
		view := updated.(*EstimateModel).View().Content
		t.Logf("pricing discovery view:\n%s", view)
		assert.Contains(t, view, "Billing mode:")
		assert.Contains(t, view, "* per_gb_month  USD 0.023/GB-month  aws-plugin")
		assert.Contains(t, view, "Pricing tiers:")
		assert.Contains(t, view, "0-100: 0.023 first 100 GB")
		assert.Contains(t, view, "Assumptions:")
		assert.Contains(t, view, "Standard storage class")
		assert.Contains(t, view, "Usage:")
		assert.Contains(t, view, "storage_gb (GB)")
		assert.Contains(t, view, "sizeGb")
		assert.NotContains(t, view, "Pricing spec: loading...")
		assert.Equal(t, 1, calls)
	})

	t.Run("left and right select another billing mode", func(t *testing.T) {
		t.Parallel()
		model := NewEstimateModel(context.Background(), resource, nil)
		updated, _ := model.Update(estimatePricingSpecMsg{discovery: discovery})
		selected := updated.(*EstimateModel)
		moved, _ := selected.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		view := moved.(*EstimateModel).View().Content
		t.Logf("selected billing mode view:\n%s", view)
		assert.Contains(t, view, "* on_demand  USD 0.01/hour  other-plugin")
		assert.Contains(t, view, "No reserved discount")
		assert.NotContains(t, view, "Standard storage class")
		assert.NotContains(t, view, "Pricing tiers:")
	})

	t.Run("missing spec keeps the estimate editable", func(t *testing.T) {
		t.Parallel()
		model := NewEstimateModel(context.Background(), resource, nil).WithPricingDiscovery(
			func(context.Context, *engine.ResourceDescriptor) engine.PricingDiscovery {
				return engine.PricingDiscovery{ResourceType: resource.Type}
			},
		)
		cmd := model.Init()
		require.NotNil(t, cmd)
		updated, _ := model.Update(cmd())
		view := updated.(*EstimateModel).View().Content
		t.Logf("degraded estimate view:\n%s", view)
		assert.NotContains(t, view, "Billing mode:")
		assert.NotContains(t, view, "Assumptions:")
		assert.NotContains(t, view, "Pricing tiers:")
		assert.Contains(t, view, "What-If")
		assert.Contains(t, view, "sizeGb")
	})

	t.Run("no discovery callback leaves init idle", func(t *testing.T) {
		t.Parallel()
		model := NewEstimateModel(context.Background(), resource, nil)
		assert.Nil(t, model.Init())
		assert.NotContains(t, model.View().Content, "Billing mode:")
	})
}

func TestEstimateModeSwitchRecalculates(t *testing.T) {
	t.Parallel()
	resource := &engine.ResourceDescriptor{
		ID:         "instance",
		Type:       "aws:ec2:Instance",
		Properties: map[string]any{"size": 2},
	}
	model := NewEstimateModelWithCallback(
		context.Background(),
		resource,
		nil,
		func(context.Context, *engine.ResourceDescriptor, map[string]string) (*engine.EstimateResult, error) {
			return &engine.EstimateResult{
				Baseline: &engine.CostResult{Monthly: 60, Currency: "USD"},
				Modified: &engine.CostResult{Monthly: 90, Currency: "USD"},
			}, nil
		},
	)
	model.Update(
		estimatePricingSpecMsg{
			discovery: engine.PricingDiscovery{
				Modes: []engine.PricingMode{
					{Plugin: "first", BillingMode: "hourly"},
					{Plugin: "second", BillingMode: "hourly"},
				},
			},
		},
	)
	_, cmd := model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	require.NotNil(t, cmd, "billing-mode changes must recalculate actual prices")
	model.Update(cmd())
	assert.Contains(t, model.View().Content, "$60.00/mo")
	assert.Contains(t, model.View().Content, "$90.00/mo")
}

func TestEstimateModeCallbackAndLatestResult(t *testing.T) {
	t.Parallel()
	resource := &engine.ResourceDescriptor{
		ID:         "instance",
		Type:       "aws:ec2:Instance",
		Properties: map[string]any{"size": 2},
	}
	model := NewEstimateModel(
		context.Background(),
		resource,
		nil,
	).WithEstimateCallback(func(_ context.Context, request *engine.EstimateRequest) (*engine.EstimateResult, error) {
		assert.Same(t, resource, request.Resource)
		assert.Empty(t, request.PropertyOverrides)
		cost := 20.0
		if request.PricingMode == `["second","hourly"]` {
			cost = 60
		}
		return &engine.EstimateResult{
			Baseline: &engine.CostResult{Monthly: cost, Currency: "USD"},
			Modified: &engine.CostResult{Monthly: cost, Currency: "USD"},
		}, nil
	})
	_, first := model.Update(
		estimatePricingSpecMsg{
			discovery: engine.PricingDiscovery{
				Modes: []engine.PricingMode{
					{Plugin: "first", BillingMode: "hourly"},
					{Plugin: "second", BillingMode: "hourly"},
				},
			},
		},
	)
	require.NotNil(t, first)
	_, second := model.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	require.NotNil(t, second)
	model.Update(second())
	assert.Contains(t, model.View().Content, "$60.00/mo")
	model.Update(first())
	assert.Contains(t, model.View().Content, "$60.00/mo")
	_, back := model.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	require.NotNil(t, back)
	model.Update(back())
	assert.Contains(t, model.View().Content, "$20.00/mo")
}

func TestEstimatePropertyProjectionParity(t *testing.T) {
	t.Parallel()
	resource := &engine.ResourceDescriptor{
		ID:         "instance",
		Type:       "aws:ec2:Instance",
		Properties: map[string]any{"zeta": "last", "alpha": 2, "middle": true},
	}
	for _, tc := range []struct {
		name      string
		deltas    []engine.CostDelta
		wantDelta float64
	}{
		{name: "single", deltas: []engine.CostDelta{{Property: "middle", CostChange: 12}}, wantDelta: 12},
		{name: "combined", deltas: []engine.CostDelta{{Property: "combined", CostChange: 30}}},
		{name: "none"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			model := NewEstimateModel(context.Background(), resource, nil)
			require.Len(t, model.properties, 3)
			model.properties[1].CurrentValue = "false"
			model.applyResult(&engine.EstimateResult{Deltas: []engine.CostDelta{{Property: "alpha", CostChange: 99}}})
			result := &engine.EstimateResult{Deltas: tc.deltas}
			model.applyResult(result)
			assert.Equal(t, []PropertyRow{
				{Key: "alpha", OriginalValue: "2", CurrentValue: "2"},
				{Key: "middle", OriginalValue: "true", CurrentValue: "false", CostDelta: tc.wantDelta},
				{Key: "zeta", OriginalValue: "last", CurrentValue: "last"},
			}, model.properties)
			web := viewmodel.BuildEstimateDisplay(context.Background(), resource, result, model.GetOverrides())
			require.Len(t, web.Properties, 3)
			for i, row := range model.properties {
				assert.Equal(t, row.Key, web.Properties[i].Key)
				assert.Equal(t, row.OriginalValue, web.Properties[i].OriginalValue)
				assert.Equal(t, row.CurrentValue, web.Properties[i].CurrentValue)
				assert.Equal(t, viewmodel.FormatEstimateDelta(row.CostDelta).Text, web.Properties[i].Delta)
			}
		})
	}
}
