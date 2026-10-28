package cloudhistory

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloudHistoryPackageBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		target    string
		forbidden []string
	}{
		{"internal/webui", []string{"internal/pulumicloud", "internal/cloudhistory"}},
		{"internal/history", []string{"internal/pulumicloud", "internal/cloudhistory"}},
		{"internal/pulumicloud", []string{"internal/engine"}},
	} {
		t.Run(tc.target, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			output, err := exec.CommandContext(ctx, "go", "list", "-deps", "github.com/rshade/finfocus/"+tc.target).
				CombinedOutput()
			require.NoError(t, err, string(output))
			dependencies := strings.Fields(string(output))
			for _, forbidden := range tc.forbidden {
				assert.NotContains(t, dependencies, "github.com/rshade/finfocus/"+forbidden)
			}
		})
	}
}
