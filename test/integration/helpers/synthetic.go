package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
)

// pulumiStep mirrors the JSON structure of a single Pulumi preview step.
type pulumiStep struct {
	Op       string                 `json:"op"`
	URN      string                 `json:"urn"`
	Type     string                 `json:"type"`
	Provider string                 `json:"provider"`
	Inputs   map[string]interface{} `json:"inputs"`
	Outputs  map[string]interface{} `json:"outputs"`
}

// pulumiPlan mirrors the top-level Pulumi preview JSON structure.
type pulumiPlan struct {
	Steps []pulumiStep `json:"steps"`
}

// GenerateSyntheticPlan creates a valid Pulumi preview JSON file with the
// specified number of resources, cycling through the provided resource types.
// It writes the file to t.TempDir() and returns the file path.
func GenerateSyntheticPlan(t *testing.T, count int, resourceTypes []string) string {
	t.Helper()

	require.Positive(t, count, "resource count must be positive")
	require.NotEmpty(t, resourceTypes, "resource types must not be empty")

	steps := make([]pulumiStep, 0, count)
	for i := range count {
		resType := resourceTypes[i%len(resourceTypes)]
		providerName := extractProviderPrefix(resType)

		steps = append(steps, pulumiStep{
			Op:       "create",
			URN:      fmt.Sprintf("urn:pulumi:dev::synthetic-project::%s::resource-%d", resType, i),
			Type:     resType,
			Provider: fmt.Sprintf("urn:pulumi:dev::synthetic-project::pulumi:providers:%s::default", providerName),
			Inputs: map[string]interface{}{
				"instanceType":     "t3.micro",
				"region":           "us-east-1",
				"availabilityZone": "us-east-1a",
				"tags": map[string]interface{}{
					"Name":        fmt.Sprintf("synthetic-resource-%d", i),
					"Environment": "test",
				},
			},
			Outputs: map[string]interface{}{},
		})
	}

	plan := pulumiPlan{Steps: steps}
	data, err := json.MarshalIndent(plan, "", "  ")
	require.NoError(t, err, "failed to marshal synthetic plan")

	dir := t.TempDir()
	filePath := filepath.Join(dir, "synthetic-plan.json")
	err = os.WriteFile(filePath, data, 0o600)
	require.NoError(t, err, "failed to write synthetic plan file")

	return filePath
}

// extractProviderPrefix extracts the provider name from a resource type string.
// For example, "aws:ec2/instance:Instance" returns "aws".
func extractProviderPrefix(resourceType string) string {
	for i, c := range resourceType {
		if c == ':' {
			return resourceType[:i]
		}
	}
	return resourceType
}

// GenerateSyntheticStack creates a slice of AnalyzerResource objects with
// unique URNs and valid properties for analyzer concurrency testing.
func GenerateSyntheticStack(count int) []*pulumirpc.AnalyzerResource {
	resourceTypes := []string{
		"aws:ec2/instance:Instance",
		"aws:s3/bucket:Bucket",
		"aws:rds/instance:Instance",
		"aws:lambda/function:Function",
	}

	resources := make([]*pulumirpc.AnalyzerResource, 0, count)
	for i := range count {
		resType := resourceTypes[i%len(resourceTypes)]
		providerName := extractProviderPrefix(resType)
		resourceName := fmt.Sprintf("test-resource-%d", i)

		props, _ := structpb.NewStruct(map[string]interface{}{
			"instanceType":     "t3.micro",
			"region":           "us-east-1",
			"availabilityZone": "us-east-1a",
		})

		resources = append(resources, &pulumirpc.AnalyzerResource{
			Type:       resType,
			Name:       resourceName,
			Urn:        fmt.Sprintf("urn:pulumi:dev::synthetic-project::%s::%s", resType, resourceName),
			Properties: props,
			Provider: &pulumirpc.AnalyzerProviderResource{
				Type: fmt.Sprintf("pulumi:providers:%s", providerName),
				Urn:  fmt.Sprintf("urn:pulumi:dev::synthetic-project::pulumi:providers:%s::default", providerName),
			},
		})
	}

	return resources
}
