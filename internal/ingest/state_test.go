package ingest_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// Test fixture: minimal Pulumi state with timestamps.
const stateWithTimestamps = `{
  "version": 3,
  "deployment": {
    "manifest": {
      "time": "2024-01-15T10:30:00.000Z",
      "magic": "test-magic",
      "version": "v3.100.0"
    },
    "resources": [
      {
        "urn": "urn:pulumi:dev::myproject::pulumi:pulumi:Stack::myproject-dev",
        "type": "pulumi:pulumi:Stack",
        "custom": false
      },
      {
        "urn": "urn:pulumi:dev::myproject::aws:ec2/instance:Instance::web",
        "type": "aws:ec2/instance:Instance",
        "id": "i-0abc123def456",
        "custom": true,
        "external": false,
        "created": "2024-01-15T10:30:00Z",
        "modified": "2024-06-20T14:22:00Z",
        "inputs": {
          "instanceType": "t3.micro",
          "ami": "ami-12345",
          "tags": {
            "Name": "web-server",
            "Environment": "dev"
          }
        },
        "outputs": {
          "arn": "arn:aws:ec2:us-east-1:123456789012:instance/i-0abc123def456",
          "publicIp": "54.123.45.67"
        }
      },
      {
        "urn": "urn:pulumi:dev::myproject::aws:s3/bucket:Bucket::data",
        "type": "aws:s3/bucket:Bucket",
        "id": "my-bucket-12345",
        "custom": true,
        "external": true,
        "created": "2024-12-24T00:00:00Z",
        "inputs": {
          "bucket": "my-bucket-12345"
        }
      }
    ]
  }
}`

// Test fixture: state without timestamps (pre-v3.60.0).
const stateWithoutTimestamps = `{
  "version": 3,
  "deployment": {
    "manifest": {
      "time": "2023-01-01T00:00:00.000Z"
    },
    "resources": [
      {
        "urn": "urn:pulumi:dev::myproject::aws:ec2/instance:Instance::web",
        "type": "aws:ec2/instance:Instance",
        "custom": true,
        "inputs": {
          "instanceType": "t3.micro"
        }
      }
    ]
  }
}`

func TestLoadStackExport(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tests := []struct {
		name          string
		content       string
		expectError   bool
		errorContains string
		validate      func(t *testing.T, state *ingest.StackExport)
	}{
		{
			name:        "valid state with timestamps",
			content:     stateWithTimestamps,
			expectError: false,
			validate: func(t *testing.T, state *ingest.StackExport) {
				assert.Equal(t, 3, state.Version)
				assert.Len(t, state.Deployment.Resources, 3)
				assert.True(t, state.HasTimestamps())
			},
		},
		{
			name:        "valid state without timestamps",
			content:     stateWithoutTimestamps,
			expectError: false,
			validate: func(t *testing.T, state *ingest.StackExport) {
				assert.Equal(t, 3, state.Version)
				assert.Len(t, state.Deployment.Resources, 1)
				assert.False(t, state.HasTimestamps())
			},
		},
		{
			name:          "invalid JSON",
			content:       `{invalid json`,
			expectError:   true,
			errorContains: "parsing state JSON",
		},
		{
			name:        "empty state",
			content:     `{"version": 3, "deployment": {"manifest": {}, "resources": []}}`,
			expectError: false,
			validate: func(t *testing.T, state *ingest.StackExport) {
				assert.Len(t, state.Deployment.Resources, 0)
				assert.False(t, state.HasTimestamps())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test content to temp file
			tmpDir := t.TempDir()
			statePath := filepath.Join(tmpDir, "state.json")
			err := os.WriteFile(statePath, []byte(tt.content), 0o600)
			require.NoError(t, err)

			// Load state
			state, err := ingest.LoadStackExport(statePath)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, state)
			if tt.validate != nil {
				tt.validate(t, state)
			}
		})
	}
}

func TestLoadStackExport_FileNotFound(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	_, err := ingest.LoadStackExport("/nonexistent/path/state.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading state file")
}

func TestGetCustomResources(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	err := os.WriteFile(statePath, []byte(stateWithTimestamps), 0o600)
	require.NoError(t, err)

	state, err := ingest.LoadStackExport(statePath)
	require.NoError(t, err)

	resources := state.GetCustomResources()

	// Should only return custom resources (ec2 instance and s3 bucket)
	assert.Len(t, resources, 2)

	// Verify resource types
	types := make([]string, len(resources))
	for i, r := range resources {
		types[i] = r.Type
	}
	assert.Contains(t, types, "aws:ec2/instance:Instance")
	assert.Contains(t, types, "aws:s3/bucket:Bucket")
}

