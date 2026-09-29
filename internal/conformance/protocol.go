package conformance

import (
	"context"
	"fmt"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// testNameReturnsIdentifier verifies that the plugin's Name RPC returns a non-empty identifier.
// It returns a TestResult with StatusPass and Details containing the plugin name when a non-empty
// name is returned. It returns StatusFail if the Name RPC returns an error or an empty name, and
// StatusError if the provided PluginClient does not implement pbc.CostSourceServiceClient.
func testNameReturnsIdentifier(ctx *TestContext) *TestResult {
	client, ok := ctx.PluginClient.(pbc.CostSourceServiceClient)
	if !ok {
		return &TestResult{Status: StatusError, Error: errInvalidPluginClientType}
	}

	// Use context with timeout from TestContext for suite-level timeout control
	rpcCtx, cancel := context.WithTimeout(context.Background(), ctx.Timeout)
	defer cancel()

	resp, err := client.Name(rpcCtx, &pbc.NameRequest{})
	if err != nil {
		return &TestResult{Status: StatusFail, Error: fmt.Sprintf("Name() RPC failed: %v", err)}
	}

	if resp.GetName() == "" {
		return &TestResult{Status: StatusFail, Error: "Name() returned empty string"}
	}

	return &TestResult{Status: StatusPass, Details: fmt.Sprintf("Plugin name: %s", resp.GetName())}
}

// testNameReturnsProtocolVersion verifies basic protocol compatibility by calling the Name RPC.
// This is a placeholder for future protocol version negotiation when the proto adds version fields.
// Returns StatusPass with Details containing the plugin name when the Name RPC succeeds,
// StatusFail if the RPC fails, or StatusError if the plugin client type is invalid.
func testNameReturnsProtocolVersion(ctx *TestContext) *TestResult {
	client, ok := ctx.PluginClient.(pbc.CostSourceServiceClient)
	if !ok {
		return &TestResult{Status: StatusError, Error: errInvalidPluginClientType}
	}

	// Use context with timeout from TestContext for suite-level timeout control
	rpcCtx, cancel := context.WithTimeout(context.Background(), ctx.Timeout)
	defer cancel()

	// In current proto, NameRequest doesn't return protocol version directly,
	// but some plugins might include it in metadata or we might have a dedicated Version RPC.
	// For now, we'll check if Name succeeds and maybe check a version field if added.
	resp, err := client.Name(rpcCtx, &pbc.NameRequest{})
	if err != nil {
		return &TestResult{Status: StatusFail, Error: fmt.Sprintf("Name() RPC failed: %v", err)}
	}

	// Assuming version might be in metadata or a separate field in future.
	// For now, passing if RPC succeeds as a placeholder for protocol compatibility check.
	return &TestResult{
		Status:  StatusPass,
		Details: fmt.Sprintf("Protocol response received for plugin: %s", resp.GetName()),
	}
}
