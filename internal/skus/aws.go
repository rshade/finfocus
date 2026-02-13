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
// (e.g., "aws:eks/cluster:Cluster" → "eks/cluster").
func awsWellKnownSKUs() map[string]string {
	return map[string]string{
		"eks/cluster": "cluster", // EKS control plane ($0.10/hr)
		"eks/addon":   "addon",   // EKS addon (most free, some paid like CoreDNS)
	}
}

// resolveAWSSKU extracts a well-known SKU for AWS resource types.
// It parses the Pulumi type token (e.g., "aws:eks/cluster:Cluster") to extract
// the module/resource segment ("eks/cluster") and looks it up in the well-known map.
func resolveAWSSKU(resourceType string, _ map[string]string) string {
	segment := extractPulumiSegment(resourceType)
	if segment == "" {
		return ""
	}
	return awsWellKnownSKUs()[segment]
}

// extractPulumiSegment extracts the module/resource segment from a Pulumi type token.
// For "aws:eks/cluster:Cluster", it returns "eks/cluster".
// For types that don't match the expected pattern, it returns empty string.
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
