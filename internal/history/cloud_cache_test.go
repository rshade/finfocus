package history

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

func TestCloudCache(t *testing.T) {
	t.Parallel()
	store, err := NewBoltStore(context.Background(), t.TempDir(), true, 90)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	_, found, err := store.GetSnapshot("s", 1)
	require.NoError(t, err)
	assert.False(t, found)
	list := UpdateListRecord{
		FetchedAt: time.Now().Unix(),
		ETag:      "tag",
		Updates:   []CloudUpdate{{Version: 2, Kind: "update", End: 123}},
	}
	require.NoError(t, store.PutUpdateList("s", list))
	got, found, err := store.GetUpdateList("s")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, list, got)
	snap := SnapshotRecord{
		Version:    2,
		ObservedAt: 123,
		Resources:  []CloudIdentity{{URN: "urn", CloudID: "id", Type: "aws:ec2/instance:Instance", Provider: "aws"}},
	}
	require.NoError(t, store.PutSnapshot("s", snap))
	gotSnap, found, err := store.GetSnapshot("s", 2)
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, snap, gotSnap)
	snap.Version = 10
	require.NoError(t, store.PutSnapshot("s", snap))
	require.NoError(t, store.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BucketPulumiCloudSnapshots))
		require.NotNil(t, b)
		key, _ := b.Cursor().First()
		assert.Equal(t, "s/0000000002", string(key))
		return b.Put([]byte("s/0000000001"), []byte("corrupt"))
	}))
	_, found, err = store.GetSnapshot("s", 1)
	require.NoError(t, err)
	assert.False(t, found)
	require.NoError(t, store.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(BucketPulumiCloudUpdates)).Put([]byte("s"), []byte("corrupt"))
	}))
	_, found, err = store.GetUpdateList("s")
	require.NoError(t, err)
	assert.False(t, found)
	dir := t.TempDir()
	old, err := bolt.Open(dir+"/history.db", 0600, nil)
	require.NoError(t, err)
	require.NoError(t, old.Close())
	upgraded, err := NewBoltStore(context.Background(), dir, true, 90)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, upgraded.Close()) })
	require.NoError(t, upgraded.db.View(func(tx *bolt.Tx) error {
		assert.NotNil(t, tx.Bucket([]byte(BucketPulumiCloudUpdates)))
		assert.NotNil(t, tx.Bucket([]byte(BucketPulumiCloudSnapshots)))
		return nil
	}))
}
