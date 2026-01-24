package conformance

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// testGetProjectedCostPermissionDenied verifies that GetProjectedCost returns a PermissionDenied gRPC error when requesting a forbidden resource.
// ctx provides the test environment and configuration for the request.
// It returns a TestResult indicating whether the observed error code matched the expected PermissionDenied code.
func testGetProjectedCostPermissionDenied(ctx *TestContext) *TestResult {
	return testGetProjectedCostErrorCode(ctx, "forbidden:resource", codes.PermissionDenied)
}

// testGetProjectedCostInternal verifies that GetProjectedCost returns a gRPC Internal error for a resource that triggers an internal server error.
// ctx provides the test environment, including the plugin client and timeout settings.
// It returns a *TestResult describing whether the observed error code matched the expected Internal code.
func testGetProjectedCostInternal(ctx *TestContext) *TestResult {
	return testGetProjectedCostErrorCode(ctx, "error:internal", codes.Internal)
}

// testGetProjectedCostUnavailable verifies that GetProjectedCost returns a gRPC
// Unavailable error for a resource that signals service unavailability.
//
// ctx is the test context providing the PluginClient and timeout settings.
// The function returns a *TestResult indicating pass when the received gRPC
// status code matches codes.Unavailable, or a failing result with details
// when the client is invalid, no error is returned, the error is not a gRPC
// status, or the status code differs.
func testGetProjectedCostUnavailable(ctx *TestContext) *TestResult {
	return testGetProjectedCostErrorCode(ctx, "error:unavailable", codes.Unavailable)
}

// testGetProjectedCostErrorCode calls GetProjectedCost with a ResourceDescriptor using the provided resourceType and verifies that the resulting gRPC status code equals expectedCode.
// It returns a TestResult with StatusPass when the service returns the expected code, StatusFail when the RPC succeeds unexpectedly, returns a non-gRPC error, or returns a different gRPC code, and StatusError when the test cannot run due to an invalid plugin client type.
// Parameters:
//   - ctx: test context containing the PluginClient and Timeout used for the RPC.
//   - resourceType: resource type string to send in the request's ResourceDescriptor.
//   - expectedCode: the gRPC codes.Code expected from the GetProjectedCost call.
//
// Returns:
//   - *TestResult describing the outcome and any relevant error or detail message.
func testGetProjectedCostErrorCode(ctx *TestContext, resourceType string, expectedCode codes.Code) *TestResult {
	client, ok := ctx.PluginClient.(pbc.CostSourceServiceClient)
	if !ok {
		return &TestResult{Status: StatusError, Error: "invalid plugin client type"}
	}

	req := &pbc.GetProjectedCostRequest{
		Resource: &pbc.ResourceDescriptor{
			Provider:     "aws",
			ResourceType: resourceType,
			Sku:          "any",
			Region:       "us-east-1",
		},
	}

	rpcCtx, cancel := context.WithTimeout(context.Background(), ctx.Timeout)
	defer cancel()

	_, err := client.GetProjectedCost(rpcCtx, req)
	if err == nil {
		return &TestResult{
			Status: StatusFail,
			Error:  fmt.Sprintf("expected error %s for resource %s, but got success", expectedCode, resourceType),
		}
	}

	st, ok := status.FromError(err)
	if !ok {
		return &TestResult{
			Status: StatusFail,
			Error:  fmt.Sprintf("expected gRPC status error, got: %v", err),
		}
	}

	if st.Code() != expectedCode {
		return &TestResult{
			Status: StatusFail,
			Error:  fmt.Sprintf("expected %s, got: %s", expectedCode, st.Code()),
		}
	}

	return &TestResult{
		Status:  StatusPass,
		Details: fmt.Sprintf("Received expected error: %s", st.Code()),
	}
}
