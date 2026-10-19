package carbon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestInfo(t *testing.T) {
	t.Parallel()
	info := Info("v0.1.0")
	require.NoError(t, info.Validate())
	assert.Equal(t, "carbon", info.Name)
	assert.Equal(t, []pbc.PluginCapability{pbc.PluginCapability_PLUGIN_CAPABILITY_CARBON}, info.Capabilities)
	require.Len(t, info.Settings, 1)
	assert.Equal(t, EnvUtilization, info.Settings[0].GetEnv())
	assert.Equal(t, "0.5", info.Settings[0].GetDefaultValue())
}

func TestPluginProjected(t *testing.T) {
	t.Parallel()
	plugin := New(Config{Utilization: 0.5})
	req := &pbc.GetProjectedCostRequest{Resource: testResource("t3.medium", "us-east-1")}
	before := time.Now()
	resp, err := plugin.GetProjectedCost(context.Background(), req)
	require.NoError(t, err)
	require.NoError(t, pluginsdk.ValidateGetProjectedCostResponse(resp))
	assert.Zero(t, resp.GetCostPerMonth())
	require.Len(t, resp.GetImpactMetrics(), 1)
	metric := resp.GetImpactMetrics()[0]
	assert.Equal(t, pbc.MetricKind_METRIC_KIND_CARBON_FOOTPRINT, metric.GetKind())
	assert.Equal(t, "gCO2e", metric.GetUnit())
	assert.InDelta(t, 1437.00152000285625, metric.GetValue(), 1e-8)
	assert.Contains(t, resp.GetBillingDetail(), DatasetRevision)
	require.NotNil(t, resp.GetExpiresAt())
	assert.False(t, resp.GetExpiresAt().AsTime().Before(before))
	assert.False(t, resp.GetExpiresAt().AsTime().After(time.Now()))
}

func TestPluginValidation(t *testing.T) {
	t.Parallel()
	plugin := New(Config{Utilization: 0.5})
	for _, tt := range []struct {
		name string
		req  *pbc.GetProjectedCostRequest
		code codes.Code
	}{
		{"nil", nil, codes.InvalidArgument}, {"missing", &pbc.GetProjectedCostRequest{}, codes.InvalidArgument},
		{"unsupported", &pbc.GetProjectedCostRequest{Resource: testResource("t3.unknown", "us-east-1")}, codes.FailedPrecondition},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := plugin.GetProjectedCost(context.Background(), tt.req)
			require.Error(t, err)
			assert.Equal(t, tt.code, status.Code(err))
		})
	}
	supported, err := plugin.Supports(
		context.Background(),
		&pbc.SupportsRequest{Resource: testResource("t3.medium", "us-east-1")},
	)
	require.NoError(t, err)
	assert.True(t, supported.GetSupported())
	declined, err := plugin.Supports(context.Background(), nil)
	require.NoError(t, err)
	assert.False(t, declined.GetSupported())
	assert.NotEmpty(t, declined.GetReason())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = plugin.GetProjectedCost(
		ctx,
		&pbc.GetProjectedCostRequest{Resource: testResource("t3.medium", "us-east-1")},
	)
	assert.Equal(t, codes.Canceled, status.Code(err))
	_, err = plugin.Supports(ctx, nil)
	assert.Equal(t, codes.Canceled, status.Code(err))
}
