package helpers_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/test/integration/helpers"
)

// pulumiPlanJSON mirrors the top-level Pulumi preview JSON structure for parsing verification.
type pulumiPlanJSON struct {
	Steps []pulumiStepJSON `json:"steps"`
}

// pulumiStepJSON mirrors a single step in the Pulumi preview JSON.
type pulumiStepJSON struct {
	Op       string                 `json:"op"`
	URN      string                 `json:"urn"`
	Type     string                 `json:"type"`
	Provider string                 `json:"provider"`
	Inputs   map[string]interface{} `json:"inputs"`
	Outputs  map[string]interface{} `json:"outputs"`
}

func TestGenerateSyntheticPlan(t *testing.T) {
	t.Run("generates correct resource count", func(t *testing.T) {
		resourceTypes := []string{
			"aws:ec2/instance:Instance",
			"aws:s3/bucket:Bucket",
		}

		filePath := helpers.GenerateSyntheticPlan(t, 10, resourceTypes)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err, "should read generated plan file")

		var plan pulumiPlanJSON
		err = json.Unmarshal(data, &plan)
		require.NoError(t, err, "generated plan should be valid JSON")
		assert.Len(t, plan.Steps, 10, "should generate exactly 10 resources")
	})

	t.Run("cycles through resource types", func(t *testing.T) {
		resourceTypes := []string{
			"aws:ec2/instance:Instance",
			"aws:s3/bucket:Bucket",
			"aws:rds/instance:Instance",
		}

		filePath := helpers.GenerateSyntheticPlan(t, 6, resourceTypes)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var plan pulumiPlanJSON
		err = json.Unmarshal(data, &plan)
		require.NoError(t, err)

		// Verify cycling: 0=ec2, 1=s3, 2=rds, 3=ec2, 4=s3, 5=rds
		assert.Equal(t, "aws:ec2/instance:Instance", plan.Steps[0].Type)
		assert.Equal(t, "aws:s3/bucket:Bucket", plan.Steps[1].Type)
		assert.Equal(t, "aws:rds/instance:Instance", plan.Steps[2].Type)
		assert.Equal(t, "aws:ec2/instance:Instance", plan.Steps[3].Type)
		assert.Equal(t, "aws:s3/bucket:Bucket", plan.Steps[4].Type)
		assert.Equal(t, "aws:rds/instance:Instance", plan.Steps[5].Type)
	})

	t.Run("generates valid URNs", func(t *testing.T) {
		resourceTypes := []string{"aws:ec2/instance:Instance"}

		filePath := helpers.GenerateSyntheticPlan(t, 3, resourceTypes)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var plan pulumiPlanJSON
		err = json.Unmarshal(data, &plan)
		require.NoError(t, err)

		for i, step := range plan.Steps {
			assert.True(t, strings.HasPrefix(step.URN, "urn:pulumi:"), "step %d URN should start with urn:pulumi:", i)
			assert.Contains(t, step.URN, "aws:ec2/instance:Instance", "step %d URN should contain resource type", i)
			assert.Equal(t, "create", step.Op, "step %d op should be create", i)
		}
	})

	t.Run("includes valid inputs", func(t *testing.T) {
		resourceTypes := []string{"aws:ec2/instance:Instance"}

		filePath := helpers.GenerateSyntheticPlan(t, 1, resourceTypes)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var plan pulumiPlanJSON
		err = json.Unmarshal(data, &plan)
		require.NoError(t, err)
		require.Len(t, plan.Steps, 1)

		step := plan.Steps[0]
		assert.Equal(t, "t3.micro", step.Inputs["instanceType"])
		assert.Equal(t, "us-east-1", step.Inputs["region"])
		assert.Equal(t, "us-east-1a", step.Inputs["availabilityZone"])
		assert.NotNil(t, step.Inputs["tags"], "should include tags")
	})

	t.Run("extracts provider prefix for provider field", func(t *testing.T) {
		resourceTypes := []string{"aws:ec2/instance:Instance"}

		filePath := helpers.GenerateSyntheticPlan(t, 1, resourceTypes)

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var plan pulumiPlanJSON
		err = json.Unmarshal(data, &plan)
		require.NoError(t, err)
		require.Len(t, plan.Steps, 1)

		assert.Contains(t, plan.Steps[0].Provider, "pulumi:providers:aws")
	})

	t.Run("single resource type", func(t *testing.T) {
		filePath := helpers.GenerateSyntheticPlan(t, 5, []string{"aws:lambda/function:Function"})

		data, err := os.ReadFile(filePath)
		require.NoError(t, err)

		var plan pulumiPlanJSON
		err = json.Unmarshal(data, &plan)
		require.NoError(t, err)
		assert.Len(t, plan.Steps, 5)

		for _, step := range plan.Steps {
			assert.Equal(t, "aws:lambda/function:Function", step.Type)
		}
	})

	t.Run("file is written to temp directory", func(t *testing.T) {
		filePath := helpers.GenerateSyntheticPlan(t, 1, []string{"aws:ec2/instance:Instance"})

		_, err := os.Stat(filePath)
		require.NoError(t, err, "generated file should exist on disk")
		assert.True(t, strings.HasSuffix(filePath, "synthetic-plan.json"))
	})
}

