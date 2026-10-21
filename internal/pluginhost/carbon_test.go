package pluginhost_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestHistoricalCarbonCapabilityAndDisabled(t *testing.T) {
	t.Parallel()
	caps := pluginhost.ConvertCapabilities(
		[]pbc.PluginCapability{pbc.PluginCapability_PLUGIN_CAPABILITY_HISTORICAL_CARBON},
	)
	assert.Equal(t, []string{"historical_carbon"}, caps)
	assert.NotContains(t, caps, "carbon")
	client := &pluginhost.Client{}
	_, err := client.GetCarbonEmissions(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FINFOCUS_AWS_CARBON_ENABLED=true")
}
