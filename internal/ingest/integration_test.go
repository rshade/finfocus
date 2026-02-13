package ingest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// getAWSResourceMappingTestData returns test data for AWS resource mapping integration tests.
func getAWSResourceMappingTestData() []struct {
	name        string
	planContent string
	expected    []ResourceMappingExpectation
} {
	return []struct {
		name        string
		planContent string
		expected    []ResourceMappingExpectation
	}{
		{
			name: "aws_resource_mappings",
			planContent: `{
				"steps": [
					{
						"op": "create",
						"urn": "urn:pulumi:dev::my-app::aws:ec2/instance:Instance::web-server",
						"type": "aws:ec2/instance:Instance",
						"provider": "urn:pulumi:dev::my-app::pulumi:providers:aws::default",
						"inputs": {
							"ami": "ami-0c02fb55956c7d316",
							"instanceType": "t3.micro",
							"tags": {
								"Name": "Web Server",
								"Environment": "dev"
							}
						},
						"outputs": {}
					},
					{
						"op": "create",
						"urn": "urn:pulumi:dev::my-app::aws:s3/bucket:Bucket::static-assets",
						"type": "aws:s3/bucket:Bucket",
						"provider": "urn:pulumi:dev::my-app::pulumi:providers:aws::default",
						"inputs": {
							"bucket": "my-static-assets-bucket",
							"acl": "private"
						},
						"outputs": {}
					},
					{
						"op": "create",
						"urn": "urn:pulumi:dev::my-app::aws:rds/instance:Instance::database",
						"type": "aws:rds/instance:Instance",
						"provider": "urn:pulumi:dev::my-app::pulumi:providers:aws::default",
						"inputs": {
							"allocatedStorage": 20,
							"dbInstanceClass": "db.t3.micro",
							"engine": "postgres"
						},
						"outputs": {}
					}
				]
			}`,
			expected: []ResourceMappingExpectation{
				{
					OriginalType:       "aws:ec2/instance:Instance",
					ExpectedProvider:   "aws",
					Description:        "AWS EC2 Instance",
					RequiredProperties: []string{"instanceType"},
				},
				{
					OriginalType:       "aws:s3/bucket:Bucket",
					ExpectedProvider:   "aws",
					Description:        "AWS S3 Bucket",
					RequiredProperties: []string{"bucket"},
				},
				{
					OriginalType:       "aws:rds/instance:Instance",
					ExpectedProvider:   "aws",
					Description:        "AWS RDS Instance",
					RequiredProperties: []string{"dbInstanceClass", "engine"},
				},
			},
		},
		{
			name: "azure_resource_mappings",
			planContent: `{
				"steps": [
					{
						"op": "create",
						"urn": "urn:pulumi:prod::app::azure:compute/virtualMachine:VirtualMachine::vm1",
						"type": "azure:compute/virtualMachine:VirtualMachine",
						"provider": "urn:pulumi:prod::app::pulumi:providers:azure::default",
						"inputs": {
							"vmSize": "Standard_B1s",
							"adminUsername": "azureuser",
							"location": "East US"
						},
						"outputs": {}
					},
					{
						"op": "create",
						"urn": "urn:pulumi:prod::app::azure:storage/account:Account::storage",
						"type": "azure:storage/account:Account",
						"provider": "urn:pulumi:prod::app::pulumi:providers:azure::default",
						"inputs": {
							"accountTier": "Standard",
							"accountReplicationType": "LRS",
							"resourceGroupName": "my-rg"
						},
						"outputs": {}
					}
				]
			}`,
			expected: []ResourceMappingExpectation{
				{
					OriginalType:       "azure:compute/virtualMachine:VirtualMachine",
					ExpectedProvider:   "azure",
					Description:        "Azure Virtual Machine",
					RequiredProperties: []string{"vmSize"},
				},
				{
					OriginalType:       "azure:storage/account:Account",
					ExpectedProvider:   "azure",
					Description:        "Azure Storage Account",
					RequiredProperties: []string{"accountTier", "accountReplicationType"},
				},
			},
		},
		{
			name: "gcp_resource_mappings",
			planContent: `{
				"steps": [
					{
						"op": "create",
						"urn": "urn:pulumi:staging::project::gcp:compute/instance:Instance::worker",
						"type": "gcp:compute/instance:Instance",
						"provider": "urn:pulumi:staging::project::pulumi:providers:gcp::default",
						"inputs": {
							"machineType": "e2-micro",
							"zone": "us-central1-a",
							"bootDisk": {
								"initializeParams": {
									"image": "debian-cloud/debian-11"
								}
							}
						},
						"outputs": {}
					},
					{
						"op": "create",
						"urn": "urn:pulumi:staging::project::gcp:storage/bucket:Bucket::data",
						"type": "gcp:storage/bucket:Bucket",
						"provider": "urn:pulumi:staging::project::pulumi:providers:gcp::default",
						"inputs": {
							"name": "my-data-bucket",
							"location": "US"
						},
						"outputs": {}
					}
				]
			}`,
			expected: []ResourceMappingExpectation{
				{
					OriginalType:       "gcp:compute/instance:Instance",
					ExpectedProvider:   "gcp",
					Description:        "GCP Compute Instance",
					RequiredProperties: []string{"machineType", "zone"},
				},
				{
					OriginalType:       "gcp:storage/bucket:Bucket",
					ExpectedProvider:   "gcp",
					Description:        "GCP Storage Bucket",
					RequiredProperties: []string{"name"},
				},
			},
		},
		{
			name: "multi_provider_mixed_plan",
			planContent: `{
				"steps": [
					{
						"op": "create",
						"urn": "urn:pulumi:dev::multi::aws:ec2/instance:Instance::web",
						"type": "aws:ec2/instance:Instance",
						"provider": "urn:pulumi:dev::multi::pulumi:providers:aws::default",
						"inputs": {
							"instanceType": "t3.micro",
							"ami": "ami-12345"
						},
						"outputs": {}
					},
					{
						"op": "create",
						"urn": "urn:pulumi:dev::multi::azure:compute/virtualMachine:VirtualMachine::vm",
						"type": "azure:compute/virtualMachine:VirtualMachine",
						"provider": "urn:pulumi:dev::multi::pulumi:providers:azure::default",
						"inputs": {
							"vmSize": "Standard_B1s",
							"location": "East US"
						},
						"outputs": {}
					},
					{
						"op": "create",
						"urn": "urn:pulumi:dev::multi::gcp:compute/instance:Instance::worker",
						"type": "gcp:compute/instance:Instance",
						"provider": "urn:pulumi:dev::multi::pulumi:providers:gcp::default",
						"inputs": {
							"machineType": "e2-micro",
							"zone": "us-west1-a"
						},
						"outputs": {}
					}
				]
			}`,
			expected: []ResourceMappingExpectation{
				{
					OriginalType:       "aws:ec2/instance:Instance",
					ExpectedProvider:   "aws",
					Description:        "AWS EC2 Instance",
					RequiredProperties: []string{"instanceType"},
				},
				{
					OriginalType:       "azure:compute/virtualMachine:VirtualMachine",
					ExpectedProvider:   "azure",
					Description:        "Azure Virtual Machine",
					RequiredProperties: []string{"vmSize"},
				},
				{
					OriginalType:       "gcp:compute/instance:Instance",
					ExpectedProvider:   "gcp",
					Description:        "GCP Compute Instance",
					RequiredProperties: []string{"machineType", "zone"},
				},
			},
		},
	}
}

