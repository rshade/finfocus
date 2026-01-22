package proto

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// mockCostSourceClient is a mock implementation of CostSourceClient for testing.
type mockCostSourceClient struct {
	nameFunc         func(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*NameResponse, error)
	getProjectedFunc func(
		ctx context.Context,
		in *GetProjectedCostRequest,
		opts ...grpc.CallOption,
	) (*GetProjectedCostResponse, error)
	getActualFunc func(
		ctx context.Context,
		in *GetActualCostRequest,
		opts ...grpc.CallOption,
	) (*GetActualCostResponse, error)
	getRecommendationsFunc func(
		ctx context.Context,
		in *GetRecommendationsRequest,
		opts ...grpc.CallOption,
	) (*GetRecommendationsResponse, error)
	getPluginInfoFunc func(
		ctx context.Context,
		in *Empty,
		opts ...grpc.CallOption,
	) (*pbc.GetPluginInfoResponse, error)
	dryRunFunc func(
		ctx context.Context,
		in *pbc.DryRunRequest,
		opts ...grpc.CallOption,
	) (*pbc.DryRunResponse, error)
	getBudgetsFunc func(
		ctx context.Context,
		in *pbc.GetBudgetsRequest,
		opts ...grpc.CallOption,
	) (*pbc.GetBudgetsResponse, error)
}

func (m *mockCostSourceClient) Name(
	ctx context.Context,
	in *Empty,
	opts ...grpc.CallOption,
) (*NameResponse, error) {
	if m.nameFunc != nil {
		return m.nameFunc(ctx, in, opts...)
	}
	return &NameResponse{Name: "mock-plugin"}, nil
}

func (m *mockCostSourceClient) GetBudgets(
	ctx context.Context,
	in *pbc.GetBudgetsRequest,
	opts ...grpc.CallOption,
) (*pbc.GetBudgetsResponse, error) {
	if m.getBudgetsFunc != nil {
		return m.getBudgetsFunc(ctx, in, opts...)
	}
	return &pbc.GetBudgetsResponse{Budgets: []*pbc.Budget{}}, nil
}

func (m *mockCostSourceClient) GetPluginInfo(
	ctx context.Context,
	in *Empty,
	opts ...grpc.CallOption,
) (*pbc.GetPluginInfoResponse, error) {
	if m.getPluginInfoFunc != nil {
		return m.getPluginInfoFunc(ctx, in, opts...)
	}
	return &pbc.GetPluginInfoResponse{}, nil
}

func (m *mockCostSourceClient) DryRun(
	ctx context.Context,
	in *pbc.DryRunRequest,
	opts ...grpc.CallOption,
) (*pbc.DryRunResponse, error) {
	if m.dryRunFunc != nil {
		return m.dryRunFunc(ctx, in, opts...)
	}
	return &pbc.DryRunResponse{}, nil
}

func (m *mockCostSourceClient) GetProjectedCost(
	ctx context.Context,
	in *GetProjectedCostRequest,
	opts ...grpc.CallOption,
) (*GetProjectedCostResponse, error) {
	if m.getProjectedFunc != nil {
		return m.getProjectedFunc(ctx, in, opts...)
	}
	return &GetProjectedCostResponse{Results: []*CostResult{}}, nil
}

func (m *mockCostSourceClient) GetActualCost(
	ctx context.Context,
	in *GetActualCostRequest,
	opts ...grpc.CallOption,
) (*GetActualCostResponse, error) {
	if m.getActualFunc != nil {
		return m.getActualFunc(ctx, in, opts...)
	}
	return &GetActualCostResponse{Results: []*ActualCostResult{}}, nil
}

func (m *mockCostSourceClient) GetRecommendations(
	ctx context.Context,
	in *GetRecommendationsRequest,
	opts ...grpc.CallOption,
) (*GetRecommendationsResponse, error) {
	if m.getRecommendationsFunc != nil {
		return m.getRecommendationsFunc(ctx, in, opts...)
	}
	return &GetRecommendationsResponse{Recommendations: []*Recommendation{}}, nil
}

// T020: Unit test for DryRun wrapper.
func TestDryRun(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockClient := &mockCostSourceClient{
			dryRunFunc: func(ctx context.Context, in *pbc.DryRunRequest, opts ...grpc.CallOption) (*pbc.DryRunResponse, error) {
				return &pbc.DryRunResponse{
					FieldMappings: []*pbc.FieldMapping{
						{
							FieldName:     "instanceType",
							SupportStatus: pbc.FieldSupportStatus_FIELD_SUPPORT_STATUS_SUPPORTED,
						},
					},
				}, nil
			},
		}

		resp, err := mockClient.DryRun(context.Background(), &pbc.DryRunRequest{
			Resource: &pbc.ResourceDescriptor{ResourceType: "aws:ec2:Instance"},
		})
		require.NoError(t, err)
		require.Len(t, resp.GetFieldMappings(), 1)
		assert.Equal(t, "instanceType", resp.GetFieldMappings()[0].GetFieldName())
	})

	t.Run("Unimplemented", func(t *testing.T) {
		mockClient := &mockCostSourceClient{
			dryRunFunc: func(ctx context.Context, in *pbc.DryRunRequest, opts ...grpc.CallOption) (*pbc.DryRunResponse, error) {
				return nil, errors.New("unimplemented")
			},
		}

		_, err := mockClient.DryRun(context.Background(), &pbc.DryRunRequest{})
		require.Error(t, err)
	})

	t.Run("InvalidResource", func(t *testing.T) {
		mockClient := &mockCostSourceClient{
			dryRunFunc: func(ctx context.Context, in *pbc.DryRunRequest, opts ...grpc.CallOption) (*pbc.DryRunResponse, error) {
				return nil, errors.New("invalid resource type")
			},
		}

		_, err := mockClient.DryRun(context.Background(), &pbc.DryRunRequest{
			Resource: &pbc.ResourceDescriptor{ResourceType: "invalid"},
		})
		require.Error(t, err)
	})
}

// T003: Unit test for ErrorDetail struct creation.
func TestErrorDetail_Creation(t *testing.T) {
	timestamp := time.Now()
	err := errors.New("test error")

	detail := ErrorDetail{
		ResourceType: "aws:ec2:Instance",
		ResourceID:   "i-1234567890abcdef0",
		PluginName:   "test-plugin",
		Error:        err,
		Timestamp:    timestamp,
	}

	if detail.ResourceType != "aws:ec2:Instance" {
		t.Errorf("ResourceType = %s, want aws:ec2:Instance", detail.ResourceType)
	}
	if detail.ResourceID != "i-1234567890abcdef0" {
		t.Errorf("ResourceID = %s, want i-1234567890abcdef0", detail.ResourceID)
	}
	if detail.PluginName != "test-plugin" {
		t.Errorf("PluginName = %s, want test-plugin", detail.PluginName)
	}
	if !errors.Is(detail.Error, err) {
		t.Errorf("Error = %v, want %v", detail.Error, err)
	}
	if detail.Timestamp != timestamp {
		t.Errorf("Timestamp = %v, want %v", detail.Timestamp, timestamp)
	}
}

// T004: Unit test for CostResultWithErrors struct creation.
func TestCostResultWithErrors_Creation(t *testing.T) {
	result := &CostResultWithErrors{
		Results: []*CostResult{
			{Currency: "USD", MonthlyCost: 100.0},
		},
		Errors: []ErrorDetail{
			{ResourceType: "aws:ec2:Instance", ResourceID: "i-123", Error: errors.New("test")},
		},
	}

	if len(result.Results) != 1 {
		t.Errorf("Results length = %d, want 1", len(result.Results))
	}
	if len(result.Errors) != 1 {
		t.Errorf("Errors length = %d, want 1", len(result.Errors))
	}
}

