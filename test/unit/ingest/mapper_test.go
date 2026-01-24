package ingest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// TestMapResource_ValidResource tests mapping a valid Pulumi resource to ResourceDescriptor.
func TestMapResource_ValidResource(t *testing.T) {
	resource := ingest.PulumiResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		Inputs: map[string]interface{}{
			"instanceType": "t3.micro",
			"ami":          "ami-12345678",
		},
	}

	descriptor, err := ingest.MapResource(resource)

	require.NoError(t, err)
	assert.Equal(t, "aws:ec2/instance:Instance", descriptor.Type)
	assert.Equal(t, "aws", descriptor.Provider)
	assert.Equal(t, "t3.micro", descriptor.Properties["instanceType"])
	assert.Equal(t, "ami-12345678", descriptor.Properties["ami"])
}

// TestMapResource_EmptyInputs tests mapping a resource with no inputs.
func TestMapResource_EmptyInputs(t *testing.T) {
	resource := ingest.PulumiResource{
		URN:      "urn:pulumi:dev::app::aws:s3/bucket:Bucket::assets",
		Type:     "aws:s3/bucket:Bucket",
		Provider: "aws",
		Inputs:   map[string]interface{}{},
	}

	descriptor, err := ingest.MapResource(resource)

	require.NoError(t, err)
	assert.Equal(t, "aws:s3/bucket:Bucket", descriptor.Type)
	assert.Equal(t, "aws", descriptor.Provider)
	assert.Empty(t, descriptor.Properties)
}

// TestMapResource_NilInputs tests mapping a resource with nil inputs.
func TestMapResource_NilInputs(t *testing.T) {
	resource := ingest.PulumiResource{
		URN:      "urn:pulumi:dev::app::aws:s3/bucket:Bucket::assets",
		Type:     "aws:s3/bucket:Bucket",
		Provider: "aws",
		Inputs:   nil,
	}

	descriptor, err := ingest.MapResource(resource)

	require.NoError(t, err)
	assert.Equal(t, "aws:s3/bucket:Bucket", descriptor.Type)
	assert.Equal(t, "aws", descriptor.Provider)
	assert.Empty(t, descriptor.Properties)
}

// TestMapResource_ComplexInputs tests mapping a resource with nested inputs.
func TestMapResource_ComplexInputs(t *testing.T) {
	resource := ingest.PulumiResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		Inputs: map[string]interface{}{
			"instanceType": "t3.micro",
			"tags": map[string]interface{}{
				"Name":        "Web Server",
				"Environment": "dev",
			},
			"blockDeviceMappings": []interface{}{
				map[string]interface{}{
					"deviceName": "/dev/sda1",
					"ebs": map[string]interface{}{
						"volumeSize": 30,
						"volumeType": "gp3",
					},
				},
			},
		},
	}

	descriptor, err := ingest.MapResource(resource)

	require.NoError(t, err)
	assert.Equal(t, "aws:ec2/instance:Instance", descriptor.Type)
	assert.Equal(t, "aws", descriptor.Provider)
	assert.Equal(t, "t3.micro", descriptor.Properties["instanceType"])

	// Verify nested structures are preserved as interface{}
	tags, ok := descriptor.Properties["tags"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Web Server", tags["Name"])
}

// TestMapResource_StringProperties tests that all properties are converted to strings.
func TestMapResource_StringProperties(t *testing.T) {
	resource := ingest.PulumiResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		Inputs: map[string]interface{}{
			"stringProp": "value",
			"intProp":    123,
			"floatProp":  45.67,
			"boolProp":   true,
		},
	}

	descriptor, err := ingest.MapResource(resource)

	require.NoError(t, err)
	// All properties should be present
	assert.Contains(t, descriptor.Properties, "stringProp")
	assert.Contains(t, descriptor.Properties, "intProp")
	assert.Contains(t, descriptor.Properties, "floatProp")
	assert.Contains(t, descriptor.Properties, "boolProp")
}

