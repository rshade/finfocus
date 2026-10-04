package proto

import (
	"context"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/logging"
)

// actualCostDescriptor builds GetActualCostRequest.resource from req.Resource
// with PrepareProjectedDescriptor, as finfocus-spec requires. The descriptor is
// optional enrichment: tags still carry the pre-v0.7.4 shape, so a descriptor
// that could get the request rejected is dropped rather than failing the
// resource. It returns nil when there is no resource, the request names more
// than one resource, provider or resource type is empty (the contract
// validator requires both), or the descriptor exceeds the SDK limits.
func actualCostDescriptor(ctx context.Context, req *GetActualCostRequest) *pbc.ResourceDescriptor {
	res := req.Resource
	if res == nil || len(req.ResourceIDs) != 1 || res.Provider == "" || res.Type == "" {
		return nil
	}

	descriptor := PrepareProjectedDescriptor(ctx, res.ID, res.Provider, res.Type, res.Properties, res.Attributes)
	if err := pluginsdk.ValidateResourceDescriptor(descriptor); err != nil {
		logging.FromContext(ctx).Warn().
			Ctx(ctx).
			Str("component", "adapter").
			Str("operation", "GetActualCost").
			Str("resource_type", res.Type).
			Err(err).
			Msg("resource descriptor exceeds plugin limits; sending actual cost request without it")
		return nil
	}
	return descriptor
}
