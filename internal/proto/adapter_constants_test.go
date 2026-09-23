package proto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPropertyConstantsMatch ensures that the adapter's local property name
// constants stay in sync with the canonical definitions in the ingest package.
// We compare against string literals here because importing ingest directly
// would create an import cycle (proto -> ingest -> engine -> proto).
// The canonical values are defined as ingest.PropertyPulumiCloudID,
// ingest.PropertyPulumiARN, ingest.PropertyTerraformCloudID, and
// ingest.PropertyTerraformARN; update these literals if those constants change.
func TestPropertyConstantsMatch(t *testing.T) {
	// These must match ingest.PropertyPulumiCloudID and ingest.PropertyPulumiARN.
	assert.Equal(t, "pulumi:cloudId", propCloudIDPulumi,
		"propCloudIDPulumi must match ingest.PropertyPulumiCloudID")
	assert.Equal(t, "pulumi:arn", propARNPulumi,
		"propARNPulumi must match ingest.PropertyPulumiARN")
	// These must match ingest.PropertyTerraformCloudID and ingest.PropertyTerraformARN.
	assert.Equal(t, "terraform:cloudId", propCloudIDTerraform,
		"propCloudIDTerraform must match ingest.PropertyTerraformCloudID")
	assert.Equal(t, "terraform:arn", propARNTerraform,
		"propARNTerraform must match ingest.PropertyTerraformARN")
}
