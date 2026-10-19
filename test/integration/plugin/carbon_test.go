package plugin_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/greenops"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/spec"
)

// TestCarbonPlugin installs and executes a binary and changes process env.
func TestCarbonPlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and installs the real carbon plugin")
	}
	root, err := filepath.Abs("../../..")
	require.NoError(t, err)
	home := t.TempDir()
	const version = "0.1.0"
	install := filepath.Join(home, "plugins", "carbon", "v"+version)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "make", "install-carbon", "CARBON_VERSION="+version, "CARBON_INSTALL_DIR="+install)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
	t.Setenv("FINFOCUS_HOME", home)
	t.Setenv("FINFOCUS_CARBON_UTILIZATION", "0.5")
	reg := registry.New(filepath.Join(home, "plugins"))
	installed, err := reg.ListPlugins()
	require.NoError(t, err)
	require.Len(t, installed, 1)
	assert.Equal(t, "carbon", installed[0].Name)
	clients, cleanup, err := reg.Open(ctx, "carbon")
	require.NoError(t, err)
	defer cleanup()
	require.Len(t, clients, 1)
	assert.True(t, clients[0].HasCapability("carbon"))
	assert.False(t, clients[0].HasCapability("projected_costs"))
	resources := []engine.ResourceDescriptor{
		{
			ID:         "vm",
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			Properties: map[string]any{"instanceType": "t3.medium", "region": "us-east-1"},
		},
	}
	eng := engine.New(clients, nil)
	missing, err := eng.GetProjectedCost(ctx, resources)
	require.NoError(t, err)
	require.Len(t, missing, 1)
	require.NotNil(t, missing[0].Error)
	assert.Equal(t, engine.ErrCodeNoCostData, missing[0].Error.Code)
	assert.InDelta(t, 1437.00152000285625, missing[0].Sustainability[greenops.CarbonMetricKey].Value, 1e-8)
	loader := spec.NewLoader(filepath.Join(root, "test", "fixtures", "specs"))
	priced, err := engine.New(clients, loader).GetProjectedCost(ctx, resources)
	require.NoError(t, err)
	require.Len(t, priced, 1)
	require.Nil(t, priced[0].Error)
	assert.Equal(t, "local-spec", priced[0].Adapter)
	assert.Positive(t, priced[0].Monthly)
	var rendered bytes.Buffer
	require.NoError(t, engine.RenderResults(&rendered, engine.OutputTable, priced))
	assert.Contains(t, rendered.String(), "carbon_footprint")
	assert.Contains(t, rendered.String(), "Equivalent")
	manifest, err := os.ReadFile(filepath.Join(install, "plugin.manifest.json"))
	require.NoError(t, err)
	assert.Contains(t, string(manifest), "v"+version)
}
