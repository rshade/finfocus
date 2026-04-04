package ingest_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/ingest"
)

// getLoadPulumiPlanTestData returns test data for LoadPulumiPlan function tests.
func getLoadPulumiPlanTestData() []struct {
	name     string
	content  string
	wantErr  bool
	errMsg   string
	validate func(*testing.T, *ingest.PulumiPlan)
} {
	return []struct {
		name     string
		content  string
		wantErr  bool
		errMsg   string
		validate func(*testing.T, *ingest.PulumiPlan)
	}{
		{
			name: "valid_simple_plan",
			content: `{
				"steps": [
					{
						"op": "create",
						"urn": "urn:pulumi:dev::my-app::aws:ec2/instance:Instance::web-server",
						"type": "aws:ec2/instance:Instance",
						"provider": "urn:pulumi:dev::my-app::pulumi:providers:aws::default_1_0_0::04da6b54-80e4-46f7-96ec-b56ff0331ba9",
						"inputs": {
							"ami": "ami-0c02fb55956c7d316",
							"instanceType": "t3.micro"
						},
						"outputs": {}
					}
				]
			}`,
			wantErr: false,
			validate: func(t *testing.T, plan *ingest.PulumiPlan) {
				require.Len(t, plan.Steps, 1)
				assert.Equal(t, "create", plan.Steps[0].Op)
				assert.Equal(t, "aws:ec2/instance:Instance", plan.Steps[0].Type)
			},
		},
		{
			name: "valid_multi_step_plan",
			content: `{
				"steps": [
					{
						"op": "create",
						"urn": "urn:pulumi:dev::my-app::aws:ec2/instance:Instance::web-server",
						"type": "aws:ec2/instance:Instance",
						"provider": "urn:pulumi:dev::my-app::pulumi:providers:aws::default",
						"inputs": {"instanceType": "t3.micro"},
						"outputs": {}
					},
					{
						"op": "update",
						"urn": "urn:pulumi:dev::my-app::aws:s3/bucket:Bucket::assets",
						"type": "aws:s3/bucket:Bucket",
						"provider": "urn:pulumi:dev::my-app::pulumi:providers:aws::default",
						"inputs": {"bucket": "my-bucket"},
						"outputs": {}
					}
				]
			}`,
			wantErr: false,
			validate: func(t *testing.T, plan *ingest.PulumiPlan) {
				require.Len(t, plan.Steps, 2)
				assert.Equal(t, "create", plan.Steps[0].Op)
				assert.Equal(t, "update", plan.Steps[1].Op)
			},
		},
		{
			name: "empty_plan",
			content: `{
				"steps": []
			}`,
			wantErr: false,
			validate: func(t *testing.T, plan *ingest.PulumiPlan) {
				assert.Empty(t, plan.Steps)
			},
		},
		{
			name:    "invalid_json",
			content: `{"steps": [`,
			wantErr: true,
			errMsg:  "parsing plan JSON",
		},
		{
			name:    "empty_file",
			content: "",
			wantErr: true,
			errMsg:  "parsing plan JSON",
		},
		{
			name: "missing_steps_field",
			content: `{
				"other": "value"
			}`,
			wantErr: false,
			validate: func(t *testing.T, plan *ingest.PulumiPlan) {
				assert.Empty(t, plan.Steps)
			},
		},
	}
}

// TestLoadPulumiPlan tests loading and parsing Pulumi plan JSON files.
func TestLoadPulumiPlan(t *testing.T) {
	tests := getLoadPulumiPlanTestData()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "plan.json")

			err := os.WriteFile(tmpFile, []byte(tt.content), 0644)
			require.NoError(t, err)

			// Test LoadPulumiPlan
			plan, err := ingest.LoadPulumiPlan(tmpFile)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, plan)

			if tt.validate != nil {
				tt.validate(t, plan)
			}
		})
	}

	t.Run("nonexistent_file", func(t *testing.T) {
		_, err := ingest.LoadPulumiPlan("/nonexistent/path/file.json")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading plan file")
	})
}

