package fixtures_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/test/fixtures"
)

func TestValidatePlan_Structure(t *testing.T) {
	// Test that loaded plans have expected structure
	path := filepath.Join("aws", "simple.json")

	plan, err := fixtures.LoadPlan(path)
	require.NoError(t, err)

	// Validate root keys - Pulumi plan JSON usually has "steps" (deployment) or "resources" (state)
	if _, hasSteps := plan["steps"]; hasSteps {
		steps, ok := plan["steps"].([]interface{})
		require.True(t, ok, "steps should be an array")
		assert.NotEmpty(t, steps, "steps should not be empty")

		if len(steps) > 0 {
			step, ok := steps[0].(map[string]interface{})
			require.True(t, ok, "step should be a map")
			assert.Contains(t, step, "type", "step should have type")
			assert.Contains(t, step, "urn", "step should have urn")
		}
	} else {
		assert.Contains(t, plan, "resources", "Plan should contain resources or steps")

		// Validate resources array
		resources, ok := plan["resources"].([]interface{})
		require.True(t, ok, "resources should be an array")
		assert.NotEmpty(t, resources, "resources should not be empty")

		// Validate resource structure
		if len(resources) > 0 {
			res, ok := resources[0].(map[string]interface{})
			require.True(t, ok, "resource should be a map")
			assert.Contains(t, res, "type", "resource should have type")
			assert.Contains(t, res, "id", "resource should have id")
		}
	}
}

func TestValidateSpec_Structure(t *testing.T) {
	// Test that loaded specs have expected structure
	path := "aws-ec2-t3.medium.yaml"

	spec, err := fixtures.LoadSpec(path)
	require.NoError(t, err)

	// Validate root keys
	assert.Contains(t, spec, "sku", "Spec should contain sku")
	assert.Contains(t, spec, "pricing_details", "Spec should contain pricing_details")
}
