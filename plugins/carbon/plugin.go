package carbon

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// PluginName is the registry and binary name suffix.
const PluginName = "carbon"

// Plugin serves operational carbon through the projected-cost transport.
type Plugin struct {
	*pluginsdk.BasePlugin

	config Config
}

// New builds an offline carbon plugin with the supplied utilization assumption.
func New(cfg Config) *Plugin {
	return &Plugin{BasePlugin: pluginsdk.NewBasePlugin(PluginName), config: cfg}
}

// Info advertises carbon independently of monetary pricing capabilities.
func Info(version string) *pluginsdk.PluginInfo {
	return pluginsdk.NewPluginInfo(PluginName, version,
		pluginsdk.WithProviders("aws"),
		pluginsdk.WithCapabilities(pbc.PluginCapability_PLUGIN_CAPABILITY_CARBON),
		pluginsdk.WithSettings(&pbc.PluginSetting{
			Key: "utilization", Type: pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING,
			Env: EnvUtilization, DefaultValue: "0.5", Description: "Projected CPU utilization as a fraction in [0, 1].",
		}),
	)
}

// Supports declines descriptors without an explicit regional or instance model.
func (p *Plugin) Supports(ctx context.Context, req *pbc.SupportsRequest) (*pbc.SupportsResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	response := &pbc.SupportsResponse{Supported: true}
	if _, err := EstimateResource(req.GetResource(), p.config); err != nil {
		response.Supported = false
		response.Reason = err.Error()
	}
	return response, nil
}

// GetProjectedCost returns a monthly carbon total; monetary fields are not prices.
// Immediate expiry prevents a direct host from caching process-local assumptions.
func (p *Plugin) GetProjectedCost(
	ctx context.Context,
	req *pbc.GetProjectedCostRequest,
) (*pbc.GetProjectedCostResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, status.FromContextError(err).Err()
	}
	if req.GetResource() == nil {
		return nil, status.Error(codes.InvalidArgument, "resource is required")
	}
	estimate, err := EstimateResource(req.GetResource(), p.config)
	if err != nil {
		return nil, status.Error(codes.FailedPrecondition, err.Error())
	}
	return &pbc.GetProjectedCostResponse{
		Currency:      "USD",
		BillingDetail: estimate.Note,
		ExpiresAt:     timestamppb.Now(),
		ImpactMetrics: []*pbc.ImpactMetric{
			{Kind: pbc.MetricKind_METRIC_KIND_CARBON_FOOTPRINT, Value: estimate.Grams, Unit: "gCO2e"},
		},
	}, nil
}
