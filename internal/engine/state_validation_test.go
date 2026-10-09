package engine_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

func TestStateResourceValidation(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, time.October, 8, 0, 0, 0, 0, time.UTC)
	resource, err := ingest.MapStateResource(ingest.StackExportResource{
		URN:  "urn:pulumi:dev::test::aws:ec2/instance:Instance::server",
		Type: "aws:ec2/instance:Instance", ID: "i-example", External: true,
		Created: &created, Modified: &created,
		Inputs:  map[string]any{"instanceType": "t3.micro", "region": "us-west-2"},
		Outputs: map[string]any{"arn": "arn:aws:ec2:us-west-2:123456789012:instance/i-example"},
	})
	require.NoError(t, err)
	assert.NoError(t, resource.Validate(), "state metadata must be accepted by estimation validation")
}

func TestResourceValidationRejectsMalformedNamespaces(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"pulumi:", "pulumi:bad:key", "pulumi:bad key", "other:region"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			resource := engine.ResourceDescriptor{
				Type: "aws:ec2/instance:Instance", Properties: map[string]any{key: "value"},
			}
			assert.ErrorIs(t, resource.Validate(), engine.ErrResourceValidation)
		})
	}
}
