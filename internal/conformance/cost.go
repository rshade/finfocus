package conformance

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// testGetProjectedCostValid verifies that GetProjectedCost returns a projected monthly cost and currency for a valid resource descriptor.
//
// The ctx parameter supplies the test context and plugin client used to call the service.
// It returns a *TestResult with StatusPass and a Details message containing the estimated monthly cost and currency when successful.
// On failure it returns a *TestResult with StatusFail describing the RPC error or a missing currency; if the plugin client has an unexpected type it returns StatusError.
func testGetProjectedCostValid(ctx *TestContext) *TestResult {
	client, ok := ctx.PluginClient.(pbc.CostSourceServiceClient)
	if !ok {
		return &TestResult{Status: StatusError, Error: "invalid plugin client type"}
	}

	// Create a standard test resource (e.g., t3.micro EC2)
	req := &pbc.GetProjectedCostRequest{
		Resource: &pbc.ResourceDescriptor{
			Provider:     "aws",
			ResourceType: "aws:ec2/instance:Instance",
			Sku:          "t3.micro",
			Region:       "us-east-1",
		},
	}

	// Use context with timeout from TestContext
	rpcCtx, cancel := context.WithTimeout(context.Background(), ctx.Timeout)
	defer cancel()

	resp, err := client.GetProjectedCost(rpcCtx, req)
	if err != nil {
		return &TestResult{
			Status: StatusFail,
			Error:  fmt.Sprintf("GetProjectedCost() RPC failed: %v", err),
		}
	}

	if resp.GetCurrency() == "" {
		return &TestResult{Status: StatusFail, Error: "response missing currency"}
	}

	return &TestResult{
		Status:  StatusPass,
		Details: fmt.Sprintf("Estimated cost: %f %s", resp.GetCostPerMonth(), resp.GetCurrency()),
	}
}

// testGetProjectedCostInvalid exercises GetProjectedCost with an invalid/unsupported resource
// descriptor and reports whether the service returns an appropriate error status.
//
// The ctx parameter provides the test context and plugin client. The function returns a
// *TestResult describing the outcome: StatusError if the plugin client is the wrong type,
// StatusFail if the RPC unexpectedly succeeds, is not a gRPC status error, or returns a
// code other than NotFound or InvalidArgument, and StatusPass when the service returns
// either NotFound or InvalidArgument.
func testGetProjectedCostInvalid(ctx *TestContext) *TestResult {
	client, ok := ctx.PluginClient.(pbc.CostSourceServiceClient)
	if !ok {
		return &TestResult{Status: StatusError, Error: "invalid plugin client type"}
	}

	// Create an invalid/unsupported resource
	req := &pbc.GetProjectedCostRequest{
		Resource: &pbc.ResourceDescriptor{
			Provider:     "aws",
			ResourceType: "invalid:resource",
			Sku:          "non-existent",
			Region:       "us-east-1",
		},
	}

	// Use context with timeout from TestContext
	rpcCtx, cancel := context.WithTimeout(context.Background(), ctx.Timeout)
	defer cancel()

	_, err := client.GetProjectedCost(rpcCtx, req)
	if err == nil {
		return &TestResult{
			Status: StatusFail,
			Error:  "expected error for invalid resource, but got success",
		}
	}

	// Check if it's a gRPC status error
	st, ok := status.FromError(err)
	if !ok {
		return &TestResult{
			Status: StatusFail,
			Error:  fmt.Sprintf("expected gRPC status error, got: %v", err),
		}
	}

	if st.Code() != codes.NotFound && st.Code() != codes.InvalidArgument {
		return &TestResult{
			Status: StatusFail,
			Error:  fmt.Sprintf("expected NotFound or InvalidArgument, got: %s", st.Code()),
		}
	}

	return &TestResult{
		Status:  StatusPass,
		Details: fmt.Sprintf("Received expected error: %s", st.Code()),
	}
}
