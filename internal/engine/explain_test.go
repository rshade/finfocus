// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

func TestExplainPricing(t *testing.T) {
	t.Parallel()

	spec := explainSpec()

	t.Run("off does not call GetPricingSpec", func(t *testing.T) {
		t.Parallel()
		plugin := explainPlugin(spec, nil)
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil)
		diff, err := eng.GetProjectedCostDiff(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, diff.Entries, 1)
		assert.Nil(t, diff.Entries[0].PricingSpec)
		assert.InDelta(t, 9.0, diff.Entries[0].After.Monthly, 1e-9)
		assert.Equal(t, 0, plugin.specCalls)

		var buf bytes.Buffer
		require.NoError(t, RenderProjectedDiff(&buf, OutputJSON, diff, false))
		assert.NotContains(t, buf.String(), "pricing_spec")
	})

	t.Run("on shows the spec and keeps the calculated cost", func(t *testing.T) {
		t.Parallel()
		plugin := explainPlugin(spec, nil)
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil).WithExplainPricing(true)
		diff, err := eng.GetProjectedCostDiff(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, diff.Entries, 1)
		assert.InDelta(t, 9.0, diff.Entries[0].After.Monthly, 1e-9)
		assert.Equal(t, 1, plugin.specCalls)
		require.NotNil(t, diff.Entries[0].PricingSpec)
		assert.Equal(t, "on-demand", diff.Entries[0].PricingSpec.BillingMode)
		assert.Equal(t, "hour", diff.Entries[0].PricingSpec.Unit)
		assert.InDelta(t, 0.5, diff.Entries[0].PricingSpec.RatePerUnit, 1e-9)
		assert.Equal(t, "AWS Price List API", diff.Entries[0].PricingSpec.Source)
		assert.Equal(t, []string{"730 hours/month"}, diff.Entries[0].PricingSpec.Assumptions)
		require.Len(t, diff.Entries[0].PricingSpec.PricingTiers, 1)
		assert.InDelta(t, 0.5, diff.Entries[0].PricingSpec.PricingTiers[0].RatePerUnit, 1e-9)

		var table bytes.Buffer
		require.NoError(t, RenderProjectedDiff(&table, OutputTable, diff, false))
		text := table.String()
		assert.Contains(t, text, "Billing Mode: on-demand")
		assert.Contains(t, text, "Unit: hour")
		assert.Contains(t, text, "Rate: $0.5/hour")
		assert.Contains(t, text, "Source: AWS Price List API")
		assert.Contains(t, text, "730 hours/month")
		assert.Contains(t, text, "Pricing Tier: $0.5/hour from 0 to 100")

		var jsonOut bytes.Buffer
		require.NoError(t, RenderProjectedDiff(&jsonOut, OutputJSON, diff, false))
		assert.Contains(t, jsonOut.String(), `"pricing_spec"`)
		assert.Contains(t, jsonOut.String(), `"billing_mode": "on-demand"`)
		assert.Contains(t, jsonOut.String(), `"monthly": 9`)

		var ndjson bytes.Buffer
		require.NoError(t, RenderProjectedDiff(&ndjson, OutputNDJSON, diff, false))
		assert.Contains(t, ndjson.String(), `"pricing_spec"`)
		assert.Contains(t, ndjson.String(), `"billing_mode":"on-demand"`)
	})

	t.Run("rpc error leaves the cost and omits the spec", func(t *testing.T) {
		t.Parallel()
		plugin := explainPlugin(spec, errors.New("unimplemented"))
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil).WithExplainPricing(true)
		diff, err := eng.GetProjectedCostDiff(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, diff.Entries, 1)
		assert.Nil(t, diff.Entries[0].PricingSpec)
		assert.InDelta(t, 9.0, diff.Entries[0].After.Monthly, 1e-9)
		assert.Equal(t, 1, plugin.specCalls)
	})

	t.Run("delete explains the old sku", func(t *testing.T) {
		t.Parallel()
		plugin := explainPlugin(spec, nil)
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil).WithExplainPricing(true)
		resource := pricingResource()
		resource.Operation = DiffOperationDelete
		resource.OldProperties = map[string]any{"instanceType": "t3.small"}
		resource.Properties = map[string]any{"instanceType": "m5.large"}
		diff, err := eng.GetProjectedCostDiff(context.Background(), []ResourceDescriptor{resource})
		require.NoError(t, err)
		require.Len(t, diff.Entries, 1)
		require.NotNil(t, diff.Entries[0].PricingSpec)
		assert.Equal(t, "t3.small", plugin.lastSKU)
		assert.InDelta(t, 9.0, diff.Entries[0].Before.Monthly, 1e-9)
		assert.InDelta(t, 0.0, diff.Entries[0].After.Monthly, 1e-9)
	})

	t.Run("update explains the new sku", func(t *testing.T) {
		t.Parallel()
		plugin := explainPlugin(spec, nil)
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil).WithExplainPricing(true)
		resource := pricingResource()
		resource.Operation = DiffOperationUpdate
		resource.OldProperties = map[string]any{"instanceType": "t3.small"}
		resource.Properties = map[string]any{"instanceType": "m5.large"}
		diff, err := eng.GetProjectedCostDiff(context.Background(), []ResourceDescriptor{resource})
		require.NoError(t, err)
		require.Len(t, diff.Entries, 1)
		require.NotNil(t, diff.Entries[0].PricingSpec)
		assert.Equal(t, "m5.large", plugin.lastSKU)
		assert.InDelta(t, 9.0, diff.Entries[0].After.Monthly, 1e-9)
	})

	t.Run("second plugin supplies the spec when the first fails", func(t *testing.T) {
		t.Parallel()
		first := explainPlugin(nil, errors.New("no spec"))
		second := explainPlugin(&pbc.PricingSpec{
			BillingMode: "reserved", Unit: "hour", RatePerUnit: 0.2, Source: "second-plugin",
		}, nil)
		eng := New([]*pluginhost.Client{
			{Name: "first", API: first},
			{Name: "second", API: second},
		}, nil).WithExplainPricing(true)
		diff, err := eng.GetProjectedCostDiff(context.Background(), []ResourceDescriptor{pricingResource()})
		require.NoError(t, err)
		require.Len(t, diff.Entries, 1)
		require.NotNil(t, diff.Entries[0].PricingSpec)
		assert.Equal(t, "second-plugin", diff.Entries[0].PricingSpec.Source)
		assert.InDelta(t, 9.0, diff.Entries[0].After.Monthly, 1e-9)
		assert.Equal(t, 1, first.specCalls)
		assert.Equal(t, 1, second.specCalls)
	})

	t.Run("unpriced internal types are not explained", func(t *testing.T) {
		t.Parallel()
		plugin := explainPlugin(spec, nil)
		eng := New([]*pluginhost.Client{{Name: "aws-plugin", API: plugin}}, nil).WithExplainPricing(true)
		stack := ResourceDescriptor{Type: "pulumi:pulumi:Stack", ID: "org/proj/dev", Provider: "pulumi"}
		diff, err := eng.GetProjectedCostDiff(context.Background(), []ResourceDescriptor{stack, pricingResource()})
		require.NoError(t, err)
		require.Len(t, diff.Entries, 1)
		assert.Equal(t, "i-123", diff.Entries[0].ResourceID)
		require.NotNil(t, diff.Entries[0].PricingSpec)
		assert.Equal(t, 1, plugin.specCalls)
	})
}

func TestWithExplainPricingNilEngine(t *testing.T) {
	t.Parallel()
	assert.Nil(t, (*Engine)(nil).WithExplainPricing(true))
}

func explainSpec() *pbc.PricingSpec {
	return &pbc.PricingSpec{
		BillingMode: "on-demand",
		Unit:        "hour",
		RatePerUnit: 0.5,
		Source:      "AWS Price List API",
		Assumptions: []string{"730 hours/month"},
		PricingTiers: []*pbc.PricingTier{{
			MinQuantity: 0,
			MaxQuantity: 100,
			RatePerUnit: 0.5,
			Description: "first",
		}},
	}
}

func explainPlugin(spec *pbc.PricingSpec, specErr error) *pricingSpecPlugin {
	return &pricingSpecPlugin{
		spec:    spec,
		specErr: specErr,
		projected: &proto.GetProjectedCostResponse{Results: []*proto.CostResult{{
			MonthlyCost: 9,
			HourlyCost:  9.0 / hoursPerMonth,
			Currency:    "USD",
			Notes:       "plugin price",
		}}},
	}
}
