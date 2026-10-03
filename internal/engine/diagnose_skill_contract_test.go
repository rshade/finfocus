package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiagnoseSkillQuotesEngineTimeouts(t *testing.T) {
	t.Parallel()

	docs := readDiagnoseSkillDocs(t)
	perResource := int(perResourceTimeout / time.Second)
	query := int(defaultQueryTimeout / time.Second)
	assert.Contains(t, docs, fmt.Sprintf("per-resource timeout is %d seconds", perResource))
	assert.Contains(t, docs, fmt.Sprintf("query timeout is %d seconds", query))
	assert.Contains(t, docs, "types that start with `"+pulumiInternalPrefix+"`")
	assert.Contains(t, docs, noteNoPricingInfo)
	assert.Contains(t, docs, noteNoActualCostData)
	assert.Contains(t, docs, "ERROR: plugin call failed")
	assert.Contains(t, docs, "no actual cost data available (use --fallback-estimate to include $0 placeholders)")
	assert.Contains(t, docs, "GetProjectedCostWithErrors`, which does not apply the per-resource timeout")
}

func readDiagnoseSkillDocs(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Join(filepath.Dir(file), "..", "..", "agent-skills", "finfocus-diagnose")
	var b strings.Builder
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		b.Write(body)
		b.WriteByte('\n')
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, b.String())
	return b.String()
}
