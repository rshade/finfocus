package tui

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

// testGolden compares stripped (no ANSI) output against a golden file.
// Set UPDATE_GOLDEN=1 to regenerate.
func testGolden(t *testing.T, name string, actual string) {
	t.Helper()
	stripped := ansi.Strip(actual)
	golden := filepath.Join("testdata", name+".golden")
	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(golden, []byte(stripped), 0o644))
		return
	}
	expected, err := os.ReadFile(golden)
	require.NoError(t, err, "golden file %s missing; run UPDATE_GOLDEN=1", golden)
	assert.Equal(t, string(expected), stripped,
		"golden mismatch for %s; run UPDATE_GOLDEN=1 to update", name)
}

func estimateGoldenModel(resourceType string, props map[string]any) *EstimateModel {
	return NewEstimateModel(context.Background(), &engine.ResourceDescriptor{
		Provider: "aws", Type: resourceType, ID: "urn:golden", Properties: props,
	}, nil)
}

func TestGoldenEstimateCurated(t *testing.T) {
	t.Parallel()
	model := estimateGoldenModel("aws:ec2/instance:Instance", map[string]any{
		"instanceType": "t3.micro", "tenancy": "default", "ami": "ami-1", "subnetId": "subnet-1",
		"arn": "arn:aws:ec2:us-east-1:1:instance/i-1", "pulumi:cloudId": "i-1",
	})

	testGolden(t, "estimate_curated", model.View().Content)
}

func TestGoldenEstimateAdvanced(t *testing.T) {
	t.Parallel()
	model := estimateGoldenModel("aws:ec2/instance:Instance", map[string]any{
		"instanceType": "t3.micro", "tenancy": "default", "ami": "ami-1", "subnetId": "subnet-1",
		"arn": "arn:aws:ec2:us-east-1:1:instance/i-1", "pulumi:cloudId": "i-1",
	})
	model.toggleShowAll()

	testGolden(t, "estimate_advanced", model.View().Content)
}

func TestGoldenEstimateFallbackNote(t *testing.T) {
	t.Parallel()
	model := estimateGoldenModel("aws:ec2/vpc:Vpc", map[string]any{
		"cidrBlock": "10.0.0.0/16", "arn": "arn:aws:ec2:us-east-1:1:vpc/vpc-1",
	})

	testGolden(t, "estimate_fallback_note", model.View().Content)
}
