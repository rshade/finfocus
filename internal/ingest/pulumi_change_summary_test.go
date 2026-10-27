package ingest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPulumiPlanChangeSummary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input string
		want        map[string]int
	}{
		{"summary", `{"steps":[],"changeSummary":{"same":5,"update":1}}`, map[string]int{"same": 5, "update": 1}},
		{"older preview", `{"steps":[]}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			plan, err := ParsePulumiPlan([]byte(tc.input))
			require.NoError(t, err)
			assert.Equal(t, tc.want, plan.ChangeSummary)
		})
	}
}
