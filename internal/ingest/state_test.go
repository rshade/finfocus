package ingest_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
          "ami": "ami-12345"
        },
        "outputs": {
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
