package proto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPropertyConstantsMatch ensures that the adapter's local property name
// constants stay in sync with the canonical definitions in the ingest package.
// We compare against string literals here because importing ingest directly
// would create an import cycle (proto -> ingest -> engine -> proto).
// The canonical values are defined as ingest.PropertyPulumiCloudID and
// ingest.PropertyPulumiARN; update these literals if those constants change.
func TestPropertyConstantsMatch(t *testing.T) {
	// These must match ingest.PropertyPulumiCloudID and ingest.PropertyPulumiARN.
	assert.Equal(t, "pulumi:cloudId", propCloudID,
		"propCloudID must match ingest.PropertyPulumiCloudID")
	assert.Equal(t, "pulumi:arn", propARN,
		"propARN must match ingest.PropertyPulumiARN")
}