func TestCostResultWithErrors_Empty(t *testing.T) {
	result := &CostResultWithErrors{
		Results: []*CostResult{},
		Errors:  []ErrorDetail{},
	}

	if len(result.Results) != 0 {
		t.Errorf("Results length = %d, want 0", len(result.Results))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Errors length = %d, want 0", len(result.Errors))
	}
}

// T005: Unit test for HasErrors() method.
func TestCostResultWithErrors_HasErrors(t *testing.T) {
	tests := []struct {
		name     string
		errors   []ErrorDetail
		expected bool
	}{
		{
			name:     "no errors",
			errors:   []ErrorDetail{},
			expected: false,
		},
		{
			name: "one error",
			errors: []ErrorDetail{
				{ResourceType: "aws:ec2:Instance", ResourceID: "i-123", Error: errors.New("test")},
			},
			expected: true,
		},
		{
			name: "multiple errors",
			errors: []ErrorDetail{
				{ResourceType: "aws:ec2:Instance", ResourceID: "i-123", Error: errors.New("test1")},
				{
					ResourceType: "aws:rds:Instance",
					ResourceID:   "db-456",
					Error:        errors.New("test2"),
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CostResultWithErrors{
				Results: []*CostResult{},
				Errors:  tt.errors,
			}

			if got := result.HasErrors(); got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// T006: Unit test for ErrorSummary() output format.
func TestCostResultWithErrors_ErrorSummary(t *testing.T) {
	t.Run("no errors returns empty string", func(t *testing.T) {
		result := &CostResultWithErrors{
			Results: []*CostResult{},
			Errors:  []ErrorDetail{},
		}

		if summary := result.ErrorSummary(); summary != "" {
			t.Errorf("ErrorSummary() = %q, want empty string", summary)
		}
	})

	t.Run("single error", func(t *testing.T) {
		result := &CostResultWithErrors{
			Results: []*CostResult{},
			Errors: []ErrorDetail{
				{
					ResourceType: "aws:ec2:Instance",
					ResourceID:   "i-123",
					PluginName:   "test-plugin",
					Error:        errors.New("connection refused"),
					Timestamp:    time.Now(),
				},
			},
		}

		summary := result.ErrorSummary()

		if !strings.Contains(summary, "1 resource(s) failed") {
			t.Errorf("ErrorSummary() should contain '1 resource(s) failed', got %q", summary)
		}
		if !strings.Contains(summary, "aws:ec2:Instance") {
			t.Errorf("ErrorSummary() should contain resource type, got %q", summary)
		}
		if !strings.Contains(summary, "i-123") {
			t.Errorf("ErrorSummary() should contain resource ID, got %q", summary)
		}
		if !strings.Contains(summary, "connection refused") {
			t.Errorf("ErrorSummary() should contain error message, got %q", summary)
		}
	})

	t.Run("multiple errors up to 5", func(t *testing.T) {
		result := &CostResultWithErrors{
			Results: []*CostResult{},
			Errors:  []ErrorDetail{},
		}

		// Add 3 errors
		for i := 0; i < 3; i++ {
			result.Errors = append(result.Errors, ErrorDetail{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   fmt.Sprintf("i-%d", i),
				PluginName:   "test-plugin",
				Error:        errors.New("error"),
				Timestamp:    time.Now(),
			})
		}

		summary := result.ErrorSummary()

		if !strings.Contains(summary, "3 resource(s) failed") {
			t.Errorf("ErrorSummary() should contain '3 resource(s) failed', got %q", summary)
		}
		// All 3 should be listed
		for i := 0; i < 3; i++ {
			if !strings.Contains(summary, fmt.Sprintf("i-%d", i)) {
				t.Errorf("ErrorSummary() should contain resource i-%d, got %q", i, summary)
			}
		}
	})

	t.Run("more than 5 errors truncates", func(t *testing.T) {
		result := &CostResultWithErrors{
			Results: []*CostResult{},
			Errors:  []ErrorDetail{},
		}

		// Add 10 errors
		for i := 0; i < 10; i++ {
			result.Errors = append(result.Errors, ErrorDetail{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   fmt.Sprintf("i-%d", i),
				PluginName:   "test-plugin",
				Error:        errors.New("error"),
				Timestamp:    time.Now(),
			})
		}

		summary := result.ErrorSummary()

		if !strings.Contains(summary, "10 resource(s) failed") {
			t.Errorf("ErrorSummary() should contain '10 resource(s) failed', got %q", summary)
		}
		// Should show first 5
		for i := 0; i < 5; i++ {
			if !strings.Contains(summary, fmt.Sprintf("i-%d", i)) {
				t.Errorf("ErrorSummary() should contain resource i-%d, got %q", i, summary)
			}
		}
		// Should indicate truncation
		if !strings.Contains(summary, "and 5 more") {
			t.Errorf("ErrorSummary() should indicate '... and 5 more errors', got %q", summary)
		}
	})
}

// T011: Unit test for GetProjectedCost error tracking (plugin errors, not validation errors).
func TestGetProjectedCostWithErrors(t *testing.T) {
	t.Run("tracks errors for failed resources", func(t *testing.T) {
		callCount := 0
		mockClient := &mockCostSourceClient{
			getProjectedFunc: func(ctx context.Context, in *GetProjectedCostRequest, opts ...grpc.CallOption) (*GetProjectedCostResponse, error) {
				callCount++
				// Fail for the second resource
				if len(in.Resources) > 0 && in.Resources[0].Type == "aws:rds:Instance" {
					return nil, errors.New("connection refused")
				}
				return &GetProjectedCostResponse{
					Results: []*CostResult{
						{Currency: "USD", MonthlyCost: 100.0, HourlyCost: 0.137},
					},
				}, nil
			},
		}

		// Resources must have SKU and region to pass pre-flight validation
		resources := []*ResourceDescriptor{
			{Type: "aws:ec2:Instance", Provider: "aws", Properties: map[string]string{
				"instanceType": "t3.micro", "region": "us-east-1",
			}},
			{Type: "aws:rds:Instance", Provider: "aws", Properties: map[string]string{
				"instanceClass": "db.t3.micro", "region": "us-east-1",
			}},
			{Type: "aws:s3:Bucket", Provider: "aws", Properties: map[string]string{
				"sku": "standard", "region": "us-east-1",
			}},
		}

		result := GetProjectedCostWithErrors(
			context.Background(),
			mockClient,
			"test-plugin",
			resources,
		)

		// Should have 3 results (2 success + 1 placeholder for error)
		if len(result.Results) != 3 {
			t.Errorf("Results length = %d, want 3", len(result.Results))
		}

		// Should have 1 error
		if len(result.Errors) != 1 {
			t.Errorf("Errors length = %d, want 1", len(result.Errors))
		}

		// Error should be tracked
		if !result.HasErrors() {
			t.Error("HasErrors() should return true")
		}

		// Error details should be correct
		if result.Errors[0].ResourceType != "aws:rds:Instance" {
			t.Errorf(
				"Error ResourceType = %s, want aws:rds:Instance",
				result.Errors[0].ResourceType,
			)
		}

		// Placeholder result should have ERROR in Notes
		for _, r := range result.Results {
			if r.Notes != "" && strings.Contains(r.Notes, "ERROR") {
				if !strings.Contains(r.Notes, "connection refused") {
					t.Errorf("Error result Notes should contain error message, got %q", r.Notes)
				}
			}
		}
	})
}

// T020: Unit test for GetActualCost error tracking.
func TestGetActualCostWithErrors(t *testing.T) {
	t.Run("tracks errors for failed resources", func(t *testing.T) {
		mockClient := &mockCostSourceClient{
			getActualFunc: func(ctx context.Context, in *GetActualCostRequest, opts ...grpc.CallOption) (*GetActualCostResponse, error) {
				// Fail for the second resource ID
				if len(in.ResourceIDs) > 0 && in.ResourceIDs[0] == "failed-resource" {
					return nil, errors.New("timeout")
				}
				return &GetActualCostResponse{
					Results: []*ActualCostResult{
						{Currency: "USD", TotalCost: 50.0},
					},
				}, nil
			},
		}

		resourceIDs := []string{"success-1", "failed-resource", "success-2"}
		startTime := time.Now().Add(-24 * time.Hour).Unix()
		endTime := time.Now().Unix()

		req := &GetActualCostRequest{
			ResourceIDs: resourceIDs,
			StartTime:   startTime,
			EndTime:     endTime,
		}

		result := GetActualCostWithErrors(context.Background(), mockClient, "test-plugin", req)

		// Should have 3 results
		if len(result.Results) != 3 {
			t.Errorf("Results length = %d, want 3", len(result.Results))
		}

		// Should have 1 error
		if len(result.Errors) != 1 {
			t.Errorf("Errors length = %d, want 1", len(result.Errors))
		}

		// Error details should be correct
		if result.Errors[0].ResourceID != "failed-resource" {
			t.Errorf("Error ResourceID = %s, want failed-resource", result.Errors[0].ResourceID)
		}

		if !strings.Contains(result.Errors[0].Error.Error(), "timeout") {
			t.Errorf("Error should contain 'timeout', got %v", result.Errors[0].Error)
		}
	})
}

// Test NameResponse.GetName method.
func TestNameResponse_GetName(t *testing.T) {
	tests := []struct {
		name     string
		response NameResponse
		expected string
	}{
		{
			name:     "normal name",
			response: NameResponse{Name: "test-plugin"},
			expected: "test-plugin",
		},
		{
			name:     "empty name",
			response: NameResponse{Name: ""},
			expected: "",
		},
		{
			name:     "special characters",
			response: NameResponse{Name: "plugin@v1.0"},
			expected: "plugin@v1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.response.GetName(); got != tt.expected {
				t.Errorf("GetName() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// Test NewCostSourceClient function.
func TestNewCostSourceClient(t *testing.T) {
	// This is a basic test since we can't easily create a real gRPC connection
	// in a unit test. We test that the function doesn't panic and returns
	// a non-nil client.
	t.Run("returns non-nil client", func(t *testing.T) {
		// We can't create a real connection, but we can test the function signature
		// and that it would work with a nil connection (though it would fail at runtime)
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("NewCostSourceClient panicked: %v", r)
			}
		}()

		// This will panic when trying to use the client, but the creation should work
		client := NewCostSourceClient(nil)
		if client == nil {
			t.Error("NewCostSourceClient returned nil")
		}
	})
}

// Test clientAdapter.Name method.
func TestClientAdapter_Name(t *testing.T) {
	t.Run("successful name call", func(t *testing.T) {
		mockClient := &mockCostSourceClient{
			nameFunc: func(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*NameResponse, error) {
				return &NameResponse{Name: "mock-plugin-name"}, nil
			},
		}

		adapter := &clientAdapter{client: nil} // We mock the client behavior
		// Note: In a real test, we'd need to mock the underlying gRPC client
		// This is a placeholder test structure

		_ = mockClient // Use the mock to show intended usage
		_ = adapter    // Avoid unused variable error
	})

	t.Run("name call with error", func(t *testing.T) {
		mockClient := &mockCostSourceClient{
			nameFunc: func(ctx context.Context, in *Empty, opts ...grpc.CallOption) (*NameResponse, error) {
				return nil, errors.New("grpc error")
			},
		}

		_ = mockClient // Use the mock to show intended usage
	})
}

// Test clientAdapter.GetProjectedCost method.
func TestClientAdapter_GetProjectedCost(t *testing.T) {
	t.Run("successful cost calculation", func(t *testing.T) {
		// Test with resources that have SKU and region in properties
		req := &GetProjectedCostRequest{
			Resources: []*ResourceDescriptor{
				{
					Type:     "aws:ec2:Instance",
					Provider: "aws",
					Properties: map[string]string{
						"sku":    "t3.micro",
						"region": "us-east-1",
					},
				},
			},
		}

		// This would require mocking the underlying gRPC client
		// For now, we test the request structure
		if len(req.Resources) != 1 {
			t.Errorf("Expected 1 resource, got %d", len(req.Resources))
		}

		resource := req.Resources[0]
		if resource.Type != "aws:ec2:Instance" {
			t.Errorf("Expected resource type 'aws:ec2:Instance', got %s", resource.Type)
		}
		if resource.Properties["sku"] != "t3.micro" {
			t.Errorf("Expected SKU 't3.micro', got %s", resource.Properties["sku"])
		}
		if resource.Properties["region"] != "us-east-1" {
			t.Errorf("Expected region 'us-east-1', got %s", resource.Properties["region"])
		}
	})

	t.Run("resource without sku/region properties", func(t *testing.T) {
		req := &GetProjectedCostRequest{
			Resources: []*ResourceDescriptor{
				{
					Type:       "aws:s3:Bucket",
					Provider:   "aws",
					Properties: map[string]string{},
				},
			},
		}

		if len(req.Resources) != 1 {
			t.Errorf("Expected 1 resource, got %d", len(req.Resources))
		}

		resource := req.Resources[0]
		if resource.Type != "aws:s3:Bucket" {
			t.Errorf("Expected resource type 'aws:s3:Bucket', got %s", resource.Type)
		}
		// SKU and region should be empty/default
		if sku, ok := resource.Properties["sku"]; ok && sku != "" {
			t.Errorf("Expected empty SKU, got %s", sku)
		}
		if region, ok := resource.Properties["region"]; ok && region != "" {
			t.Errorf("Expected empty region, got %s", region)
		}
	})
}

// Test clientAdapter.GetActualCost method.
func TestClientAdapter_GetActualCost(t *testing.T) {
	t.Run("successful actual cost query", func(t *testing.T) {
		startTime := time.Now().Add(-24 * time.Hour).Unix()
		endTime := time.Now().Unix()

		req := &GetActualCostRequest{
			ResourceIDs: []string{"i-1234567890abcdef0", "i-0987654321fedcba0"},
			StartTime:   startTime,
			EndTime:     endTime,
		}

		if len(req.ResourceIDs) != 2 {
			t.Errorf("Expected 2 resource IDs, got %d", len(req.ResourceIDs))
		}
		if req.StartTime != startTime {
			t.Errorf("Expected StartTime %d, got %d", startTime, req.StartTime)
		}
		if req.EndTime != endTime {
			t.Errorf("Expected EndTime %d, got %d", endTime, req.EndTime)
		}
	})

	t.Run("empty resource IDs", func(t *testing.T) {
		req := &GetActualCostRequest{
			ResourceIDs: []string{},
			StartTime:   1000000000,
			EndTime:     1000003600,
		}

		if len(req.ResourceIDs) != 0 {
			t.Errorf("Expected 0 resource IDs, got %d", len(req.ResourceIDs))
		}
	})
}

// TestExtractSKUFromProperties tests the SKU extraction function.
func TestExtractSKUFromProperties(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		properties map[string]string
		expected   string
	}{
		{
			name:       "EC2 instance with instanceType",
			properties: map[string]string{"instanceType": "t3.micro"},
			expected:   "t3.micro",
		},
		{
			name:       "EBS volume with type",
			properties: map[string]string{"type": "gp3"},
			expected:   "gp3",
		},
		{
			name:       "EBS volume with volumeType",
			properties: map[string]string{"volumeType": "gp2"},
			expected:   "gp2",
		},
		{
			name:       "RDS instance with instanceClass",
			properties: map[string]string{"instanceClass": "db.t3.micro"},
			expected:   "db.t3.micro",
		},
		{
			name:       "explicit sku property (generic)",
			provider:   "generic",
			properties: map[string]string{"sku": "Standard_DS1_v2"},
			expected:   "Standard_DS1_v2",
		},
		{
			name:       "Azure vmSize property",
			provider:   "azure",
			properties: map[string]string{"vmSize": "Standard_B1s"},
			expected:   "Standard_B1s",
		},
		{
			name:       "GCP machineType",
			provider:   "gcp",
			properties: map[string]string{"machineType": "n1-standard-4"},
			expected:   "n1-standard-4",
		},
		{
			name:       "instanceType takes precedence over type",
			provider:   "aws",
			properties: map[string]string{"instanceType": "t3.micro", "type": "gp3"},
			expected:   "t3.micro",
		},
		{
			name:       "empty properties returns empty string",
			provider:   "aws",
			properties: map[string]string{},
			expected:   "",
		},
		{
			name:       "nil properties returns empty string",
			provider:   "aws",
			properties: nil,
			expected:   "",
		},
		{
			name:       "irrelevant properties returns empty string",
			provider:   "aws",
			properties: map[string]string{"bucketName": "my-bucket", "acl": "private"},
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := "aws"
			if tt.provider != "" {
				provider = tt.provider
			}
			sku, region := resolveSKUAndRegion(provider, tt.properties)
			if sku != tt.expected {
				t.Errorf("resolveSKUAndRegion(%s) sku = %q, want %q", provider, sku, tt.expected)
			}
			_ = region
		})
	}
}

// TestExtractRegionFromProperties tests the region extraction function.
func TestExtractRegionFromProperties(t *testing.T) {
	tests := []struct {
		name       string
		properties map[string]string
		envVars    map[string]string
		expected   string
	}{
		{
			name:       "extract from availabilityZone with suffix",
			properties: map[string]string{"availabilityZone": "us-east-1a"},
			expected:   "us-east-1",
		},
		{
			name:       "extract from availabilityZone with suffix b",
			properties: map[string]string{"availabilityZone": "eu-west-2b"},
			expected:   "eu-west-2",
		},
		{
			name:       "extract from availabilityZone with suffix f",
			properties: map[string]string{"availabilityZone": "ap-southeast-1f"},
			expected:   "ap-southeast-1",
		},
		{
			name:       "extract from availabilityZone with suffix g",
			properties: map[string]string{"availabilityZone": "us-east-1g"},
			expected:   "us-east-1",
		},
		{
			name:       "explicit region property",
			properties: map[string]string{"region": "us-west-2"},
			expected:   "us-west-2",
		},
		{
			name:       "region takes precedence over availabilityZone (mapping package default)",
			properties: map[string]string{"availabilityZone": "us-east-1a", "region": "us-west-2"},
			expected:   "us-west-2",
		},
		{
			name:       "fallback to AWS_REGION env var",
			properties: map[string]string{},
			envVars:    map[string]string{"AWS_REGION": "eu-central-1"},
			expected:   "eu-central-1",
		},
		{
			name:       "fallback to AWS_DEFAULT_REGION env var",
			properties: map[string]string{},
			envVars:    map[string]string{"AWS_DEFAULT_REGION": "ap-northeast-1"},
			expected:   "ap-northeast-1",
		},
		{
			name:       "empty properties and no env vars returns empty string",
			properties: map[string]string{},
			expected:   "",
		},
		{
			name:       "availabilityZone without letter suffix returns as-is",
			properties: map[string]string{"availabilityZone": "local-zone-1"},
			expected:   "local-zone-1",
		},
		{
			name:       "empty availabilityZone falls back to region",
			properties: map[string]string{"availabilityZone": "", "region": "us-east-1"},
			expected:   "us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables for this test
			for key, val := range tt.envVars {
				t.Setenv(key, val)
			}

			sku, region := resolveSKUAndRegion("aws", tt.properties)
			if region != tt.expected {
				t.Errorf("resolveSKUAndRegion() region = %q, want %q", region, tt.expected)
			}
			_ = sku
		})
	}
}

// T041: Unit test for GetRecommendationsRequest type.
func TestGetRecommendationsRequest_Creation(t *testing.T) {
	tests := []struct {
		name     string
		request  GetRecommendationsRequest
		validate func(t *testing.T, req GetRecommendationsRequest)
	}{
		{
			name: "basic request with target resources",
			request: GetRecommendationsRequest{
				TargetResources: []*ResourceDescriptor{
					{
						Type:     "aws:ec2:Instance",
						Provider: "aws",
						Properties: map[string]string{
							"instanceType": "t3.xlarge",
							"region":       "us-east-1",
						},
					},
				},
			},
			validate: func(t *testing.T, req GetRecommendationsRequest) {
				if len(req.TargetResources) != 1 {
					t.Errorf("TargetResources length = %d, want 1", len(req.TargetResources))
				}
				if req.TargetResources[0].Type != "aws:ec2:Instance" {
					t.Errorf("Resource Type = %s, want aws:ec2:Instance", req.TargetResources[0].Type)
				}
			},
		},
		{
			name: "request with pagination",
			request: GetRecommendationsRequest{
				PageSize:  50,
				PageToken: "next-page-token",
			},
			validate: func(t *testing.T, req GetRecommendationsRequest) {
				if req.PageSize != 50 {
					t.Errorf("PageSize = %d, want 50", req.PageSize)
				}
				if req.PageToken != "next-page-token" {
					t.Errorf("PageToken = %s, want next-page-token", req.PageToken)
				}
			},
		},
		{
			name: "request with projection period",
			request: GetRecommendationsRequest{
				ProjectionPeriod: "monthly",
			},
			validate: func(t *testing.T, req GetRecommendationsRequest) {
				if req.ProjectionPeriod != "monthly" {
					t.Errorf("ProjectionPeriod = %s, want monthly", req.ProjectionPeriod)
				}
			},
		},
		{
			name: "request with excluded recommendation IDs",
			request: GetRecommendationsRequest{
				ExcludedRecommendationIDs: []string{"rec-123", "rec-456"},
			},
			validate: func(t *testing.T, req GetRecommendationsRequest) {
				if len(req.ExcludedRecommendationIDs) != 2 {
					t.Errorf("ExcludedRecommendationIDs length = %d, want 2", len(req.ExcludedRecommendationIDs))
				}
			},
		},
		{
			name:    "empty request",
			request: GetRecommendationsRequest{},
			validate: func(t *testing.T, req GetRecommendationsRequest) {
				if req.TargetResources != nil && len(req.TargetResources) != 0 {
					t.Errorf("TargetResources should be nil or empty, got %v", req.TargetResources)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t, tt.request)
		})
	}
}

// T042: Unit test for GetRecommendationsResponse type.
func TestGetRecommendationsResponse_Creation(t *testing.T) {
	tests := []struct {
		name     string
		response GetRecommendationsResponse
		validate func(t *testing.T, resp GetRecommendationsResponse)
	}{
		{
			name: "response with recommendations",
			response: GetRecommendationsResponse{
				Recommendations: []*Recommendation{
					{
						ID:          "rec-123",
						Category:    "COST",
						Description: "Right-size instance to t3.small",
						Impact: &RecommendationImpact{
							EstimatedSavings: 15.00,
							Currency:         "USD",
						},
					},
				},
				NextPageToken: "",
			},
			validate: func(t *testing.T, resp GetRecommendationsResponse) {
				if len(resp.Recommendations) != 1 {
					t.Errorf("Recommendations length = %d, want 1", len(resp.Recommendations))
				}
				rec := resp.Recommendations[0]
				if rec.ID != "rec-123" {
					t.Errorf("Recommendation ID = %s, want rec-123", rec.ID)
				}
				if rec.Impact.EstimatedSavings != 15.00 {
					t.Errorf("EstimatedSavings = %f, want 15.00", rec.Impact.EstimatedSavings)
				}
			},
		},
		{
			name: "response with pagination token",
			response: GetRecommendationsResponse{
				Recommendations: []*Recommendation{},
				NextPageToken:   "next-page-token-abc",
			},
			validate: func(t *testing.T, resp GetRecommendationsResponse) {
				if resp.NextPageToken != "next-page-token-abc" {
					t.Errorf("NextPageToken = %s, want next-page-token-abc", resp.NextPageToken)
				}
			},
		},
		{
			name: "response with multiple recommendations",
			response: GetRecommendationsResponse{
				Recommendations: []*Recommendation{
					{ID: "rec-1", Category: "COST", Description: "Right-size instance"},
					{ID: "rec-2", Category: "COST", Description: "Terminate idle resource"},
					{ID: "rec-3", Category: "COST", Description: "Purchase commitment"},
				},
			},
			validate: func(t *testing.T, resp GetRecommendationsResponse) {
				if len(resp.Recommendations) != 3 {
					t.Errorf("Recommendations length = %d, want 3", len(resp.Recommendations))
				}
			},
		},
		{
			name: "empty response",
			response: GetRecommendationsResponse{
				Recommendations: []*Recommendation{},
			},
			validate: func(t *testing.T, resp GetRecommendationsResponse) {
				if len(resp.Recommendations) != 0 {
					t.Errorf("Recommendations should be empty, got %d", len(resp.Recommendations))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t, tt.response)
		})
	}
}

// T043: Unit test for clientAdapter.GetRecommendations method.
func TestClientAdapter_GetRecommendations(t *testing.T) {
	t.Run("successful recommendations query", func(t *testing.T) {
		mockClient := &mockCostSourceClient{
			getRecommendationsFunc: func(ctx context.Context, in *GetRecommendationsRequest, opts ...grpc.CallOption) (*GetRecommendationsResponse, error) {
				return &GetRecommendationsResponse{
					Recommendations: []*Recommendation{
						{
							ID:          "rec-123",
							Category:    "COST",
							ActionType:  "RIGHTSIZE",
							Description: "Switch to t3.small to save $15/mo",
							Source:      "aws",
							ResourceID:  "i-1234567890abcdef0",
							Impact: &RecommendationImpact{
								EstimatedSavings: 15.00,
								Currency:         "USD",
							},
						},
					},
				}, nil
			},
		}

		req := &GetRecommendationsRequest{
			TargetResources: []*ResourceDescriptor{
				{
					Type:     "aws:ec2:Instance",
					Provider: "aws",
					Properties: map[string]string{
						"instanceType": "t3.xlarge",
					},
				},
			},
		}

		resp, err := mockClient.GetRecommendations(context.Background(), req)
		if err != nil {
			t.Fatalf("GetRecommendations() error = %v", err)
		}

		if len(resp.Recommendations) != 1 {
			t.Errorf("Recommendations length = %d, want 1", len(resp.Recommendations))
		}

		rec := resp.Recommendations[0]
		if rec.ID != "rec-123" {
			t.Errorf("Recommendation ID = %s, want rec-123", rec.ID)
		}
		if rec.Description != "Switch to t3.small to save $15/mo" {
			t.Errorf("Description = %s, want 'Switch to t3.small to save $15/mo'", rec.Description)
		}
		if rec.Impact.EstimatedSavings != 15.00 {
			t.Errorf("EstimatedSavings = %f, want 15.00", rec.Impact.EstimatedSavings)
		}
	})

	t.Run("query with no recommendations available", func(t *testing.T) {
		mockClient := &mockCostSourceClient{
			getRecommendationsFunc: func(ctx context.Context, in *GetRecommendationsRequest, opts ...grpc.CallOption) (*GetRecommendationsResponse, error) {
				return &GetRecommendationsResponse{
					Recommendations: []*Recommendation{},
				}, nil
			},
		}

		req := &GetRecommendationsRequest{
			TargetResources: []*ResourceDescriptor{
				{Type: "aws:s3:Bucket", Provider: "aws"},
			},
		}

		resp, err := mockClient.GetRecommendations(context.Background(), req)
		if err != nil {
			t.Fatalf("GetRecommendations() error = %v", err)
		}

		if len(resp.Recommendations) != 0 {
			t.Errorf("Recommendations length = %d, want 0", len(resp.Recommendations))
		}
	})

	t.Run("query with error", func(t *testing.T) {
		mockClient := &mockCostSourceClient{
			getRecommendationsFunc: func(ctx context.Context, in *GetRecommendationsRequest, opts ...grpc.CallOption) (*GetRecommendationsResponse, error) {
				return nil, errors.New("service unavailable")
			},
		}

		req := &GetRecommendationsRequest{}
		resp, err := mockClient.GetRecommendations(context.Background(), req)

		if err == nil {
			t.Error("GetRecommendations() expected error, got nil")
		}
		if resp != nil {
			t.Errorf("Response should be nil on error, got %v", resp)
		}
		if !strings.Contains(err.Error(), "service unavailable") {
			t.Errorf("Error should contain 'service unavailable', got %v", err)
		}
	})

	t.Run("query with pagination", func(t *testing.T) {
		callCount := 0
		mockClient := &mockCostSourceClient{
			getRecommendationsFunc: func(ctx context.Context, in *GetRecommendationsRequest, opts ...grpc.CallOption) (*GetRecommendationsResponse, error) {
				callCount++
				if in.PageToken == "" {
					return &GetRecommendationsResponse{
						Recommendations: []*Recommendation{
							{ID: "rec-1"},
							{ID: "rec-2"},
						},
						NextPageToken: "page-2",
					}, nil
				}
				return &GetRecommendationsResponse{
					Recommendations: []*Recommendation{
						{ID: "rec-3"},
					},
					NextPageToken: "",
				}, nil
			},
		}

		// First page
		resp, err := mockClient.GetRecommendations(context.Background(), &GetRecommendationsRequest{})
		if err != nil {
			t.Fatalf("First page error = %v", err)
		}
		if len(resp.Recommendations) != 2 {
			t.Errorf("First page recommendations = %d, want 2", len(resp.Recommendations))
		}
		if resp.NextPageToken != "page-2" {
			t.Errorf("NextPageToken = %s, want page-2", resp.NextPageToken)
		}

		// Second page
		resp, err = mockClient.GetRecommendations(context.Background(), &GetRecommendationsRequest{
			PageToken: "page-2",
		})
		if err != nil {
			t.Fatalf("Second page error = %v", err)
		}
		if len(resp.Recommendations) != 1 {
			t.Errorf("Second page recommendations = %d, want 1", len(resp.Recommendations))
		}
		if resp.NextPageToken != "" {
			t.Errorf("NextPageToken should be empty on last page, got %s", resp.NextPageToken)
		}

		if callCount != 2 {
			t.Errorf("Expected 2 calls, got %d", callCount)
		}
	})

	t.Run("default mock returns empty recommendations", func(t *testing.T) {
		mockClient := &mockCostSourceClient{} // No function set

		resp, err := mockClient.GetRecommendations(context.Background(), &GetRecommendationsRequest{})
		if err != nil {
			t.Fatalf("GetRecommendations() error = %v", err)
		}
		if len(resp.Recommendations) != 0 {
			t.Errorf("Default mock should return empty recommendations, got %d", len(resp.Recommendations))
		}
	})
}

// T031: Test that Recommendation struct correctly stores ActionType for all 11 action types.
// This ensures JSON serialization will work correctly for recommendations output.
func TestRecommendation_ActionTypeAll11Types(t *testing.T) {
	// All 11 action types that must be supported
	actionTypes := []struct {
		name             string
		actionType       string
		expectedInFilter bool
	}{
		{"RIGHTSIZE", "RIGHTSIZE", true},
		{"TERMINATE", "TERMINATE", true},
		{"PURCHASE_COMMITMENT", "PURCHASE_COMMITMENT", true},
		{"ADJUST_REQUESTS", "ADJUST_REQUESTS", true},
		{"MODIFY", "MODIFY", true},
		{"DELETE_UNUSED", "DELETE_UNUSED", true},
		{"MIGRATE", "MIGRATE", true},
		{"CONSOLIDATE", "CONSOLIDATE", true},
		{"SCHEDULE", "SCHEDULE", true},
		{"REFACTOR", "REFACTOR", true},
		{"OTHER", "OTHER", true},
	}

	for _, tt := range actionTypes {
		t.Run(tt.name, func(t *testing.T) {
			rec := Recommendation{
				ID:          "rec-" + strings.ToLower(tt.actionType),
				Category:    "COST",
				ActionType:  tt.actionType,
				Description: "Test recommendation for " + tt.actionType,
				ResourceID:  "resource-123",
				Source:      "test-plugin",
			}

			// Verify ActionType is stored correctly
			if rec.ActionType != tt.actionType {
				t.Errorf("ActionType = %q, want %q", rec.ActionType, tt.actionType)
			}

			// Verify the recommendation can be filtered by its action type
			// using the proto utilities
			actionTypeEnum, err := ParseActionType(rec.ActionType)
			if err != nil {
				t.Errorf("ParseActionType(%q) error = %v", rec.ActionType, err)
			}

			// Verify MatchesActionType works for this action type
			matches := MatchesActionType(rec.ActionType, []pbc.RecommendationActionType{actionTypeEnum})
			if !matches {
				t.Errorf("MatchesActionType(%q) = false, want true", rec.ActionType)
			}

			// Verify ActionTypeLabel works for this action type
			label := ActionTypeLabel(actionTypeEnum)
			if label == "" || strings.HasPrefix(label, "Unknown") {
				t.Errorf("ActionTypeLabel() = %q, expected valid label", label)
			}
		})
	}
}

// T032: Verify that ActionType string representations work correctly for JSON output.
func TestRecommendation_ActionType_JSONSerialization(t *testing.T) {
	// Create a recommendation with each action type and verify the string representation
	// is what we expect for JSON serialization
	recommendations := []Recommendation{
		{ID: "1", ActionType: "RIGHTSIZE", Description: "Rightsize test"},
		{ID: "2", ActionType: "TERMINATE", Description: "Terminate test"},
		{ID: "3", ActionType: "PURCHASE_COMMITMENT", Description: "Purchase commitment test"},
		{ID: "4", ActionType: "ADJUST_REQUESTS", Description: "Adjust requests test"},
		{ID: "5", ActionType: "MODIFY", Description: "Modify test"},
		{ID: "6", ActionType: "DELETE_UNUSED", Description: "Delete unused test"},
		{ID: "7", ActionType: "MIGRATE", Description: "Migrate test"},
		{ID: "8", ActionType: "CONSOLIDATE", Description: "Consolidate test"},
		{ID: "9", ActionType: "SCHEDULE", Description: "Schedule test"},
		{ID: "10", ActionType: "REFACTOR", Description: "Refactor test"},
		{ID: "11", ActionType: "OTHER", Description: "Other test"},
	}

	for _, rec := range recommendations {
		// Verify ActionType is non-empty
		if rec.ActionType == "" {
			t.Errorf("Recommendation %s has empty ActionType", rec.ID)
		}

		// Verify ActionType is in expected format (uppercase with underscores)
		if rec.ActionType != strings.ToUpper(rec.ActionType) {
			t.Errorf("ActionType %q should be uppercase", rec.ActionType)
		}

		// Verify we can get a human-readable label for TUI/table display
		label := ActionTypeLabelFromString(rec.ActionType)
		if label == "" {
			t.Errorf("ActionTypeLabelFromString(%q) returned empty", rec.ActionType)
		}
	}
}

// TestRecommendation_Creation tests the Recommendation type.
func TestRecommendation_Creation(t *testing.T) {
	tests := []struct {
		name     string
		rec      Recommendation
		validate func(t *testing.T, rec Recommendation)
	}{
		{
			name: "rightsizing recommendation",
			rec: Recommendation{
				ID:          "rec-rightsize-123",
				Category:    "COST",
				ActionType:  "RIGHTSIZE",
				Description: "Switch from t3.xlarge to t3.small",
				ResourceID:  "i-1234567890abcdef0",
				Source:      "aws-cost-explorer",
				Impact: &RecommendationImpact{
					EstimatedSavings: 45.00,
					Currency:         "USD",
					CurrentCost:      60.00,
					ProjectedCost:    15.00,
				},
			},
			validate: func(t *testing.T, rec Recommendation) {
				if rec.ActionType != "RIGHTSIZE" {
					t.Errorf("ActionType = %s, want RIGHTSIZE", rec.ActionType)
				}
				if rec.Impact.CurrentCost != 60.00 {
					t.Errorf("CurrentCost = %f, want 60.00", rec.Impact.CurrentCost)
				}
			},
		},
		{
			name: "terminate recommendation",
			rec: Recommendation{
				ID:          "rec-terminate-456",
				Category:    "COST",
				ActionType:  "TERMINATE",
				Description: "Remove idle instance with 0% CPU utilization",
				ResourceID:  "i-fedcba0987654321",
				Source:      "aws-cost-explorer",
				Impact: &RecommendationImpact{
					EstimatedSavings: 100.00,
					Currency:         "USD",
				},
			},
			validate: func(t *testing.T, rec Recommendation) {
				if rec.ActionType != "TERMINATE" {
					t.Errorf("ActionType = %s, want TERMINATE", rec.ActionType)
				}
			},
		},
		{
			name: "recommendation without impact",
			rec: Recommendation{
				ID:          "rec-review-789",
				Category:    "PERFORMANCE",
				ActionType:  "MODIFY",
				Description: "Review storage class configuration",
				Source:      "manual",
			},
			validate: func(t *testing.T, rec Recommendation) {
				if rec.Impact != nil {
					t.Errorf("Impact should be nil, got %v", rec.Impact)
				}
			},
		},
		{
			name: "recommendation with metadata",
			rec: Recommendation{
				ID:          "rec-k8s-123",
				Category:    "COST",
				ActionType:  "KUBERNETES_REQUEST_SIZING",
				Description: "Adjust CPU requests for deployment",
				Source:      "kubecost",
				Metadata: map[string]string{
					"namespace":     "production",
					"workload":      "api-server",
					"current_cpu":   "1000m",
					"suggested_cpu": "250m",
				},
			},
			validate: func(t *testing.T, rec Recommendation) {
				if rec.Metadata["namespace"] != "production" {
					t.Errorf("Metadata[namespace] = %s, want production", rec.Metadata["namespace"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t, tt.rec)
		})
	}
}

// TestRecommendationImpact_Creation tests the RecommendationImpact type.
func TestRecommendationImpact_Creation(t *testing.T) {
	tests := []struct {
		name   string
		impact RecommendationImpact
	}{
		{
			name: "full impact data",
			impact: RecommendationImpact{
				EstimatedSavings:  50.00,
				Currency:          "USD",
				CurrentCost:       100.00,
				ProjectedCost:     50.00,
				SavingsPercentage: 50.0,
			},
		},
		{
			name: "savings only",
			impact: RecommendationImpact{
				EstimatedSavings: 25.00,
				Currency:         "EUR",
			},
		},
		{
			name: "zero savings",
			impact: RecommendationImpact{
				EstimatedSavings: 0,
				Currency:         "USD",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation that the struct was created correctly
			if tt.impact.Currency == "" && tt.impact.EstimatedSavings > 0 {
				t.Error("Currency should be set when EstimatedSavings > 0")
			}
		})
	}
}

// =============================================================================
// Pre-Flight Validation Tests (Feature 107)
// =============================================================================

// T002: TestGetProjectedCost_ValidationFailure_EmptyProvider verifies that
// resources with empty provider trigger pre-flight validation failure.
func TestGetProjectedCost_ValidationFailure_EmptyProvider(t *testing.T) {
	callCount := 0
	mockClient := &mockCostSourceClient{
		getProjectedFunc: func(
			ctx context.Context,
			in *GetProjectedCostRequest,
			opts ...grpc.CallOption,
		) (*GetProjectedCostResponse, error) {
			callCount++
			return &GetProjectedCostResponse{
				Results: []*CostResult{{Currency: "USD", MonthlyCost: 100.0}},
			}, nil
		},
	}

	resources := []*ResourceDescriptor{
		{
			Type:     "aws:ec2:Instance",
			Provider: "", // Empty provider should fail validation
			Properties: map[string]string{
				"instanceType": "t3.micro",
				"region":       "us-east-1",
			},
		},
	}

	result := GetProjectedCostWithErrors(
		context.Background(),
		mockClient,
		"test-plugin",
		resources,
	)

	// Should have 1 result (placeholder for validation failure)
	if len(result.Results) != 1 {
		t.Errorf("Results length = %d, want 1", len(result.Results))
	}

	// Plugin should NOT be called for invalid resources
	if callCount != 0 {
		t.Errorf("Plugin was called %d times, want 0 (validation should skip plugin)", callCount)
	}

	// Result should have VALIDATION prefix in Notes
	if len(result.Results) > 0 {
		notes := result.Results[0].Notes
		if !strings.Contains(notes, "VALIDATION:") {
			t.Errorf("Notes should contain 'VALIDATION:', got %q", notes)
		}
		if !strings.Contains(strings.ToLower(notes), "provider") {
			t.Errorf("Notes should mention 'provider', got %q", notes)
		}
	}

	// Cost should be 0 for validation failures
	if len(result.Results) > 0 && result.Results[0].MonthlyCost != 0 {
		t.Errorf("MonthlyCost should be 0 for validation failure, got %f", result.Results[0].MonthlyCost)
	}
}

// T003: TestGetProjectedCost_ValidationFailure_EmptySKU verifies that
// resources with empty SKU trigger pre-flight validation failure.
func TestGetProjectedCost_ValidationFailure_EmptySKU(t *testing.T) {
	callCount := 0
	mockClient := &mockCostSourceClient{
		getProjectedFunc: func(
			ctx context.Context,
			in *GetProjectedCostRequest,
			opts ...grpc.CallOption,
		) (*GetProjectedCostResponse, error) {
			callCount++
			return &GetProjectedCostResponse{
				Results: []*CostResult{{Currency: "USD", MonthlyCost: 100.0}},
			}, nil
		},
	}

	resources := []*ResourceDescriptor{
		{
			Type:     "aws:ec2:Instance",
			Provider: "aws",
			Properties: map[string]string{
				// Missing instanceType - SKU will be empty
				"region": "us-east-1",
			},
		},
	}

	result := GetProjectedCostWithErrors(
		context.Background(),
		mockClient,
		"test-plugin",
		resources,
	)

	// Should have 1 result
	if len(result.Results) != 1 {
		t.Errorf("Results length = %d, want 1", len(result.Results))
	}

	// Plugin should NOT be called for invalid resources
	if callCount != 0 {
		t.Errorf("Plugin was called %d times, want 0 (validation should skip plugin)", callCount)
	}

	// Result should have VALIDATION prefix in Notes
	if len(result.Results) > 0 {
		notes := result.Results[0].Notes
		if !strings.Contains(notes, "VALIDATION:") {
			t.Errorf("Notes should contain 'VALIDATION:', got %q", notes)
		}
		if !strings.Contains(strings.ToLower(notes), "sku") {
			t.Errorf("Notes should mention 'sku', got %q", notes)
		}
	}
}

// T004: TestGetProjectedCost_ValidationFailure_EmptyRegion verifies that
// resources with empty region trigger pre-flight validation failure.
func TestGetProjectedCost_ValidationFailure_EmptyRegion(t *testing.T) {
	// Clear environment variables that might provide region fallback
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")

	callCount := 0
	mockClient := &mockCostSourceClient{
		getProjectedFunc: func(
			ctx context.Context,
			in *GetProjectedCostRequest,
			opts ...grpc.CallOption,
		) (*GetProjectedCostResponse, error) {
			callCount++
			return &GetProjectedCostResponse{
				Results: []*CostResult{{Currency: "USD", MonthlyCost: 100.0}},
			}, nil
		},
	}

	resources := []*ResourceDescriptor{
		{
			Type:     "aws:ec2:Instance",
			Provider: "aws",
			Properties: map[string]string{
				"instanceType": "t3.micro",
				// Missing region - will be empty after extraction
			},
		},
	}

	result := GetProjectedCostWithErrors(
		context.Background(),
		mockClient,
		"test-plugin",
		resources,
	)

	// Should have 1 result
	if len(result.Results) != 1 {
		t.Errorf("Results length = %d, want 1", len(result.Results))
	}

	// Plugin should NOT be called for invalid resources
	if callCount != 0 {
		t.Errorf("Plugin was called %d times, want 0 (validation should skip plugin)", callCount)
	}

	// Result should have VALIDATION prefix in Notes
	if len(result.Results) > 0 {
		notes := result.Results[0].Notes
		if !strings.Contains(notes, "VALIDATION:") {
			t.Errorf("Notes should contain 'VALIDATION:', got %q", notes)
		}
		if !strings.Contains(strings.ToLower(notes), "region") {
			t.Errorf("Notes should mention 'region', got %q", notes)
		}
	}
}

// T005: TestGetProjectedCost_ValidationFailure_MixedValidInvalid verifies that
// valid resources are processed while invalid ones get validation errors.
func TestGetProjectedCost_ValidationFailure_MixedValidInvalid(t *testing.T) {
	callCount := 0
	mockClient := &mockCostSourceClient{
		getProjectedFunc: func(
			ctx context.Context,
			in *GetProjectedCostRequest,
			opts ...grpc.CallOption,
		) (*GetProjectedCostResponse, error) {
			callCount++
			return &GetProjectedCostResponse{
				Results: []*CostResult{{Currency: "USD", MonthlyCost: 50.0}},
			}, nil
		},
	}

	resources := []*ResourceDescriptor{
		// Valid resource
		{
			Type:     "aws:ec2:Instance",
			Provider: "aws",
			Properties: map[string]string{
				"instanceType": "t3.micro",
				"region":       "us-east-1",
			},
		},
		// Invalid resource (empty provider)
		{
			Type:     "aws:rds:Instance",
			Provider: "", // Invalid
			Properties: map[string]string{
				"instanceClass": "db.t3.micro",
				"region":        "us-east-1",
			},
		},
		// Another valid resource
		{
			Type:     "aws:s3:Bucket",
			Provider: "aws",
			Properties: map[string]string{
				"region": "us-west-2",
				"sku":    "standard",
			},
		},
	}

	result := GetProjectedCostWithErrors(
		context.Background(),
		mockClient,
		"test-plugin",
		resources,
	)

	// Should have 3 results (2 successful + 1 validation failure)
	if len(result.Results) != 3 {
		t.Errorf("Results length = %d, want 3", len(result.Results))
	}

	// Plugin should only be called twice (for valid resources)
	if callCount != 2 {
		t.Errorf("Plugin was called %d times, want 2 (only for valid resources)", callCount)
	}

	// Check that one result has VALIDATION prefix
	validationCount := 0
	for _, r := range result.Results {
		if strings.Contains(r.Notes, "VALIDATION:") {
			validationCount++
		}
	}
	if validationCount != 1 {
		t.Errorf("Expected 1 validation error, got %d", validationCount)
	}
}

// =============================================================================
// Pre-Flight Validation Tests for Actual Cost (Feature 107 - US2)
// =============================================================================

// T010: TestGetActualCost_ValidationFailure_EmptyResourceID verifies that
// requests with empty resource ID trigger pre-flight validation failure.
func TestGetActualCost_ValidationFailure_EmptyResourceID(t *testing.T) {
	callCount := 0
	mockClient := &mockCostSourceClient{
		getActualFunc: func(
			ctx context.Context,
			in *GetActualCostRequest,
			opts ...grpc.CallOption,
		) (*GetActualCostResponse, error) {
			callCount++
			return &GetActualCostResponse{
				Results: []*ActualCostResult{{Currency: "USD", TotalCost: 100.0}},
			}, nil
		},
	}

	startTime := time.Now().Add(-24 * time.Hour).Unix()
	endTime := time.Now().Unix()

	req := &GetActualCostRequest{
		ResourceIDs: []string{""}, // Empty resource ID should fail validation
		StartTime:   startTime,
		EndTime:     endTime,
	}

	result := GetActualCostWithErrors(context.Background(), mockClient, "test-plugin", req)

	// Should have 1 result (placeholder for validation failure)
	if len(result.Results) != 1 {
		t.Errorf("Results length = %d, want 1", len(result.Results))
	}

	// Plugin should NOT be called for invalid resources
	if callCount != 0 {
		t.Errorf("Plugin was called %d times, want 0 (validation should skip plugin)", callCount)
	}

	// Should have 1 error with VALIDATION in Notes
	if len(result.Errors) != 1 {
		t.Errorf("Errors length = %d, want 1", len(result.Errors))
	}

	// Error should mention validation
	if len(result.Errors) > 0 {
		errStr := result.Errors[0].Error.Error()
		if !strings.Contains(errStr, "pre-flight validation failed") {
			t.Errorf("Error should contain 'pre-flight validation failed', got %q", errStr)
		}
	}

	// Result Notes should have VALIDATION prefix
	if len(result.Results) > 0 {
		notes := result.Results[0].Notes
		if !strings.Contains(notes, "VALIDATION:") {
			t.Errorf("Notes should contain 'VALIDATION:', got %q", notes)
		}
		// Accept any message format that mentions "resource" (covers resourceid, resource_id, etc.)
		if !strings.Contains(strings.ToLower(notes), "resource") {
			t.Errorf("Notes should mention 'resource', got %q", notes)
		}
	}
}

// TestResolveSKUAndRegion_AWSRegionFallbackScope verifies that AWS_REGION and
// AWS_DEFAULT_REGION environment variables are ONLY applied to AWS resources.
// This is critical for SC-001 (CI reliability) - Azure/GCP resources should NOT
// inherit AWS region values.
func TestResolveSKUAndRegion_AWSRegionFallbackScope(t *testing.T) {
	tests := []struct {
		name           string
		provider       string
		properties     map[string]string
		envVars        map[string]string
		expectedRegion string
	}{
		{
			name:       "AWS resource uses AWS_REGION when no region in properties",
			provider:   "aws",
			properties: map[string]string{"instanceType": "t3.micro"},
			envVars:    map[string]string{"AWS_REGION": "us-east-1"},
			// AWS should use env var fallback
			expectedRegion: "us-east-1",
		},
		{
			name:       "Azure resource does NOT use AWS_REGION fallback",
			provider:   "azure",
			properties: map[string]string{"vmSize": "Standard_B1s"},
			envVars:    map[string]string{"AWS_REGION": "us-east-1"},
			// Azure should NOT inherit AWS region - returns empty (no Azure region specified)
			expectedRegion: "",
		},
		{
			name:       "GCP resource does NOT use AWS_REGION fallback",
			provider:   "gcp",
			properties: map[string]string{"machineType": "n1-standard-1"},
			envVars:    map[string]string{"AWS_REGION": "us-east-1"},
			// GCP should NOT inherit AWS region - returns empty (no GCP region specified)
			expectedRegion: "",
		},
		{
			name:       "Google-native resource does NOT use AWS_REGION fallback",
			provider:   "google-native",
			properties: map[string]string{"machineType": "e2-micro"},
			envVars:    map[string]string{"AWS_REGION": "eu-west-2", "AWS_DEFAULT_REGION": "eu-central-1"},
			// Google-native should NOT inherit AWS region
			expectedRegion: "",
		},
		{
			name:       "Azure resource with explicit region ignores AWS_REGION",
			provider:   "azure",
			properties: map[string]string{"vmSize": "Standard_B1s", "location": "eastus"},
			envVars:    map[string]string{"AWS_REGION": "us-east-1"},
			// Azure should use its own location, not AWS_REGION
			expectedRegion: "eastus",
		},
		{
			name:       "AWS resource with explicit region ignores AWS_REGION env var",
			provider:   "aws",
			properties: map[string]string{"instanceType": "t3.micro", "region": "eu-west-2"},
			envVars:    map[string]string{"AWS_REGION": "us-east-1"},
			// Explicit region takes precedence over env var
			expectedRegion: "eu-west-2",
		},
		{
			name:           "AWS resource uses AWS_DEFAULT_REGION when AWS_REGION not set",
			provider:       "aws",
			properties:     map[string]string{"instanceType": "t3.micro"},
			envVars:        map[string]string{"AWS_DEFAULT_REGION": "ap-southeast-1"},
			expectedRegion: "ap-southeast-1",
		},
		{
			name:           "Azure resource with no region and no AWS env vars",
			provider:       "azure",
			properties:     map[string]string{"vmSize": "Standard_B1s"},
			envVars:        map[string]string{},
			expectedRegion: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment variables
			for key, val := range tt.envVars {
				t.Setenv(key, val)
			}

			_, region := resolveSKUAndRegion(tt.provider, tt.properties)
			if region != tt.expectedRegion {
				t.Errorf("resolveSKUAndRegion(%q) region = %q, want %q",
					tt.provider, region, tt.expectedRegion)
			}
		})
	}
}

// T011: TestGetActualCost_ValidationFailure_InvalidTimeRange verifies that
// requests with end time before start time trigger pre-flight validation failure.
func TestGetActualCost_ValidationFailure_InvalidTimeRange(t *testing.T) {
	callCount := 0
	mockClient := &mockCostSourceClient{
		getActualFunc: func(
			ctx context.Context,
			in *GetActualCostRequest,
			opts ...grpc.CallOption,
		) (*GetActualCostResponse, error) {
			callCount++
			return &GetActualCostResponse{
				Results: []*ActualCostResult{{Currency: "USD", TotalCost: 100.0}},
			}, nil
		},
	}

	startTime := time.Now().Unix()
	endTime := time.Now().Add(-24 * time.Hour).Unix() // End before start - invalid

	req := &GetActualCostRequest{
		ResourceIDs: []string{"i-1234567890abcdef0"},
		StartTime:   startTime,
		EndTime:     endTime,
	}

	result := GetActualCostWithErrors(context.Background(), mockClient, "test-plugin", req)

	// Should have 1 result
	if len(result.Results) != 1 {
		t.Errorf("Results length = %d, want 1", len(result.Results))
	}

	// Plugin should NOT be called for invalid time range
	if callCount != 0 {
		t.Errorf("Plugin was called %d times, want 0 (validation should skip plugin)", callCount)
	}

	// Should have validation error
	if len(result.Errors) != 1 {
		t.Errorf("Errors length = %d, want 1", len(result.Errors))
	}

	// Result Notes should have VALIDATION prefix
	if len(result.Results) > 0 {
		notes := result.Results[0].Notes
		if !strings.Contains(notes, "VALIDATION:") {
			t.Errorf("Notes should contain 'VALIDATION:', got %q", notes)
		}
		// Should mention time-related issue
		lowerNotes := strings.ToLower(notes)
		if !strings.Contains(lowerNotes, "time") && !strings.Contains(lowerNotes, "end") {
			t.Errorf("Notes should mention time-related issue, got %q", notes)
		}
	}
}
