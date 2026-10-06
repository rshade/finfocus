package fixedcost

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestInfo_ActualCostsOnly(t *testing.T) {
	t.Parallel()

	info := Info("v0.1.0")
	require.Len(t, info.Capabilities, 1)
	assert.Equal(t, pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS, info.Capabilities[0])
	assert.Equal(t, pluginsdk.SpecVersion, info.SpecVersion)
}

func TestGetActualCost_UsesEnvTotal(t *testing.T) {
	t.Setenv(totalEnv, "42.5")

	resp, err := New().GetActualCost(context.Background(), &pbc.GetActualCostRequest{ResourceId: "i-1"})
	require.NoError(t, err)
	require.Len(t, resp.GetResults(), 1)
	assert.InDelta(t, 42.5, resp.GetResults()[0].GetCost(), 1e-9)
	assert.Equal(t, "USD", resp.GetResults()[0].GetFocusRecord().GetBillingCurrency())
	assert.Equal(t, PluginName, resp.GetResults()[0].GetSource())
}

func TestGetProjectedCost_DoesNotSubstitute(t *testing.T) {
	t.Setenv(totalEnv, "42.5")

	resp, err := New().GetProjectedCost(context.Background(), &pbc.GetProjectedCostRequest{
		Resource: &pbc.ResourceDescriptor{
			Id: "i-1", Provider: "aws", ResourceType: "aws:ec2/instance:Instance",
		},
	})
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.NotContains(t, err.Error(), "42.5")
}

func TestGetActualCost_RejectsBadTotal(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "unset"},
		{name: "zero", raw: "0"},
		{name: "negative", raw: "-1"},
		{name: "text", raw: "nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(totalEnv, tt.raw)
			_, err := New().GetActualCost(context.Background(), &pbc.GetActualCostRequest{ResourceId: "i-1"})
			require.Error(t, err)
		})
	}
}
