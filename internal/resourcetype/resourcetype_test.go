package resourcetype

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractProvider(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		want         string
	}{
		// Standard Pulumi resource types
		{"AWS EC2 instance", "aws:ec2/instance:Instance", "aws"},
		{"GCP compute instance", "gcp:compute:Instance", "gcp"},
		{"Azure VM", "azure:compute/vm:VM", "azure"},
		{"Kubernetes pod", "kubernetes:core/v1:Pod", "kubernetes"},
		{"AWS native provider", "aws-native:ec2:Instance", "aws-native"},
		{"Pulumi provider reference", "pulumi:providers:aws", "pulumi"},
		{"single colon", "aws:ec2", "aws"},
		// Terraform-style types (no colon)
		{"terraform aws", "aws_instance", "aws"},
		{"terraform azure", "azurerm_linux_virtual_machine", "azurerm"},
		{"terraform random", "random_pet", "random"},
		{"no colon no underscore", "kubernetes", "kubernetes"},
		{"dashes only", "aws-ec2-instance", "aws-ec2-instance"},
		// Edge cases
		{"empty string", "", ProviderUnknown},
		{"starts with colon", ":ec2:Instance", ProviderUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ExtractProvider(tt.resourceType))
		})
	}
}
