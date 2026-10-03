package tui

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
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
