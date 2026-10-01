// Package resourcetype provides helpers for parsing resource type strings
// (Pulumi tokens and Terraform types). It is a leaf package with no internal
// dependencies so both engine and router can consume it without import cycles.
package resourcetype

import "strings"

// ProviderUnknown is the sentinel value for resources with indeterminate providers.
const ProviderUnknown = "unknown"

// ExtractProvider returns the provider name from a resource type string.
//
// Pulumi types follow the format "provider:service/module:Type" where:
//   - provider: Cloud provider identifier (aws, gcp, azure, kubernetes)
//   - service/module: Service or module within the provider
//   - Type: The specific resource type
//
// Types without a colon are treated as Terraform-style and split on the first
// underscore ("aws_instance" → "aws"); types with neither a colon nor an
// underscore are returned as-is.
//
// Examples:
//   - "aws:ec2/instance:Instance" → "aws"
//   - "gcp:compute:Instance" → "gcp"
//   - "azure:compute/vm:VM" → "azure"
//   - "kubernetes:core/v1:Pod" → "kubernetes"
//   - "aws-native:ec2:Instance" → "aws-native"
//   - "pulumi:providers:aws" → "pulumi"
//   - "aws_instance" → "aws" (Terraform-style type, no colon)
//   - "azurerm_linux_virtual_machine" → "azurerm"
//   - "" → "unknown"
//
// If resourceType is empty or has an empty first colon-separated segment, it
// returns ProviderUnknown.
func ExtractProvider(resourceType string) string {
	if resourceType == "" {
		return ProviderUnknown
	}
	if idx := strings.Index(resourceType, ":"); idx >= 0 {
		if idx > 0 {
			return resourceType[:idx]
		}
		return ProviderUnknown
	}
	// Terraform-style types carry no colon: "aws_instance" -> "aws".
	if idx := strings.Index(resourceType, "_"); idx > 0 {
		return resourceType[:idx]
	}
	return resourceType
}