func TestMapStateResource(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	created := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	modified := time.Date(2024, 6, 20, 14, 22, 0, 0, time.UTC)

	tests := []struct {
		name     string
		resource ingest.StackExportResource
		validate func(t *testing.T, desc engine.ResourceDescriptor)
	}{
		{
			name: "resource with timestamps",
			resource: ingest.StackExportResource{
				URN:      "urn:pulumi:dev::project::aws:ec2/instance:Instance::web",
				Type:     "aws:ec2/instance:Instance",
				Custom:   true,
				External: false,
				Created:  &created,
				Modified: &modified,
				Inputs: map[string]interface{}{
					"instanceType": "t3.micro",
				},
			},
			validate: func(t *testing.T, desc engine.ResourceDescriptor) {
				// Original inputs preserved
				assert.Equal(t, "t3.micro", desc.Properties["instanceType"])

				// Timestamps injected
				assert.Equal(t, "2024-01-15T10:30:00Z", desc.Properties[ingest.PropertyPulumiCreated])
				assert.Equal(t, "2024-06-20T14:22:00Z", desc.Properties[ingest.PropertyPulumiModified])

				// External flag not set (false)
				_, hasExternal := desc.Properties[ingest.PropertyPulumiExternal]
				assert.False(t, hasExternal)
			},
		},
		{
			name: "imported resource (external)",
			resource: ingest.StackExportResource{
				URN:      "urn:pulumi:dev::project::aws:s3/bucket:Bucket::data",
				Type:     "aws:s3/bucket:Bucket",
				Custom:   true,
				External: true,
				Created:  &created,
				Inputs: map[string]interface{}{
					"bucket": "my-bucket",
				},
			},
			validate: func(t *testing.T, desc engine.ResourceDescriptor) {
				// External flag set
				assert.Equal(t, "true", desc.Properties[ingest.PropertyPulumiExternal])

				// Created timestamp present
				assert.Equal(t, "2024-01-15T10:30:00Z", desc.Properties[ingest.PropertyPulumiCreated])

				// Modified not present (nil in input)
				_, hasModified := desc.Properties[ingest.PropertyPulumiModified]
				assert.False(t, hasModified)
			},
		},
		{
			name: "resource without timestamps",
			resource: ingest.StackExportResource{
				URN:    "urn:pulumi:dev::project::aws:ec2/instance:Instance::old",
				Type:   "aws:ec2/instance:Instance",
				Custom: true,
				Inputs: map[string]interface{}{
					"instanceType": "t2.micro",
				},
			},
			validate: func(t *testing.T, desc engine.ResourceDescriptor) {
				// Original inputs preserved
				assert.Equal(t, "t2.micro", desc.Properties["instanceType"])

				// No timestamps injected
				_, hasCreated := desc.Properties[ingest.PropertyPulumiCreated]
				_, hasModified := desc.Properties[ingest.PropertyPulumiModified]
				assert.False(t, hasCreated)
				assert.False(t, hasModified)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := ingest.MapStateResource(tt.resource)
			require.NoError(t, err)

			tt.validate(t, desc)
		})
	}
}

func TestMapStateResources(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	err := os.WriteFile(statePath, []byte(stateWithTimestamps), 0o600)
	require.NoError(t, err)

	state, err := ingest.LoadStackExport(statePath)
	require.NoError(t, err)

	customResources := state.GetCustomResources()
	descriptors, err := ingest.MapStateResources(customResources)
	require.NoError(t, err)

	assert.Len(t, descriptors, 2)

	// Find the EC2 instance
	var ec2Desc *struct {
		Type       string
		Properties map[string]interface{}
	}
	for _, d := range descriptors {
		if d.Type == "aws:ec2/instance:Instance" {
			ec2Desc = &struct {
				Type       string
				Properties map[string]interface{}
			}{
				Type:       d.Type,
				Properties: d.Properties,
			}
			break
		}
	}
	require.NotNil(t, ec2Desc)

	// Verify timestamp injection
	assert.Equal(t, "2024-01-15T10:30:00Z", ec2Desc.Properties[ingest.PropertyPulumiCreated])
	assert.Equal(t, "2024-06-20T14:22:00Z", ec2Desc.Properties[ingest.PropertyPulumiModified])
}

