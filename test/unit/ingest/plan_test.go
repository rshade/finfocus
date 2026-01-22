package ingest_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/ingest"
)

// TestLoadPulumiPlan_ValidPlan tests loading a valid Pulumi plan.
func TestLoadPulumiPlan_ValidPlan(t *testing.T) {
	planJSON := `{
		"steps": [
			{
				"op": "create",
				"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				"type": "aws:ec2/instance:Instance",
				"provider": "urn:pulumi:dev::app::pulumi:providers:aws::default",
				"inputs": {
					"instanceType": "t3.micro",
					"ami": "ami-12345678"
				},
				"outputs": {}
			}
		]
	}`

	path := createPlanFile(t, planJSON)

	plan, err := ingest.LoadPulumiPlan(path)

	require.NoError(t, err)
	require.NotNil(t, plan)
	assert.Len(t, plan.Steps, 1)
	assert.Equal(t, "create", plan.Steps[0].Op)
	assert.Equal(t, "aws:ec2/instance:Instance", plan.Steps[0].Type)
}

// TestLoadPulumiPlan_MultipleSteps tests loading a plan with multiple steps.
func TestLoadPulumiPlan_MultipleSteps(t *testing.T) {
	planJSON := `{
		"steps": [
			{
				"op": "create",
				"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				"type": "aws:ec2/instance:Instance",
				"inputs": {"instanceType": "t3.micro"}
			},
			{
				"op": "update",
				"urn": "urn:pulumi:dev::app::aws:s3/bucket:Bucket::assets",
				"type": "aws:s3/bucket:Bucket",
				"inputs": {"bucket": "my-bucket"}
			},
			{
				"op": "delete",
				"urn": "urn:pulumi:dev::app::aws:rds/instance:Instance::old-db",
				"type": "aws:rds/instance:Instance",
				"inputs": {}
			}
		]
	}`

	path := createPlanFile(t, planJSON)

	plan, err := ingest.LoadPulumiPlan(path)

	require.NoError(t, err)
	assert.Len(t, plan.Steps, 3)
	assert.Equal(t, "create", plan.Steps[0].Op)
	assert.Equal(t, "update", plan.Steps[1].Op)
	assert.Equal(t, "delete", plan.Steps[2].Op)
}

// TestLoadPulumiPlan_EmptyPlan tests loading an empty plan.
func TestLoadPulumiPlan_EmptyPlan(t *testing.T) {
	planJSON := `{"steps": []}`

	path := createPlanFile(t, planJSON)

	plan, err := ingest.LoadPulumiPlan(path)

	require.NoError(t, err)
	assert.Empty(t, plan.Steps)
}

// TestLoadPulumiPlan_NonExistentFile tests error handling for missing file.
func TestLoadPulumiPlan_NonExistentFile(t *testing.T) {
	plan, err := ingest.LoadPulumiPlan("/nonexistent/plan.json")

	assert.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "reading plan file")
}

// TestLoadPulumiPlan_InvalidJSON tests error handling for invalid JSON.
func TestLoadPulumiPlan_InvalidJSON(t *testing.T) {
	planJSON := `{"steps": [`

	path := createPlanFile(t, planJSON)

	plan, err := ingest.LoadPulumiPlan(path)

	assert.Error(t, err)
	assert.Nil(t, plan)
	assert.Contains(t, err.Error(), "parsing plan JSON")
}

