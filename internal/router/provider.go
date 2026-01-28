package router

import "strings"

// ProviderUnknown is the sentinel value for resources with indeterminate providers.
const ProviderUnknown = "unknown"

// ProviderWildcard represents a plugin that handles all providers.
const ProviderWildcard = "*"

// ExtractProviderFromType extracts the provider name from a Pulumi resource type string.
//
// Resource types follow the format "provider:service/module:Type" where:
//   - provider: Cloud provider identifier (aws, gcp, azure, kubernetes)
//   - service/module: Service or module within the provider
//   - Type: The specific resource type
//
// Examples:
//   - "aws:ec2/instance:Instance" → "aws"
//   - "gcp:compute:Instance" → "gcp"
//   - "azure:compute/vm:VM" → "azure"
//   - "kubernetes:core/v1:Pod" → "kubernetes"
//   - "aws-native:ec2:Instance" → "aws-native"
//   - "pulumi:providers:aws" → "pulumi"
//   - "" → "unknown"
//
// The function extracts the first colon-separated segment as the provider.
// Returns "unknown" if the input is empty or malformed.
func ExtractProviderFromType(resourceType string) string {
	if resourceType == "" {
		return ProviderUnknown
	}

	parts := strings.Split(resourceType, ":")
	if len(parts) >= 1 && parts[0] != "" {
		return parts[0]
	}

	return ProviderUnknown
}

// IsGlobalProvider checks if the provider value indicates a global plugin.
// Global plugins handle all providers (empty string or "*").
func IsGlobalProvider(provider string) bool {
	return provider == "" || provider == ProviderWildcard
}

// NormalizeProvider normalizes a provider string for comparison.
// Converts to lowercase and trims whitespace.
func NormalizeProvider(provider string) string {
	return strings.ToLower(strings.TrimSpace(provider))
}

// ProviderMatches checks if a resource's provider matches a plugin's supported provider.
// Returns true if:
//   - The supported provider is global (empty or "*")
//   - The providers match exactly (case-insensitive)
func ProviderMatches(resourceProvider, supportedProvider string) bool {
	if IsGlobalProvider(supportedProvider) {
		return true
	}
	return NormalizeProvider(resourceProvider) == NormalizeProvider(supportedProvider)
}