// getPulumiPlanGetResourcesTestData returns test data for PulumiPlan GetResources method tests.
func getPulumiPlanGetResourcesTestData() []struct {
	name      string
	plan      *ingest.PulumiPlan
	wantCount int
	validate  func(*testing.T, []ingest.PulumiResource)
} {
	return []struct {
		name      string
		plan      *ingest.PulumiPlan
		wantCount int
		validate  func(*testing.T, []ingest.PulumiResource)
	}{
		{
			name: "mixed_operations",
			plan: &ingest.PulumiPlan{
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
						Op:   "delete",
						URN:  "urn:pulumi:dev::app::aws:s3/bucket:Bucket::old",
						Type: "aws:s3/bucket:Bucket",
						OldState: &ingest.PulumiState{
							ID:   "old-bucket-id",
							Type: "aws:s3/bucket:Bucket",
							Inputs: map[string]interface{}{
								"bucket": "old-bucket",
							},
						},
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
						Op:   "same",
						URN:  "urn:pulumi:dev::app::aws:s3/bucket:Bucket::assets",
						Type: "aws:s3/bucket:Bucket",
						Inputs: map[string]interface{}{
							"bucket": "assets-bucket",
						},
					},
				},
			},
			wantCount: 4, // all operations including delete are now extracted
			validate: func(t *testing.T, resources []ingest.PulumiResource) {
				// Verify delete resource has OldID populated
				var foundDelete bool
				for _, r := range resources {
					if strings.Contains(r.URN, "old") {
						foundDelete = true
						assert.Equal(t, "old-bucket-id", r.OldID,
							"delete resource should have OldID populated")
					}
				}
				assert.True(t, foundDelete, "delete resource should be included")
			},
		},
		{
			name: "provider_extraction",
			plan: &ingest.PulumiPlan{
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
						Op:   "create",
						URN:  "urn:pulumi:dev::app::azure:compute/virtualMachine:VirtualMachine::vm",
						Type: "azure:compute/virtualMachine:VirtualMachine",
						Inputs: map[string]interface{}{
							"vmSize": "Standard_B1s",
						},
					},
				},
			},
			wantCount: 2,
			validate: func(t *testing.T, resources []ingest.PulumiResource) {
				providers := make(map[string]bool)
				for _, r := range resources {
					providers[r.Provider] = true
				}
				assert.True(t, providers["aws"], "GetResources() should extract 'aws' provider")
				assert.True(t, providers["azure"], "GetResources() should extract 'azure' provider")
			},
		},
		{
			name: "complex_inputs",
			plan: &ingest.PulumiPlan{
				Steps: []ingest.PulumiStep{
					{
						Op:   "create",
						URN:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
						Type: "aws:ec2/instance:Instance",
						Inputs: map[string]interface{}{
							"instanceType": "t3.micro",
							"tags": map[string]interface{}{
								"Name":        "Web Server",
								"Environment": "dev",
							},
							"userData": "#!/bin/bash\necho 'Hello World'",
							"count":    float64(1),
							"enabled":  true,
						},
					},
				},
			},
			wantCount: 1,
			validate: func(t *testing.T, resources []ingest.PulumiResource) {
				r := resources[0]
				assert.Equal(t, "t3.micro", r.Inputs["instanceType"])

				tags, ok := r.Inputs["tags"].(map[string]interface{})
				require.True(t, ok, "expected tags to be map[string]interface{}")
				assert.Equal(t, "Web Server", tags["Name"])

				assert.Equal(t, true, r.Inputs["enabled"])
				assert.Equal(t, float64(1), r.Inputs["count"])
			},
		},
		{
			name: "resource_ordering_preservation",
			plan: &ingest.PulumiPlan{
				Steps: []ingest.PulumiStep{
					{
						Op:   "create",
						URN:  "urn:pulumi:dev::app::aws:s3/bucket:Bucket::bucket",
						Type: "aws:s3/bucket:Bucket",
						Inputs: map[string]interface{}{
							"bucket": "my-bucket",
						},
					},
					{
						Op:   "create",
						URN:  "urn:pulumi:dev::app::aws:s3/bucketPolicy:BucketPolicy::policy",
						Type: "aws:s3/bucketPolicy:BucketPolicy",
						Inputs: map[string]interface{}{
							"bucket": "${aws:s3/bucket:Bucket.bucket.id}",
							"policy": "{}",
						},
					},
					{
						Op:   "create",
						URN:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
						Type: "aws:ec2/instance:Instance",
						Inputs: map[string]interface{}{
							"instanceType": "t3.micro",
							"userData":     "aws s3 cp s3://${aws:s3/bucket:Bucket.bucket.id}/script.sh",
						},
					},
				},
			},
			wantCount: 3,
			validate: func(t *testing.T, resources []ingest.PulumiResource) {
				expectedOrder := []string{
					"urn:pulumi:dev::app::aws:s3/bucket:Bucket::bucket",
					"urn:pulumi:dev::app::aws:s3/bucketPolicy:BucketPolicy::policy",
					"urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				}
				for i, expected := range expectedOrder {
					assert.Equal(t, expected, resources[i].URN, "resource ordering not preserved at position %d", i)
				}

				bucketRef, ok := resources[1].Inputs["bucket"].(string)
				require.True(t, ok, "bucket input should be a string")
				assert.Contains(t, bucketRef, "bucket.id", "dependency reference in bucket policy not preserved")

				userData, ok := resources[2].Inputs["userData"].(string)
				require.True(t, ok, "userData input should be a string")
				assert.Contains(t, userData, "bucket.id", "dependency reference in EC2 user data not preserved")
			},
		},
		{
			name:      "empty_plan",
			plan:      &ingest.PulumiPlan{Steps: []ingest.PulumiStep{}},
			wantCount: 0,
		},
		{
			name: "update_op_with_old_state_outputs",
			plan: &ingest.PulumiPlan{
				Steps: []ingest.PulumiStep{
					{
						Op:   "update",
						URN:  "urn:pulumi:dev::app::aws:ebs/volume:Volume::data",
						Type: "aws:ebs/volume:Volume",
						Inputs: map[string]interface{}{
							"availabilityZone": "us-east-1a",
						},
						OldState: &ingest.PulumiState{
							Type: "aws:ebs/volume:Volume",
							Outputs: map[string]interface{}{
								"size": float64(100),
								"iops": float64(3000),
							},
						},
					},
				},
			},
			wantCount: 1,
			validate: func(t *testing.T, resources []ingest.PulumiResource) {
				r := resources[0]
				require.NotNil(t, r.Outputs, "expected Outputs to be populated from OldState")
				assert.Equal(t, float64(100), r.Outputs["size"])
				assert.Equal(t, float64(3000), r.Outputs["iops"])
			},
		},
		{
			name: "create_op_with_no_outputs",
			plan: &ingest.PulumiPlan{
				Steps: []ingest.PulumiStep{
					{
						Op:   "create",
						URN:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::new-web",
						Type: "aws:ec2/instance:Instance",
						Inputs: map[string]interface{}{
							"instanceType": "t3.micro",
						},
					},
				},
			},
			wantCount: 1,
			validate: func(t *testing.T, resources []ingest.PulumiResource) {
				r := resources[0]
				assert.Nil(t, r.Outputs, "expected nil Outputs for create op")
				assert.Equal(t, "t3.micro", r.Inputs["instanceType"])
			},
		},
		{
			name: "step_outputs_take_precedence_over_state",
			plan: &ingest.PulumiPlan{
				Steps: []ingest.PulumiStep{
					{
						Op:   "update",
						URN:  "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
						Type: "aws:ec2/instance:Instance",
						Inputs: map[string]interface{}{
							"instanceType": "t3.micro",
						},
						Outputs: map[string]interface{}{
							"publicIp": "10.0.0.1",
						},
						OldState: &ingest.PulumiState{
							Type: "aws:ec2/instance:Instance",
							Outputs: map[string]interface{}{
								"publicIp": "10.0.0.99",
							},
						},
					},
				},
			},
			wantCount: 1,
			validate: func(t *testing.T, resources []ingest.PulumiResource) {
				assert.Equal(t, "10.0.0.1", resources[0].Outputs["publicIp"],
					"step-level outputs should win over OldState")
			},
		},
		{
			name: "same_op_gets_old_state_outputs",
			plan: &ingest.PulumiPlan{
				Steps: []ingest.PulumiStep{
					{
						Op:   "same",
						URN:  "urn:pulumi:dev::app::aws:s3/bucket:Bucket::assets",
						Type: "aws:s3/bucket:Bucket",
						Inputs: map[string]interface{}{
							"bucket": "my-assets",
						},
						OldState: &ingest.PulumiState{
							Type: "aws:s3/bucket:Bucket",
							Outputs: map[string]interface{}{
								"arn":    "arn:aws:s3:::my-assets",
								"region": "us-east-1",
							},
						},
					},
				},
			},
			wantCount: 1,
			validate: func(t *testing.T, resources []ingest.PulumiResource) {
				require.NotNil(t, resources[0].Outputs, "expected Outputs for same op with OldState")
				assert.Equal(t, "arn:aws:s3:::my-assets", resources[0].Outputs["arn"])
			},
		},
	}
}

