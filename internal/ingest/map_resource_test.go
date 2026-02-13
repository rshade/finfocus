package ingest_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

func TestMergeProperties(t *testing.T) {
	tests := []struct {
		name     string
		outputs  map[string]interface{}
		inputs   map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name:     "both nil",
			outputs:  nil,
			inputs:   nil,
			expected: nil,
		},
		{
			name:    "nil outputs",
			outputs: nil,
			inputs:  map[string]interface{}{"instanceType": "t3.micro"},
			expected: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		},
		{
			name:    "nil inputs",
			outputs: map[string]interface{}{"publicIp": "1.2.3.4"},
			inputs:  nil,
			expected: map[string]interface{}{
				"publicIp": "1.2.3.4",
			},
		},
		{
			name:    "inputs win on conflict",
			outputs: map[string]interface{}{"size": float64(100), "iops": float64(3000)},
			inputs:  map[string]interface{}{"size": float64(200)},
			expected: map[string]interface{}{
				"size": float64(200),
				"iops": float64(3000),
			},
		},
		{
			name:    "no conflict merges all keys",
			outputs: map[string]interface{}{"arn": "arn:aws:ec2:us-east-1:123:instance/i-123", "publicIp": "1.2.3.4"},
			inputs:  map[string]interface{}{"instanceType": "t3.micro", "ami": "ami-123"},
			expected: map[string]interface{}{
				"arn":          "arn:aws:ec2:us-east-1:123:instance/i-123",
				"publicIp":     "1.2.3.4",
				"instanceType": "t3.micro",
				"ami":          "ami-123",
			},
		},
		{
			name:     "both empty maps",
			outputs:  map[string]interface{}{},
			inputs:   map[string]interface{}{},
			expected: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ingest.MergeProperties(tt.outputs, tt.inputs)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestMapResource(t *testing.T) {
	tests := []struct {
		name           string
		pulumiResource ingest.PulumiResource
		expected       engine.ResourceDescriptor
		wantErr        bool
	}{
		{
			name: "aws_ec2_instance",
			pulumiResource: ingest.PulumiResource{
				Type:     "aws:ec2/instance:Instance",
				URN:      "urn:pulumi:dev::my-app::aws:ec2/instance:Instance::web-server",
				Provider: "aws",
				Inputs: map[string]interface{}{
					"ami":          "ami-0c02fb55956c7d316",
					"instanceType": "t3.micro",
					"tags": map[string]interface{}{
						"Name":        "Web Server",
						"Environment": "dev",
					},
				},
			},
			expected: engine.ResourceDescriptor{
				Type:     "aws:ec2/instance:Instance",
				ID:       "urn:pulumi:dev::my-app::aws:ec2/instance:Instance::web-server",
				Provider: "aws",
				Properties: map[string]interface{}{
					"ami":          "ami-0c02fb55956c7d316",
					"instanceType": "t3.micro",
					"tags": map[string]interface{}{
						"Name":        "Web Server",
						"Environment": "dev",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "azure_vm",
			pulumiResource: ingest.PulumiResource{
				Type:     "azure:compute/virtualMachine:VirtualMachine",
				URN:      "urn:pulumi:prod::app::azure:compute/virtualMachine:VirtualMachine::vm1",
				Provider: "azure",
				Inputs: map[string]interface{}{
					"vmSize":                        "Standard_B1s",
					"adminUsername":                 "azureuser",
					"disablePasswordAuthentication": true,
				},
			},
			expected: engine.ResourceDescriptor{
				Type:     "azure:compute/virtualMachine:VirtualMachine",
				ID:       "urn:pulumi:prod::app::azure:compute/virtualMachine:VirtualMachine::vm1",
				Provider: "azure",
				Properties: map[string]interface{}{
					"vmSize":                        "Standard_B1s",
					"adminUsername":                 "azureuser",
					"disablePasswordAuthentication": true,
				},
			},
			wantErr: false,
		},
		{
			name: "gcp_instance",
			pulumiResource: ingest.PulumiResource{
				Type:     "gcp:compute/instance:Instance",
				URN:      "urn:pulumi:staging::project::gcp:compute/instance:Instance::worker",
				Provider: "gcp",
				Inputs: map[string]interface{}{
					"machineType": "e2-micro",
					"zone":        "us-central1-a",
					"bootDisk": map[string]interface{}{
						"initializeParams": map[string]interface{}{
							"image": "debian-cloud/debian-11",
							"size":  float64(10),
						},
					},
				},
			},
			expected: engine.ResourceDescriptor{
				Type:     "gcp:compute/instance:Instance",
				ID:       "urn:pulumi:staging::project::gcp:compute/instance:Instance::worker",
				Provider: "gcp",
				Properties: map[string]interface{}{
					"machineType": "e2-micro",
					"zone":        "us-central1-a",
					"bootDisk": map[string]interface{}{
						"initializeParams": map[string]interface{}{
							"image": "debian-cloud/debian-11",
							"size":  float64(10),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "kubernetes_deployment",
			pulumiResource: ingest.PulumiResource{
				Type:     "kubernetes:apps/v1:Deployment",
				URN:      "urn:pulumi:dev::k8s-app::kubernetes:apps/v1:Deployment::nginx",
				Provider: "kubernetes",
				Inputs: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "nginx-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"replicas": float64(3),
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "nginx",
							},
						},
					},
				},
			},
			expected: engine.ResourceDescriptor{
				Type:     "kubernetes:apps/v1:Deployment",
				ID:       "urn:pulumi:dev::k8s-app::kubernetes:apps/v1:Deployment::nginx",
				Provider: "kubernetes",
				Properties: map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      "nginx-deployment",
						"namespace": "default",
					},
					"spec": map[string]interface{}{
						"replicas": float64(3),
						"selector": map[string]interface{}{
							"matchLabels": map[string]interface{}{
								"app": "nginx",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "complex_nested_properties",
			pulumiResource: ingest.PulumiResource{
				Type:     "aws:rds/instance:Instance",
				URN:      "urn:pulumi:dev::app::aws:rds/instance:Instance::database",
				Provider: "aws",
				Inputs: map[string]interface{}{
					"allocatedStorage": float64(20),
					"dbInstanceClass":  "db.t3.micro",
					"engine":           "postgres",
					"engineVersion":    "13.7",
					"parameterGroup": map[string]interface{}{
						"name": "default.postgres13",
						"parameters": map[string]interface{}{
							"shared_preload_libraries": "pg_stat_statements",
							"log_statement":            "all",
						},
					},
					"tags": map[string]interface{}{
						"Name":        "Application Database",
						"Environment": "dev",
						"Backup":      true,
					},
					"enabledCloudwatchLogsExports": []interface{}{"postgresql"},
					"monitoring": map[string]interface{}{
						"interval": float64(60),
						"enabled":  true,
					},
				},
			},
			expected: engine.ResourceDescriptor{
				Type:     "aws:rds/instance:Instance",
				ID:       "urn:pulumi:dev::app::aws:rds/instance:Instance::database",
				Provider: "aws",
				Properties: map[string]interface{}{
					"allocatedStorage": float64(20),
					"dbInstanceClass":  "db.t3.micro",
					"engine":           "postgres",
					"engineVersion":    "13.7",
					"parameterGroup": map[string]interface{}{
						"name": "default.postgres13",
						"parameters": map[string]interface{}{
							"shared_preload_libraries": "pg_stat_statements",
							"log_statement":            "all",
						},
					},
					"tags": map[string]interface{}{
						"Name":        "Application Database",
						"Environment": "dev",
						"Backup":      true,
					},
					"enabledCloudwatchLogsExports": []interface{}{"postgresql"},
					"monitoring": map[string]interface{}{
						"interval": float64(60),
						"enabled":  true,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty_inputs",
			pulumiResource: ingest.PulumiResource{
				Type:     "aws:s3/bucket:Bucket",
				URN:      "urn:pulumi:dev::app::aws:s3/bucket:Bucket::simple-bucket",
				Provider: "aws",
				Inputs:   map[string]interface{}{},
			},
			expected: engine.ResourceDescriptor{
				Type:       "aws:s3/bucket:Bucket",
				ID:         "urn:pulumi:dev::app::aws:s3/bucket:Bucket::simple-bucket",
				Provider:   "aws",
				Properties: map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name: "nil_inputs",
			pulumiResource: ingest.PulumiResource{
				Type:     "aws:s3/bucket:Bucket",
				URN:      "urn:pulumi:dev::app::aws:s3/bucket:Bucket::nil-bucket",
				Provider: "aws",
				Inputs:   nil,
			},
			expected: engine.ResourceDescriptor{
				Type:       "aws:s3/bucket:Bucket",
				ID:         "urn:pulumi:dev::app::aws:s3/bucket:Bucket::nil-bucket",
				Provider:   "aws",
				Properties: nil,
			},
			wantErr: false,
		},
		{
			name: "outputs_merged_with_inputs",
			pulumiResource: ingest.PulumiResource{
				Type:     "aws:ebs/volume:Volume",
				URN:      "urn:pulumi:dev::app::aws:ebs/volume:Volume::data",
				Provider: "aws",
				Inputs: map[string]interface{}{
					"availabilityZone": "us-east-1a",
					"snapshotId":       "snap-0123456789abcdef0",
				},
				Outputs: map[string]interface{}{
					"size": float64(100),
					"iops": float64(3000),
					"arn":  "arn:aws:ec2:us-east-1:123:volume/vol-123",
				},
			},
			expected: engine.ResourceDescriptor{
				Type:     "aws:ebs/volume:Volume",
				ID:       "urn:pulumi:dev::app::aws:ebs/volume:Volume::data",
				Provider: "aws",
				Properties: map[string]interface{}{
					"availabilityZone": "us-east-1a",
					"snapshotId":       "snap-0123456789abcdef0",
					"size":             float64(100),
					"iops":             float64(3000),
					"arn":              "arn:aws:ec2:us-east-1:123:volume/vol-123",
				},
			},
			wantErr: false,
		},
		{
			name: "inputs_override_outputs_on_conflict",
			pulumiResource: ingest.PulumiResource{
				Type:     "aws:ec2/instance:Instance",
				URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::conflict",
				Provider: "aws",
				Inputs: map[string]interface{}{
					"instanceType": "t3.large",
				},
				Outputs: map[string]interface{}{
					"instanceType": "t3.micro",
					"publicIp":     "54.1.2.3",
				},
			},
			expected: engine.ResourceDescriptor{
				Type:     "aws:ec2/instance:Instance",
				ID:       "urn:pulumi:dev::app::aws:ec2/instance:Instance::conflict",
				Provider: "aws",
				Properties: map[string]interface{}{
					"instanceType": "t3.large",
					"publicIp":     "54.1.2.3",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ingest.MapResource(tt.pulumiResource)

			if tt.wantErr {
				if err == nil {
					t.Errorf("MapResource() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("MapResource() unexpected error = %v", err)
				return
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("MapResource() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}

// getMapResourcesTestData returns test data for MapResources function tests.
func getMapResourcesTestData() []struct {
	name      string
	resources []ingest.PulumiResource
	wantCount int
	wantErr   bool
	validate  func(*testing.T, []engine.ResourceDescriptor)
} {
	return []struct {
		name      string
		resources []ingest.PulumiResource
		wantCount int
		wantErr   bool
		validate  func(*testing.T, []engine.ResourceDescriptor)
	}{
		{
			name: "multiple_resources",
			resources: []ingest.PulumiResource{
				{
					Type:     "aws:ec2/instance:Instance",
					URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
					Provider: "aws",
					Inputs: map[string]interface{}{
						"instanceType": "t3.micro",
					},
				},
				{
					Type:     "aws:s3/bucket:Bucket",
					URN:      "urn:pulumi:dev::app::aws:s3/bucket:Bucket::assets",
					Provider: "aws",
					Inputs: map[string]interface{}{
						"bucket": "my-assets-bucket",
					},
				},
			},
			wantCount: 2,
			wantErr:   false,
			validate: func(t *testing.T, descriptors []engine.ResourceDescriptor) {
				// Verify first resource
				if descriptors[0].Type != "aws:ec2/instance:Instance" {
					t.Errorf(
						"expected first resource type 'aws:ec2/instance:Instance', got %s",
						descriptors[0].Type,
					)
				}
				if descriptors[0].Provider != "aws" {
					t.Errorf(
						"expected first resource provider 'aws', got %s",
						descriptors[0].Provider,
					)
				}

				// Verify second resource
				if descriptors[1].Type != "aws:s3/bucket:Bucket" {
					t.Errorf(
						"expected second resource type 'aws:s3/bucket:Bucket', got %s",
						descriptors[1].Type,
					)
				}
				if descriptors[1].Provider != "aws" {
					t.Errorf(
						"expected second resource provider 'aws', got %s",
						descriptors[1].Provider,
					)
				}
			},
		},
		{
			name: "mixed_providers",
			resources: []ingest.PulumiResource{
				{
					Type:     "aws:ec2/instance:Instance",
					URN:      "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
					Provider: "aws",
					Inputs:   map[string]interface{}{"instanceType": "t3.micro"},
				},
				{
					Type:     "azure:compute/virtualMachine:VirtualMachine",
					URN:      "urn:pulumi:dev::app::azure:compute/virtualMachine:VirtualMachine::vm",
					Provider: "azure",
					Inputs:   map[string]interface{}{"vmSize": "Standard_B1s"},
				},
				{
					Type:     "gcp:compute/instance:Instance",
					URN:      "urn:pulumi:dev::app::gcp:compute/instance:Instance::worker",
					Provider: "gcp",
					Inputs:   map[string]interface{}{"machineType": "e2-micro"},
				},
			},
			wantCount: 3,
			wantErr:   false,
			validate: func(t *testing.T, descriptors []engine.ResourceDescriptor) {
				providers := make(map[string]bool)
				for _, desc := range descriptors {
					providers[desc.Provider] = true
				}

				expectedProviders := []string{"aws", "azure", "gcp"}
				for _, provider := range expectedProviders {
					if !providers[provider] {
						t.Errorf("expected provider %s not found in results", provider)
					}
				}
			},
		},
		{
			name:      "empty_list",
			resources: []ingest.PulumiResource{},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "nil_list",
			resources: nil,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "preserve_complex_properties",
			resources: []ingest.PulumiResource{
				{
					Type:     "aws:rds/instance:Instance",
					URN:      "urn:pulumi:dev::app::aws:rds/instance:Instance::db",
					Provider: "aws",
					Inputs: map[string]interface{}{
						"dbInstanceClass": "db.t3.micro",
						"tags": map[string]interface{}{
							"Environment": "dev",
							"Application": "web-app",
						},
						"parameterGroup": map[string]interface{}{
							"parameters": map[string]interface{}{
								"max_connections": float64(100),
								"shared_buffers":  "256MB",
							},
						},
						"backupRetentionPeriod": float64(7),
						"enabledCloudwatchLogsExports": []interface{}{
							"postgresql",
							"upgrade",
						},
					},
				},
			},
			wantCount: 1,
			wantErr:   false,
			validate: func(t *testing.T, descriptors []engine.ResourceDescriptor) {
				desc := descriptors[0]

				// Check that complex nested properties are preserved
				tags := desc.Properties["tags"].(map[string]interface{})
				if tags["Environment"] != "dev" {
					t.Errorf("expected Environment tag 'dev', got %v", tags["Environment"])
				}

				// Check nested object
				paramGroup := desc.Properties["parameterGroup"].(map[string]interface{})
				params := paramGroup["parameters"].(map[string]interface{})
				if params["max_connections"] != float64(100) {
					t.Errorf("expected max_connections 100, got %v", params["max_connections"])
				}

				// Check array
				exports := desc.Properties["enabledCloudwatchLogsExports"].([]interface{})
				if len(exports) != 2 {
					t.Errorf("expected 2 log exports, got %d", len(exports))
				}
				if exports[0] != "postgresql" {
					t.Errorf("expected first export 'postgresql', got %v", exports[0])
				}
			},
		},
	}
}

// TestMapResources tests the mapping of multiple Pulumi resources to engine resource descriptors.
func TestMapResources(t *testing.T) {
	tests := getMapResourcesTestData()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ingest.MapResources(tt.resources)

			if tt.wantErr {
				if err == nil {
					t.Errorf("MapResources() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("MapResources() unexpected error = %v", err)
				return
			}

			if len(result) != tt.wantCount {
				t.Errorf("MapResources() returned %d resources, want %d", len(result), tt.wantCount)
			}

			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}
