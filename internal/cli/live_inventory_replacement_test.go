package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

const replacementPreview = `{"steps":[
 {"op":"create-replacement","urn":"urn:pulumi:dev::p::aws:ec2/instance:Instance::web",
  "oldState":{"type":"aws:ec2/instance:Instance","inputs":{"instanceType":"t3.small","ami":"ami-old"}},
  "newState":{"type":"aws:ec2/instance:Instance","inputs":{"instanceType":"t3.small","ami":"ami-new"}}},
 {"op":"replace","urn":"urn:pulumi:dev::p::aws:ec2/instance:Instance::web",
  "oldState":{"type":"aws:ec2/instance:Instance","inputs":{"instanceType":"t3.small","ami":"ami-old"}},
  "newState":{"type":"aws:ec2/instance:Instance","inputs":{"instanceType":"t3.small","ami":"ami-new"}}},
 {"op":"delete-replaced","urn":"urn:pulumi:dev::p::aws:ec2/instance:Instance::web",
  "oldState":{"type":"aws:ec2/instance:Instance","inputs":{"instanceType":"t3.small","ami":"ami-old"}}},
 {"op":"create","urn":"urn:pulumi:dev::p::aws:ec2/instance:Instance::other",
  "newState":{"type":"aws:ec2/instance:Instance","inputs":{"instanceType":"t3.micro"}}}
]}`

func writeReplacementPreview(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "preview.json")
	require.NoError(t, os.WriteFile(path, []byte(replacementPreview), 0o600))
	return path
}

func assertOneDescriptorPerURN(t *testing.T, resources []engine.ResourceDescriptor) {
	t.Helper()
	require.Len(t, resources, 2, "the replacement is one descriptor, plus the unrelated create")
	byID := map[string]engine.ResourceDescriptor{}
	for _, resource := range resources {
		byID[resource.ID] = resource
	}
	web := byID["urn:pulumi:dev::p::aws:ec2/instance:Instance::web"]
	assert.Equal(t, "ami-new", web.Properties["ami"], "keeps the replacement's new state")
	assert.Equal(t, "ami-old", web.OldProperties["ami"], "keeps the old state as OldProperties")
	assert.Contains(t, byID, "urn:pulumi:dev::p::aws:ec2/instance:Instance::other")
}

// Not parallel: it sets the process environment (t.Setenv).
func TestEstimateInventoryCountsReplacementOnce(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	resources, _, err := loadEstimateResources(context.Background(), &cobra.Command{}, CostEstimateParams{
		PulumiJSONPath: writeReplacementPreview(t),
	})
	require.NoError(t, err)
	assertOneDescriptorPerURN(t, resources)
}

// Not parallel: it sets the process environment (t.Setenv).
func TestRecommendationsInventoryCountsReplacementOnce(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	resources, _, err := loadRecommendationsResources(context.Background(), &cobra.Command{},
		costRecommendationsParams{pulumiJSONPath: writeReplacementPreview(t)}, nil)
	require.NoError(t, err)
	assertOneDescriptorPerURN(t, resources)
}
