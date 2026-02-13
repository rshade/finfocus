// Package awsutil provides shared AWS utility functions.
package awsutil

import "strings"

const (
	// arnMinSegments is the minimum number of colon-separated segments in a valid ARN.
	arnMinSegments = 6
	// arnRegionIndex is the zero-based index of the region segment in an ARN.
	arnRegionIndex = 3
)

// RegionFromARN extracts the AWS region from an ARN string.
// Returns empty string if the ARN is malformed or the region segment is empty.
func RegionFromARN(arn string) string {
	// ARN format: arn:partition:service:region:account:resource
	parts := strings.Split(arn, ":")
	if len(parts) < arnMinSegments {
		return ""
	}
	return parts[arnRegionIndex]
}