func TestGetResourceByURN(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	err := os.WriteFile(statePath, []byte(stateWithTimestamps), 0o600)
	require.NoError(t, err)

	state, err := ingest.LoadStackExport(statePath)
	require.NoError(t, err)

	tests := []struct {
		name     string
		urn      string
		expected bool
	}{
		{
			name:     "existing resource",
			urn:      "urn:pulumi:dev::myproject::aws:ec2/instance:Instance::web",
			expected: true,
		},
		{
			name:     "non-existing resource",
			urn:      "urn:pulumi:dev::myproject::aws:ec2/instance:Instance::nonexistent",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := state.GetResourceByURN(tt.urn)
			if tt.expected {
				require.NotNil(t, resource)
				assert.Equal(t, tt.urn, resource.URN)
			} else {
				assert.Nil(t, resource)
			}
		})
	}
}

func TestLoadStackExportWithContext(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	err := os.WriteFile(statePath, []byte(stateWithTimestamps), 0o600)
	require.NoError(t, err)

	ctx := context.Background()
	state, err := ingest.LoadStackExportWithContext(ctx, statePath)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, 3, state.Version)
}

func TestGetCustomResourcesWithContext(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")
	err := os.WriteFile(statePath, []byte(stateWithTimestamps), 0o600)
	require.NoError(t, err)

	state, err := ingest.LoadStackExport(statePath)
	require.NoError(t, err)

	ctx := context.Background()
	resources := state.GetCustomResourcesWithContext(ctx)
	assert.Len(t, resources, 2)
}

