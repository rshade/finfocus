// Package resourcetype provides helpers for parsing resource type strings
// (Pulumi tokens and Terraform types). It is a leaf package with no internal
// dependencies so both engine and router can consume it without import cycles.
package resourcetype

import "strings"

// ProviderUnknown is the sentinel value for resources with indeterminate providers.
const ProviderUnknown = "unknown"

// NormalizeProvider trims and lowercases name and maps a package or provider
// name to the cloud that bills it. This switch is the only alias table: keep it
// small and explicit, because unknown names pass through unchanged rather than
// being guessed.
//
//   - "aws-native" → "aws"
//   - "azure-native", "azurerm" → "azure"
//   - "google-native", "google" → "gcp"
//
// Names with no alias, including "pulumi", "*", and the empty string, pass
// through trimmed and lowercased.
func NormalizeProvider(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	switch key {
	case "aws-native":
		return "aws"
	case "azure-native", "azurerm":
		return "azure"
	case "google-native", "google":
		return "gcp"
	default:
		return key
	}
}

// ExtractProvider returns the cloud that bills a resource type:
// NormalizeProvider applied to ExtractPackage. It is the value for
// ResourceDescriptor.Provider and for provider routing.
//
// Examples:
//   - "aws:ec2/instance:Instance" → "aws"
//   - "aws-native:ec2:Instance" → "aws"
//   - "azure-native:compute:VirtualMachine" → "azure"
//   - "azurerm_linux_virtual_machine" → "azure"
//   - "google_compute_instance" → "gcp"
//   - "kubernetes:core/v1:Pod" → "kubernetes"
//   - "pulumi:providers:aws" → "pulumi"
//   - "" → "unknown"
func ExtractProvider(resourceType string) string {
	return NormalizeProvider(ExtractPackage(resourceType))
}

// ExtractPackage returns the raw prefix of a resource type string: the Pulumi
// package ("aws-native", "azure") or the Terraform provider ("azurerm",
// "google"). Use it where the package itself matters, such as display; use
// ExtractProvider for routing and aggregation.
//
// Pulumi types follow the format "package:service/module:Type". Types without a
// colon are treated as Terraform-style and split on the first underscore
// ("aws_instance" → "aws"); types with neither a colon nor an underscore are
// returned as-is.
//
// Examples:
//   - "aws:ec2/instance:Instance" → "aws"
//   - "aws-native:ec2:Instance" → "aws-native"
//   - "pulumi:providers:aws" → "pulumi"
//   - "aws_instance" → "aws" (Terraform-style type, no colon)
//   - "azurerm_linux_virtual_machine" → "azurerm"
//   - "" → "unknown"
//
// If resourceType is empty or has an empty first colon-separated segment, it
// returns ProviderUnknown.
func ExtractPackage(resourceType string) string {
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
