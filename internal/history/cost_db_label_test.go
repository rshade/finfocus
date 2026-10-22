package history

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func TestCostDBProjectLabelPreservesSchemaAndIdentity(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "project@unscoped.history.db")
	db, err := OpenCostDBFor(path, "project", "unscoped")
	require.NoError(t, err)
	require.NoError(t, db.SetProjectLabel("modules/compute"))
	require.NoError(t, db.SetProjectLabel(""))
	require.NoError(t, db.Close())
	db, err = OpenCostDBRead(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	stats, err := db.Stats()
	require.NoError(t, err)
	assert.EqualValues(t, 1, stats.Schema)
	assert.Equal(t, "project", stats.Project)
	assert.Equal(t, "unscoped", stats.Stack)
	require.NoError(t, db.db.View(func(tx *bolt.Tx) error {
		assert.Equal(
			t,
			"modules/compute",
			string(tx.Bucket([]byte(costBucketMeta)).Get([]byte(metaKeyProjectLabel))),
		)
		return nil
	}))
}
