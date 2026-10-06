// Package fixedcost is an e2e actual-cost plugin. GetActualCost returns the
// positive amount in FINFOCUS_FIXEDCOST_TOTAL. Projected cost stays unsupported.
package fixedcost

import (
	"context"
	"math"
	"os"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// PluginName is the registry and binary name suffix.
const PluginName = "fixedcost"

// totalEnv is the actual-cost amount for every resource.
const totalEnv = "FINFOCUS_FIXEDCOST_TOTAL"

// Plugin serves actual cost only. Embedding BasePlugin keeps projected cost,
// pricing spec, and estimate on their default unsupported paths.
type Plugin struct {
	*pluginsdk.BasePlugin
}

// New builds the plugin.
func New() *Plugin {
	return &Plugin{BasePlugin: pluginsdk.NewBasePlugin(PluginName)}
}

// Info advertises actual cost and nothing else. inferCapabilities would also
// advertise pricing because BasePlugin implements the cost methods.
func Info(version string) *pluginsdk.PluginInfo {
	return pluginsdk.NewPluginInfo(PluginName, version,
		pluginsdk.WithCapabilities(pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS),
	)
}

// GetActualCost returns one result whose cost is FINFOCUS_FIXEDCOST_TOTAL.
// A missing, non-numeric, or non-positive amount is an error. Zero is not a price.
func (p *Plugin) GetActualCost(
	_ context.Context, req *pbc.GetActualCostRequest,
) (*pbc.GetActualCostResponse, error) {
	if req == nil || req.GetResourceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "resource_id is required")
	}
	raw := os.Getenv(totalEnv)
	if raw == "" {
		return nil, status.Error(codes.FailedPrecondition, totalEnv+" is unset")
	}
	total, err := strconv.ParseFloat(raw, 64)
	if err != nil || total <= 0 || math.IsNaN(total) || math.IsInf(total, 0) {
		return nil, status.Errorf(codes.InvalidArgument, "%s %q is not a positive amount", totalEnv, raw)
	}
	return &pbc.GetActualCostResponse{Results: []*pbc.ActualCostResult{{
		Cost:        total,
		Source:      PluginName,
		FocusRecord: &pbc.FocusCostRecord{BillingCurrency: "USD"},
	}}}, nil
}
