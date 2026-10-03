package pluginhost

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

func TestDiagnoseSkillQuotesPluginBindTimeouts(t *testing.T) {
	t.Parallel()

	docs := readDiagnoseSkillDocs(t)
	assert.Contains(t, docs, fmt.Sprintf("bind timeout is %d seconds", int(pluginBindTimeout/time.Second)))
	assert.Contains(t, docs, fmt.Sprintf("CI bind timeout is %d seconds", int(ciPluginBindTimeout/time.Second)))
	assert.Contains(t, docs, fmt.Sprintf("port retries are %d", maxPortRetries))
	assert.Contains(t, docs, fmt.Sprintf("CI port retries are %d", ciMaxPortRetries))
	assert.Contains(t, docs, fmt.Sprintf("stdout port fallback is %d seconds", int(stdoutPortFallback/time.Second)))
	assert.Contains(t, docs, "when `CI` is `"+envValueTrue+"`")
	assert.Contains(t, docs, ErrPluginIncompatible.Error())
	assert.Contains(t, docs, TraceIDMetadataKey)
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
