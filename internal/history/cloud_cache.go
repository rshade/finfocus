package history

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const (
	// BucketPulumiCloudUpdates holds per-stack update-list metadata.
	BucketPulumiCloudUpdates = "pulumi_cloud_updates"
	// BucketPulumiCloudSnapshots holds extracted identities by stack and version.
	BucketPulumiCloudSnapshots = "pulumi_cloud_snapshots"
)

// CloudUpdate is cached update metadata without service configuration or properties.
type CloudUpdate struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Result  string `json:"result"`
	Start   int64  `json:"start"`
	End     int64  `json:"end"`
}

// UpdateListRecord is a complete stack update list and its freshness metadata.
type UpdateListRecord struct {
	FetchedAt int64         `json:"fetched_at"`
	ETag      string        `json:"etag"`
	Updates   []CloudUpdate `json:"updates"`
}

// SnapshotRecord contains only identity extracted from one deployment snapshot.
type SnapshotRecord struct {
	Version    int             `json:"version"`
	ObservedAt int64           `json:"observed_at"`
	FetchedAt  int64           `json:"fetched_at"`
	Resources  []CloudIdentity `json:"resources"`
}

// CloudCache stores extracted Cloud observations separately from resource history.
type CloudCache interface {
	GetUpdateList(stackHash string) (UpdateListRecord, bool, error)
	PutUpdateList(stackHash string, rec UpdateListRecord) error
	GetSnapshot(stackHash string, version int) (SnapshotRecord, bool, error)
	PutSnapshot(stackHash string, rec SnapshotRecord) error
}

// GetUpdateList reads a cached list; missing or undecodable records are absent.
func (s *BoltStore) GetUpdateList(stackHash string) (UpdateListRecord, bool, error) {
	var rec UpdateListRecord
	found, err := s.getCloudCache(BucketPulumiCloudUpdates, stackHash, &rec)
	return rec, found, err
}

// PutUpdateList replaces the cached list and its freshness metadata.
func (s *BoltStore) PutUpdateList(stackHash string, rec UpdateListRecord) error {
	return s.putCloudCache(BucketPulumiCloudUpdates, stackHash, rec)
}

// GetSnapshot reads extracted identities for a stack version.
func (s *BoltStore) GetSnapshot(stackHash string, version int) (SnapshotRecord, bool, error) {
	var rec SnapshotRecord
	found, err := s.getCloudCache(BucketPulumiCloudSnapshots, cloudSnapshotKey(stackHash, version), &rec)
	return rec, found, err
}

// PutSnapshot records an extracted snapshot; valid existing records are immutable.
func (s *BoltStore) PutSnapshot(stackHash string, rec SnapshotRecord) error {
	if _, found, err := s.GetSnapshot(stackHash, rec.Version); err != nil || found {
		return err
	}
	return s.putCloudCache(BucketPulumiCloudSnapshots, cloudSnapshotKey(stackHash, rec.Version), rec)
}

func cloudSnapshotKey(stackHash string, version int) string {
	return fmt.Sprintf("%s/%010d", stackHash, version)
}

func (s *BoltStore) getCloudCache(bucket, key string, dst any) (bool, error) {
	if s == nil || !s.IsEnabled() {
		return false, ErrHistoryDisabled
	}
	found := false
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return nil
		}
		data := b.Get([]byte(key))
		if len(data) == 0 {
			return nil
		}
		found = json.Unmarshal(data, dst) == nil
		return nil
	})
	return found, err
}

func (s *BoltStore) putCloudCache(bucket, key string, rec any) error {
	if s == nil || !s.IsEnabled() {
		return ErrHistoryDisabled
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("encode cloud cache: %w", err)
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("cloud cache bucket %q missing", bucket)
		}
		return b.Put([]byte(key), data)
	})
}
