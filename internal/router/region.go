package router

import (
	"strings"

	"github.com/rshade/finfocus/internal/awsutil"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
)

// ExtractResourceRegion extracts the region associated with the given resource descriptor.
// It checks common properties (region, availabilityZone, availability_zone, location) and
// normalizes availability zones to their region form; if none are present it attempts to
// parse a region from the resource's ARN or id.
// The returned string is the region (for example "us-west-2"), or an empty string if no
// region can be determined.
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
			if s, isStr := val.(string); isStr && s != "" {
				return normalizeToRegion(s)
			}
		}
	}

	// Try to extract from ARN if present
	for _, key := range []string{"arn", "id"} {
		if val, ok := resource.Properties[key]; ok {
			if s, isStr := val.(string); isStr {
				if region := extractRegionFromARN(s); region != "" {
					return region
				}
			}
		}
	}

	return ""
}

// normalizeToRegion converts an availability zone string to its region form.
// It trims surrounding whitespace and, for AZs that end with a lowercase letter,
// removes the zone suffix and returns the resulting region.
//
// Supported formats:
//   - AWS: "us-west-2a" → "us-west-2" (single letter suffix after digit)
//   - GCP: "us-central1-a" → "us-central1" (dash + letter suffix after digit)
//
// If the input is empty or does not match an AZ-like pattern, the trimmed input
// (or empty string) is returned.
func normalizeToRegion(zone string) string {
	zone = strings.TrimSpace(zone)
	if zone == "" {
		return ""
	}

	lastChar := zone[len(zone)-1]
	if lastChar >= 'a' && lastChar <= 'z' {
		candidate := zone[:len(zone)-1]

		// AWS: us-west-2a → us-west-2 (candidate ends with digit)
		if len(candidate) > 0 && candidate[len(candidate)-1] >= '0' && candidate[len(candidate)-1] <= '9' {
			return candidate
		}

		// GCP: us-central1-a → us-central1 (candidate ends with '-', char before is digit)
		if len(candidate) > 1 && candidate[len(candidate)-1] == '-' {
			beforeDash := candidate[len(candidate)-2]
			if beforeDash >= '0' && beforeDash <= '9' {
				return candidate[:len(candidate)-1]
			}
		}
	}
	return zone
}

// extractRegionFromARN extracts the AWS region component from an ARN string.
// It returns the region if present (the fourth colon-separated field of a well-formed ARN);
// if the input does not begin with "arn:", lacks a region, or the region is empty or "*", it returns an empty string.
func extractRegionFromARN(arn string) string {
	if !strings.HasPrefix(arn, "arn:") {
		return ""
	}
	region := awsutil.RegionFromARN(arn)
	if region == "*" {
		return ""
	}
	return region
}

// PluginRegion returns the region configured for the given plugin client.
// If the client or its metadata is nil, or no region is set, it returns an empty
// string to indicate a universal (all-region) plugin.
func PluginRegion(client *pluginhost.Client) string {
	if client == nil || client.Metadata == nil || client.Metadata.Metadata == nil {
		return ""
	}
	return client.Metadata.Metadata["region"]
}

// RegionMatches reports whether the plugin region matches the resource region.
// If either region is empty it is treated as a wildcard and the function returns true.
// Otherwise the comparison is performed case-insensitively; the function returns
// true if the regions match, false otherwise.
func RegionMatches(pluginRegion, resourceRegion string) bool {
	if pluginRegion == "" || resourceRegion == "" {
		return true
	}
	return strings.EqualFold(pluginRegion, resourceRegion)
}
