package skus

import "strings"

const (
	// pulumiTypeMaxParts is the maximum number of colon-separated segments
	// in a Pulumi type token (provider:module/resource:ClassName).
	pulumiTypeMaxParts = 3
	// pulumiTypeMinParts is the minimum number of segments needed to extract
	// the module/resource segment from a Pulumi type token.
	pulumiTypeMinParts = 2
)

// awsWellKnownSKUs returns the well-known SKU map for AWS resource types.
// Keys use the module/resource pattern extracted from full Pulumi types
// awsWellKnownSKUs returns a map from Pulumi module/resource strings to well-known AWS SKU identifiers.
// The keys use the module/resource pattern extracted from Pulumi type tokens (for example, "aws:eks/cluster:Cluster" -> "eks/cluster").
// Values are canonical SKU names used to identify metered AWS resources (for example, "cluster" or "addon").
func awsWellKnownSKUs() map[string]string {
	return map[string]string{
		"eks/cluster": "cluster", // EKS control plane ($0.10/hr)
		"eks/addon":   "addon",   // EKS addon (most free, some paid like CoreDNS)
	}
}

// resolveAWSSKU extracts a well-known SKU for AWS resource types.
// It parses the Pulumi type token (e.g., "aws:eks/cluster:Cluster") to extract
// resolveAWSSKU returns the well-known AWS SKU name for a Pulumi resource type token.
// It extracts the module/resource segment from resourceType and looks it up in the
// well-known AWS SKU map. It returns the SKU string, or an empty string if the
// token cannot be parsed or no matching SKU is found.
func resolveAWSSKU(resourceType string, _ map[string]string) string {
	segment := extractPulumiSegment(resourceType)
	if segment == "" {
		return ""
	}
	return awsWellKnownSKUs()[segment]
}

// extractPulumiSegment extracts the module/resource segment from a Pulumi type token.
// For "aws:eks/cluster:Cluster", it returns "eks/cluster".
// extractPulumiSegment extracts the module/resource segment from a Pulumi type token.
// Given a token of the form "provider:module/resource:ClassName" (for example,
// "aws:eks/cluster:Cluster"), it returns the lowercase module/resource portion
// (for example, "eks/cluster"). If the token does not contain at least two
// colon-separated segments, it returns an empty string.
func extractPulumiSegment(resourceType string) string {
	// Pulumi type tokens follow the pattern: provider:module/resource:ClassName
	// e.g., "aws:eks/cluster:Cluster" → split by ":" → ["aws", "eks/cluster", "Cluster"]
	parts := strings.SplitN(resourceType, ":", pulumiTypeMaxParts)
	if len(parts) < pulumiTypeMinParts {
		return ""
	}
	// The middle segment contains module/resource (e.g., "eks/cluster")
	return strings.ToLower(parts[1])
}