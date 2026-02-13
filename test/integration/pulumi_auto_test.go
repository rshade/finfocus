package integration_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/ingest"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

// TestPulumiAutoDetection verifies the full detection → resolve stack →
// parse flow using a fixture Pulumi project. The test requires the pulumi
// CLI to be installed; it skips otherwise.
func TestPulumiAutoDetection(t *testing.T) {
	t.Setenv("FINFOCUS_LOG_LEVEL", "error")

	// Skip if pulumi CLI not available
	if _, err := exec.LookPath("pulumi"); err != nil {
		t.Skip("pulumi CLI not installed, skipping integration test")
	}

	t.Run("FindBinary returns path", func(t *testing.T) {
		path, err := pulumidetect.FindBinary()
		require.NoError(t, err)
		assert.NotEmpty(t, path)
	})

	t.Run("FindProject detects fixture", func(t *testing.T) {
		// Create a temporary fixture Pulumi project
		dir := t.TempDir()
		pulumiYAML := filepath.Join(dir, "Pulumi.yaml")
		err := os.WriteFile(pulumiYAML, []byte("name: test-fixture\nruntime: yaml\n"), 0644)
		require.NoError(t, err)

		projectDir, findErr := pulumidetect.FindProject(dir)
		require.NoError(t, findErr)
		assert.Equal(t, dir, projectDir)
	})

	t.Run("FindProject detects from subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		pulumiYAML := filepath.Join(dir, "Pulumi.yaml")
		err := os.WriteFile(pulumiYAML, []byte("name: test-fixture\nruntime: yaml\n"), 0644)
		require.NoError(t, err)

		subDir := filepath.Join(dir, "src", "infra")
		require.NoError(t, os.MkdirAll(subDir, 0755))

		projectDir, findErr := pulumidetect.FindProject(subDir)
		require.NoError(t, findErr)
		assert.Equal(t, dir, projectDir)
	})

	t.Run("FindProject returns error when no project", func(t *testing.T) {
		dir := t.TempDir()

		_, err := pulumidetect.FindProject(dir)
		require.Error(t, err)
		assert.ErrorIs(t, err, pulumidetect.ErrNoProject)
	})

	t.Run("ParsePulumiPlan from bytes", func(t *testing.T) {
		planJSON := []byte(`{
			"steps": [
				{
					"op": "create",
					"urn": "urn:pulumi:dev::test::aws:s3/bucket:Bucket::my-bucket",
					"newState": {
						"type": "aws:s3/bucket:Bucket",
						"inputs": {"bucket": "my-bucket", "region": "us-east-1"}
					}
				}
			]
		}`)

		plan, err := ingest.ParsePulumiPlanWithContext(context.Background(), planJSON)
		require.NoError(t, err)
		require.NotNil(t, plan)

		resources := plan.GetResourcesWithContext(context.Background())
		assert.NotEmpty(t, resources)

		mapped, mapErr := ingest.MapResources(resources)
		require.NoError(t, mapErr)
		assert.NotEmpty(t, mapped)
	})

	t.Run("ParseStackExport from bytes", func(t *testing.T) {
		stateJSON := []byte(`{
			"version": 3,
			"deployment": {
				"manifest": {"time": "2025-01-01T00:00:00Z"},
				"resources": [
					{
						"urn": "urn:pulumi:dev::test::pulumi:pulumi:Stack::test-dev",
						"type": "pulumi:pulumi:Stack",
						"custom": false
					},
					{
						"urn": "urn:pulumi:dev::test::aws:s3/bucket:Bucket::my-bucket",
						"type": "aws:s3/bucket:Bucket",
						"custom": true,
						"id": "my-bucket-id",
						"inputs": {"bucket": "my-bucket"},
						"outputs": {"arn": "arn:aws:s3:::my-bucket"}
					}
				]
			}
		}`)

		state, err := ingest.ParseStackExportWithContext(context.Background(), stateJSON)
		require.NoError(t, err)
		require.NotNil(t, state)

		customResources := state.GetCustomResourcesWithContext(context.Background())
		assert.NotEmpty(t, customResources)

		mapped, mapErr := ingest.MapStateResources(customResources)
		require.NoError(t, mapErr)
		assert.NotEmpty(t, mapped)
	})
}
