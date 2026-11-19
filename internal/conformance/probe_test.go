package conformance

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func kubernetesProbe() ProbeResource {
	return ProbeResource{
		Provider:     "kubernetes",
		ResourceType: "kubernetes:apps/v1:Deployment",
	}
}

func TestProbeResource_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		probe   ProbeResource
		wantErr bool
	}{
		{name: "unset", probe: ProbeResource{}},
		{name: "provider and type", probe: kubernetesProbe()},
		{name: "provider only", probe: ProbeResource{Provider: "kubernetes"}, wantErr: true},
		{name: "type only", probe: ProbeResource{ResourceType: "kubernetes:apps/v1:Deployment"}, wantErr: true},
		{name: "sku without type", probe: ProbeResource{SKU: "m5.large"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.probe.Validate()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestNewSuite_RejectsPartialProbe(t *testing.T) {
	t.Parallel()

	_, err := NewSuite(SuiteConfig{
		PluginPath: "/path/to/plugin",
		Probe:      ProbeResource{Provider: "kubernetes"},
	})

	require.Error(t, err)
}

func TestResolveProbe(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configured ProbeResource
		providers  []string
		want       probeSource
	}{
		{name: "configured wins", configured: kubernetesProbe(), providers: []string{"aws"}, want: probeCustom},
		{name: "no providers listed", want: probeDefault},
		{name: "aws listed", providers: []string{"aws"}, want: probeDefault},
		{name: "aws among others", providers: []string{"kubernetes", "AWS"}, want: probeDefault},
		{name: "wildcard", providers: []string{"*"}, want: probeDefault},
		{name: "kubernetes only", providers: []string{"kubernetes"}, want: probeUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, resolveProbe(tt.configured, tt.providers).source)
		})
	}
}

func TestResolveProbe_UnavailableNamesProvidersAndFlags(t *testing.T) {
	t.Parallel()

	plan := resolveProbe(ProbeResource{}, []string{"kubernetes"})

	assert.Contains(t, plan.reason, "kubernetes")
	assert.Contains(t, plan.reason, "--provider")
	assert.Contains(t, plan.reason, "--resource-type")
}

func TestProbePlan_SkipReason(t *testing.T) {
	t.Parallel()

	defaultPlan := defaultProbePlan()
	customPlan := resolveProbe(kubernetesProbe(), nil)
	unavailable := resolveProbe(ProbeResource{}, []string{"kubernetes"})

	assert.Empty(t, defaultPlan.skipReason(ProbeSupportedType))
	assert.Empty(t, defaultPlan.skipReason(ProbeSentinelType))
	assert.Empty(t, customPlan.skipReason(ProbeSupportedType))
	assert.NotEmpty(t, customPlan.skipReason(ProbeSentinelType))
	assert.NotEmpty(t, unavailable.skipReason(ProbeSupportedType))
	assert.NotEmpty(t, unavailable.skipReason(ProbeSentinelType))
	for _, plan := range []probePlan{defaultPlan, customPlan, unavailable} {
		assert.Empty(t, plan.skipReason(ProbeNone))
	}
}

// pricesOnly is a plugin that, like a Kubernetes-only plugin, validates the
// resource type before anything else.
type pricesOnly struct {
	pbc.CostSourceServiceClient

	provider string
}

func (p pricesOnly) Name(context.Context, *pbc.NameRequest, ...grpc.CallOption) (*pbc.NameResponse, error) {
	return &pbc.NameResponse{Name: "prices-only"}, nil
}

func (p pricesOnly) GetProjectedCost(
	ctx context.Context,
	req *pbc.GetProjectedCostRequest,
	_ ...grpc.CallOption,
) (*pbc.GetProjectedCostResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	if req.GetResource().GetProvider() != p.provider {
		return nil, status.Error(codes.InvalidArgument, "resource type is not supported")
	}
	if req.GetResource().GetResourceType() != kubernetesProbe().ResourceType {
		return nil, status.Error(codes.NotFound, "unknown resource type")
	}
	return &pbc.GetProjectedCostResponse{Currency: "USD", CostPerMonth: 1}, nil
}

func resultsByName(t *testing.T, plan probePlan) map[string]*TestResult {
	t.Helper()

	suite, err := NewSuite(SuiteConfig{PluginPath: "/path/to/plugin"})
	require.NoError(t, err)

	runner := NewRunner(zerolog.Nop(), VerbosityNormal)
	runner.probe = plan
	client := pricesOnly{provider: "kubernetes"}

	results := make(map[string]*TestResult)
	for _, tc := range suite.GetTestCases() {
		results[tc.Name] = runner.RunTest(context.Background(), tc, client)
	}
	return results
}

func TestKubernetesOnlyPlugin_NoFlagsSkipsInsteadOfFailing(t *testing.T) {
	t.Parallel()

	results := resultsByName(t, resolveProbe(ProbeResource{}, []string{"kubernetes"}))

	for _, name := range []string{
		"GetProjectedCost_ValidResource",
		"Batch_Handling",
		"GetProjectedCost_PermissionDenied",
		"GetProjectedCost_InternalError",
		"GetProjectedCost_Unavailable",
	} {
		assert.Equal(t, StatusSkip, results[name].Status, name)
		assert.Contains(t, results[name].Error, "--resource-type", name)
	}
	for name, result := range results {
		assert.NotEqual(t, StatusFail, result.Status, "%s: %s", name, result.Error)
	}
}

func TestKubernetesOnlyPlugin_ProbeFlagsLetPricingProbesPass(t *testing.T) {
	t.Parallel()

	results := resultsByName(t, resolveProbe(kubernetesProbe(), nil))

	assert.Equal(t, StatusPass, results["GetProjectedCost_ValidResource"].Status)
	assert.Equal(t, StatusPass, results["Batch_Handling"].Status)
	assert.Equal(t, StatusPass, results["GetProjectedCost_InvalidResource"].Status)
	assert.Equal(t, StatusSkip, results["GetProjectedCost_PermissionDenied"].Status)
	for name, result := range results {
		assert.NotEqual(t, StatusFail, result.Status, "%s: %s", name, result.Error)
	}
}

func TestPluginProviders_InfoUnavailableKeepsBuiltInProbe(t *testing.T) {
	t.Parallel()

	providers := pluginProviders(context.Background(), pricesOnlyNoInfo{})

	assert.Empty(t, providers)
	assert.Equal(t, probeDefault, resolveProbe(ProbeResource{}, providers).source)
}

type pricesOnlyNoInfo struct {
	pbc.CostSourceServiceClient
}

func (pricesOnlyNoInfo) GetPluginInfo(
	context.Context,
	*pbc.GetPluginInfoRequest,
	...grpc.CallOption,
) (*pbc.GetPluginInfoResponse, error) {
	return nil, status.Error(codes.Unimplemented, "no plugin info")
}
