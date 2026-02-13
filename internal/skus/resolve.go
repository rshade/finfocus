// Package skus provides resource-type-aware SKU resolution as a fallback
// when property-based extraction returns empty. Resources like EKS clusters
// have fixed costs but no SKU property in their Pulumi state, so this
// package maps well-known resource types to their canonical SKU identifiers.
package skus

import "strings"

// ResolveSKU returns a well-known SKU for the given provider and resource type.
// It is intended as a fallback after property-based SKU extraction fails.
// Returns empty string for unknown resource types (callers should treat this as a no-op).
func ResolveSKU(provider, resourceType string, properties map[string]string) string {
	switch strings.ToLower(provider) {
	case "aws":
		return resolveAWSSKU(resourceType, properties)
	default:
		return ""
	}
}
