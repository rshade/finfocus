//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	awsPublicPlugin              = "aws-public"
	capabilityResolveTypes       = "resolve_resource_types"
	realTerraformStateFixture    = "../fixtures/terraform/aws-realistic/terraform.tfstate"
	resolveTypesNotYetSupportMsg = "aws-public does not yet implement ResolveResourceTypes " +
		"(needs finfocus-spec >= v0.6.1); see #1506"
)

// realStateInstances maps each aws_instance address in the real Terraform
// state fixture to its instance type.
var realStateInstances = map[string]string{
	"aws_instance.web":               "t3.micro",
	"module.app.aws_instance.app[0]": "m5.large",
	"module.app.aws_instance.app[1]": "m5.large",
	`aws_instance.worker["small"]`:   "t3.small",
	`aws_instance.worker["large"]`:   "c5.xlarge",
}

type pluginListEntry struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
}

type costRow struct {
	ResourceType string  `json:"resourceType"`
	ResourceID   string  `json:"resourceId"`
	Monthly      float64 `json:"monthly"`
	Notes        string  `json:"notes"`
}

type costOutput struct {
	Finfocus struct {
		Summary struct {
			TotalMonthly float64            `json:"totalMonthly"`
			ByProvider   map[string]float64 `json:"byProvider"`
		} `json:"summary"`
		Resources []costRow `json:"resources"`
	} `json:"finfocus"`
}

// runFinFocusJSON runs the real finfocus binary and decodes its JSON stdout.
// The cost cache is redirected to a temp dir so the run neither reads stale
// resolve_types entries nor writes to the developer's cache.
func runFinFocusJSON(t *testing.T, binary, cacheDir string, v interface{}, args ...string) {
	t.Helper()
	cmd := newCommand(context.Background(), binary, args...)
	cmd.Env = append(os.Environ(), "FINFOCUS_CACHE_DIR="+cacheDir)
	output, err := cmd.Output()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		require.NoError(t, err, "finfocus %v failed with stderr: %s", args, exitErr.Stderr)
	}
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(output, v), "invalid JSON from finfocus %v: %s", args, output)
}

// writeEquivalentPulumiPlan writes a Pulumi preview JSON holding one
// aws:ec2/instance:Instance per distinct instance type, in the same AZ as the
// Terraform fixture, and returns its path and the URN for each type.
func writeEquivalentPulumiPlan(t *testing.T) (string, map[string]string) {
	t.Helper()
	urns := make(map[string]string)
	var steps []map[string]interface{}
	for _, instanceType := range realStateInstances {
		if _, seen := urns[instanceType]; seen {
			continue
		}
		urn := fmt.Sprintf("urn:pulumi:e2e::terraform-parity::aws:ec2/instance:Instance::%s", instanceType)
		urns[instanceType] = urn
		steps = append(steps, map[string]interface{}{
			"op":   "create",
			"urn":  urn,
			"type": "aws:ec2/instance:Instance",
			"newState": map[string]interface{}{
				"type": "aws:ec2/instance:Instance",
				"inputs": map[string]interface{}{
					"instanceType":     instanceType,
					"availabilityZone": "us-east-1a",
				},
			},
		})
	}
	data, err := json.Marshal(map[string]interface{}{"steps": steps})
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "terraform-parity-plan.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path, urns
}

// TestE2E_TerraformState_ResolveResourceTypesReadiness activates once the
// installed aws-public plugin advertises resolve_resource_types. It prices the
// real Terraform state fixture with the real binary and plugin (no cloud
// resources are created) and checks each instance against the same instance
// type priced from an equivalent Pulumi plan.
func TestE2E_TerraformState_ResolveResourceTypesReadiness(t *testing.T) {
	binary := findFinFocusBinary()
	require.NotEmpty(t, binary, "finfocus binary not found - run 'make build' first")
	cacheDir := t.TempDir()

	var plugins []pluginListEntry
	runFinFocusJSON(t, binary, cacheDir, &plugins, "plugin", "list", "--output", "json")
	idx := slices.IndexFunc(plugins, func(p pluginListEntry) bool { return p.Name == awsPublicPlugin })
	if idx < 0 {
		t.Skipf("%s plugin not installed; install it with 'finfocus plugin install %s'", awsPublicPlugin, awsPublicPlugin)
	}
	plugin := plugins[idx]
	if !slices.Contains(plugin.Capabilities, capabilityResolveTypes) {
		t.Skip(resolveTypesNotYetSupportMsg)
	}
	t.Logf("%s %s advertises %s", plugin.Name, plugin.Version, capabilityResolveTypes)

	statePath, err := filepath.Abs(realTerraformStateFixture)
	require.NoError(t, err)

	var tf costOutput
	runFinFocusJSON(t, binary, cacheDir, &tf,
		"cost", "projected", "--terraform-state", statePath, "--adapter", awsPublicPlugin, "--output", "json")

	byID := make(map[string]costRow, len(tf.Finfocus.Resources))
	for _, r := range tf.Finfocus.Resources {
		byID[r.ResourceID] = r
	}
	assert.Positive(t, tf.Finfocus.Summary.TotalMonthly)
	assert.Positive(t, tf.Finfocus.Summary.ByProvider["aws"])

	planPath, urns := writeEquivalentPulumiPlan(t)
	var pulumi costOutput
	runFinFocusJSON(t, binary, cacheDir, &pulumi,
		"cost", "projected", "--pulumi-json", planPath, "--adapter", awsPublicPlugin, "--output", "json")
	pulumiByURN := make(map[string]costRow, len(pulumi.Finfocus.Resources))
	for _, r := range pulumi.Finfocus.Resources {
		pulumiByURN[r.ResourceID] = r
	}

	tolerance := LoadConfig().Tolerance
	for id, instanceType := range realStateInstances {
		row, ok := byID[id]
		require.True(t, ok, "missing %s in terraform output", id)
		assert.Equal(t, "aws:ec2/instance:Instance", row.ResourceType, id)
		require.Positive(t, row.Monthly, "%s (%s) priced at $0: %s", id, instanceType, row.Notes)

		reference, ok := pulumiByURN[urns[instanceType]]
		require.True(t, ok, "missing %s in pulumi output", instanceType)
		require.Positive(t, reference.Monthly, "pulumi reference for %s priced at $0: %s", instanceType, reference.Notes)
		assert.InEpsilon(t, reference.Monthly, row.Monthly, tolerance,
			"%s (%s): terraform $%.2f vs pulumi $%.2f", id, instanceType, row.Monthly, reference.Monthly)
	}
}
