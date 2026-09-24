package ingest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// The real-state fixtures are produced by scripts/gen-terraform-goldens.sh
// (OpenTofu + hashicorp/aws against a moto emulator), not written by hand.
const (
	realStatePath        = "../../test/fixtures/terraform/aws-realistic/terraform.tfstate"
	realTaintedStatePath = "../../test/fixtures/terraform/aws-realistic/terraform-tainted.tfstate"
	realStateGoldenPath  = "testdata/terraform/aws-realistic.descriptors.golden.json"
)

// pricingPropertyKeys returns the camelCased properties plugins and the
// adapter use for SKU, region and sizing. Cloud-assigned IDs and ARNs are left
// out of the golden so regenerating the state does not churn it.
func pricingPropertyKeys() []string {
	return []string{
		"allocatedStorage",
		"availabilityZone",
		"billingMode",
		"engine",
		"engineVersion",
		"instanceClass",
		"instanceType",
		"iops",
		"multiAz",
		"region",
		"size",
		"storageType",
		"throughput",
		"type",
		ingest.PropertyTerraformModule,
		ingest.PropertyTerraformProvider,
	}
}

type goldenDescriptor struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Provider   string                 `json:"provider"`
	Properties map[string]interface{} `json:"properties"`
}

func loadRealStateDescriptors(t *testing.T, path string) []engine.ResourceDescriptor {
	t.Helper()
	state, err := ingest.LoadTerraformState(path)
	require.NoError(t, err)
	descriptors, err := ingest.MapTerraformResources(state.GetManagedResources())
	require.NoError(t, err)
	return descriptors
}

func toGoldenDescriptors(descriptors []engine.ResourceDescriptor) []goldenDescriptor {
	out := make([]goldenDescriptor, 0, len(descriptors))
	for _, d := range descriptors {
		props := make(map[string]interface{})
		for _, k := range pricingPropertyKeys() {
			if v, ok := d.Properties[k]; ok {
				props[k] = v
			}
		}
		out = append(out, goldenDescriptor{ID: d.ID, Type: d.Type, Provider: d.Provider, Properties: props})
	}
	return out
}

func assertJSONGolden(t *testing.T, goldenPath string, v interface{}) {
	t.Helper()
	actual, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	actual = append(actual, '\n')

	if os.Getenv("UPDATE_GOLDEN") != "" {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, actual, 0o644))
		return
	}
	expected, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "golden file %s missing; run UPDATE_GOLDEN=1", goldenPath)
	assert.JSONEq(t, string(expected), string(actual),
		"golden mismatch for %s; run UPDATE_GOLDEN=1 to update", goldenPath)
}

func descriptorsByID(descriptors []engine.ResourceDescriptor) map[string]engine.ResourceDescriptor {
	byID := make(map[string]engine.ResourceDescriptor, len(descriptors))
	for _, d := range descriptors {
		byID[d.ID] = d
	}
	return byID
}

func TestRealTerraformState_Golden(t *testing.T) {
	descriptors := loadRealStateDescriptors(t, realStatePath)
	assertJSONGolden(t, realStateGoldenPath, toGoldenDescriptors(descriptors))
}

func TestRealTerraformState_Metadata(t *testing.T) {
	state, err := ingest.LoadTerraformState(realStatePath)
	require.NoError(t, err)

	assert.Equal(t, ingest.TerraformStateVersion, state.Version)
	assert.NotEmpty(t, state.TerraformVersion)
	assert.NotEmpty(t, state.Lineage)

	var dataSources []string
	for _, r := range state.Resources {
		if r.Mode == "data" {
			dataSources = append(dataSources, r.Type)
		}
	}
	assert.ElementsMatch(t, []string{"aws_ami", "aws_availability_zones"}, dataSources,
		"fixture must keep data sources so exclusion is exercised")
}

