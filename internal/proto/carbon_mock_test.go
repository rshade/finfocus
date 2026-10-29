package proto

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// GetCarbonEmissions keeps the cost-only generated-client mock compatible with
// the additive historical RPC. Core CostSourceClient deliberately stays unchanged.
func (*mockPbcCostSourceServiceClient) GetCarbonEmissions(
	context.Context,
	*pbc.GetCarbonEmissionsRequest,
	...grpc.CallOption,
) (*pbc.GetCarbonEmissionsResponse, error) {
	return nil, status.Error(codes.Unimplemented, "historical carbon unavailable in cost-only mock")
}
