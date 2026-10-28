package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
)

func TestInputStackMismatch(t *testing.T) {
	t.Parallel()
	withURN := func(urn string) []engine.ResourceDescriptor {
		return []engine.ResourceDescriptor{{Properties: map[string]any{engine.PropertyPulumiURN: urn}}}
	}
	for _, tc := range []struct {
		name      string
		resources []engine.ResourceDescriptor
		stack     string
		want      bool
	}{
		{"same stack", withURN("urn:pulumi:dev::app::aws:s3/bucket:Bucket::b"), "dev", false},
		{"other stack", withURN("urn:pulumi:prod::app::aws:s3/bucket:Bucket::b"), "dev", true},
		{"organization-qualified selection", withURN("urn:pulumi:dev::app::aws:s3/bucket:Bucket::b"), "org/dev", false},
		{"urn from the descriptor id", []engine.ResourceDescriptor{{ID: "urn:pulumi:prod::app::t::n"}}, "dev", true},
		{"no urn gives nothing to compare", []engine.ResourceDescriptor{{ID: "i-123"}}, "dev", false},
		{"no resolved stack", withURN("urn:pulumi:prod::app::t::n"), "", false},
		{"no resources", nil, "dev", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(
				t,
				tc.want,
				inputStackMismatch(tc.resources, history.StackContext{Project: "app", Stack: tc.stack}),
			)
		})
	}
}
