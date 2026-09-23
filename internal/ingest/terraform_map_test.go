package ingest_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/ingest"
)

func TestMapTerraformResource_Simple(t *testing.T) {
	data, err := os.ReadFile("../../examples/plans/terraform-simple-state.json")
	require.NoError(t, err)
	state, err := ingest.ParseTerraformState(data)
	require.NoError(t, err)

	desc, err := ingest.MapTerraformResource(state.Resources[0], state.Resources[0].Instances[0])
	require.NoError(t, err)

	assert.Equal(t, "aws_instance", desc.Type)
	assert.Equal(t, "aws_instance.web", desc.ID)
	assert.Equal(t, "aws", desc.Provider)
	assert.Equal(t, "t3.micro", desc.Properties["instanceType"])
	assert.Equal(t, "us-east-1a", desc.Properties["availabilityZone"])
	assert.Equal(t, "aws_instance.web", desc.Properties["terraform:id"])
	assert.Equal(t, "arn:aws:ec2:us-east-1:123456789012:instance/i-0abc123", desc.Properties["terraform:arn"])
	// Cloud identifiers stay in the terraform: namespace; the proto adapter
	// reads both namespaces (see propCloudIDKeys/propARNKeys).
	assert.Equal(t, "i-0abc123", desc.Properties["terraform:cloudId"])
	assert.NotContains(t, desc.Properties, "pulumi:arn")
	assert.NotContains(t, desc.Properties, "pulumi:cloudId")
	assert.Equal(t, `provider["registry.terraform.io/hashicorp/aws"]`, desc.Properties["terraform:provider"])
}

func TestMapTerraformResources_Addresses(t *testing.T) {
	data, err := os.ReadFile("../../examples/plans/terraform-multimodule-state.json")
	require.NoError(t, err)
	state, err := ingest.ParseTerraformState(data)
	require.NoError(t, err)

	descriptors, err := ingest.MapTerraformResources(state.GetManagedResources())
	require.NoError(t, err)
	require.Len(t, descriptors, 4)

	ids := make([]string, 0, len(descriptors))
	for _, d := range descriptors {
		ids = append(ids, d.ID)
	}
	assert.Equal(t, []string{
		"module.networking.aws_subnet.public[0]",
		"module.networking.aws_subnet.public[1]",
		`aws_instance.replica["us-east-1"]`,
		`aws_instance.replica["us-west-2"]`,
	}, ids)
}

func TestMapTerraformResource_NestedBlocks(t *testing.T) {
	data, err := os.ReadFile("../../examples/plans/terraform-nested-blocks-state.json")
	require.NoError(t, err)
	state, err := ingest.ParseTerraformState(data)
	require.NoError(t, err)

	desc, err := ingest.MapTerraformResource(state.Resources[0], state.Resources[0].Instances[0])
	require.NoError(t, err)

	root, ok := desc.Properties["rootBlockDevice"].(map[string]interface{})
	require.True(t, ok, "root_block_device should convert to rootBlockDevice map")
	assert.Equal(t, "gp3", root["volumeType"])
	assert.Equal(t, float64(100), root["volumeSize"])

	ebs, ok := desc.Properties["ebsBlockDevice"].([]interface{})
	require.True(t, ok, "ebs_block_device should convert to ebsBlockDevice array")
	require.Len(t, ebs, 1)
	first, ok := ebs[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "/dev/sdf", first["deviceName"])
	assert.Equal(t, float64(50), first["volumeSize"])

	tags, ok := desc.Properties["tagsAll"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "db", tags["Name"])
}

func TestMapTerraformResource_ProviderDerivation(t *testing.T) {
	resource := ingest.TerraformStateResource{Type: "aws_s3_bucket", Name: "b"}
	instance := ingest.TerraformStateInstance{Attributes: map[string]interface{}{}}

	desc, err := ingest.MapTerraformResource(resource, instance)
	require.NoError(t, err)
	assert.Equal(t, "aws", desc.Provider)

	_, err = ingest.MapTerraformResource(
		ingest.TerraformStateResource{Type: "nounderscore", Name: "x"},
		ingest.TerraformStateInstance{Attributes: map[string]interface{}{}},
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot derive provider")
}
