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
				if len(plan.Steps) != 1 {
					t.Errorf("expected 1 step, got %d", len(plan.Steps))
				}
				step := plan.Steps[0]
				if step.Op != "create" {
					t.Errorf("expected op 'create', got '%s'", step.Op)
				}
				if step.Type != "aws:ec2/instance:Instance" {
					t.Errorf("expected type 'aws:ec2/instance:Instance', got '%s'", step.Type)
				}
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
				if len(plan.Steps) != 2 {
					t.Errorf("expected 2 steps, got %d", len(plan.Steps))
				}
				if plan.Steps[0].Op != "create" {
					t.Errorf("expected first step op 'create', got '%s'", plan.Steps[0].Op)
				}
				if plan.Steps[1].Op != "update" {
					t.Errorf("expected second step op 'update', got '%s'", plan.Steps[1].Op)
				}
			},
		},
		{
			name: "empty_plan",
			content: `{
				"steps": []
			}`,
			wantErr: false,
			validate: func(t *testing.T, plan *ingest.PulumiPlan) {
				if len(plan.Steps) != 0 {
					t.Errorf("expected 0 steps, got %d", len(plan.Steps))
				}
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
				if len(plan.Steps) != 0 {
					t.Errorf("expected 0 steps when steps field missing, got %d", len(plan.Steps))
				}
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
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			// Test LoadPulumiPlan
			plan, err := ingest.LoadPulumiPlan(tmpFile)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadPulumiPlan() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf(
						"LoadPulumiPlan() error = %v, want error containing %v",
						err,
						tt.errMsg,
					)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadPulumiPlan() unexpected error = %v", err)
				return
			}

			if plan == nil {
				t.Errorf("LoadPulumiPlan() returned nil plan")
				return
			}

			if tt.validate != nil {
				tt.validate(t, plan)
			}
		})
	}
}

func TestLoadPulumiPlan_FileErrors(t *testing.T) {
	t.Run("nonexistent_file", func(t *testing.T) {
		_, err := ingest.LoadPulumiPlan("/nonexistent/path/file.json")
		if err == nil {
			t.Error("LoadPulumiPlan() expected error for nonexistent file, got nil")
		}
		if !strings.Contains(err.Error(), "reading plan file") {
			t.Errorf("LoadPulumiPlan() error = %v, want error containing 'reading plan file'", err)
		}
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
						Inputs: map[string]interface{}{
							"bucket": "old-bucket",
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
			wantCount: 3, // delete operations should be excluded
			validate: func(t *testing.T, resources []ingest.PulumiResource) {
				ops := make(map[string]bool)
				for _, r := range resources {
					// Check that we can extract operation from the steps that created these resources
					for _, step := range []string{"create", "update", "same"} {
						if strings.Contains(r.URN, step) {
							ops[step] = true
						}
					}
				}
				// Should not contain any delete operations
				for _, r := range resources {
					if strings.Contains(r.URN, "old") {
						t.Error("GetResources() should not include deleted resources")
					}
				}
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
				if !providers["aws"] {
					t.Error("GetResources() should extract 'aws' provider")
				}
				if !providers["azure"] {
					t.Error("GetResources() should extract 'azure' provider")
				}
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

				// Check string value
				if r.Inputs["instanceType"] != "t3.micro" {
					t.Errorf("expected instanceType 't3.micro', got %v", r.Inputs["instanceType"])
				}

				// Check nested map
				tags, ok := r.Inputs["tags"].(map[string]interface{})
				if !ok {
					t.Error("expected tags to be map[string]interface{}")
				} else if tags["Name"] != "Web Server" {
					t.Errorf("expected Name tag 'Web Server', got %v", tags["Name"])
				}

				// Check boolean
				if r.Inputs["enabled"] != true {
					t.Errorf("expected enabled true, got %v", r.Inputs["enabled"])
				}

				// Check number
				if r.Inputs["count"] != float64(1) {
					t.Errorf("expected count 1, got %v", r.Inputs["count"])
				}
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
				// Verify that resources are returned in the same order as defined in steps
				expectedOrder := []string{
					"urn:pulumi:dev::app::aws:s3/bucket:Bucket::bucket",
					"urn:pulumi:dev::app::aws:s3/bucketPolicy:BucketPolicy::policy",
					"urn:pulumi:dev::app::aws:ec2/instance:Instance::web",
				}

				for i, expected := range expectedOrder {
					if i < len(resources) && resources[i].URN != expected {
						t.Errorf(
							"resource ordering not preserved: expected %s at position %d, got %s",
							expected,
							i,
							resources[i].URN,
						)
					}
				}

				// Verify that dependency references in properties are preserved
				policyResource := resources[1]
				bucketRef, ok := policyResource.Inputs["bucket"].(string)
				if !ok || !strings.Contains(bucketRef, "bucket.id") {
					t.Error("dependency reference in bucket policy not preserved")
				}

				webResource := resources[2]
				userData, ok := webResource.Inputs["userData"].(string)
				if !ok || !strings.Contains(userData, "bucket.id") {
					t.Error("dependency reference in EC2 user data not preserved")
				}
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
				// Outputs should be populated from OldState
				if r.Outputs == nil {
					t.Error("expected Outputs to be populated from OldState")
					return
				}
				if r.Outputs["size"] != float64(100) {
					t.Errorf("expected size 100, got %v", r.Outputs["size"])
				}
				if r.Outputs["iops"] != float64(3000) {
					t.Errorf("expected iops 3000, got %v", r.Outputs["iops"])
				}
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
				// Create ops have no outputs
				if r.Outputs != nil {
					t.Errorf("expected nil Outputs for create op, got %v", r.Outputs)
				}
				// Inputs should still be set
				if r.Inputs["instanceType"] != "t3.micro" {
					t.Errorf("expected instanceType t3.micro, got %v", r.Inputs["instanceType"])
				}
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
				r := resources[0]
				// Step-level outputs should win over OldState
				if r.Outputs["publicIp"] != "10.0.0.1" {
					t.Errorf("expected step-level publicIp 10.0.0.1, got %v", r.Outputs["publicIp"])
				}
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
				r := resources[0]
				if r.Outputs == nil {
					t.Error("expected Outputs for same op with OldState")
					return
				}
				if r.Outputs["arn"] != "arn:aws:s3:::my-assets" {
					t.Errorf("expected arn from OldState, got %v", r.Outputs["arn"])
				}
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

			if len(resources) != tt.wantCount {
				t.Errorf(
					"GetResources() returned %d resources, want %d",
					len(resources),
					tt.wantCount,
				)
			}

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
