package analyzer

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/router"
)

// mockCostCalculator implements the CostCalculator interface for testing.
type mockCostCalculator struct {
	results    []engine.CostResult
	recResults *engine.RecommendationsResult
	err        error
	recErr     error
}

func (m *mockCostCalculator) GetRecommendationsForResources(
	_ context.Context,
	resources []engine.ResourceDescriptor,
) (*engine.RecommendationsResult, error) {
	if m.recErr != nil {
		return nil, m.recErr
	}
	if m.err != nil {
		return nil, m.err
	}
	if m.recResults == nil {
		return &engine.RecommendationsResult{}, nil
	}
	return m.recResults, nil
}

func (m *mockCostCalculator) GetProjectedCost(
	_ context.Context,
	resources []engine.ResourceDescriptor,
) ([]engine.CostResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	// Return only results that match the requested resources
	var matched []engine.CostResult
	for _, resource := range resources {
		for _, result := range m.results {
			if result.ResourceID == resource.ID {
				matched = append(matched, result)
				break
			}
		}
	}
	return matched, nil
}

func TestNewServer(t *testing.T) {
	calc := &mockCostCalculator{}
	server := NewServer(calc, "1.0.0")

	require.NotNil(t, server)
	assert.Equal(t, "1.0.0", server.version)
}

// T006: Test WithConfig builder method preserving backward compatibility.
func TestServer_WithConfig(t *testing.T) {
	calc := &mockCostCalculator{}

	t.Run("nil config preserves backward compatibility", func(t *testing.T) {
		server := NewServer(calc, "1.0.0").WithConfig(nil)
		require.NotNil(t, server)
		assert.Nil(t, server.cfg)
		assert.Equal(t, "1.0.0", server.version)

		// Should work normally without config
		resp, err := server.GetAnalyzerInfo(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("config stored on server", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Analyzer.MaxMonthlyCost = 5000
		cfg.Analyzer.Enforcement = "mandatory"

		server := NewServer(calc, "1.0.0").WithConfig(cfg)
		require.NotNil(t, server)
		require.NotNil(t, server.cfg)
		assert.Equal(t, float64(5000), server.cfg.Analyzer.MaxMonthlyCost)
		assert.Equal(t, "mandatory", server.cfg.Analyzer.Enforcement)
	})

	t.Run("chaining works", func(t *testing.T) {
		cfg := &config.Config{}
		server := NewServer(calc, "2.0.0").WithConfig(cfg)
		require.NotNil(t, server)
		assert.Equal(t, "2.0.0", server.version)
		assert.NotNil(t, server.cfg)
	})
}

func TestServer_AnalyzeStack(t *testing.T) {
	tests := []struct {
		name                 string
		resources            []*pulumirpc.AnalyzerResource
		calcResults          []engine.CostResult
		calcErr              error
		wantAnalyzeDiagCount int      // Diagnostics from Analyze() calls
		wantStackDiagCount   int      // Diagnostics from AnalyzeStack() (should be 1 - summary only)
		wantAnalyzeContains  []string // Expected content in Analyze() diagnostics
		wantStackContains    []string // Expected content in AnalyzeStack() summary
	}{
		{
			name: "single resource with cost",
			resources: []*pulumirpc.AnalyzerResource{
				{
					Type: "aws:ec2/instance:Instance",
					Urn:  "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
					Name: "webserver",
				},
			},
			calcResults: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance:Instance",
					ResourceID:   "webserver",
					Adapter:      "local-spec",
					Currency:     "USD",
					Monthly:      8.45,
					Hourly:       0.0116,
				},
			},
			wantAnalyzeDiagCount: 1,
			wantStackDiagCount:   1, // Summary only
			wantAnalyzeContains:  []string{"$8.45 USD"},
			wantStackContains: []string{
				"Total Estimated Monthly Cost",
				"$8.45 USD",
				"1 resources analyzed",
			},
		},
		{
			name: "multiple resources",
			resources: []*pulumirpc.AnalyzerResource{
				{
					Type: "aws:ec2/instance:Instance",
					Urn:  "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::web1",
					Name: "web1",
				},
				{
					Type: "aws:rds/instance:Instance",
					Urn:  "urn:pulumi:dev::myapp::aws:rds/instance:Instance::db1",
					Name: "db1",
				},
			},
			calcResults: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance:Instance",
					ResourceID:   "web1",
					Adapter:      "local-spec",
					Currency:     "USD",
					Monthly:      8.45,
				},
				{
					ResourceType: "aws:rds/instance:Instance",
					ResourceID:   "db1",
					Adapter:      "local-spec",
					Currency:     "USD",
					Monthly:      50.00,
				},
			},
			wantAnalyzeDiagCount: 1, // Per resource
			wantStackDiagCount:   1, // Summary only
			wantAnalyzeContains:  []string{"$8.45 USD", "$50.00 USD"},
			wantStackContains:    []string{"$58.45 USD", "2 resources analyzed"},
		},
		{
			name:                 "empty resources",
			resources:            []*pulumirpc.AnalyzerResource{},
			calcResults:          []engine.CostResult{},
			wantAnalyzeDiagCount: 0,
			wantStackDiagCount:   1, // Summary only
			wantAnalyzeContains:  []string{},
			wantStackContains:    []string{"$0.00 USD", "0 resources analyzed"},
		},
		{
			name: "resource with zero cost (unsupported)",
			resources: []*pulumirpc.AnalyzerResource{
				{
					Type: "custom:component:Widget",
					Urn:  "urn:pulumi:dev::myapp::custom:component:Widget::widget1",
					Name: "widget1",
				},
			},
			calcResults: []engine.CostResult{
				{
					ResourceType: "custom:component:Widget",
					ResourceID:   "widget1",
					Adapter:      "none",
					Currency:     "USD",
					Monthly:      0,
					Notes:        "Unsupported resource type",
				},
			},
			wantAnalyzeDiagCount: 1,
			wantStackDiagCount:   1, // Summary only
			wantAnalyzeContains:  []string{"Unsupported resource type"},
			wantStackContains: []string{
				"0 resources analyzed",
			}, // Zero cost resources don't count as "analyzed"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := &mockCostCalculator{
				results: tt.calcResults,
				err:     tt.calcErr,
			}
			server := NewServer(calc, "0.1.0")

			// Simulate the real workflow: Analyze() is called for each resource before AnalyzeStack()
			// This populates the cost cache used by AnalyzeStack for the summary
			var allAnalyzeMessages []string
			for _, resource := range tt.resources {
				analyzeReq := &pulumirpc.AnalyzeRequest{
					Type:       resource.GetType(),
					Urn:        resource.GetUrn(),
					Properties: resource.GetProperties(),
				}
				analyzeResp, err := server.Analyze(context.Background(), analyzeReq)
				require.NoError(t, err)
				require.Len(t, analyzeResp.GetDiagnostics(), tt.wantAnalyzeDiagCount)

				for _, diag := range analyzeResp.GetDiagnostics() {
					allAnalyzeMessages = append(allAnalyzeMessages, diag.GetMessage())
				}
			}

			// Verify Analyze() diagnostics contain expected content
			allAnalyzeText := strings.Join(allAnalyzeMessages, " ")
			for _, want := range tt.wantAnalyzeContains {
				assert.Contains(t, allAnalyzeText, want)
			}

			// Now call AnalyzeStack - should only return summary
			req := &pulumirpc.AnalyzeStackRequest{
				Resources: tt.resources,
			}

			resp, err := server.AnalyzeStack(context.Background(), req)

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Len(t, resp.GetDiagnostics(), tt.wantStackDiagCount)

			// Verify AnalyzeStack() summary contains expected content
			var stackMessages []string
			for _, diag := range resp.GetDiagnostics() {
				stackMessages = append(stackMessages, diag.GetMessage())
			}
			allStackText := strings.Join(stackMessages, " ")
			for _, want := range tt.wantStackContains {
				assert.Contains(t, allStackText, want)
			}
		})
	}
}