func TestRealTerraformState_Invariants(t *testing.T) {
	descriptors := loadRealStateDescriptors(t, realStatePath)
	byID := descriptorsByID(descriptors)
	require.Len(t, byID, len(descriptors), "descriptor IDs must be unique")

	t.Run("data sources excluded", func(t *testing.T) {
		for _, d := range descriptors {
			assert.NotContains(t, d.ID, "data.")
			assert.NotEqual(t, "aws_ami", d.Type)
			assert.NotEqual(t, "aws_availability_zones", d.Type)
		}
	})

	t.Run("module count instances", func(t *testing.T) {
		for _, id := range []string{"module.app.aws_instance.app[0]", "module.app.aws_instance.app[1]"} {
			d, ok := byID[id]
			require.True(t, ok, "missing %s", id)
			assert.Equal(t, "aws_instance", d.Type)
			assert.Equal(t, "m5.large", d.Properties["instanceType"])
			assert.Equal(t, "module.app", d.Properties[ingest.PropertyTerraformModule])
		}
		var moduleInstances int
		for _, d := range descriptors {
			if strings.HasPrefix(d.ID, "module.app.") {
				moduleInstances++
			}
		}
		assert.Equal(t, 2, moduleInstances)
	})

	t.Run("for_each instances keyed by string", func(t *testing.T) {
		want := map[string]string{
			`aws_instance.worker["small"]`: "t3.small",
			`aws_instance.worker["large"]`: "c5.xlarge",
		}
		for id, instanceType := range want {
			d, ok := byID[id]
			require.True(t, ok, "missing %s", id)
			assert.Equal(t, instanceType, d.Properties["instanceType"])
			assert.Empty(t, d.Properties[ingest.PropertyTerraformModule])
		}
	})

	t.Run("region and availability zone extracted", func(t *testing.T) {
		for _, d := range descriptors {
			assert.Equal(t, "aws", d.Provider, d.ID)
			if d.Type == "aws_iam_role" {
				assert.NotContains(t, d.Properties, "region", "IAM is global")
				continue
			}
			assert.Equal(t, "us-east-1", d.Properties["region"], d.ID)
		}
		for _, id := range []string{"aws_instance.web", "aws_ebs_volume.data", "aws_db_instance.main"} {
			assert.Equal(t, "us-east-1a", byID[id].Properties["availabilityZone"], id)
		}
	})

	t.Run("pricing attributes", func(t *testing.T) {
		assert.Equal(t, "t3.micro", byID["aws_instance.web"].Properties["instanceType"])

		volume := byID["aws_ebs_volume.data"]
		assert.Equal(t, "gp3", volume.Properties["type"])
		assert.InDelta(t, 100, volume.Properties["size"], 0)

		db := byID["aws_db_instance.main"]
		assert.Equal(t, "db.t3.micro", db.Properties["instanceClass"])
		assert.Equal(t, "postgres", db.Properties["engine"])
	})

	t.Run("cloud identifiers carried for actual-cost lookups", func(t *testing.T) {
		for _, d := range descriptors {
			if d.Type != "aws_instance" {
				continue
			}
			assert.True(t, strings.HasPrefix(d.Properties[ingest.PropertyTerraformCloudID].(string), "i-"), d.ID)
			assert.Contains(t, d.Properties[ingest.PropertyTerraformARN], ":instance/", d.ID)
		}
	})

	t.Run("provider reference from state", func(t *testing.T) {
		for _, d := range descriptors {
			assert.Equal(t, `provider["registry.opentofu.org/hashicorp/aws"]`,
				d.Properties[ingest.PropertyTerraformProvider], d.ID)
		}
	})
}

func TestRealTerraformState_TaintedInstanceDropped(t *testing.T) {
	state, err := ingest.LoadTerraformState(realTaintedStatePath)
	require.NoError(t, err)

	var tainted []string
	for _, r := range state.Resources {
		for _, inst := range r.Instances {
			if inst.Status == "tainted" {
				tainted = append(tainted, r.Type+"."+r.Name)
			}
		}
	}
	require.Equal(t, []string{"aws_instance.worker"}, tainted, "fixture must carry exactly one tainted instance")

	descriptors, err := ingest.MapTerraformResources(state.GetManagedResources())
	require.NoError(t, err)
	byID := descriptorsByID(descriptors)

	assert.NotContains(t, byID, `aws_instance.worker["small"]`)
	assert.Contains(t, byID, `aws_instance.worker["large"]`)
	assert.Len(t, descriptors, len(loadRealStateDescriptors(t, realStatePath))-1)
}