// TestLoadPulumiPlan_ComplexInputs tests handling of complex nested inputs.
func TestLoadPulumiPlan_ComplexInputs(t *testing.T) {
	planJSON := `{
		"steps": [
			{
				"op": "create",
				"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				"type": "aws:ec2/instance:Instance",
				"inputs": {
					"instanceType": "t3.micro",
					"tags": {
						"Name": "Web Server",
						"Environment": "dev"
					},
					"blockDeviceMappings": [
						{
							"deviceName": "/dev/sda1",
							"ebs": {
								"volumeSize": 30,
								"volumeType": "gp3"
							}
						}
					],
					"enabled": true,
					"count": 2
				}
			}
		]
	}`

	path := createPlanFile(t, planJSON)

	plan, err := ingest.LoadPulumiPlan(path)

	require.NoError(t, err)
	require.Len(t, plan.Steps, 1)

	inputs := plan.Steps[0].Inputs
	assert.Equal(t, "t3.micro", inputs["instanceType"])

	tags, ok := inputs["tags"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Web Server", tags["Name"])

	assert.Equal(t, true, inputs["enabled"])
	assert.Equal(t, float64(2), inputs["count"])
}

// TestGetResources_FiltersByOperation tests that GetResources filters by operation type.
func TestGetResources_FiltersByOperation(t *testing.T) {
	plan := &ingest.PulumiPlan{
		Steps: []ingest.PulumiStep{
			{
				Op:   "create",
				URN:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				Type: "aws:ec2/instance:Instance",
				Inputs: map[string]interface{}{
					"instanceType": "t3.micro",
				},
			},
			{
				Op:     "delete",
				URN:    "urn:pulumi:dev::app::aws:s3/bucket:Bucket::old",
				Type:   "aws:s3/bucket:Bucket",
				Inputs: map[string]interface{}{},
			},
			{
				Op:   "update",
				URN:  "urn:pulumi:dev::app::aws:rds/instance:Instance::db",
				Type: "aws:rds/instance:Instance",
				Inputs: map[string]interface{}{
					"dbInstanceClass": "db.t3.micro",
				},
			},
			{
				Op:     "same",
				URN:    "urn:pulumi:dev::app::aws:s3/bucket:Bucket::assets",
				Type:   "aws:s3/bucket:Bucket",
				Inputs: map[string]interface{}{},
			},
		},
	}

	resources := plan.GetResources()

	// Should include create, update, and same, but not delete
	assert.Len(t, resources, 3)

	// Verify delete operation was excluded
	for _, r := range resources {
		assert.NotContains(t, r.URN, "old", "Deleted resources should be excluded")
	}
}

// TestGetResources_ExtractsProvider tests provider extraction from URN.
func TestGetResources_ExtractsProvider(t *testing.T) {
	plan := &ingest.PulumiPlan{
		Steps: []ingest.PulumiStep{
			{
				Op:     "create",
				URN:    "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				Type:   "aws:ec2/instance:Instance",
				Inputs: map[string]interface{}{},
			},
			{
				Op:     "create",
				URN:    "urn:pulumi:dev::app::azure:compute/virtualMachine:VirtualMachine::vm",
				Type:   "azure:compute/virtualMachine:VirtualMachine",
				Inputs: map[string]interface{}{},
			},
			{
				Op:     "create",
				URN:    "urn:pulumi:dev::app::gcp:compute/instance:Instance::worker",
				Type:   "gcp:compute/instance:Instance",
				Inputs: map[string]interface{}{},
			},
		},
	}

	resources := plan.GetResources()

	require.Len(t, resources, 3)
	assert.Equal(t, "aws", resources[0].Provider)
	assert.Equal(t, "azure", resources[1].Provider)
	assert.Equal(t, "gcp", resources[2].Provider)
}

// TestGetResources_PreservesInputs tests that resource inputs are preserved.
func TestGetResources_PreservesInputs(t *testing.T) {
	plan := &ingest.PulumiPlan{
		Steps: []ingest.PulumiStep{
			{
				Op:   "create",
				URN:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				Type: "aws:ec2/instance:Instance",
				Inputs: map[string]interface{}{
					"instanceType": "t3.micro",
					"ami":          "ami-12345",
					"tags": map[string]interface{}{
						"Name": "Test",
					},
				},
			},
		},
	}

	resources := plan.GetResources()

	require.Len(t, resources, 1)
	assert.Equal(t, "t3.micro", resources[0].Inputs["instanceType"])
	assert.Equal(t, "ami-12345", resources[0].Inputs["ami"])

	tags, ok := resources[0].Inputs["tags"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Test", tags["Name"])
}

// TestGetResources_EmptyPlan tests behavior with empty plan.
func TestGetResources_EmptyPlan(t *testing.T) {
	plan := &ingest.PulumiPlan{
		Steps: []ingest.PulumiStep{},
	}

	resources := plan.GetResources()

	assert.Empty(t, resources)
}

// TestGetResources_PreservesOrder tests that resource order is preserved.
func TestGetResources_PreservesOrder(t *testing.T) {
	plan := &ingest.PulumiPlan{
		Steps: []ingest.PulumiStep{
			{
				Op:     "create",
				URN:    "urn:pulumi:dev::app::aws:s3/bucket:Bucket::first",
				Type:   "aws:s3/bucket:Bucket",
				Inputs: map[string]interface{}{},
			},
			{
				Op:     "create",
				URN:    "urn:pulumi:dev::app::aws:s3/bucketPolicy:BucketPolicy::second",
				Type:   "aws:s3/bucketPolicy:BucketPolicy",
				Inputs: map[string]interface{}{},
			},
			{
				Op:     "create",
				URN:    "urn:pulumi:dev::app::aws:ec2/instance:Instance::third",
				Type:   "aws:ec2/instance:Instance",
				Inputs: map[string]interface{}{},
			},
		},
	}

	resources := plan.GetResources()

	require.Len(t, resources, 3)
	assert.Contains(t, resources[0].URN, "first")
	assert.Contains(t, resources[1].URN, "second")
	assert.Contains(t, resources[2].URN, "third")
}

// Helper functions

// createPlanFile creates a temporary Pulumi plan JSON file.
func createPlanFile(t *testing.T, content string) string {
	t.Helper()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "plan.json")

	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	return path
}