func TestMapStateResource_CloudIdentifiers(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	created := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		resource ingest.StackExportResource
		validate func(t *testing.T, desc engine.ResourceDescriptor)
	}{
		{
			name: "cloud ID and ARN injected from state",
			resource: ingest.StackExportResource{
				URN:    "urn:pulumi:dev::project::aws:ec2/instance:Instance::web",
				Type:   "aws:ec2/instance:Instance",
				ID:     "i-0abc123def456",
				Custom: true,
				Inputs: map[string]interface{}{
					"instanceType": "t3.micro",
				},
				Outputs: map[string]interface{}{
					"arn":      "arn:aws:ec2:us-east-1:123456789012:instance/i-0abc123def456",
					"publicIp": "54.123.45.67",
				},
				Created: &created,
			},
			validate: func(t *testing.T, desc engine.ResourceDescriptor) {
				// Cloud ID injected
				assert.Equal(t, "i-0abc123def456", desc.Properties[ingest.PropertyPulumiCloudID])

				// ARN injected from outputs
				assert.Equal(t, "arn:aws:ec2:us-east-1:123456789012:instance/i-0abc123def456",
					desc.Properties[ingest.PropertyPulumiARN])

				// URN preserved
				assert.Equal(t, "urn:pulumi:dev::project::aws:ec2/instance:Instance::web",
					desc.Properties[ingest.PropertyPulumiURN])

				// ID field still uses URN for display/correlation
				assert.Equal(t, "urn:pulumi:dev::project::aws:ec2/instance:Instance::web", desc.ID)

				// Original inputs preserved
				assert.Equal(t, "t3.micro", desc.Properties["instanceType"])

				// Outputs merged into properties — publicIp now accessible
				assert.Equal(t, "54.123.45.67", desc.Properties["publicIp"])
			},
		},
		{
			name: "provider-computed defaults from outputs (EBS volume)",
			resource: ingest.StackExportResource{
				URN:    "urn:pulumi:dev::project::aws:ebs/volume:Volume::data",
				Type:   "aws:ebs/volume:Volume",
				ID:     "vol-0abc123",
				Custom: true,
				Inputs: map[string]interface{}{
					"availabilityZone": "us-east-1a",
					"snapshotId":       "snap-0123456789abcdef0",
				},
				Outputs: map[string]interface{}{
					"arn":  "arn:aws:ec2:us-east-1:123:volume/vol-0abc123",
					"size": float64(100),
					"iops": float64(3000),
					"type": "gp3",
				},
			},
			validate: func(t *testing.T, desc engine.ResourceDescriptor) {
				// Provider-computed values from outputs available in properties
				assert.Equal(t, float64(100), desc.Properties["size"])
				assert.Equal(t, float64(3000), desc.Properties["iops"])
				assert.Equal(t, "gp3", desc.Properties["type"])

				// Inputs still present
				assert.Equal(t, "us-east-1a", desc.Properties["availabilityZone"])
				assert.Equal(t, "snap-0123456789abcdef0", desc.Properties["snapshotId"])

				// ARN extracted into namespaced key
				assert.Equal(t, "arn:aws:ec2:us-east-1:123:volume/vol-0abc123",
					desc.Properties[ingest.PropertyPulumiARN])
			},
		},
		{
			name: "resource without outputs still gets cloud ID",
			resource: ingest.StackExportResource{
				URN:    "urn:pulumi:dev::project::aws:s3/bucket:Bucket::data",
				Type:   "aws:s3/bucket:Bucket",
				ID:     "my-bucket-12345",
				Custom: true,
				Inputs: map[string]interface{}{
					"bucket": "my-bucket-12345",
				},
			},
			validate: func(t *testing.T, desc engine.ResourceDescriptor) {
				// Cloud ID injected from state ID
				assert.Equal(t, "my-bucket-12345", desc.Properties[ingest.PropertyPulumiCloudID])

				// No ARN (no outputs)
				_, hasARN := desc.Properties[ingest.PropertyPulumiARN]
				assert.False(t, hasARN)

				// URN still preserved
				assert.Equal(t, "urn:pulumi:dev::project::aws:s3/bucket:Bucket::data",
					desc.Properties[ingest.PropertyPulumiURN])
			},
		},
		{
			name: "resource without cloud ID (empty ID field)",
			resource: ingest.StackExportResource{
				URN:    "urn:pulumi:dev::project::aws:ec2/instance:Instance::pending",
				Type:   "aws:ec2/instance:Instance",
				ID:     "",
				Custom: true,
				Inputs: map[string]interface{}{
					"instanceType": "t3.micro",
				},
			},
			validate: func(t *testing.T, desc engine.ResourceDescriptor) {
				// No cloud ID injected (empty)
				_, hasCloudID := desc.Properties[ingest.PropertyPulumiCloudID]
				assert.False(t, hasCloudID)

				// URN still preserved
				assert.Equal(t, "urn:pulumi:dev::project::aws:ec2/instance:Instance::pending",
					desc.Properties[ingest.PropertyPulumiURN])
			},
		},
		{
			name: "tags extracted from inputs",
			resource: ingest.StackExportResource{
				URN:    "urn:pulumi:dev::project::aws:ec2/instance:Instance::tagged",
				Type:   "aws:ec2/instance:Instance",
				ID:     "i-tagged123",
				Custom: true,
				Inputs: map[string]interface{}{
					"instanceType": "t3.micro",
					"tags": map[string]interface{}{
						"Name":        "tagged-instance",
						"Environment": "dev",
					},
				},
			},
			validate: func(t *testing.T, desc engine.ResourceDescriptor) {
				// Tags should be present in properties
				tags, ok := desc.Properties["tags"]
				require.True(t, ok, "tags should exist in properties")

				tagMap, ok := tags.(map[string]interface{})
				require.True(t, ok, "tags should be a map")
				assert.Equal(t, "tagged-instance", tagMap["Name"])
				assert.Equal(t, "dev", tagMap["Environment"])
			},
		},
		{
			name: "tagsAll from outputs takes precedence",
			resource: ingest.StackExportResource{
				URN:    "urn:pulumi:dev::project::aws:ec2/launchTemplate:LaunchTemplate::nodes",
				Type:   "aws:ec2/launchTemplate:LaunchTemplate",
				ID:     "lt-0fedcba987654",
				Custom: true,
				Inputs: map[string]interface{}{
					"tags": map[string]interface{}{
						"Name": "nodes",
					},
				},
				Outputs: map[string]interface{}{
					"arn": "arn:aws:ec2:us-west-2:123456789012:launch-template/lt-0fedcba987654",
					"tagsAll": map[string]interface{}{
						"Name":      "nodes",
						"ManagedBy": "pulumi",
					},
				},
			},
			validate: func(t *testing.T, desc engine.ResourceDescriptor) {
				// tagsAll should be present (from outputs)
				tagsAll, ok := desc.Properties["tagsAll"]
				require.True(t, ok, "tagsAll should exist in properties")

				tagMap, ok := tagsAll.(map[string]interface{})
				require.True(t, ok, "tagsAll should be a map")
				assert.Equal(t, "nodes", tagMap["Name"])
				assert.Equal(t, "pulumi", tagMap["ManagedBy"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := ingest.MapStateResource(tt.resource)
			require.NoError(t, err)
			tt.validate(t, desc)
		})
	}
}

// --- ParseStackExport tests (T013) ---

func TestParseStackExport(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	tests := []struct {
		name          string
		data          []byte
		wantErr       bool
		errContains   string
		wantVersion   int
		wantResources int
		wantTimestamp bool
	}{
		{
			name:          "valid JSON with timestamps",
			data:          []byte(stateWithTimestamps),
			wantVersion:   3,
			wantResources: 3,
			wantTimestamp: true,
		},
		{
			name:        "invalid JSON",
			data:        []byte(`{invalid json`),
			wantErr:     true,
			errContains: "parsing state JSON",
		},
		{
			name:        "empty bytes",
			data:        []byte(""),
			wantErr:     true,
			errContains: "parsing state JSON",
		},
		{
			name:          "empty state",
			data:          []byte(`{"version": 3, "deployment": {"manifest": {}, "resources": []}}`),
			wantVersion:   3,
			wantResources: 0,
			wantTimestamp: false,
		},
		{
			name:          "without timestamps",
			data:          []byte(stateWithoutTimestamps),
			wantVersion:   3,
			wantResources: 1,
			wantTimestamp: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := ingest.ParseStackExport(tt.data)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, state)
			if tt.wantVersion != 0 {
				assert.Equal(t, tt.wantVersion, state.Version)
			}
			assert.Len(t, state.Deployment.Resources, tt.wantResources)
			assert.Equal(t, tt.wantTimestamp, state.HasTimestamps())
		})
	}
}