func TestServer_AnalyzeStack_WithProperties(t *testing.T) {
	// Test that properties are correctly passed through to the cost calculator
	props, err := structpb.NewStruct(map[string]interface{}{
		"instanceType": "t3.micro",
		"ami":          "ami-0123456789abcdef0",
	})
	require.NoError(t, err)

	resources := []*pulumirpc.AnalyzerResource{
		{
			Type:       "aws:ec2/instance:Instance",
			Urn:        "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::web",
			Name:       "web",
			Properties: props,
		},
	}

	calc := &mockCostCalculator{
		results: []engine.CostResult{
			{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "web",
				Adapter:      "local-spec",
				Currency:     "USD",
				Monthly:      7.59,
			},
		},
	}
	server := NewServer(calc, "0.1.0")

	// Call Analyze first to populate the cache (simulates real workflow)
	for _, resource := range resources {
		analyzeReq := &pulumirpc.AnalyzeRequest{
			Type:       resource.GetType(),
			Urn:        resource.GetUrn(),
			Properties: resource.GetProperties(),
		}
		_, err := server.Analyze(context.Background(), analyzeReq)
		require.NoError(t, err)
	}

	req := &pulumirpc.AnalyzeStackRequest{Resources: resources}
	resp, err := server.AnalyzeStack(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	// AnalyzeStack now only returns the summary diagnostic
	assert.Len(t, resp.GetDiagnostics(), 1)
	assert.Contains(t, resp.GetDiagnostics()[0].GetMessage(), "$7.59 USD")
}

func TestServer_AnalyzeStack_DiagnosticFields(t *testing.T) {
	// Verify diagnostic fields are correctly set
	resources := []*pulumirpc.AnalyzerResource{
		{
			Type: "aws:ec2/instance:Instance",
			Urn:  "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::web",
			Name: "web",
		},
	}

	calc := &mockCostCalculator{
		results: []engine.CostResult{
			{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "web",
				Adapter:      "local-spec",
				Currency:     "USD",
				Monthly:      10.00,
			},
		},
	}
	server := NewServer(calc, "1.2.3")

	// Call Analyze first to get per-resource diagnostics and populate cache
	analyzeReq := &pulumirpc.AnalyzeRequest{
		Type: resources[0].GetType(),
		Urn:  resources[0].GetUrn(),
	}
	analyzeResp, err := server.Analyze(context.Background(), analyzeReq)
	require.NoError(t, err)
	require.Len(t, analyzeResp.GetDiagnostics(), 1)

	// Check per-resource diagnostic from Analyze()
	resourceDiag := analyzeResp.GetDiagnostics()[0]
	assert.Equal(t, "cost-estimate", resourceDiag.GetPolicyName())
	assert.Equal(t, "finfocus", resourceDiag.GetPolicyPackName())
	assert.Equal(t, "1.2.3", resourceDiag.GetPolicyPackVersion())
	assert.Equal(t, "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::web", resourceDiag.GetUrn())
	assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, resourceDiag.GetEnforcementLevel())

	// Call AnalyzeStack to get summary
	req := &pulumirpc.AnalyzeStackRequest{Resources: resources}
	resp, err := server.AnalyzeStack(context.Background(), req)

	require.NoError(t, err)
	// AnalyzeStack now only returns the summary diagnostic
	require.Len(t, resp.GetDiagnostics(), 1)

	// Check summary diagnostic from AnalyzeStack()
	summaryDiag := resp.GetDiagnostics()[0]
	assert.Equal(t, "stack-cost-summary", summaryDiag.GetPolicyName())
	assert.Empty(t, summaryDiag.GetUrn()) // Stack-level has no URN
}