// TestPulumiPlan_GetResources tests the GetResources method of PulumiPlan.
func TestPulumiPlan_GetResources(t *testing.T) {
	tests := getPulumiPlanGetResourcesTestData()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := tt.plan.GetResources()
			assert.Len(t, resources, tt.wantCount)

			if tt.validate != nil {
				tt.validate(t, resources)
			}
		})
	}
}

// --- ParsePulumiPlan tests (T012) ---

func TestParsePulumiPlan(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		wantErr     bool
		errContains string
		wantSteps   int
		validate    func(*testing.T, *ingest.PulumiPlan)
	}{
		{
			name: "valid JSON",
			data: []byte(`{
				"steps": [
					{
						"op": "create",
						"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
						"type": "aws:ec2/instance:Instance",
						"inputs": {"instanceType": "t3.micro"}
					}
				]
			}`),
			wantSteps: 1,
			validate: func(t *testing.T, plan *ingest.PulumiPlan) {
				assert.Equal(t, "create", plan.Steps[0].Op)
				assert.Equal(t, "aws:ec2/instance:Instance", plan.Steps[0].Type)
			},
		},
		{
			name:        "invalid JSON",
			data:        []byte(`{not valid json`),
			wantErr:     true,
			errContains: "parsing plan JSON",
		},
		{
			name:        "empty bytes",
			data:        []byte(""),
			wantErr:     true,
			errContains: "parsing plan JSON",
		},
		{
			name:        "nil input",
			data:        nil,
			wantErr:     true,
			errContains: "parsing plan JSON",
		},
		{
			name:      "empty plan",
			data:      []byte(`{"steps": []}`),
			wantSteps: 0,
		},
		{
			name: "multi step",
			data: []byte(`{
				"steps": [
					{
						"op": "create",
						"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
						"type": "aws:ec2/instance:Instance",
						"inputs": {"instanceType": "t3.micro"}
					},
					{
						"op": "update",
						"urn": "urn:pulumi:dev::app::aws:s3/bucket:Bucket::data",
						"type": "aws:s3/bucket:Bucket",
						"inputs": {"bucket": "my-bucket"}
					}
				]
			}`),
			wantSteps: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan, err := ingest.ParsePulumiPlan(tt.data)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			require.NotNil(t, plan)
			assert.Len(t, plan.Steps, tt.wantSteps)
			if tt.validate != nil {
				tt.validate(t, plan)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// T029: PulumiState.ID field deserialization
// ---------------------------------------------------------------------------

func TestPulumiState_IDField_OldAndNewState(t *testing.T) {
	data := []byte(`{
		"steps": [
			{
				"op": "replace",
				"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				"type": "aws:ec2/instance:Instance",
				"oldState": {
					"id": "i-old-abc123",
					"type": "aws:ec2/instance:Instance",
					"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
					"inputs": {"instanceType": "t3.micro"}
				},
				"newState": {
					"id": "i-new-def456",
					"type": "aws:ec2/instance:Instance",
					"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
					"inputs": {"instanceType": "t3.large"}
				}
			}
		]
	}`)

	plan, err := ingest.ParsePulumiPlan(data)
	require.NoError(t, err)
	require.Len(t, plan.Steps, 1)

	step := plan.Steps[0]
	assert.Equal(t, "replace", step.Op)
	require.NotNil(t, step.OldState, "oldState should be deserialized")
	require.NotNil(t, step.NewState, "newState should be deserialized")
	assert.Equal(t, "i-old-abc123", step.OldState.ID, "oldState.id should be deserialized")
	assert.Equal(t, "i-new-def456", step.NewState.ID, "newState.id should be deserialized")
}

func TestPulumiState_IDField_MissingID(t *testing.T) {
	data := []byte(`{
		"steps": [
			{
				"op": "create",
				"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				"type": "aws:ec2/instance:Instance",
				"newState": {
					"type": "aws:ec2/instance:Instance",
					"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
					"inputs": {"instanceType": "t3.micro"}
				}
			}
		]
	}`)

	plan, err := ingest.ParsePulumiPlan(data)
	require.NoError(t, err)
	require.Len(t, plan.Steps, 1)

	step := plan.Steps[0]
	require.NotNil(t, step.NewState)
	assert.Empty(t, step.NewState.ID, "missing id field should result in empty string")
}

func TestPulumiState_IDField_ReplaceProducesBothIDs(t *testing.T) {
	data := []byte(`{
		"steps": [
			{
				"op": "replace",
				"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				"type": "aws:ec2/instance:Instance",
				"oldState": {
					"id": "i-replaced-old",
					"type": "aws:ec2/instance:Instance",
					"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
					"inputs": {}
				},
				"newState": {
					"id": "i-replaced-new",
					"type": "aws:ec2/instance:Instance",
					"urn": "urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
					"inputs": {}
				}
			}
		]
	}`)

	plan, err := ingest.ParsePulumiPlan(data)
	require.NoError(t, err)
	require.Len(t, plan.Steps, 1)

	step := plan.Steps[0]
	require.NotNil(t, step.OldState)
	require.NotNil(t, step.NewState)

	// Both IDs should be non-empty and different
	assert.NotEmpty(t, step.OldState.ID)
	assert.NotEmpty(t, step.NewState.ID)
	assert.NotEqual(t, step.OldState.ID, step.NewState.ID,
		"replace operation should produce different old and new cloud IDs")
}

// TestLoadPulumiPlan_DelegationEquivalence verifies that LoadPulumiPlan and
// ParsePulumiPlan produce identical results for each fixture file.
func TestLoadPulumiPlan_DelegationEquivalence(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	fixtures := []string{
		filepath.Join(repoRoot, "test", "fixtures", "plans", "aws-simple-plan.json"),
		filepath.Join(repoRoot, "test", "fixtures", "plans", "aws-multi-resource-plan.json"),
		filepath.Join(repoRoot, "test", "fixtures", "plans", "azure-simple-plan.json"),
		filepath.Join(repoRoot, "test", "fixtures", "plans", "gcp-simple-plan.json"),
		filepath.Join(repoRoot, "test", "fixtures", "plans", "multi-resource-plan.json"),
	}

	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			data, err := os.ReadFile(fixture)
			require.NoError(t, err)

			parsedPlan, parseErr := ingest.ParsePulumiPlan(data)
			loadedPlan, loadErr := ingest.LoadPulumiPlan(fixture)

			assert.Equal(t, parseErr, loadErr)
			assert.Equal(t, parsedPlan, loadedPlan)
		})
	}
}
