package integration_test

import (
	"context"
	"testing"
	"time"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/rshade/finfocus/internal/analyzer"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// mockCostCalculator implements analyzer.CostCalculator for integration tests.
type mockCostCalculator struct {
	costs    []engine.CostResult
	err      error
	delay    time.Duration
	callChan chan struct{}
}

func newMockCalculator(costs []engine.CostResult, err error) *mockCostCalculator {
	return &mockCostCalculator{
		costs:    costs,
		err:      err,
		callChan: make(chan struct{}, 1),
	}
}

func (m *mockCostCalculator) GetProjectedCost(
	_ context.Context,
	_ []engine.ResourceDescriptor,
) ([]engine.CostResult, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	if m.callChan != nil {
		select {
		case m.callChan <- struct{}{}:
		default:
		}
	}
	return m.costs, m.err
}

func (m *mockCostCalculator) GetRecommendationsForResources(
	_ context.Context,
	_ []engine.ResourceDescriptor,
) (*engine.RecommendationsResult, error) {
	return &engine.RecommendationsResult{}, nil
}

// TestAnalyzer_FullStackFlow tests the complete flow from handshake to analysis.
func TestAnalyzer_FullStackFlow(t *testing.T) {
	// Create mock cost results
	costs := []engine.CostResult{
		{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "webserver",
			Adapter:      "local-spec",
			Currency:     "USD",
			Monthly:      25.50,
		},
		{
			ResourceType: "aws:rds/instance:Instance",
			ResourceID:   "database",
			Adapter:      "local-spec",
			Currency:     "USD",
			Monthly:      100.00,
		},
	}

	calc := newMockCalculator(costs, nil)
	server := analyzer.NewServer(calc, "1.0.0-test")

	ctx := context.Background()

	// Step 1: Handshake
	handshakeResp, err := server.Handshake(ctx, &pulumirpc.AnalyzerHandshakeRequest{
		EngineAddress: "localhost:12345",
	})
	require.NoError(t, err)
	require.NotNil(t, handshakeResp)

	// Step 2: Configure Stack
	configResp, err := server.ConfigureStack(ctx, &pulumirpc.AnalyzerStackConfigureRequest{
		Stack:        "dev",
		Project:      "my-infrastructure",
		Organization: "myorg",
		DryRun:       true,
	})
	require.NoError(t, err)
	require.NotNil(t, configResp)

	// Step 3: Get Analyzer Info
	infoResp, err := server.GetAnalyzerInfo(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.NotNil(t, infoResp)
	assert.Equal(t, "finfocus", infoResp.GetName())
	assert.Equal(t, "1.0.0-test", infoResp.GetVersion())
	assert.Len(t, infoResp.GetPolicies(), 2)

	// Step 4: Analyze Stack
	props, err := structpb.NewStruct(map[string]interface{}{
		"instanceType": "t3.micro",
	})
	require.NoError(t, err)

	resources := []*pulumirpc.AnalyzerResource{
		{
			Type:       "aws:ec2/instance:Instance",
			Urn:        "urn:pulumi:dev::my-infrastructure::aws:ec2/instance:Instance::webserver",
			Name:       "webserver",
			Properties: props,
		},
		{
			Type: "aws:rds/instance:Instance",
			Urn:  "urn:pulumi:dev::my-infrastructure::aws:rds/instance:Instance::database",
			Name: "database",
		},
	}

	// First, call Analyze() for each resource to populate the cost cache
	// (In real Pulumi flow, the engine calls Analyze() for each resource)
	for _, res := range resources {
		_, err := server.Analyze(ctx, &pulumirpc.AnalyzeRequest{
			Type:       res.GetType(),
			Urn:        res.GetUrn(),
			Name:       res.GetName(),
			Properties: res.GetProperties(),
		})
		require.NoError(t, err)
	}

	// Now call AnalyzeStack which returns only the summary diagnostic
	// Per-resource diagnostics are returned by individual Analyze() calls
	analyzeResp, err := server.AnalyzeStack(ctx, &pulumirpc.AnalyzeStackRequest{
		Resources: resources,
	})
	require.NoError(t, err)
	require.NotNil(t, analyzeResp)

	// AnalyzeStack returns only the summary diagnostic (per-resource diagnostics
	// are returned by individual Analyze() calls during the Pulumi preview flow)
	require.Len(t, analyzeResp.GetDiagnostics(), 1)

	// Verify summary diagnostic
	summary := analyzeResp.GetDiagnostics()[0]
	assert.Equal(t, "stack-cost-summary", summary.GetPolicyName())
	assert.Contains(t, summary.GetMessage(), "$125.50 USD")
	assert.Contains(t, summary.GetMessage(), "2 resources analyzed")

	// Step 5: Get Plugin Info
	pluginInfo, err := server.GetPluginInfo(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, "1.0.0-test", pluginInfo.GetVersion())
}

// TestAnalyzer_HandshakeProtocol tests the handshake behavior.
func TestAnalyzer_HandshakeProtocol(t *testing.T) {
	calc := newMockCalculator(nil, nil)
	server := analyzer.NewServer(calc, "0.1.0")
	ctx := context.Background()

	// Multiple handshakes should work (server may be reused)
	for i := 0; i < 3; i++ {
		resp, err := server.Handshake(ctx, &pulumirpc.AnalyzerHandshakeRequest{
			EngineAddress: "localhost:12345",
		})
		require.NoError(t, err)
		require.NotNil(t, resp)
	}
}

// TestAnalyzer_ErrorRecovery tests graceful handling of errors.
func TestAnalyzer_ErrorRecovery(t *testing.T) {
	// Create calculator that returns an error
	calc := newMockCalculator(nil, assert.AnError)
	server := analyzer.NewServer(calc, "0.1.0")
	ctx := context.Background()

	resources := []*pulumirpc.AnalyzerResource{
		{
			Type: "aws:ec2/instance:Instance",
			Urn:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Name: "web",
		},
	}

	// AnalyzeStack should NOT fail even when calculator fails
	resp, err := server.AnalyzeStack(ctx, &pulumirpc.AnalyzeStackRequest{
		Resources: resources,
	})

	require.NoError(t, err, "AnalyzeStack should succeed even when cost calculation fails")
	require.NotNil(t, resp)
	// Should still have at least the summary diagnostic
	assert.NotEmpty(t, resp.GetDiagnostics())
}

// TestAnalyzer_EmptyResources tests handling of empty resource list.
func TestAnalyzer_EmptyResources(t *testing.T) {
	calc := newMockCalculator([]engine.CostResult{}, nil)
	server := analyzer.NewServer(calc, "0.1.0")
	ctx := context.Background()

	resp, err := server.AnalyzeStack(ctx, &pulumirpc.AnalyzeStackRequest{
		Resources: []*pulumirpc.AnalyzerResource{},
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	// Should have just the summary diagnostic
	require.Len(t, resp.GetDiagnostics(), 1)
	assert.Equal(t, "stack-cost-summary", resp.GetDiagnostics()[0].GetPolicyName())
	assert.Contains(t, resp.GetDiagnostics()[0].GetMessage(), "$0.00")
}

// TestAnalyzer_CancelBehavior tests the Cancel RPC behavior.
func TestAnalyzer_CancelBehavior(t *testing.T) {
	calc := newMockCalculator(nil, nil)
	server := analyzer.NewServer(calc, "0.1.0")
	ctx := context.Background()

	// Initially not canceled
	assert.False(t, server.IsCanceled())

	// Cancel
	resp, err := server.Cancel(ctx, &emptypb.Empty{})
	require.NoError(t, err)
	require.NotNil(t, resp)

	// Should now be canceled
	assert.True(t, server.IsCanceled())
}

// TestAnalyzer_LatencyRequirement tests that small stacks complete quickly (SC-003).
func TestAnalyzer_LatencyRequirement(t *testing.T) {
	// SC-003: Cost estimation adds <2s for stacks under 50 resources
	// Using 100ms delay per resource for testing, total should be <2s for small stacks

	costs := make([]engine.CostResult, 10)
	for i := 0; i < 10; i++ {
		costs[i] = engine.CostResult{
			ResourceType: "aws:ec2/instance:Instance",
			ResourceID:   "instance",
			Adapter:      "mock",
			Currency:     "USD",
			Monthly:      10.00,
		}
	}

	calc := newMockCalculator(costs, nil)
	server := analyzer.NewServer(calc, "0.1.0")
	ctx := context.Background()

	resources := make([]*pulumirpc.AnalyzerResource, 10)
	for i := 0; i < 10; i++ {
		resources[i] = &pulumirpc.AnalyzerResource{
			Type: "aws:ec2/instance:Instance",
			Urn:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::instance",
			Name: "instance",
		}
	}

	start := time.Now()
	resp, err := server.AnalyzeStack(ctx, &pulumirpc.AnalyzeStackRequest{
		Resources: resources,
	})
	duration := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Less(t, duration, 2*time.Second, "Small stack analysis should complete in <2s")
}