// TestResourceTypeMappingIntegration tests the complete pipeline from Pulumi JSON to ResourceDescriptor.
// This validates the acceptance criteria resource mapping examples from the GitHub issue.
func TestResourceTypeMappingIntegration(t *testing.T) {
	tests := getAWSResourceMappingTestData()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file with plan content
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "plan.json")

			err := os.WriteFile(tmpFile, []byte(tt.planContent), 0644)
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			// Test the complete pipeline: Load -> GetResources -> MapResources
			plan, err := ingest.LoadPulumiPlan(tmpFile)
			if err != nil {
				t.Fatalf("LoadPulumiPlan() failed: %v", err)
			}

			pulumiResources := plan.GetResources()
			descriptors, err := ingest.MapResources(pulumiResources)
			if err != nil {
				t.Fatalf("MapResources() failed: %v", err)
			}

			// Validate the resource mappings
			if len(descriptors) != len(tt.expected) {
				t.Fatalf(
					"expected %d resource descriptors, got %d",
					len(tt.expected),
					len(descriptors),
				)
			}

			for i, expected := range tt.expected {
				descriptor := descriptors[i]

				// Validate resource type mapping
				if descriptor.Type != expected.OriginalType {
					t.Errorf("resource %d: expected type %s, got %s",
						i, expected.OriginalType, descriptor.Type)
				}

				// Validate provider extraction
				if descriptor.Provider != expected.ExpectedProvider {
					t.Errorf("resource %d: expected provider %s, got %s",
						i, expected.ExpectedProvider, descriptor.Provider)
				}

				// Validate that required properties are preserved
				for _, prop := range expected.RequiredProperties {
					if _, exists := descriptor.Properties[prop]; !exists {
						t.Errorf("resource %d (%s): missing required property %s",
							i, expected.Description, prop)
					}
				}

				// Validate that URN is preserved as ID
				expectedURNSubstring := expected.ExpectedProvider + ":" +
					extractResourceServiceType(expected.OriginalType)
				if !strings.Contains(descriptor.ID, expectedURNSubstring) {
					t.Errorf("resource %d: ID should contain %s, got %s",
						i, expectedURNSubstring, descriptor.ID)
				}
			}
		})
	}
}

