package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/proto"
)

func TestResourceDescriptorJSONOmitsProviderRegion(t *testing.T) {
	t.Parallel()
	desc := ResourceDescriptor{
		Type:     "aws:ec2/instance:Instance",
		ID:       "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Provider: "aws",
		Properties: map[string]any{
			proto.ProviderRegionProperty: "us-east-1",
			"instanceType":               "t3.micro",
		},
	}

	data, err := json.Marshal(desc)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(data, &decoded))
	props, ok := decoded["properties"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"instanceType": "t3.micro"}, props)
	assert.Equal(
		t,
		"us-east-1",
		desc.Properties[proto.ProviderRegionProperty],
		"marshaling must not modify the descriptor",
	)
}

func TestRecommendationsCacheKeyChangesWithProviderRegion(t *testing.T) {
	t.Parallel()
	descriptorIn := func(region string) ResourceDescriptor {
		return ResourceDescriptor{
			Type:       "aws:ec2/instance:Instance",
			ID:         "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Provider:   "aws",
			Properties: map[string]any{proto.ProviderRegionProperty: region, "instanceType": "t3.micro"},
		}
	}
	var e *Engine
	east := e.generateRecommendationsCacheKey([]ResourceDescriptor{descriptorIn("us-east-1")}, nil)
	assert.Equal(t, east, e.generateRecommendationsCacheKey([]ResourceDescriptor{descriptorIn("us-east-1")}, nil))
	assert.NotEqual(t, east, e.generateRecommendationsCacheKey([]ResourceDescriptor{descriptorIn("us-west-2")}, nil))
}

func TestRenderOverviewAsJSONOmitsProviderRegion(t *testing.T) {
	t.Parallel()
	result := OverviewResult{Rows: []OverviewRowResult{{
		Source: OverviewRow{
			URN:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type: "aws:ec2/instance:Instance",
			Properties: map[string]any{
				proto.ProviderRegionProperty: "us-east-1",
				"instanceType":               "t3.micro",
			},
			ProjectedProperties: map[string]any{proto.ProviderRegionProperty: "us-east-1"},
		},
	}}}

	var buf bytes.Buffer
	require.NoError(t, RenderOverviewAsJSON(context.Background(), &buf, result, StackContext{}, nil))
	assert.NotContains(t, buf.String(), proto.ProviderRegionProperty)
	assert.Contains(t, buf.String(), "instanceType")
	assert.Equal(t, "us-east-1", result.Rows[0].Source.Properties[proto.ProviderRegionProperty])
}