// TestMapResource_DifferentProviders tests mapping resources from different cloud providers.
func TestMapResource_DifferentProviders(t *testing.T) {
	testCases := []struct {
		name             string
		resource         ingest.PulumiResource
		expectedProvider string
		expectedType     string
	}{
		{
			name: "AWS Resource",
			resource: ingest.PulumiResource{
				URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				Type:     "aws:ec2/instance:Instance",
				Provider: "aws",
				Inputs:   map[string]interface{}{},
			},
			expectedProvider: "aws",
			expectedType:     "aws:ec2/instance:Instance",
		},
		{
			name: "Azure Resource",
			resource: ingest.PulumiResource{
				URN:      "urn:pulumi:dev::app::azure:compute/virtualMachine:VirtualMachine::vm",
				Type:     "azure:compute/virtualMachine:VirtualMachine",
				Provider: "azure",
				Inputs:   map[string]interface{}{},
			},
			expectedProvider: "azure",
			expectedType:     "azure:compute/virtualMachine:VirtualMachine",
		},
		{
			name: "GCP Resource",
			resource: ingest.PulumiResource{
				URN:      "urn:pulumi:dev::app::gcp:compute/instance:Instance::worker",
				Type:     "gcp:compute/instance:Instance",
				Provider: "gcp",
				Inputs:   map[string]interface{}{},
			},
			expectedProvider: "gcp",
			expectedType:     "gcp:compute/instance:Instance",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			descriptor, err := ingest.MapResource(tc.resource)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedProvider, descriptor.Provider)
			assert.Equal(t, tc.expectedType, descriptor.Type)
		})
	}
}

// TestMapResources_MultipleResources tests mapping a slice of resources.
func TestMapResources_MultipleResources(t *testing.T) {
	resources := []ingest.PulumiResource{
		{
			URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Inputs: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		},
		{
			URN:      "urn:pulumi:dev::app::aws:s3/bucket:Bucket::assets",
			Type:     "aws:s3/bucket:Bucket",
			Provider: "aws",
			Inputs: map[string]interface{}{
				"bucket": "my-bucket",
			},
		},
		{
			URN:      "urn:pulumi:dev::app::aws:rds/instance:Instance::db",
			Type:     "aws:rds/instance:Instance",
			Provider: "aws",
			Inputs: map[string]interface{}{
				"instanceClass": "db.t3.micro",
			},
		},
	}

	descriptors, err := ingest.MapResources(resources)

	require.NoError(t, err)
	require.Len(t, descriptors, 3)
	assert.Equal(t, "aws:ec2/instance:Instance", descriptors[0].Type)
	assert.Equal(t, "aws:s3/bucket:Bucket", descriptors[1].Type)
	assert.Equal(t, "aws:rds/instance:Instance", descriptors[2].Type)
}

// TestMapResources_EmptySlice tests mapping an empty resource slice.
func TestMapResources_EmptySlice(t *testing.T) {
	resources := []ingest.PulumiResource{}

	descriptors, err := ingest.MapResources(resources)

	require.NoError(t, err)
	assert.Empty(t, descriptors)
}

// TestMapResources_NilSlice tests mapping a nil resource slice.
func TestMapResources_NilSlice(t *testing.T) {
	var resources []ingest.PulumiResource

	descriptors, err := ingest.MapResources(resources)

	require.NoError(t, err)
	assert.Empty(t, descriptors)
}

// TestMapResources_PreservesOrder tests that resource order is maintained.
func TestMapResources_PreservesOrder(t *testing.T) {
	resources := []ingest.PulumiResource{
		{
			URN:      "urn:pulumi:dev::app::aws:s3/bucket:Bucket::first",
			Type:     "aws:s3/bucket:Bucket",
			Provider: "aws",
			Inputs:   map[string]interface{}{},
		},
		{
			URN:      "urn:pulumi:dev::app::aws:s3/bucketPolicy:BucketPolicy::second",
			Type:     "aws:s3/bucketPolicy:BucketPolicy",
			Provider: "aws",
			Inputs:   map[string]interface{}{},
		},
		{
			URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::third",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Inputs:   map[string]interface{}{},
		},
	}

	descriptors, err := ingest.MapResources(resources)

	require.NoError(t, err)
	require.Len(t, descriptors, 3)
	assert.Equal(t, "aws:s3/bucket:Bucket", descriptors[0].Type)
	assert.Equal(t, "aws:s3/bucketPolicy:BucketPolicy", descriptors[1].Type)
	assert.Equal(t, "aws:ec2/instance:Instance", descriptors[2].Type)
}