func TestServer_GetAnalyzerInfo(t *testing.T) {
	calc := &mockCostCalculator{}
	server := NewServer(calc, "0.2.0")

	resp, err := server.GetAnalyzerInfo(context.Background(), &emptypb.Empty{})

	require.NoError(t, err)
	require.NotNil(t, resp)

	assert.Equal(t, "finfocus", resp.GetName())
	assert.Equal(t, "FinFocus Analyzer", resp.GetDisplayName())
	assert.Equal(t, "0.2.0", resp.GetVersion())
	assert.NotEmpty(t, resp.GetDescription())

	// Check policies are defined
	require.Len(t, resp.GetPolicies(), 2)

	// Check cost-estimate policy
	costPolicy := resp.GetPolicies()[0]
	assert.Equal(t, "cost-estimate", costPolicy.GetName())
	assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, costPolicy.GetEnforcementLevel())

	// Check stack-cost-summary policy
	summaryPolicy := resp.GetPolicies()[1]
	assert.Equal(t, "stack-cost-summary", summaryPolicy.GetName())
	assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, summaryPolicy.GetEnforcementLevel())
}

func TestServer_GetPluginInfo(t *testing.T) {
	calc := &mockCostCalculator{}
	server := NewServer(calc, "1.5.0")

	resp, err := server.GetPluginInfo(context.Background(), &emptypb.Empty{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "1.5.0", resp.GetVersion())
}

func TestServer_GetPluginInfo_EmptyVersion(t *testing.T) {
	calc := &mockCostCalculator{}
	server := NewServer(calc, "")

	resp, err := server.GetPluginInfo(context.Background(), &emptypb.Empty{})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "0.0.0-dev", resp.GetVersion()) // Should use default
}

func TestServer_Handshake(t *testing.T) {
	rootDir := "/home/user/.pulumi"
	programDir := "/home/user/myproject"

	tests := []struct {
		name     string
		request  *pulumirpc.AnalyzerHandshakeRequest
		wantErr  bool
		checkCtx bool
	}{
		{
			name: "standard handshake with engine address",
			request: &pulumirpc.AnalyzerHandshakeRequest{
				EngineAddress:    "localhost:12345",
				RootDirectory:    &rootDir,
				ProgramDirectory: &programDir,
			},
			wantErr: false,
		},
		{
			name: "handshake with minimal data",
			request: &pulumirpc.AnalyzerHandshakeRequest{
				EngineAddress: "127.0.0.1:9999",
			},
			wantErr: false,
		},
		{
			name:    "handshake with nil request fields",
			request: &pulumirpc.AnalyzerHandshakeRequest{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := &mockCostCalculator{}
			server := NewServer(calc, "1.0.0")

			resp, err := server.Handshake(context.Background(), tt.request)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
		})
	}
}

func TestServer_ConfigureStack(t *testing.T) {
	tests := []struct {
		name    string
		request *pulumirpc.AnalyzerStackConfigureRequest
		wantErr bool
	}{
		{
			name: "full stack configuration",
			request: &pulumirpc.AnalyzerStackConfigureRequest{
				Stack:        "dev",
				Project:      "my-infrastructure",
				Organization: "myorg",
				DryRun:       true,
				Config:       map[string]string{"aws:region": "us-east-1"},
				Tags:         map[string]string{"environment": "development"},
			},
			wantErr: false,
		},
		{
			name: "minimal stack configuration",
			request: &pulumirpc.AnalyzerStackConfigureRequest{
				Stack:   "prod",
				Project: "api",
			},
			wantErr: false,
		},
		{
			name:    "empty configuration",
			request: &pulumirpc.AnalyzerStackConfigureRequest{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calc := &mockCostCalculator{}
			server := NewServer(calc, "1.0.0")

			resp, err := server.ConfigureStack(context.Background(), tt.request)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, resp)
		})
	}
}

