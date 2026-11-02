package conformance

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// TestCarbonPluginConformance builds and executes a binary, so it is sequential.
func TestCarbonPluginConformance(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the real carbon plugin")
	}
	t.Setenv("FINFOCUS_CARBON_UTILIZATION", "0.5")
	binary := filepath.Join(t.TempDir(), "finfocus-plugin-carbon")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "go", "-C", "../../plugins/carbon", "build", "-o", binary, "./cmd")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	conn, closePlugin, err := pluginhost.NewProcessLauncher().Start(ctx, binary)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, closePlugin()) })
	client := pbc.NewCostSourceServiceClient(conn)
	tc := &TestContext{PluginClient: client, Timeout: 10 * time.Second}
	for _, result := range []*TestResult{
		testNameReturnsIdentifier(tc), testGetProjectedCostValid(tc),
		testContextCancellation(tc), testTimeoutRespected(tc),
	} {
		assert.Equal(t, StatusPass, result.Status, result.Error)
	}
	resp, err := client.GetProjectedCost(ctx, &pbc.GetProjectedCostRequest{Resource: &pbc.ResourceDescriptor{
		Provider: "aws", ResourceType: "aws:ec2/instance:Instance", Sku: "t3.medium", Region: "us-east-1",
	}})
	require.NoError(t, err)
	require.NoError(t, pluginsdk.ValidateGetProjectedCostResponse(resp))
	require.Len(t, resp.GetImpactMetrics(), 1)
	assert.Equal(t, pbc.MetricKind_METRIC_KIND_CARBON_FOOTPRINT, resp.GetImpactMetrics()[0].GetKind())
	assert.InDelta(t, 1437.00152000285625, resp.GetImpactMetrics()[0].GetValue(), 1e-8)
}
