package ingest_test

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/ingest"
)

// TestScalabilitySnapshotsParse checks that the generated E2E scalability
// plans load through the same path as a real preview: every step carries its
// type only under newState, and the resources map to the expected AWS mix.
func TestScalabilitySnapshotsParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		instances, volumes int
	}{
		{10, 4},
		{50, 20},
		{100, 40},
		{500, 200},
		{1000, 400},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.instances), func(t *testing.T) {
			t.Parallel()
			path := filepath.Join("..", "..", "test", "e2e", "fixtures", "scalability", "snapshots",
				fmt.Sprintf("plan-%d.json", tt.instances))
			plan, err := ingest.LoadPulumiPlan(path)
			require.NoError(t, err)

			for _, step := range plan.Steps {
				assert.Empty(t, step.Type, "step %s has a top-level type", step.URN)
				require.NotNil(t, step.NewState, "step %s has no newState", step.URN)
			}

			byType := map[string]int{}
			aws := 0
			for _, res := range plan.GetResources() {
				byType[res.Type]++
				if strings.HasPrefix(res.Type, "aws:") {
					aws++
				}
			}
			assert.Equal(t, tt.instances, byType["aws:ec2/instance:Instance"])
			assert.Equal(t, tt.volumes, byType["aws:ebs/volume:Volume"])
			assert.Equal(t, 17+tt.instances+tt.volumes, aws)
			assert.Equal(t, len(plan.Steps), plan.ChangeSummary["create"])
		})
	}
}
