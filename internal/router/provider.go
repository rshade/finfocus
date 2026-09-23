package router

import "strings"

// ProviderUnknown is the sentinel value for resources with indeterminate providers.
const ProviderUnknown = "unknown"

// ProviderWildcard represents a plugin that handles all providers.
const ProviderWildcard = "*"

// ExtractProviderFromType returns the provider name from a resource type string.
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
func ExtractProviderFromType(resourceType string) string {
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

// IsGlobalProvider checks if the provider value indicates a global plugin.
// IsGlobalProvider reports whether the provider string denotes a global plugin.
// It returns true when the provider is empty or equals ProviderWildcard ("*").
func IsGlobalProvider(provider string) bool {
	return provider == "" || provider == ProviderWildcard
}

// NormalizeProvider normalizes a provider string for comparison.
// NormalizeProvider returns the provider string with surrounding whitespace removed and all characters lowercased.
// Use the result for case-insensitive comparisons of provider identifiers.
func NormalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

// ProviderMatches checks if a resource's provider matches a plugin's supported provider.
// Returns true if:
//   - The supported provider is global (empty or "*")
//
// ProviderMatches reports whether a resource provider matches a supported provider.
// It returns true if the supportedProvider denotes a global provider (empty or "*")
// or if the resourceProvider and supportedProvider are equal after normalization.
// resourceProvider is the provider identifier from a resource type.
// supportedProvider is the provider identifier declared by a plugin.
func ProviderMatches(resourceProvider, supportedProvider string) bool {
	if IsGlobalProvider(supportedProvider) {
		return true
	}
	return NormalizeProvider(resourceProvider) == NormalizeProvider(supportedProvider)
}