func TestServer_ConfigureStack_StoresContext(t *testing.T) {
	calc := &mockCostCalculator{}
	server := NewServer(calc, "1.0.0")

	req := &pulumirpc.AnalyzerStackConfigureRequest{
		Stack:        "staging",
		Project:      "web-app",
		Organization: "acme-corp",
		DryRun:       true,
	}

	_, err := server.ConfigureStack(context.Background(), req)
	require.NoError(t, err)

	// Verify server stored the stack context
	assert.Equal(t, "staging", server.stackName)
	assert.Equal(t, "web-app", server.projectName)
	assert.Equal(t, "acme-corp", server.organization)
	assert.True(t, server.dryRun)
}

func TestServer_Cancel(t *testing.T) {
	calc := &mockCostCalculator{}
	server := NewServer(calc, "1.0.0")

	// Initially not canceled
	assert.False(t, server.IsCanceled())

	// Call Cancel
	resp, err := server.Cancel(context.Background(), &emptypb.Empty{})

	require.NoError(t, err)
	require.NotNil(t, resp)

	// Now should be canceled
	assert.True(t, server.IsCanceled())
}

func TestServer_Cancel_ThreadSafe(t *testing.T) {
	calc := &mockCostCalculator{}
	server := NewServer(calc, "1.0.0")

	// Call Cancel concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := server.Cancel(context.Background(), &emptypb.Empty{})
			assert.NoError(t, err)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should be canceled
	assert.True(t, server.IsCanceled())
}

func TestIsInternalPulumiType(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		want         bool
	}{
		{
			name:         "pulumi stack resource",
			resourceType: "pulumi:pulumi:Stack",
			want:         true,
		},
		{
			name:         "pulumi AWS provider",
			resourceType: "pulumi:providers:aws",
			want:         true,
		},
		{
			name:         "pulumi Azure provider",
			resourceType: "pulumi:providers:azure",
			want:         true,
		},
		{
			name:         "AWS EC2 instance",
			resourceType: "aws:ec2/instance:Instance",
			want:         false,
		},
		{
			name:         "Azure VM",
			resourceType: "azure:compute/virtualMachine:VirtualMachine",
			want:         false,
		},
		{
			name:         "GCP compute instance",
			resourceType: "gcp:compute/instance:Instance",
			want:         false,
		},
		{
			name:         "empty type",
			resourceType: "",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := router.IsInternalPulumiType(tt.resourceType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestZeroCostResult(t *testing.T) {
	result := zeroCostResult("pulumi:pulumi:Stack", "my-stack")

	assert.Equal(t, "pulumi:pulumi:Stack", result.ResourceType)
	assert.Equal(t, "my-stack", result.ResourceID)
	assert.Equal(t, "USD", result.Currency)
	assert.Equal(t, float64(0), result.Monthly)
	assert.Equal(t, float64(0), result.Hourly)
	assert.Equal(t, "Internal Pulumi resource (no cloud cost)", result.Notes)
}

func TestServer_Analyze_InternalPulumiType(t *testing.T) {
	calc := &mockCostCalculator{
		// Should NOT be called for internal types
		results: []engine.CostResult{
			{
				ResourceType: "should-not-use-this",
				Monthly:      999.99,
			},
		},
	}
	server := NewServer(calc, "1.0.0")

	tests := []struct {
		name         string
		resourceType string
		urn          string
		wantNotes    string
	}{
		{
			name:         "pulumi stack",
			resourceType: "pulumi:pulumi:Stack",
			urn:          "urn:pulumi:dev::myapp::pulumi:pulumi:Stack::dev",
			wantNotes:    "Internal Pulumi resource (no cloud cost)",
		},
		{
			name:         "aws provider",
			resourceType: "pulumi:providers:aws",
			urn:          "urn:pulumi:dev::myapp::pulumi:providers:aws::default",
			wantNotes:    "Internal Pulumi resource (no cloud cost)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &pulumirpc.AnalyzeRequest{
				Type: tt.resourceType,
				Urn:  tt.urn,
			}

			resp, err := server.Analyze(context.Background(), req)

			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Len(t, resp.GetDiagnostics(), 1)

			diag := resp.GetDiagnostics()[0]
			assert.Contains(t, diag.GetMessage(), tt.wantNotes)
			assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, diag.GetEnforcementLevel())
		})
	}
}