func TestParseStackExportWithContext(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	ctx := context.Background()
	data := []byte(stateWithTimestamps)
	state, err := ingest.ParseStackExportWithContext(ctx, data)
	require.NoError(t, err)
	require.NotNil(t, state)
	assert.Equal(t, 3, state.Version)
}

// TestLoadStackExport_DelegationEquivalence verifies that LoadStackExport and
// ParseStackExport produce identical results for each fixture file.
func TestLoadStackExport_DelegationEquivalence(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")

	fixtures := []string{
		filepath.Join(repoRoot, "test", "fixtures", "state", "valid-state.json"),
		filepath.Join(repoRoot, "test", "fixtures", "state", "no-timestamps.json"),
		filepath.Join(repoRoot, "test", "fixtures", "state", "imported-resources.json"),
		filepath.Join(repoRoot, "test", "fixtures", "state", "multi-provider.json"),
		filepath.Join(repoRoot, "test", "fixtures", "state", "golden-eks-state.json"),
	}

	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			data, err := os.ReadFile(fixture)
			require.NoError(t, err)

			parsedState, parseErr := ingest.ParseStackExport(data)
			loadedState, loadErr := ingest.LoadStackExport(fixture)

			assert.Equal(t, parseErr != nil, loadErr != nil)
			if parseErr == nil && loadErr == nil {
				assert.Equal(t, parsedState, loadedState)
			}
		})
	}
}

func TestMapStateResource_GoldenEKSState(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	statePath := "../../test/fixtures/state/golden-eks-state.json"

	state, err := ingest.LoadStackExport(statePath)
	require.NoError(t, err)
	require.NotNil(t, state)

	assert.Equal(t, 3, state.Version)
	assert.True(t, state.HasTimestamps())

	customResources := state.GetCustomResources()
	// Provider + IAM role + EKS cluster + SG + launch template + bastion = 6
	assert.Len(t, customResources, 6)

	resources, err := ingest.MapStateResources(customResources)
	require.NoError(t, err)
	assert.Len(t, resources, 6)

	// Verify EKS cluster has cloud ID and ARN
	var eksCluster *engine.ResourceDescriptor
	for i := range resources {
		if resources[i].Type == "aws:eks/cluster:Cluster" {
			eksCluster = &resources[i]
			break
		}
	}
	require.NotNil(t, eksCluster, "should find EKS cluster resource")
	assert.Equal(t, "main-eks-cluster", eksCluster.Properties[ingest.PropertyPulumiCloudID])
	assert.Equal(t, "arn:aws:eks:us-west-2:123456789012:cluster/main-eks-cluster",
		eksCluster.Properties[ingest.PropertyPulumiARN])

	// Verify bastion host is marked as external and has cloud ID
	var bastion *engine.ResourceDescriptor
	for i := range resources {
		if resources[i].Type == "aws:ec2/instance:Instance" {
			bastion = &resources[i]
			break
		}
	}
	require.NotNil(t, bastion, "should find bastion instance")
	assert.Equal(t, "i-0bastion1234abcde", bastion.Properties[ingest.PropertyPulumiCloudID])
	assert.Equal(t, "true", bastion.Properties[ingest.PropertyPulumiExternal])
}
