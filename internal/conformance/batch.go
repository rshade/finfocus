package conformance

import (
	"context"
	"fmt"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// testBatchHandling verifies that the plugin can process a sequence of GetProjectedCost
// requests without failing and returns a TestResult summarizing the outcome.
func testBatchHandling(ctx *TestContext) *TestResult {
	client, ok := ctx.PluginClient.(pbc.CostSourceServiceClient)
	if !ok {
		return &TestResult{Status: StatusError, Error: errInvalidPluginClientType}
	}

	const batchSize = 5
	successCount := 0

	// Use context with timeout from TestContext for each RPC
	rpcCtx, cancel := context.WithTimeout(context.Background(), ctx.Timeout)
	defer cancel()

	for range batchSize {
		req := &pbc.GetProjectedCostRequest{
			Resource: &pbc.ResourceDescriptor{
				Provider:     testProviderAWS,
				ResourceType: testResourceTypeEC2Instance,
				Sku:          testSKUT3Micro,
				Region:       testRegionUSEast1,
			},
		}

		_, err := client.GetProjectedCost(rpcCtx, req)
		if err == nil {
			successCount++
		}
	}

	if successCount != batchSize {
		return &TestResult{
			Status: StatusFail,
			Error: fmt.Sprintf(
				"Batch handling failed: %d/%d calls succeeded",
				successCount,
				batchSize,
			),
		}
	}

	return &TestResult{
		Status:  StatusPass,
		Details: fmt.Sprintf("Successfully processed %d sequential calls", batchSize),
	}
}