func TestServer_AnalyzeStack_InternalPulumiTypes(t *testing.T) {
	calc := &mockCostCalculator{
		// Only non-internal types should be sent to calculator
		results: []engine.CostResult{
			{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "webserver",
				Monthly:      10.00,
				Hourly:       0.01,
				Currency:     "USD",
			},
		},
	}
	server := NewServer(calc, "1.0.0")
	ctx := context.Background()

	// Simulate Pulumi workflow: Analyze() is called for each resource first
	// Internal types should return $0.00 with "Internal Pulumi resource" message
	internalResources := []struct {
		resourceType string
		urn          string
	}{
		{"pulumi:pulumi:Stack", "urn:pulumi:dev::myapp::pulumi:pulumi:Stack::dev"},
		{"pulumi:providers:aws", "urn:pulumi:dev::myapp::pulumi:providers:aws::default"},
	}

	for _, res := range internalResources {
		req := &pulumirpc.AnalyzeRequest{
			Type: res.resourceType,
			Urn:  res.urn,
		}
		resp, err := server.Analyze(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Len(t, resp.GetDiagnostics(), 1)
		assert.Contains(t, resp.GetDiagnostics()[0].GetMessage(), "Internal Pulumi resource")
	}

	// Analyze the real resource
	realReq := &pulumirpc.AnalyzeRequest{
		Type: "aws:ec2/instance:Instance",
		Urn:  "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
	}
	realResp, err := server.Analyze(ctx, realReq)
	require.NoError(t, err)
	require.NotNil(t, realResp)
	require.Len(t, realResp.GetDiagnostics(), 1)
	assert.Contains(t, realResp.GetDiagnostics()[0].GetMessage(), "$10.00 USD")

	// Now AnalyzeStack should return only the summary diagnostic
	// Empty request is valid - AnalyzeStack uses costs cached from prior Analyze() calls
	stackReq := &pulumirpc.AnalyzeStackRequest{}

	stackResp, err := server.AnalyzeStack(ctx, stackReq)

	require.NoError(t, err)
	require.NotNil(t, stackResp)

	// Only 1 summary diagnostic (per-resource diagnostics come from Analyze())
	require.Len(t, stackResp.GetDiagnostics(), 1)

	// Check it's the summary
	summary := stackResp.GetDiagnostics()[0]
	assert.Equal(t, "stack-cost-summary", summary.GetPolicyName())
	// Summary should include the real resource cost ($10.00)
	// Only resources with Monthly > 0 are counted in "resources analyzed"
	// Internal types have $0.00 cost so they don't increment the analyzed count
	assert.Contains(t, summary.GetMessage(), "$10.00 USD")
	assert.Contains(t, summary.GetMessage(), "1 resources analyzed")
}

func TestServer_Analyze_WithRecommendations(t *testing.T) {
	calc := &mockCostCalculator{
		results: []engine.CostResult{
			{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "webserver",
				Currency:     "USD",
				Monthly:      100.0,
			},
		},
		recResults: &engine.RecommendationsResult{
			Recommendations: []engine.Recommendation{
				{
					ResourceID:       "webserver",
					Type:             "Right-sizing",
					Description:      "Switch to t3.small",
					EstimatedSavings: 20.0,
					Currency:         "USD",
				},
			},
		},
	}
	server := NewServer(calc, "1.0.0")

	req := &pulumirpc.AnalyzeRequest{
		Type: "aws:ec2/instance:Instance",
		Urn:  "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
	}

	resp, err := server.Analyze(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.GetDiagnostics(), 1)

	diag := resp.GetDiagnostics()[0]
	assert.Contains(t, diag.GetMessage(), "Switch to t3.small")
	assert.Contains(t, diag.GetMessage(), "save $20.00/mo")
}

func TestServer_Analyze_RecommendationFailure(t *testing.T) {
	calc := &mockCostCalculator{
		results: []engine.CostResult{
			{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "webserver",
				Currency:     "USD",
				Monthly:      100.0,
			},
		},
		recErr: assert.AnError, // Recommendation fetch fails
	}
	server := NewServer(calc, "1.0.0")

	req := &pulumirpc.AnalyzeRequest{
		Type: "aws:ec2/instance:Instance",
		Urn:  "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
	}

	// Should still succeed and return cost diagnostic (without recommendations)
	resp, err := server.Analyze(context.Background(), req)
	require.NoError(t, err)
	require.Len(t, resp.GetDiagnostics(), 1)

	diag := resp.GetDiagnostics()[0]
	assert.Contains(t, diag.GetMessage(), "$100.00 USD")
	assert.NotContains(t, diag.GetMessage(), "save")
}

// =============================================================================
// T013: AnalyzeStack Threshold Integration Tests (Issue #604)
// =============================================================================

func TestServer_AnalyzeStack_ThresholdIntegration(t *testing.T) {
	// Helper to populate cost cache via Analyze() calls
	analyzeResources := func(t *testing.T, server *Server, resources []struct {
		resType string
		urn     string
	}) {
		t.Helper()
		for _, res := range resources {
			req := &pulumirpc.AnalyzeRequest{
				Type: res.resType,
				Urn:  res.urn,
			}
			_, err := server.Analyze(context.Background(), req)
			require.NoError(t, err)
		}
	}

	t.Run("no threshold configured unchanged behavior", func(t *testing.T) {
		calc := &mockCostCalculator{
			results: []engine.CostResult{
				{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web1", Currency: "USD", Monthly: 100.0},
			},
		}
		server := NewServer(calc, "1.0.0") // No WithConfig()

		analyzeResources(t, server, []struct {
			resType string
			urn     string
		}{
			{"aws:ec2/instance:Instance", "urn:pulumi:dev::app::aws:ec2/instance:Instance::web1"},
		})

		resp, err := server.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
		require.NoError(t, err)
		// Should only have summary diagnostic, no threshold
		require.Len(t, resp.GetDiagnostics(), 1)
		assert.Equal(t, policyNameSum, resp.GetDiagnostics()[0].GetPolicyName())
	})

	t.Run("no threshold configured with config", func(t *testing.T) {
		calc := &mockCostCalculator{
			results: []engine.CostResult{
				{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web1", Currency: "USD", Monthly: 100.0},
			},
		}
		cfg := &config.Config{}
		cfg.Analyzer.MaxMonthlyCost = 0 // Threshold disabled
		cfg.Analyzer.Enforcement = "advisory"
		server := NewServer(calc, "1.0.0").WithConfig(cfg)

		analyzeResources(t, server, []struct {
			resType string
			urn     string
		}{
			{"aws:ec2/instance:Instance", "urn:pulumi:dev::app::aws:ec2/instance:Instance::web1"},
		})

		resp, err := server.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
		require.NoError(t, err)
		require.Len(t, resp.GetDiagnostics(), 1)
		assert.Equal(t, policyNameSum, resp.GetDiagnostics()[0].GetPolicyName())
	})

	t.Run("threshold exceeded advisory mode", func(t *testing.T) {
		calc := &mockCostCalculator{
			results: []engine.CostResult{
				{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web1", Currency: "USD", Monthly: 3000.0},
				{ResourceType: "aws:rds/instance:Instance", ResourceID: "db1", Currency: "USD", Monthly: 4500.0},
			},
		}
		cfg := &config.Config{}
		cfg.Analyzer.MaxMonthlyCost = 5000
		cfg.Analyzer.Enforcement = "advisory"
		server := NewServer(calc, "1.0.0").WithConfig(cfg)

		analyzeResources(t, server, []struct {
			resType string
			urn     string
		}{
			{"aws:ec2/instance:Instance", "urn:pulumi:dev::app::aws:ec2/instance:Instance::web1"},
			{"aws:rds/instance:Instance", "urn:pulumi:dev::app::aws:rds/instance:Instance::db1"},
		})

		resp, err := server.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
		require.NoError(t, err)
		// Summary + threshold diagnostic
		require.Len(t, resp.GetDiagnostics(), 2)

		// First should be summary
		assert.Equal(t, policyNameSum, resp.GetDiagnostics()[0].GetPolicyName())

		// Second should be threshold
		thresholdDiag := resp.GetDiagnostics()[1]
		assert.Equal(t, policyNameThreshold, thresholdDiag.GetPolicyName())
		assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, thresholdDiag.GetEnforcementLevel())
		assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_HIGH, thresholdDiag.GetSeverity())
		assert.Contains(t, thresholdDiag.GetMessage(), "exceeds threshold")
		assert.Contains(t, thresholdDiag.GetMessage(), "$7500.00 USD/mo")
	})

	t.Run("threshold exceeded mandatory mode", func(t *testing.T) {
		calc := &mockCostCalculator{
			results: []engine.CostResult{
				{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web1", Currency: "USD", Monthly: 6000.0},
			},
		}
		cfg := &config.Config{}
		cfg.Analyzer.MaxMonthlyCost = 5000
		cfg.Analyzer.Enforcement = "mandatory"
		server := NewServer(calc, "1.0.0").WithConfig(cfg)

		analyzeResources(t, server, []struct {
			resType string
			urn     string
		}{
			{"aws:ec2/instance:Instance", "urn:pulumi:dev::app::aws:ec2/instance:Instance::web1"},
		})

		resp, err := server.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
		require.NoError(t, err)
		require.Len(t, resp.GetDiagnostics(), 2)

		thresholdDiag := resp.GetDiagnostics()[1]
		assert.Equal(t, policyNameThreshold, thresholdDiag.GetPolicyName())
		assert.Equal(t, pulumirpc.EnforcementLevel_MANDATORY, thresholdDiag.GetEnforcementLevel())
		assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_HIGH, thresholdDiag.GetSeverity())
		assert.Contains(t, thresholdDiag.GetMessage(), "exceeds threshold")
		assert.Contains(t, thresholdDiag.GetMessage(), "deployment blocked")
	})

	t.Run("within budget confirmation", func(t *testing.T) {
		calc := &mockCostCalculator{
			results: []engine.CostResult{
				{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web1", Currency: "USD", Monthly: 2000.0},
				{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web2", Currency: "USD", Monthly: 1000.0},
			},
		}
		cfg := &config.Config{}
		cfg.Analyzer.MaxMonthlyCost = 5000
		cfg.Analyzer.Enforcement = "mandatory"
		server := NewServer(calc, "1.0.0").WithConfig(cfg)

		analyzeResources(t, server, []struct {
			resType string
			urn     string
		}{
			{"aws:ec2/instance:Instance", "urn:pulumi:dev::app::aws:ec2/instance:Instance::web1"},
			{"aws:ec2/instance:Instance", "urn:pulumi:dev::app::aws:ec2/instance:Instance::web2"},
		})

		resp, err := server.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
		require.NoError(t, err)
		require.Len(t, resp.GetDiagnostics(), 2)

		thresholdDiag := resp.GetDiagnostics()[1]
		assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, thresholdDiag.GetEnforcementLevel())
		assert.Equal(t, pulumirpc.PolicySeverity_POLICY_SEVERITY_MEDIUM, thresholdDiag.GetSeverity())
		assert.Contains(t, thresholdDiag.GetMessage(), "within threshold")
		assert.Contains(t, thresholdDiag.GetMessage(), "$3000.00 USD/mo")
	})

	t.Run("mixed currencies skip enforcement", func(t *testing.T) {
		calc := &mockCostCalculator{
			results: []engine.CostResult{
				{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web1", Currency: "USD", Monthly: 3000.0},
				{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web2", Currency: "EUR", Monthly: 4000.0},
			},
		}
		cfg := &config.Config{}
		cfg.Analyzer.MaxMonthlyCost = 5000
		cfg.Analyzer.Enforcement = "mandatory"
		server := NewServer(calc, "1.0.0").WithConfig(cfg)

		analyzeResources(t, server, []struct {
			resType string
			urn     string
		}{
			{"aws:ec2/instance:Instance", "urn:pulumi:dev::app::aws:ec2/instance:Instance::web1"},
			{"aws:ec2/instance:Instance", "urn:pulumi:dev::app::aws:ec2/instance:Instance::web2"},
		})

		resp, err := server.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
		require.NoError(t, err)
		// Mixed currencies: only summary, no threshold diagnostic
		require.Len(t, resp.GetDiagnostics(), 1)
		assert.Equal(t, policyNameSum, resp.GetDiagnostics()[0].GetPolicyName())
	})

	t.Run("all resources failed no threshold diagnostic", func(t *testing.T) {
		calc := &mockCostCalculator{
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance:Instance",
					ResourceID:   "web1",
					Currency:     "USD",
					Monthly:      0,
					Notes:        "ERROR: Plugin failed",
					Error:        &engine.StructuredError{Code: "PLUGIN_ERROR", Message: "Plugin failed"},
				},
				{
					ResourceType: "aws:rds/instance:Instance",
					ResourceID:   "db1",
					Currency:     "USD",
					Monthly:      0,
					Notes:        "VALIDATION: missing required field",
				},
			},
		}
		cfg := &config.Config{}
		cfg.Analyzer.MaxMonthlyCost = 5000
		cfg.Analyzer.Enforcement = "mandatory"
		server := NewServer(calc, "1.0.0").WithConfig(cfg)

		analyzeResources(t, server, []struct {
			resType string
			urn     string
		}{
			{"aws:ec2/instance:Instance", "urn:pulumi:dev::app::aws:ec2/instance:Instance::web1"},
			{"aws:rds/instance:Instance", "urn:pulumi:dev::app::aws:rds/instance:Instance::db1"},
		})

		resp, err := server.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
		require.NoError(t, err)
		// All resources failed: only summary, threshold skipped (FR-007 exception)
		require.Len(t, resp.GetDiagnostics(), 1)
		assert.Equal(t, policyNameSum, resp.GetDiagnostics()[0].GetPolicyName())
	})
}

// T013 extension: Test GetAnalyzerInfo with threshold policy registration.
func TestServer_GetAnalyzerInfo_WithThreshold(t *testing.T) {
	calc := &mockCostCalculator{}

	t.Run("no config no threshold policy", func(t *testing.T) {
		server := NewServer(calc, "1.0.0")
		resp, err := server.GetAnalyzerInfo(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		require.Len(t, resp.GetPolicies(), 2) // cost-estimate + stack-cost-summary only
	})

	t.Run("threshold configured advisory", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Analyzer.MaxMonthlyCost = 5000
		cfg.Analyzer.Enforcement = "advisory"
		server := NewServer(calc, "1.0.0").WithConfig(cfg)

		resp, err := server.GetAnalyzerInfo(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		require.Len(t, resp.GetPolicies(), 3) // +cost-threshold
		thresholdPolicy := resp.GetPolicies()[2]
		assert.Equal(t, policyNameThreshold, thresholdPolicy.GetName())
		assert.Equal(t, pulumirpc.EnforcementLevel_ADVISORY, thresholdPolicy.GetEnforcementLevel())
	})

	t.Run("threshold configured mandatory", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Analyzer.MaxMonthlyCost = 10000
		cfg.Analyzer.Enforcement = "mandatory"
		server := NewServer(calc, "1.0.0").WithConfig(cfg)

		resp, err := server.GetAnalyzerInfo(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		require.Len(t, resp.GetPolicies(), 3)
		thresholdPolicy := resp.GetPolicies()[2]
		assert.Equal(t, policyNameThreshold, thresholdPolicy.GetName())
		assert.Equal(t, pulumirpc.EnforcementLevel_MANDATORY, thresholdPolicy.GetEnforcementLevel())
	})

	t.Run("zero threshold no threshold policy", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Analyzer.MaxMonthlyCost = 0
		server := NewServer(calc, "1.0.0").WithConfig(cfg)

		resp, err := server.GetAnalyzerInfo(context.Background(), &emptypb.Empty{})
		require.NoError(t, err)
		require.Len(t, resp.GetPolicies(), 2) // No threshold policy when 0
	})
}

// =============================================================================
// T019: AnalyzeStack Summary File Integration Tests (Issue #604)
// =============================================================================

func TestServer_AnalyzeStack_SummaryFileIntegration(t *testing.T) {
	t.Run("summary file written after successful analysis", func(t *testing.T) {
		dir := t.TempDir()
		calc := &mockCostCalculator{
			results: []engine.CostResult{
				{
					ResourceType: "aws:ec2/instance:Instance",
					ResourceID:   "web1",
					Currency:     "USD",
					Monthly:      150.0,
					Adapter:      "aws-public",
				},
			},
		}
		server := NewServer(calc, "1.0.0").WithSummaryDir(dir)

		// Configure stack context
		_, err := server.ConfigureStack(context.Background(), &pulumirpc.AnalyzerStackConfigureRequest{
			Stack:   "dev",
			Project: "my-infra",
		})
		require.NoError(t, err)

		// Analyze resource to populate cache
		_, err = server.Analyze(context.Background(), &pulumirpc.AnalyzeRequest{
			Type: "aws:ec2/instance:Instance",
			Urn:  "urn:pulumi:dev::my-infra::aws:ec2/instance:Instance::web1",
		})
		require.NoError(t, err)

		// AnalyzeStack should write summary file
		_, err = server.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
		require.NoError(t, err)

		// Verify file exists and has correct content
		data, readErr := os.ReadFile(filepath.Join(dir, costSummaryFilename))
		require.NoError(t, readErr)

		var summary CostSummary
		require.NoError(t, json.Unmarshal(data, &summary))
		assert.Equal(t, "1", summary.SchemaVersion)
		assert.Equal(t, "dev", summary.Stack)
		assert.Equal(t, "my-infra", summary.Project)
		assert.Equal(t, 150.0, summary.TotalMonthlyCost)
		assert.Equal(t, "USD", summary.Currency)
		assert.Equal(t, 1, summary.ResourceCount)
		require.Len(t, summary.Resources, 1)
		assert.Equal(t, "web1", summary.Resources[0].Name)
	})

	t.Run("no summary dir skips write", func(t *testing.T) {
		calc := &mockCostCalculator{
			results: []engine.CostResult{
				{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web1", Currency: "USD", Monthly: 100.0},
			},
		}
		server := NewServer(calc, "1.0.0") // No WithSummaryDir

		_, err := server.Analyze(context.Background(), &pulumirpc.AnalyzeRequest{
			Type: "aws:ec2/instance:Instance",
			Urn:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web1",
		})
		require.NoError(t, err)

		// Should succeed without writing any file
		resp, err := server.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("write failure does not fail RPC", func(t *testing.T) {
		// Use a read-only directory to force write failure
		readOnlyDir := filepath.Join(t.TempDir(), "readonly")
		require.NoError(t, os.MkdirAll(readOnlyDir, 0o500))

		calc := &mockCostCalculator{
			results: []engine.CostResult{
				{ResourceType: "aws:ec2/instance:Instance", ResourceID: "web1", Currency: "USD", Monthly: 100.0},
			},
		}
		server := NewServer(calc, "1.0.0").WithSummaryDir(readOnlyDir)

		_, err := server.Analyze(context.Background(), &pulumirpc.AnalyzeRequest{
			Type: "aws:ec2/instance:Instance",
			Urn:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web1",
		})
		require.NoError(t, err)

		// AnalyzeStack should succeed even if summary write fails
		resp, err := server.AnalyzeStack(context.Background(), &pulumirpc.AnalyzeStackRequest{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		// Should still have the summary diagnostic
		require.GreaterOrEqual(t, len(resp.GetDiagnostics()), 1)
	})
}

func TestServer_AnalyzeStack_SummaryWithRecommendations(t *testing.T) {
	calc := &mockCostCalculator{
		results: []engine.CostResult{
			{
				ResourceType: "aws:ec2/instance:Instance",
				ResourceID:   "webserver",
				Currency:     "USD",
				Monthly:      100.0,
			},
		},
		recResults: &engine.RecommendationsResult{
			Recommendations: []engine.Recommendation{
				{
					ResourceID:       "webserver",
					Type:             "Right-sizing",
					EstimatedSavings: 20.0,
					Currency:         "USD",
				},
			},
		},
	}
	server := NewServer(calc, "1.0.0")

	// Call Analyze to populate cache (including recommendations)
	req := &pulumirpc.AnalyzeRequest{
		Type: "aws:ec2/instance:Instance",
		Urn:  "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
	}
	_, err := server.Analyze(context.Background(), req)
	require.NoError(t, err)

	// Call AnalyzeStack
	stackReq := &pulumirpc.AnalyzeStackRequest{
		Resources: []*pulumirpc.AnalyzerResource{
			{
				Type: "aws:ec2/instance:Instance",
				Urn:  "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
			},
		},
	}
	resp, err := server.AnalyzeStack(context.Background(), stackReq)
	require.NoError(t, err)
	require.Len(t, resp.GetDiagnostics(), 1)

	summary := resp.GetDiagnostics()[0]
	assert.Contains(t, summary.GetMessage(), "1 recommendations with $20.00/mo potential savings")
}
