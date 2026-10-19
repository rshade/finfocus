package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestEstimateResourceBillability(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		resourceType string
		modes        []PricingMode
		cached       bool
		want         ResourceBillability
	}{
		{name: "provider", resourceType: "pulumi:providers:aws", want: NonBillable},
		{name: "stack", resourceType: "pulumi:pulumi:Stack", want: NonBillable},
		{name: "unknown", want: BillabilityUnknown},
		{name: "failed discovery", cached: true, want: BillabilityUnknown},
		{name: "explicit zero cost", cached: true, modes: []PricingMode{{Plugin: "pricing", BillingMode: "zero_cost"}}, want: NonBillable},
		{name: "zero rate is potentially billable", cached: true, modes: []PricingMode{{Plugin: "pricing", BillingMode: "per_hour", RatePerUnit: 0}}, want: PotentiallyBillable},
		{name: "unrelated plugin cannot hide", cached: true, modes: []PricingMode{{Plugin: "other", BillingMode: "zero_cost"}}, want: BillabilityUnknown},
		{name: "conflicting modes", cached: true, modes: []PricingMode{{Plugin: "pricing", BillingMode: "zero_cost"}, {Plugin: "pricing", BillingMode: "per_hour"}}, want: BillabilityUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			plugin := &pricingSpecPlugin{spec: &pbc.PricingSpec{BillingMode: "zero_cost"}}
			eng := New([]*pluginhost.Client{{Name: "pricing", API: plugin}}, nil)
			resource := pricingResource()
			if tc.resourceType != "" {
				resource.Type = tc.resourceType
			}
			if tc.cached {
				eng.storePricingDiscovery(
					pricingDiscoveryKey(ctx, &resource, resource.Type),
					PricingDiscovery{ResourceType: resource.Type, Modes: tc.modes},
				)
			}
			assert.Equal(t, tc.want, eng.ResourceBillability(ctx, resource))
			assert.Zero(t, plugin.specCalls, "listing must never probe pricing")
		})
	}
}

func TestSelectEstimateResources(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	eng := New(nil, nil)
	resources := []ResourceDescriptor{
		{ID: "provider", Type: "pulumi:providers:aws"},
		{ID: "unknown", Type: "custom:module:Unsupported"},
		{
			ID:         "instance",
			Provider:   "aws",
			Type:       "aws:ec2:Instance",
			Properties: map[string]any{"instanceType": "t3.micro"},
		},
		{ID: "stack", Type: "pulumi:pulumi:Stack"},
	}
	selected := eng.SelectEstimateResources(ctx, resources, false)
	require.Len(t, selected.Resources, 2)
	assert.Equal(t, "instance", selected.PreferredURN)
	assert.Equal(t, 2, selected.HiddenCount)
	assert.Equal(t, resources[1:3], selected.Resources)
	all := eng.SelectEstimateResources(ctx, resources, true)
	assert.Equal(t, resources, all.Resources)
	assert.Equal(t, "instance", all.PreferredURN)
	assert.Zero(t, all.HiddenCount)
	assert.Equal(t, NonBillable, all.Billability["provider"])
	empty := eng.SelectEstimateResources(ctx, resources[:1], false)
	assert.Empty(t, empty.Resources)
	assert.Empty(t, empty.PreferredURN)
	assert.Equal(t, "provider", eng.SelectEstimateResources(ctx, resources[:1], true).PreferredURN)
	assert.Len(t, resources, 4)
}

func TestBillabilityDoesNotReuseMetadataAfterEdits(t *testing.T) {
	t.Parallel()
	resource := pricingResource()
	eng := New(
		[]*pluginhost.Client{
			{Name: "pricing", API: &pricingSpecPlugin{spec: &pbc.PricingSpec{BillingMode: "zero_cost"}}},
		},
		nil,
	)
	eng.DiscoverPricingSpec(context.Background(), &resource)
	require.Equal(t, NonBillable, eng.ResourceBillability(context.Background(), resource))
	resource.Properties["instanceType"] = "different"
	assert.Equal(t, BillabilityUnknown, eng.ResourceBillability(context.Background(), resource))
}

