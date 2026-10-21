package pluginhost

import (
	"context"
	"errors"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// CapabilityHistoricalCarbon is explicit and separate from projected carbon.
const CapabilityHistoricalCarbon = "historical_carbon"

// GetCarbonEmissions invokes the typed historical RPC without expanding the
// legacy CostSourceClient interface used by cost-only plugins and mocks.
func (c *Client) GetCarbonEmissions(
	ctx context.Context,
	req *pbc.GetCarbonEmissionsRequest,
) (*pbc.GetCarbonEmissionsResponse, error) {
	if !c.HasCapability(CapabilityHistoricalCarbon) {
		return nil, errors.New(
			"aws-ce historical carbon reporting unavailable: enable FINFOCUS_AWS_CARBON_ENABLED=true (or plugins.aws-ce.carbon_enabled=true in config) and restart; requires a plugin with historical_carbon support",
		)
	}
	if c.Conn == nil {
		return nil, errors.New("historical carbon plugin connection unavailable")
	}
	return pbc.NewCostSourceServiceClient(c.Conn).GetCarbonEmissions(ctx, req)
}
