package analyzer

import (
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestMapResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resource *pulumirpc.AnalyzerResource
		wantType string
		wantID   string
		wantProv string
	}{
		{
			name: "AWS EC2 instance with provider",
			resource: &pulumirpc.AnalyzerResource{
				Type: "aws:ec2/instance:Instance",
				Urn:  "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
				Name: "webserver",
				Provider: &pulumirpc.AnalyzerProviderResource{
					Type: "pulumi:providers:aws",
					Urn:  "urn:pulumi:dev::myapp::pulumi:providers:aws::default",
				},
			},
			wantType: "aws:ec2/instance:Instance",
			wantID:   "webserver",
			wantProv: "aws",
		},
		{
			name: "Azure VM without explicit provider",
			resource: &pulumirpc.AnalyzerResource{
				Type: "azure:compute/virtualMachine:VirtualMachine",
				Urn:  "urn:pulumi:prod::api::azure:compute/virtualMachine:VirtualMachine::apiserver",
				Name: "apiserver",
			},
			wantType: "azure:compute/virtualMachine:VirtualMachine",
			wantID:   "apiserver",
			wantProv: "azure",
		},
		{
			name: "GCP compute instance",
			resource: &pulumirpc.AnalyzerResource{
				Type: "gcp:compute/instance:Instance",
				Urn:  "urn:pulumi:staging::k8s::gcp:compute/instance:Instance::worker",
				Name: "worker",
				Provider: &pulumirpc.AnalyzerProviderResource{
					Type: "pulumi:providers:gcp",
				},
			},
			wantType: "gcp:compute/instance:Instance",
			wantID:   "worker",
			wantProv: "gcp",
		},
		{
			name: "Pulumi Stack resource",
			resource: &pulumirpc.AnalyzerResource{
				Type: "pulumi:pulumi:Stack",
				Urn:  "urn:pulumi:dev::myapp::pulumi:pulumi:Stack::myapp-dev",
				Name: "myapp-dev",
			},
			wantType: "pulumi:pulumi:Stack",
			wantID:   "myapp-dev",
			wantProv: "pulumi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := MapResource(tt.resource)

			assert.Equal(t, tt.wantType, result.Type)
			assert.Equal(t, tt.wantID, result.ID)
			assert.Equal(t, tt.wantProv, result.Provider)
		})
	}
}

func TestMapResources(t *testing.T) {
	t.Parallel()

	resources := []*pulumirpc.AnalyzerResource{
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
		{
			Type: "aws:s3/bucket:Bucket",
			Urn:  "urn:pulumi:dev::myapp::aws:s3/bucket:Bucket::assets",
			Name: "assets",
		},
	}

	results := MapResources(resources)

	require.Len(t, results, 3)
	assert.Equal(t, "web1", results[0].ID)
	assert.Equal(t, "db1", results[1].ID)
	assert.Equal(t, "assets", results[2].ID)

	// All should have aws provider
	for _, r := range results {
		assert.Equal(t, "aws", r.Provider)
	}
}

func TestMapResources_Empty(t *testing.T) {
	t.Parallel()

	results := MapResources(nil)
	assert.Empty(t, results)

	results = MapResources([]*pulumirpc.AnalyzerResource{})
	assert.Empty(t, results)
}

