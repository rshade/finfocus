//go:build unix

package history

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenDailyFile_DoesNotFollowASymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "elsewhere.db")
	require.NoError(t, os.WriteFile(target, nil, 0o600))
	link := filepath.Join(dir, "demo@dev.daily.db")
	require.NoError(t, os.Symlink(target, link))

	file, err := openDailyFile(link, os.O_RDWR|os.O_CREATE, 0o600)
	if err == nil {
		_ = file.Close()
	}
	require.Error(t, err, "the check and the open must be one step, or a swapped link is followed")

	regular := filepath.Join(dir, "demo@prod.daily.db")
	file, err = openDailyFile(regular, os.O_RDWR|os.O_CREATE, 0o600)
	require.NoError(t, err)
	assert.NoError(t, file.Close())
}
