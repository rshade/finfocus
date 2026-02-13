// Package skus provides resource-type-aware SKU resolution as a fallback
// when property-based extraction returns empty. Resources like EKS clusters
// have fixed costs but no SKU property in their Pulumi state, so this
// package maps well-known resource types to their canonical SKU identifiers.
package skus

import "strings"

// ResolveSKU returns a well-known SKU for the given provider and resource type.
// It is intended as a fallback after property-based SKU extraction fails.
// ResolveSKU maps a provider and resource type to a well-known SKU identifier as a fallback
// when property-based SKU extraction yields no result.
//
// For supported providers (currently only "aws"), it returns a canonical SKU for known
// resource types; for unknown resource types or unsupported providers it returns the empty
// string, which callers should treat as a no-op. The properties map may be consulted to
// refine the mapping for certain resource types.
func ResolveSKU(provider, resourceType string, properties map[string]string) string {
	switch strings.ToLower(provider) {
	case "aws":
		return resolveAWSSKU(resourceType, properties)
	default:
		return ""
	}
}