func TestGenerateSyntheticStack(t *testing.T) {
	t.Run("generates correct resource count", func(t *testing.T) {
		resources := helpers.GenerateSyntheticStack(10)
		assert.Len(t, resources, 10)
	})

	t.Run("resources have valid URNs", func(t *testing.T) {
		resources := helpers.GenerateSyntheticStack(5)

		for i, res := range resources {
			assert.True(t, strings.HasPrefix(res.GetUrn(), "urn:pulumi:"),
				"resource %d URN should start with urn:pulumi:", i)
			assert.NotEmpty(t, res.GetName(), "resource %d should have a name", i)
			assert.NotEmpty(t, res.GetType(), "resource %d should have a type", i)
		}
	})

	t.Run("resources cycle through types", func(t *testing.T) {
		resources := helpers.GenerateSyntheticStack(8)

		// Types cycle: ec2, s3, rds, lambda, ec2, s3, rds, lambda
		assert.Equal(t, "aws:ec2/instance:Instance", resources[0].GetType())
		assert.Equal(t, "aws:s3/bucket:Bucket", resources[1].GetType())
		assert.Equal(t, "aws:rds/instance:Instance", resources[2].GetType())
		assert.Equal(t, "aws:lambda/function:Function", resources[3].GetType())
		assert.Equal(t, "aws:ec2/instance:Instance", resources[4].GetType())
	})

	t.Run("resources have properties", func(t *testing.T) {
		resources := helpers.GenerateSyntheticStack(1)
		require.Len(t, resources, 1)

		res := resources[0]
		require.NotNil(t, res.GetProperties(), "resource should have properties")

		fields := res.GetProperties().GetFields()
		assert.Contains(t, fields, "instanceType")
		assert.Contains(t, fields, "region")
		assert.Contains(t, fields, "availabilityZone")
	})

	t.Run("resources have provider info", func(t *testing.T) {
		resources := helpers.GenerateSyntheticStack(1)
		require.Len(t, resources, 1)

		res := resources[0]
		require.NotNil(t, res.GetProvider(), "resource should have provider")
		assert.Contains(t, res.GetProvider().GetType(), "pulumi:providers:")
		assert.Contains(t, res.GetProvider().GetUrn(), "urn:pulumi:")
	})

	t.Run("unique names and URNs", func(t *testing.T) {
		resources := helpers.GenerateSyntheticStack(50)

		names := make(map[string]bool)
		urns := make(map[string]bool)
		for _, res := range resources {
			name := res.GetName()
			urn := res.GetUrn()
			assert.False(t, names[name], "duplicate name: %s", name)
			assert.False(t, urns[urn], "duplicate URN: %s", urn)
			names[name] = true
			urns[urn] = true
		}
	})

	t.Run("zero count returns empty slice", func(t *testing.T) {
		resources := helpers.GenerateSyntheticStack(0)
		assert.Empty(t, resources)
	})
}
