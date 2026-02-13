package router

import (
	"fmt"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// ExtractResourceRegion extracts the region from a resource descriptor's properties.
// It checks common property keys for AWS, Azure, and GCP resources.
// Returns empty string if no region can be determined.
func ExtractResourceRegion(resource engine.ResourceDescriptor) string {
	// Try common region property keys in priority order
	regionKeys := []string{
		"region",
		"availabilityZone",
		"availability_zone",
		"location",
	}

	for _, key := range regionKeys {
		if val, ok := resource.Properties[key]; ok {
			region := fmt.Sprintf("%v", val)
			if region != "" {
				// For availability zones like "us-west-2a", extract the region "us-west-2"
				return normalizeToRegion(region)
			}
		}
	}

	// Try to extract from ARN if present
	for _, key := range []string{"arn", "id"} {
		if val, ok := resource.Properties[key]; ok {
			if region := extractRegionFromARN(fmt.Sprintf("%v", val)); region != "" {
				return region
			}
		}
	}

	return ""
}

// normalizeToRegion converts an availability zone to a region if needed.
// Example: "us-west-2a" -> "us-west-2", "us-west-2" -> "us-west-2".
func normalizeToRegion(zone string) string {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return ""
	}

	// AWS AZ format: region + single letter suffix (e.g., us-west-2a)
	// Check if last char is a lowercase letter and removing it yields a valid-looking region
	if len(zone) > 0 {
		lastChar := zone[len(zone)-1]
		if lastChar >= 'a' && lastChar <= 'z' {
			candidate := zone[:len(zone)-1]
			// Check if it looks like a region (ends with a digit)
			if len(candidate) > 0 && candidate[len(candidate)-1] >= '0' && candidate[len(candidate)-1] <= '9' {
				return candidate
			}
		}
	}
	return zone
}

// extractRegionFromARN extracts the region from an AWS ARN.
// ARN format: arn:partition:service:region:account:resource.
func extractRegionFromARN(arn string) string {
	if !strings.HasPrefix(arn, "arn:") {
		return ""
	}
	parts := strings.SplitN(arn, ":", 6) //nolint:mnd // ARN has 6 colon-separated parts
	if len(parts) < 4 {                  //nolint:mnd // need at least 4 parts for region
		return ""
	}
	region := parts[3]
	if region == "" || region == "*" {
		return ""
	}
	return region
}

// PluginRegion returns the region a plugin is configured for, or empty string
// if it's a universal (all-region) plugin.
func PluginRegion(client *pluginhost.Client) string {
	if client == nil || client.Metadata == nil || client.Metadata.Metadata == nil {
		return ""
	}
	return client.Metadata.Metadata["region"]
}

// RegionMatches checks if a plugin's region is compatible with a resource's region.
// Returns true if:
//   - Plugin has no region (universal plugin, matches all)
//   - Resource has no region (can't filter)
//   - Plugin region equals resource region
func RegionMatches(pluginRegion, resourceRegion string) bool {
	if pluginRegion == "" || resourceRegion == "" {
		return true
	}
	return strings.EqualFold(pluginRegion, resourceRegion)
}