// TestMapResources_MixedProviders tests mapping resources from multiple providers.
func TestMapResources_MixedProviders(t *testing.T) {
	resources := []ingest.PulumiResource{
		{
			URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Inputs:   map[string]interface{}{},
		},
		{
			URN:      "urn:pulumi:dev::app::azure:compute/virtualMachine:VirtualMachine::vm",
			Type:     "azure:compute/virtualMachine:VirtualMachine",
			Provider: "azure",
			Inputs:   map[string]interface{}{},
		},
		{
			URN:      "urn:pulumi:dev::app::gcp:compute/instance:Instance::worker",
			Type:     "gcp:compute/instance:Instance",
			Provider: "gcp",
			Inputs:   map[string]interface{}{},
		},
	}

	descriptors, err := ingest.MapResources(resources)

	require.NoError(t, err)
	require.Len(t, descriptors, 3)
	assert.Equal(t, "aws", descriptors[0].Provider)
	assert.Equal(t, "azure", descriptors[1].Provider)
	assert.Equal(t, "gcp", descriptors[2].Provider)
}

// TestMapResources_VerifyDescriptorStructure tests that mapped descriptors have correct structure.
func TestMapResources_VerifyDescriptorStructure(t *testing.T) {
	resources := []ingest.PulumiResource{
		{
			URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
			Type:     "aws:ec2/instance:Instance",
			Provider: "aws",
			Inputs: map[string]interface{}{
				"instanceType": "t3.micro",
				"ami":          "ami-12345",
			},
		},
	}

	descriptors, err := ingest.MapResources(resources)

	require.NoError(t, err)
	require.Len(t, descriptors, 1)

	descriptor := descriptors[0]
	assert.Equal(t, "aws:ec2/instance:Instance", descriptor.Type)
	assert.Equal(t, "aws", descriptor.Provider)
	assert.IsType(t, map[string]interface{}{}, descriptor.Properties)
	assert.Len(t, descriptor.Properties, 2)
}

// TestMapResource_ProviderExtraction tests provider extraction from resource type.
func TestMapResource_ProviderExtraction(t *testing.T) {
	testCases := []struct {
		name             string
		resourceType     string
		expectedProvider string
	}{
		{
			name:             "AWS Provider",
			resourceType:     "aws:ec2/instance:Instance",
			expectedProvider: "aws",
		},
		{
			name:             "Azure Provider",
			resourceType:     "azure:compute/virtualMachine:VirtualMachine",
			expectedProvider: "azure",
		},
		{
			name:             "GCP Provider",
			resourceType:     "gcp:compute/instance:Instance",
			expectedProvider: "gcp",
		},
		{
			name:             "Kubernetes Provider",
			resourceType:     "kubernetes:core/v1:Service",
			expectedProvider: "kubernetes",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := ingest.PulumiResource{
				URN:      "urn:pulumi:dev::app::" + tc.resourceType + "::test",
				Type:     tc.resourceType,
				Provider: tc.expectedProvider,
				Inputs:   map[string]interface{}{},
			}

			descriptor, err := ingest.MapResource(resource)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedProvider, descriptor.Provider)
		})
	}
}

// TestMapResource_VerifyEngineDescriptorType tests that mapped result is engine.ResourceDescriptor.
func TestMapResource_VerifyEngineDescriptorType(t *testing.T) {
	resource := ingest.PulumiResource{
		URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
		Type:     "aws:ec2/instance:Instance",
		Provider: "aws",
		Inputs: map[string]interface{}{
			"instanceType": "t3.micro",
		},
	}

	descriptor, err := ingest.MapResource(resource)

	require.NoError(t, err)
	// Verify it's the correct type
	assert.IsType(t, engine.ResourceDescriptor{}, descriptor)
}
