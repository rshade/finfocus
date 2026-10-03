package resourcetype

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractPackage(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
			assert.Equal(t, tt.want, ExtractPackage(tt.resourceType))
		})
	}
}

func TestExtractProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		resourceType string
		want         string
	}{
		{"aws", "aws:ec2/instance:Instance", "aws"},
		{"aws native", "aws-native:ec2:Instance", "aws"},
		{"aws native mixed case", "AWS-Native:ec2:Instance", "aws"},
		{"terraform aws", "aws_instance", "aws"},
		{"azure", "azure:compute/virtualMachine:VirtualMachine", "azure"},
		{"azure native", "azure-native:compute:VirtualMachine", "azure"},
		{"terraform azure", "azurerm_linux_virtual_machine", "azure"},
		{"gcp", "gcp:compute:Instance", "gcp"},
		{"google native", "google-native:compute/v1:Instance", "gcp"},
		{"terraform google", "google_compute_instance", "gcp"},
		{"kubernetes", "kubernetes:apps/v1:Deployment", "kubernetes"},
		{"pulumi provider reference", "pulumi:providers:aws", "pulumi"},
		{"custom", "custom:foo:Bar", "custom"},
		{"unmapped terraform provider", "random_pet", "random"},
		{"empty string", "", ProviderUnknown},
		{"starts with colon", ":ec2:Instance", ProviderUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ExtractProvider(tt.resourceType))
		})
	}
}

func TestNormalizeProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"aws", "aws", "aws"},
		{"aws native", "aws-native", "aws"},
		{"azure", "azure", "azure"},
		{"azure native", "azure-native", "azure"},
		{"azurerm", "azurerm", "azure"},
		{"gcp", "gcp", "gcp"},
		{"google native", "google-native", "gcp"},
		{"google", "google", "gcp"},
		{"kubernetes", "kubernetes", "kubernetes"},
		{"trims and lowercases", "  AWS-Native ", "aws"},
		{"unknown passes through lowercased", "Pulumi", "pulumi"},
		{"wildcard passes through", "*", "*"},
		{"empty passes through", "", ""},
		{"no substring guessing", "awsx", "awsx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, NormalizeProvider(tt.input))
		})
	}
}
