package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

// Installs the Pulumi runner and changes process environment.
func TestProjectedLiveInventory(t *testing.T) {
	state, err := os.ReadFile("testdata/pulumi/stack-export-dev.json")
	require.NoError(t, err)
	plan, err := os.ReadFile("testdata/pulumi/preview-default.json")
	require.NoError(t, err)
	project := t.TempDir()
	writePulumiProject(t, project, false)
	t.Chdir(project)
	installFakePulumi(t)
	t.Cleanup(pulumidetect.SetRunnerForTest(scriptedPulumi{export: state, preview: plan}))
	cmd := &cobra.Command{}
	cmd.Flags().String("stack", "dev", "")
	cmd.SetErr(&bytes.Buffer{})
	resources, unchanged, err := loadProjectedResources(
		context.Background(),
		cmd,
		costProjectedParams{},
		nil,
	)
	require.NoError(t, err)
	assert.False(t, unchanged)
	var priceable []engine.ResourceDescriptor
	for _, r := range resources {
		if !engine.IsPulumiInternalType(r.Type) {
			priceable = append(priceable, r)
		}
	}
	require.Len(t, priceable, 5, "live inventory must include unchanged state resources")
	assert.Equal(t, "update", priceable[0].Operation)
	for _, r := range priceable[1:] {
		assert.Equal(t, "same", r.Operation)
	}
}

//nolint:paralleltest // Uses the Pulumi state fixture and command flags.
func TestProjectedStateOnlyIsUnchanged(t *testing.T) {
	state, err := filepath.Abs("testdata/pulumi/stack-export-dev.json")
	require.NoError(t, err)
	cmd := &cobra.Command{}
	resources, unchanged, err := loadProjectedResources(
		context.Background(),
		cmd,
		costProjectedParams{pulumiState: state},
		newAuditContext(context.Background(), "test", nil),
	)
	require.NoError(t, err)
	assert.False(t, unchanged)
	require.Len(t, resources, 6)
	for _, r := range resources {
		assert.Equal(t, "same", r.Operation)
	}
}
