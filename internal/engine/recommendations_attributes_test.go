package engine

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
)

// TestRecommendationsRequest_SendsTargetAttributes covers a resource with more
// top-level properties than the 50-tag cap leaves room for, as an EC2 instance
// has. Its nested rootBlockDevice.volumeId never becomes a dotted tag, so the
// attributes are the only way a plugin can read it.
func TestRecommendationsRequest_SendsTargetAttributes(t *testing.T) {
	t.Parallel()

	props := map[string]any{
		"rootBlockDevice": map[string]any{"volumeId": "vol-0abc"},
		"password":        "hunter2",
	}
	for i := range 64 {
		props["prop"+strconv.Itoa(i)] = "value"
	}
	plugin := &recordingRecClient{name: "p"}
	eng := New([]*pluginhost.Client{plugin.client()}, nil)

	_, err := eng.GetRecommendationsForResources(context.Background(), []ResourceDescriptor{
		{Type: "aws:ec2/instance:Instance", ID: "urn:web", Provider: "aws", Properties: props},
		{Type: "aws:s3/bucket:Bucket", ID: "urn:bucket", Provider: "aws"},
	})
	require.NoError(t, err)

	reqs := plugin.snapshot()
	require.Len(t, reqs, 1)
	require.Len(t, reqs[0].TargetResources, 2)
	instance := reqs[0].TargetResources[0]
	assert.NotContains(t, instance.Properties, "rootBlockDevice.volumeId",
		"the tag cap leaves no room for the dotted key")
	require.NotNil(t, instance.Attributes)
	attrs := instance.Attributes.AsMap()
	rootBlockDevice, ok := attrs["rootBlockDevice"].(map[string]any)
	require.True(t, ok, "rootBlockDevice is sent as a nested value")
	assert.Equal(t, "vol-0abc", rootBlockDevice["volumeId"])
	assert.NotContains(t, attrs, "password", "attributes are redacted as on the cost path")

	assert.Nil(t, reqs[0].TargetResources[1].Attributes, "no properties sends no attributes")
}
