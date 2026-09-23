package plugin

import (
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// AWSTerraformTypeMappings returns ResolveResourceTypes entries for the AWS
// Terraform types in test/fixtures/terraform/aws-realistic, shaped like the
// response a type-resolving aws plugin returns. The ebs_volume "type" ->
// "volumeType" mapping renames a key, so tests can observe that
// property_mappings were applied.
//
//nolint:gosec // G101 misreads PulumiToken (a Pulumi type token) as a credential.
func AWSTerraformTypeMappings() map[string]*pbc.ResourceTypeMapping {
	return map[string]*pbc.ResourceTypeMapping{
		"aws_instance": {
			PulumiToken: "aws:ec2/instance:Instance",
			Supported:   true,
			PropertyMappings: map[string]string{
				"instance_type":     "instanceType",
				"availability_zone": "availabilityZone",
			},
		},
		"aws_ebs_volume": {
			PulumiToken:      "aws:ebs/volume:Volume",
			Supported:        true,
			PropertyMappings: map[string]string{"type": "volumeType"},
		},
		"aws_db_instance": {
			PulumiToken:      "aws:rds/instance:Instance",
			Supported:        true,
			PropertyMappings: map[string]string{"instance_class": "instanceClass"},
		},
		"aws_volume_attachment": {PulumiToken: "aws:ec2/volumeAttachment:VolumeAttachment", Supported: true},
		"aws_s3_bucket":         {PulumiToken: "aws:s3/bucket:Bucket", Supported: true},
		"aws_dynamodb_table":    {PulumiToken: "aws:dynamodb/table:Table", Supported: true},
		"aws_iam_role":          {PulumiToken: "aws:iam/role:Role", Supported: true},
		"aws_vpc":               {PulumiToken: "aws:ec2/vpc:Vpc", Supported: true},
		"aws_subnet":            {PulumiToken: "aws:ec2/subnet:Subnet", Supported: true},
		"aws_security_group":    {PulumiToken: "aws:ec2/securityGroup:SecurityGroup", Supported: true},
	}
}

// ConfigureTerraformResolver makes the mock advertise the aws provider and the
// resolve_resource_types capability and serve mappings from ResolveResourceTypes.
func (m *MockPlugin) ConfigureTerraformResolver(mappings map[string]*pbc.ResourceTypeMapping) {
	m.SetPluginInfo([]string{"aws"}, []pbc.PluginCapability{
		pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS,
		pbc.PluginCapability_PLUGIN_CAPABILITY_RESOLVE_RESOURCE_TYPES,
	})
	m.SetResourceTypeMappings(mappings)
}