func TestExtractResourceID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		urn  string
		want string
	}{
		{
			name: "standard URN",
			urn:  "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
			want: "webserver",
		},
		{
			name: "URN with complex name",
			urn:  "urn:pulumi:prod::api::azure:compute/vm:VM::api-server-prod-01",
			want: "api-server-prod-01",
		},
		{
			name: "empty URN",
			urn:  "",
			want: "",
		},
		{
			name: "malformed URN without separators",
			urn:  "just-a-string",
			want: "just-a-string",
		},
		{
			name: "URN with only one separator",
			urn:  "urn::partial",
			want: "partial",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractResourceID(tt.urn)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resource *pulumirpc.AnalyzerResource
		want     string
	}{
		{
			name: "provider from provider resource",
			resource: &pulumirpc.AnalyzerResource{
				Type: "aws:ec2/instance:Instance",
				Provider: &pulumirpc.AnalyzerProviderResource{
					Type: "pulumi:providers:aws",
				},
			},
			want: "aws",
		},
		{
			name: "provider from azure provider resource",
			resource: &pulumirpc.AnalyzerResource{
				Type: "azure:compute/vm:VM",
				Provider: &pulumirpc.AnalyzerProviderResource{
					Type: "pulumi:providers:azure",
				},
			},
			want: "azure",
		},
		{
			name: "provider from resource type (no provider resource)",
			resource: &pulumirpc.AnalyzerResource{
				Type: "aws:s3/bucket:Bucket",
			},
			want: "aws",
		},
		{
			name: "gcp from resource type",
			resource: &pulumirpc.AnalyzerResource{
				Type: "gcp:compute/instance:Instance",
			},
			want: "gcp",
		},
		{
			name: "kubernetes from resource type",
			resource: &pulumirpc.AnalyzerResource{
				Type: "kubernetes:core/v1:Pod",
			},
			want: "kubernetes",
		},
		{
			name: "empty provider resource type falls back to resource type",
			resource: &pulumirpc.AnalyzerResource{
				Type:     "aws:ec2/instance:Instance",
				Provider: &pulumirpc.AnalyzerProviderResource{},
			},
			want: "aws",
		},
		{
			name: "unknown provider format",
			resource: &pulumirpc.AnalyzerResource{
				Type: "",
			},
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractProvider(tt.resource)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStructToMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input *structpb.Struct
		want  map[string]interface{}
	}{
		{
			name:  "nil struct",
			input: nil,
			want:  map[string]interface{}{},
		},
		{
			name: "struct with string value",
			input: func() *structpb.Struct {
				s, _ := structpb.NewStruct(map[string]interface{}{
					"instanceType": "t3.micro",
				})
				return s
			}(),
			want: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		},
		{
			name: "struct with multiple values",
			input: func() *structpb.Struct {
				s, _ := structpb.NewStruct(map[string]interface{}{
					"instanceType":     "t3.micro",
					"allocatedStorage": float64(20),
					"enabled":          true,
				})
				return s
			}(),
			want: map[string]interface{}{
				"instanceType":     "t3.micro",
				"allocatedStorage": float64(20),
				"enabled":          true,
			},
		},
		{
			name: "struct with nested object",
			input: func() *structpb.Struct {
				s, _ := structpb.NewStruct(map[string]interface{}{
					"tags": map[string]interface{}{
						"Name":        "webserver",
						"Environment": "dev",
					},
				})
				return s
			}(),
			want: map[string]interface{}{
				"tags": map[string]interface{}{
					"Name":        "webserver",
					"Environment": "dev",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := structToMap(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapResource_WithProperties(t *testing.T) {
	t.Parallel()

	props, err := structpb.NewStruct(map[string]interface{}{
		"instanceType":     "t3.micro",
		"ami":              "ami-0123456789abcdef0",
		"availabilityZone": "us-east-1a",
	})
	require.NoError(t, err)

	resource := &pulumirpc.AnalyzerResource{
		Type:       "aws:ec2/instance:Instance",
		Urn:        "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver",
		Name:       "webserver",
		Properties: props,
		Provider: &pulumirpc.AnalyzerProviderResource{
			Type: "pulumi:providers:aws",
		},
	}

	result := MapResource(resource)

	assert.Equal(t, "aws:ec2/instance:Instance", result.Type)
	assert.Equal(t, "webserver", result.ID)
	assert.Equal(t, "aws", result.Provider)
	assert.Equal(t, "t3.micro", result.Properties["instanceType"])
	assert.Equal(t, "ami-0123456789abcdef0", result.Properties["ami"])
	assert.Equal(t, "us-east-1a", result.Properties["availabilityZone"])
}

// Phase 5 (US3) - Error Handling Tests

func TestMapResource_UnsupportedResourceType(t *testing.T) {
	t.Parallel()

	// Custom/component resources that have no cost should still map correctly
	resource := &pulumirpc.AnalyzerResource{
		Type: "custom:my-org:CustomWidget",
		Urn:  "urn:pulumi:dev::myapp::custom:my-org:CustomWidget::widget1",
		Name: "widget1",
	}

	result := MapResource(resource)

	// Should still map successfully, just with custom provider
	assert.Equal(t, "custom:my-org:CustomWidget", result.Type)
	assert.Equal(t, "widget1", result.ID)
	assert.Equal(t, "custom", result.Provider)
}

func TestMapResource_EmptyType(t *testing.T) {
	t.Parallel()

	// Resource with empty type should handle gracefully
	resource := &pulumirpc.AnalyzerResource{
		Type: "",
		Urn:  "urn:pulumi:dev::myapp::unknown::resource",
		Name: "resource",
	}

	result := MapResource(resource)

	assert.Empty(t, result.Type)
	assert.Equal(t, "resource", result.ID)
	assert.Equal(t, "unknown", result.Provider)
}

func TestMapResources_WithNilElements(t *testing.T) {
	t.Parallel()

	resources := []*pulumirpc.AnalyzerResource{
		{
			Type: "aws:ec2/instance:Instance",
			Urn:  "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::web1",
			Name: "web1",
		},
		nil, // Nil element should be handled
		{
			Type: "aws:s3/bucket:Bucket",
			Urn:  "urn:pulumi:dev::myapp::aws:s3/bucket:Bucket::bucket1",
			Name: "bucket1",
		},
	}

	results := MapResourcesWithErrors(resources)

	// Should process non-nil resources and report error for nil
	assert.NotEmpty(t, results.Resources)
	// If we have error tracking, verify it captured the nil resource
}

func TestExtractProviderFromRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request *pulumirpc.AnalyzeRequest
		want    string
	}{
		{
			name: "provider from provider resource type",
			request: &pulumirpc.AnalyzeRequest{
				Type: "aws:ec2/instance:Instance",
				Provider: &pulumirpc.AnalyzerProviderResource{
					Type: "pulumi:providers:aws",
				},
			},
			want: "aws",
		},
		{
			name: "azure provider from provider resource",
			request: &pulumirpc.AnalyzeRequest{
				Type: "azure:compute/virtualMachine:VirtualMachine",
				Provider: &pulumirpc.AnalyzerProviderResource{
					Type: "pulumi:providers:azure",
				},
			},
			want: "azure",
		},
		{
			name: "gcp provider from provider resource",
			request: &pulumirpc.AnalyzeRequest{
				Type: "gcp:compute/instance:Instance",
				Provider: &pulumirpc.AnalyzerProviderResource{
					Type: "pulumi:providers:gcp",
				},
			},
			want: "gcp",
		},
		{
			name: "fallback to resource type when no provider",
			request: &pulumirpc.AnalyzeRequest{
				Type: "aws:s3/bucket:Bucket",
			},
			want: "aws",
		},
		{
			name: "fallback when provider resource is empty",
			request: &pulumirpc.AnalyzeRequest{
				Type:     "aws:ec2/instance:Instance",
				Provider: &pulumirpc.AnalyzerProviderResource{},
			},
			want: "aws",
		},
		{
			name: "fallback when provider type is empty string",
			request: &pulumirpc.AnalyzeRequest{
				Type: "kubernetes:core/v1:Pod",
				Provider: &pulumirpc.AnalyzerProviderResource{
					Type: "",
				},
			},
			want: "kubernetes",
		},
		{
			name: "unknown when resource type is empty",
			request: &pulumirpc.AnalyzeRequest{
				Type: "",
			},
			want: "unknown",
		},
		{
			name: "malformed provider type falls back to resource type",
			request: &pulumirpc.AnalyzeRequest{
				Type: "aws:lambda/function:Function",
				Provider: &pulumirpc.AnalyzerProviderResource{
					Type: "invalid", // Not enough colons
				},
			},
			want: "aws",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractProviderFromRequest(tt.request)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractProviderFromType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		resourceType string
		want         string
	}{
		{
			name:         "aws resource type",
			resourceType: "aws:ec2/instance:Instance",
			want:         "aws",
		},
		{
			name:         "azure resource type",
			resourceType: "azure:compute/virtualMachine:VirtualMachine",
			want:         "azure",
		},
		{
			name:         "gcp resource type",
			resourceType: "gcp:compute/instance:Instance",
			want:         "gcp",
		},
		{
			name:         "kubernetes resource type",
			resourceType: "kubernetes:core/v1:Pod",
			want:         "kubernetes",
		},
		{
			name:         "pulumi internal type",
			resourceType: "pulumi:pulumi:Stack",
			want:         "pulumi",
		},
		{
			name:         "empty resource type",
			resourceType: "",
			want:         "unknown",
		},
		{
			name:         "resource type starting with colon",
			resourceType: ":invalid:type",
			want:         "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractProviderFromType(tt.resourceType)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapResourcesWithErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		resources     []*pulumirpc.AnalyzerResource
		wantResources int
		wantSkipped   int
	}{
		{
			name: "all valid resources",
			resources: []*pulumirpc.AnalyzerResource{
				{
					Type: "aws:ec2/instance:Instance",
					Urn:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				},
				{
					Type: "aws:s3/bucket:Bucket",
					Urn:  "urn:pulumi:dev::app::aws:s3/bucket:Bucket::bucket",
				},
			},
			wantResources: 2,
			wantSkipped:   0,
		},
		{
			name: "mixed with nil resources",
			resources: []*pulumirpc.AnalyzerResource{
				{
					Type: "aws:ec2/instance:Instance",
					Urn:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				},
				nil,
				{
					Type: "aws:s3/bucket:Bucket",
					Urn:  "urn:pulumi:dev::app::aws:s3/bucket:Bucket::bucket",
				},
			},
			wantResources: 2,
			wantSkipped:   1,
		},
		{
			name:          "all nil resources",
			resources:     []*pulumirpc.AnalyzerResource{nil, nil},
			wantResources: 0,
			wantSkipped:   2,
		},
		{
			name:          "empty slice",
			resources:     []*pulumirpc.AnalyzerResource{},
			wantResources: 0,
			wantSkipped:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := MapResourcesWithErrors(tt.resources)

			assert.Len(t, result.Resources, tt.wantResources)
			assert.Equal(t, tt.wantSkipped, result.Skipped)
			// Verify error tracking matches skipped count (each nil resource produces one error)
			assert.Len(t, result.Errors, tt.wantSkipped)
		})
	}
}
