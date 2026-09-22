package history

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	bolt "go.etcd.io/bbolt"
)

// readTagRecord reads the tag record stored at the given composite tag key.
// Returns the record and whether it was found.
func readTagRecord(t *testing.T, store *BoltStore, tagKeyStr string) (tagRecord, bool) {
	t.Helper()

	var rec tagRecord
	found := false
	err := store.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BucketResourceTags))
		if b == nil {
			return nil
		}
		data := b.Get([]byte(tagKeyStr))
		if data == nil {
			return nil
		}
		found = true

		return json.Unmarshal(data, &rec)
	})
	require.NoError(t, err)

	return rec, found
}

func TestBoltStore_UpsertTags_MergesTimestamps(t *testing.T) {
	ctx := context.Background()
	store, err := NewBoltStore(ctx, t.TempDir(), true, 365)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	stackHash := "testhash"
	urn := "urn:pulumi:dev::proj::aws:ec2/instance:Instance::tagged"
	tagKeyStr := BuildTagKey(stackHash, "env", "prod", URNHash(urn))

	first := ResourceHistoryEntry{
		URN: urn, CloudID: "i-old", Type: "aws:ec2/instance:Instance",
		Provider: "aws", FirstSeen: now - 7200, LastSeen: now - 3600,
		Source: SourceStateSnapshot, Tags: map[string]string{"env": "prod"},
	}
	require.NoError(t, store.Upsert(stackHash, first))

	rec, found := readTagRecord(t, store, tagKeyStr)
	require.True(t, found, "tag record should exist after first upsert")
	assert.Equal(t, now-7200, rec.FirstSeen)
	assert.Equal(t, now-3600, rec.LastSeen)

	// Second observation of the same URN and tag with a different cloud ID.
	second := ResourceHistoryEntry{
		URN: urn, CloudID: "i-new", Type: "aws:ec2/instance:Instance",
		Provider: "aws", FirstSeen: now - 1800, LastSeen: now,
		Source: SourceStateSnapshot, Tags: map[string]string{"env": "prod"},
	}
	require.NoError(t, store.Upsert(stackHash, second))

	rec, found = readTagRecord(t, store, tagKeyStr)
	require.True(t, found, "tag record should exist after second upsert")
	assert.Equal(t, now-7200, rec.FirstSeen, "first_seen should keep the earliest observation")
	assert.Equal(t, now, rec.LastSeen, "last_seen should keep the latest observation")
	assert.Equal(t, "i-old", rec.CloudID, "non-timestamp fields should be preserved")
	assert.Equal(t, URNHash(urn), rec.URNHash)
	assert.Equal(t, "env", rec.TagKey)
	assert.Equal(t, "prod", rec.TagValue)
}

func TestBoltStore_UpsertTags_CorruptExistingOverwritten(t *testing.T) {
	ctx := context.Background()
	store, err := NewBoltStore(ctx, t.TempDir(), true, 365)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now().Unix()
	stackHash := "testhash"
	urn := "urn:pulumi:dev::proj::aws:ec2/instance:Instance::corrupt"
	tagKeyStr := BuildTagKey(stackHash, "env", "prod", URNHash(urn))

	first := ResourceHistoryEntry{
		URN: urn, CloudID: "i-1", Type: "aws:ec2/instance:Instance",
		Provider: "aws", FirstSeen: now - 7200, LastSeen: now - 3600,
		Source: SourceStateSnapshot, Tags: map[string]string{"env": "prod"},
	}
	require.NoError(t, store.Upsert(stackHash, first))

	// Corrupt the stored tag record.
	require.NoError(t, store.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(BucketResourceTags)).Put([]byte(tagKeyStr), []byte("{not json"))
	}))

	second := ResourceHistoryEntry{
		URN: urn, CloudID: "i-2", Type: "aws:ec2/instance:Instance",
		Provider: "aws", FirstSeen: now - 1800, LastSeen: now,
		Source: SourceStateSnapshot, Tags: map[string]string{"env": "prod"},
	}
	require.NoError(t, store.Upsert(stackHash, second))

	rec, found := readTagRecord(t, store, tagKeyStr)
	require.True(t, found, "corrupt tag record should be replaced")
	assert.Equal(t, now-1800, rec.FirstSeen)
	assert.Equal(t, now, rec.LastSeen)
	assert.Equal(t, "i-2", rec.CloudID)
}
