package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToProtoOmitsTerraformUnknownProperties(t *testing.T) {
	t.Parallel()

	props := map[string]any{
		"instanceType":                "m5.large",
		"terraform:unknownProperties": []any{"instanceType", "subnetId"},
	}
	tags := ConvertToProto(props)
	assert.Equal(t, "m5.large", tags["instanceType"])
	assert.NotContains(t, tags, "terraform:unknownProperties")
	for key := range tags {
		assert.NotContains(t, key, "unknownProperties")
	}

	attrs := BuildAttributes(t.Context(), props)
	require.NotNil(t, attrs)
	_, kept := attrs.GetFields()["terraform:unknownProperties"]
	assert.True(t, kept, "BuildAttributes keeps the unknown list for the plugin")

	other := map[string]any{
		"instanceType":                "m5.large",
		"terraform:unknownProperties": []any{"subnetId"},
	}
	assert.NotEqual(t, attributesCacheSuffix(props), attributesCacheSuffix(other))
}
