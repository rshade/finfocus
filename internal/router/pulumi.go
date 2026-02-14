package router

import "strings"

const (
	// PulumiInternalPrefix identifies all internal Pulumi resource types.
	// Resources with this prefix represent Pulumi framework constructs
	// rather than cloud infrastructure and have no associated cloud cost.
	PulumiInternalPrefix = "pulumi:"

	// PulumiProviderPrefix identifies Pulumi provider resources
	// (e.g., "pulumi:providers:aws", "pulumi:providers:gcp").
	PulumiProviderPrefix = "pulumi:providers:"

	// PulumiComponentPrefix identifies Pulumi component resources
	// (e.g., "pulumi:pulumi:Stack").
	PulumiComponentPrefix = "pulumi:pulumi:"
)

// IsInternalPulumiType reports whether resourceType is an internal Pulumi
// resource type (prefixed with "pulumi:") that should not be routed to
// cost plugins by default.
//
// Internal types include provider resources (pulumi:providers:*) and
// component resources (pulumi:pulumi:*). These represent Pulumi framework
// constructs with no associated cloud cost.
//
// A future "pulumi cost plugin" can still opt-in to handle these types
// IsInternalPulumiType reports whether resourceType identifies an internal Pulumi resource type.
// It returns true if resourceType begins with the PulumiInternalPrefix ("pulumi:"), false otherwise.
func IsInternalPulumiType(resourceType string) bool {
	return strings.HasPrefix(resourceType, PulumiInternalPrefix)
}

// IsPulumiProviderResource reports whether resourceType is a Pulumi
// IsPulumiProviderResource reports whether resourceType is a Pulumi provider resource.
// It returns true if resourceType begins with "pulumi:providers:", false otherwise.
func IsPulumiProviderResource(resourceType string) bool {
	return strings.HasPrefix(resourceType, PulumiProviderPrefix)
}

// IsPulumiComponentResource reports whether resourceType is a Pulumi
// IsPulumiComponentResource reports whether resourceType identifies a Pulumi component
// resource by checking whether it starts with PulumiComponentPrefix.
func IsPulumiComponentResource(resourceType string) bool {
	return strings.HasPrefix(resourceType, PulumiComponentPrefix)
}