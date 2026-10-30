//go:build unix

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A Terraform resource id resolves its database through git. A stalled git
// must stop when the request context ends, or a dashboard query outlives its
// timeout and blocks session shutdown.
func TestLoadDetailHistory_TerraformLookupHonorsContext(t *testing.T) {
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "main.tf"), nil, 0o600))
	bin := t.TempDir()
	script := "#!/bin/sh\nexec /bin/sleep 30\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o700))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(project)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _ = loadDetailHistory(ctx, t.TempDir(), "dev", "aws_instance.web")

	assert.Less(t, time.Since(start), 10*time.Second)
}