func TestBillabilityRequiresAllEffectiveProviders(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		secondMode string
		want       ResourceBillability
	}{
		{name: "missing provider metadata", want: BillabilityUnknown},
		{name: "conflicting provider metadata", secondMode: "per_hour", want: BillabilityUnknown},
		{name: "all providers explicitly zero cost", secondMode: "zero_cost", want: NonBillable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			resource := pricingResource()
			first := &pluginhost.Client{
				Name: "first",
				API:  &pricingSpecPlugin{spec: &pbc.PricingSpec{BillingMode: "zero_cost"}},
			}
			second := &pluginhost.Client{Name: "second", API: &pricingSpecPlugin{}}
			eng := New(
				[]*pluginhost.Client{first, second},
				nil,
			).WithRouter(&mockRouter{selectPluginsFunc: func(context.Context, ResourceDescriptor, string) []PluginMatch { return []PluginMatch{{Client: first}} }})
			modes := []PricingMode{{Plugin: "first", BillingMode: "zero_cost"}}
			if tc.secondMode != "" {
				modes = append(modes, PricingMode{Plugin: "second", BillingMode: tc.secondMode})
			}
			eng.storePricingDiscovery(
				pricingDiscoveryKey(ctx, &resource, resource.Type),
				PricingDiscovery{Modes: modes},
			)
			assert.Equal(t, tc.want, eng.ResourceBillability(ctx, resource))
		})
	}
}

func TestBillabilityIncompleteEvidence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resource := pricingResource()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	assert.Equal(t, BillabilityUnknown, (*Engine)(nil).ResourceBillability(ctx, resource))
	//nolint:staticcheck // Intentionally verifies the nil-context guard.
	nilContextClass := New(nil, nil).ResourceBillability(nil, resource)
	assert.Equal(t, BillabilityUnknown, nilContextClass)
	assert.Equal(t, BillabilityUnknown, New(nil, nil).ResourceBillability(cancelled, resource))
	for _, tc := range []struct {
		name    string
		clients []*pluginhost.Client
		modes   []PricingMode
	}{
		{name: "no effective clients", modes: []PricingMode{{Plugin: "old", BillingMode: "zero_cost"}}},
		{name: "nil client", clients: []*pluginhost.Client{nil}, modes: []PricingMode{{Plugin: "old", BillingMode: "zero_cost"}}},
		{name: "nil API", clients: []*pluginhost.Client{{Name: "old"}}, modes: []PricingMode{{Plugin: "old", BillingMode: "zero_cost"}}},
		{name: "empty billing mode", clients: []*pluginhost.Client{{Name: "pricing", API: &pricingSpecPlugin{}}}, modes: []PricingMode{{Plugin: "pricing"}}},
		{name: "not implemented", clients: []*pluginhost.Client{{Name: "pricing", API: &pricingSpecPlugin{}}}, modes: []PricingMode{{Plugin: "pricing", BillingMode: "not_implemented"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			eng := New(tc.clients, nil)
			eng.storePricingDiscovery(
				pricingDiscoveryKey(ctx, &resource, resource.Type),
				PricingDiscovery{Modes: tc.modes},
			)
			assert.Equal(t, BillabilityUnknown, eng.ResourceBillability(ctx, resource))
		})
	}
}

func TestSelectEstimateResourcesPrefersExplicitBilling(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	resource := pricingResource()
	eng := New([]*pluginhost.Client{{Name: "pricing", API: &pricingSpecPlugin{}}}, nil)
	eng.storePricingDiscovery(
		pricingDiscoveryKey(ctx, &resource, resource.Type),
		PricingDiscovery{Modes: []PricingMode{{Plugin: "pricing", BillingMode: "per_hour"}}},
	)
	resources := []ResourceDescriptor{
		{ID: "unknown", Type: "custom:module:Unknown", Properties: map[string]any{"size": 0}},
		resource,
	}
	selection := eng.SelectEstimateResources(ctx, resources, false)
	assert.Equal(t, resource.ID, selection.PreferredURN)
	assert.Equal(t, PotentiallyBillable, selection.Billability[resource.ID])
	assert.Equal(t, resources, selection.Resources)
}