// getEdgeCaseTestData returns test data for edge case integration tests.
func getEdgeCaseTestData() []struct {
	name        string
	planContent string
	expectError bool
	validate    func(*testing.T, []engine.ResourceDescriptor)
} {
	return []struct {
		name        string
		planContent string
		expectError bool
		validate    func(*testing.T, []engine.ResourceDescriptor)
	}{
		{
			name: "custom_provider_types",
			planContent: `{
				"steps": [
					{
						"op": "create",
						"urn": "urn:pulumi:dev::app::custom-provider:service/resource:Resource::item",
						"type": "custom-provider:service/resource:Resource",
						"provider": "urn:pulumi:dev::app::pulumi:providers:custom::default",
						"inputs": {
							"customProperty": "value"
						},
						"outputs": {}
					}
				]
			}`,
			expectError: false,
			validate: func(t *testing.T, descriptors []engine.ResourceDescriptor) {
				if len(descriptors) != 1 {
					t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
				}

				desc := descriptors[0]
				if desc.Provider != "custom-provider" {
					t.Errorf("expected provider 'custom-provider', got %s", desc.Provider)
				}
				if desc.Type != "custom-provider:service/resource:Resource" {
					t.Errorf("expected type preserved, got %s", desc.Type)
				}
			},
		},
		{
			name: "kubernetes_resources",
			planContent: `{
				"steps": [
					{
						"op": "create",
						"urn": "urn:pulumi:dev::k8s::kubernetes:apps/v1:Deployment::nginx",
						"type": "kubernetes:apps/v1:Deployment",
						"provider": "urn:pulumi:dev::k8s::pulumi:providers:kubernetes::default",
						"inputs": {
							"metadata": {
								"name": "nginx-deployment"
							},
							"spec": {
								"replicas": 3
							}
						},
						"outputs": {}
					}
				]
			}`,
			expectError: false,
			validate: func(t *testing.T, descriptors []engine.ResourceDescriptor) {
				if len(descriptors) != 1 {
					t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
				}

				desc := descriptors[0]
				if desc.Provider != "kubernetes" {
					t.Errorf("expected provider 'kubernetes', got %s", desc.Provider)
				}

				// Validate nested properties are preserved
				metadata := desc.Properties["metadata"].(map[string]interface{})
				if metadata["name"] != "nginx-deployment" {
					t.Error("nested metadata properties not preserved")
				}
			},
		},
		{
			name: "malformed_resource_types",
			planContent: `{
				"steps": [
					{
						"op": "create",
						"urn": "urn:pulumi:dev::app::malformed::item",
						"type": "",
						"provider": "urn:pulumi:dev::app::pulumi:providers:unknown::default",
						"inputs": {},
						"outputs": {}
					}
				]
			}`,
			// When type is empty in plan, it gets extracted from URN as fallback
			expectError: false,
			validate: func(t *testing.T, descriptors []engine.ResourceDescriptor) {
				if len(descriptors) != 1 {
					t.Fatalf("expected 1 descriptor, got %d", len(descriptors))
				}

				desc := descriptors[0]
				if desc.Provider != "malformed" {
					t.Errorf(
						"expected provider 'malformed' extracted from URN, got %s",
						desc.Provider,
					)
				}
				if desc.Type != "malformed" {
					t.Errorf("expected type 'malformed' extracted from URN, got %s", desc.Type)
				}
			},
		},
	}
}

// TestResourceTypeMappingEdgeCases tests edge cases in resource type mapping.
func TestResourceTypeMappingEdgeCases(t *testing.T) {
	tests := getEdgeCaseTestData()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "plan.json")

			err := os.WriteFile(tmpFile, []byte(tt.planContent), 0644)
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			// Test the pipeline
			plan, err := ingest.LoadPulumiPlan(tmpFile)
			if err != nil {
				t.Fatalf("LoadPulumiPlan() failed: %v", err)
			}

			pulumiResources := plan.GetResources()
			descriptors, err := ingest.MapResources(pulumiResources)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, descriptors)
			}
		})
	}
}

type ResourceMappingExpectation struct {
	OriginalType       string
	ExpectedProvider   string
	Description        string
	RequiredProperties []string
}

// Helper function to extract service type from resource type (e.g., "ec2/instance" from "aws:ec2/instance:Instance").
func extractResourceServiceType(resourceType string) string {
	// Split by first colon, then take everything between first and second colon
	parts := splitString(resourceType, ":")
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// Helper function to split strings (avoiding imports).
func splitString(s, sep string) []string {
	if len(sep) == 0 {
		return []string{s}
	}

	var result []string
	start := 0

	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i = start - 1
		}
	}
	result = append(result, s[start:])
	return result
}
