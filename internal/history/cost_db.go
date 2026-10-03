package history

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	costSchemaVersion     uint64 = 1
	costBucketMeta               = "meta"
	costBucketSnapshots          = "snapshots"
	costBucketAnnotations        = "annotations"
	metaKeyVersion               = "version"
	metaKeyStack                 = "stack"
	metaKeyLastVersion           = "last_version"
	metaKeyCollectedAt           = "collected_at"
	timeKeyLen                   = 8
)

// CostDB is the per-stack bbolt timeline. It is not the resource-observation store.
type CostDB struct {
	db        *bolt.DB
	path      string
	clock     func() time.Time
	closeOnce sync.Once
	closeErr  error
}

// OpenCostDB creates or opens a cost timeline database for stack.
func OpenCostDB(path, stack string) (*CostDB, error) {
	if path == "" {
		return nil, errors.New("cost history path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), dbDirPermissions); err != nil {
		return nil, fmt.Errorf("creating cost history directory: %w", err)
	}
	db, err := bolt.Open(path, dbFilePermissions, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("opening cost history: %w", err)
	}
	store := newCostDB(db, path)
	if initErr := store.init(stack); initErr != nil {
		_ = store.Close()
		return nil, initErr
	}
	return store, nil
}

// OpenCostDBRead opens an existing timeline without creating buckets.
func OpenCostDBRead(path string) (*CostDB, error) {
	db, err := bolt.Open(path, dbFilePermissions, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("opening cost history: %w", err)
	}
	return newCostDB(db, path), nil
}

func newCostDB(db *bolt.DB, path string) *CostDB {
	return &CostDB{db: db, path: path, clock: time.Now}
}

func (d *CostDB) init(stack string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		meta, err := tx.CreateBucketIfNotExists([]byte(costBucketMeta))
		if err != nil {
			return fmt.Errorf("creating meta bucket: %w", err)
		}
		if _, snapErr := tx.CreateBucketIfNotExists([]byte(costBucketSnapshots)); snapErr != nil {
			return fmt.Errorf("creating snapshots bucket: %w", snapErr)
		}
		if _, annErr := tx.CreateBucketIfNotExists([]byte(costBucketAnnotations)); annErr != nil {
			return fmt.Errorf("creating annotations bucket: %w", annErr)
		}
		return initMeta(meta, stack)
	})
}

func initMeta(meta *bolt.Bucket, stack string) error {
	version := getUint64(meta, metaKeyVersion)
	switch {
	case version == 0:
		if err := putUint64(meta, metaKeyVersion, costSchemaVersion); err != nil {
			return err
		}
	case version != costSchemaVersion:
		return fmt.Errorf("unsupported cost history schema version %d", version)
	}
	existing := string(meta.Get([]byte(metaKeyStack)))
	if existing == "" && stack != "" {
		if err := meta.Put([]byte(metaKeyStack), []byte(stack)); err != nil {
			return fmt.Errorf("writing stack name: %w", err)
		}
	} else if stack != "" && existing != "" && existing != stack {
		return fmt.Errorf("cost history database stack %q does not match %q", existing, stack)
	}
	return nil
}

// Close releases the database. A second call returns the first error.
func (d *CostDB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	d.closeOnce.Do(func() {
		d.closeErr = d.db.Close()
	})
	return d.closeErr
}

// Path returns the database file path.
func (d *CostDB) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

// Put stores a snapshot and its annotation in one transaction.
// A timestamp collision bumps the key by one second. The JSON keeps the real timestamp.
// last_version becomes the maximum stored checkpoint.
func (d *CostDB) Put(snapshot CostSnapshot, annotation CostAnnotation) error {
	body, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("encoding snapshot: %w", err)
	}
	if annotation.ResourceChanges == nil {
		annotation.ResourceChanges = map[string]int{}
	}
	annBody, err := json.Marshal(annotation)
	if err != nil {
		return fmt.Errorf("encoding annotation: %w", err)
	}
	return d.db.Update(func(tx *bolt.Tx) error {
		return writeSnapshot(tx, d.now(), snapshot, body, annBody)
	})
}

func writeSnapshot(tx *bolt.Tx, now time.Time, snapshot CostSnapshot, body, annBody []byte) error {
	snaps := tx.Bucket([]byte(costBucketSnapshots))
	anns := tx.Bucket([]byte(costBucketAnnotations))
	meta := tx.Bucket([]byte(costBucketMeta))
	if snaps == nil || anns == nil || meta == nil {
		return errors.New("cost history buckets are missing")
	}
	key := freeTimeKey(snaps, anns, snapshot.Timestamp)
	if err := snaps.Put(key, body); err != nil {
		return fmt.Errorf("writing snapshot: %w", err)
	}
	if err := anns.Put(key, annBody); err != nil {
		return fmt.Errorf("writing annotation: %w", err)
	}
	last := getUint64(meta, metaKeyLastVersion)
	if snapshot.Version >= 0 && uint64(snapshot.Version) > last {
		if err := putUint64(meta, metaKeyLastVersion, uint64(snapshot.Version)); err != nil {
			return err
		}
	}
	collected := now.UTC().Format(time.RFC3339)
	if err := meta.Put([]byte(metaKeyCollectedAt), []byte(collected)); err != nil {
		return fmt.Errorf("writing collected_at: %w", err)
	}
	return nil
}

// Snapshots returns checkpoints whose timestamp is inside the inclusive range.
// A zero bound is open.
func (d *CostDB) Snapshots(from, to time.Time) ([]CostSnapshot, error) {
	var out []CostSnapshot
	err := d.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(costBucketSnapshots))
		if bucket == nil {
			return nil
		}
		return bucket.ForEach(func(_, value []byte) error {
			var snapshot CostSnapshot
			if err := json.Unmarshal(value, &snapshot); err != nil {
				return fmt.Errorf("decoding snapshot: %w", err)
			}
			if inRange(snapshot.Timestamp, from, to) {
				out = append(out, snapshot)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []CostSnapshot{}
	}
	return out, nil
}

// Annotations returns annotations stored beside snapshots in the same time range.
func (d *CostDB) Annotations(from, to time.Time) ([]CostAnnotation, error) {
	var out []CostAnnotation
	err := d.db.View(func(tx *bolt.Tx) error {
		snaps := tx.Bucket([]byte(costBucketSnapshots))
		anns := tx.Bucket([]byte(costBucketAnnotations))
		if snaps == nil || anns == nil {
			return nil
		}
		return snaps.ForEach(func(key, value []byte) error {
			var snapshot CostSnapshot
			if err := json.Unmarshal(value, &snapshot); err != nil {
				return fmt.Errorf("decoding snapshot: %w", err)
			}
			if !inRange(snapshot.Timestamp, from, to) {
				return nil
			}
			raw := anns.Get(key)
			if raw == nil {
				return nil
			}
			var annotation CostAnnotation
			if err := json.Unmarshal(raw, &annotation); err != nil {
				return fmt.Errorf("decoding annotation: %w", err)
			}
			out = append(out, annotation)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []CostAnnotation{}
	}
	return out, nil
}

func inRange(when, from, to time.Time) bool {
	if !from.IsZero() && when.Before(from) {
		return false
	}
	if !to.IsZero() && when.After(to) {
		return false
	}
	return true
}

// Reset drops snapshots and annotations and sets last_version to 0.
func (d *CostDB) Reset() error {
	return d.db.Update(func(tx *bolt.Tx) error {
		if err := clearBucket(tx.Bucket([]byte(costBucketSnapshots))); err != nil {
			return err
		}
		if err := clearBucket(tx.Bucket([]byte(costBucketAnnotations))); err != nil {
			return err
		}
		meta := tx.Bucket([]byte(costBucketMeta))
		if meta == nil {
			return errors.New("cost history buckets are missing")
		}
		if err := putUint64(meta, metaKeyLastVersion, 0); err != nil {
			return err
		}
		collected := d.now().UTC().Format(time.RFC3339)
		if err := meta.Put([]byte(metaKeyCollectedAt), []byte(collected)); err != nil {
			return fmt.Errorf("writing collected_at: %w", err)
		}
		return nil
	})
}

func clearBucket(bucket *bolt.Bucket) error {
	if bucket == nil {
		return errors.New("cost history buckets are missing")
	}
	var keys [][]byte
	if err := bucket.ForEach(func(key, _ []byte) error {
		keys = append(keys, append([]byte(nil), key...))
		return nil
	}); err != nil {
		return err
	}
	for _, key := range keys {
		if err := bucket.Delete(key); err != nil {
			return fmt.Errorf("clearing cost history: %w", err)
		}
	}
	return nil
}

// Stats summarizes the open database.
func (d *CostDB) Stats() (CostDBStats, error) {
	info, err := os.Stat(d.path)
	if err != nil {
		return CostDBStats{}, fmt.Errorf("statting cost history: %w", err)
	}
	snapshots, err := d.Snapshots(time.Time{}, time.Time{})
	if err != nil {
		return CostDBStats{}, err
	}
	stats := CostDBStats{
		Path:      d.path,
		Snapshots: len(snapshots),
		Size:      info.Size(),
	}
	if len(snapshots) > 0 {
		stats.First = snapshots[0].Timestamp
		stats.Last = snapshots[len(snapshots)-1].Timestamp
	}
	err = d.db.View(func(tx *bolt.Tx) error {
		meta := tx.Bucket([]byte(costBucketMeta))
		if meta == nil {
			return errors.New("cost history buckets are missing")
		}
		stats.Stack = string(meta.Get([]byte(metaKeyStack)))
		stats.LastVersion = getUint64(meta, metaKeyLastVersion)
		stats.Schema = getUint64(meta, metaKeyVersion)
		if raw := meta.Get([]byte(metaKeyCollectedAt)); len(raw) > 0 {
			parsed, parseErr := time.Parse(time.RFC3339, string(raw))
			if parseErr != nil {
				return fmt.Errorf("parsing collected_at: %w", parseErr)
			}
			stats.CollectedAt = parsed
		}
		return nil
	})
	if err != nil {
		return CostDBStats{}, err
	}
	return stats, nil
}

// ListCostDBs opens every `*.history.db` in dir. A missing directory is an empty list.
func ListCostDBs(dir string) ([]CostDBStats, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []CostDBStats{}, nil
		}
		return nil, fmt.Errorf("reading cost history directory: %w", err)
	}
	stats := make([]CostDBStats, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !IsCostHistoryFile(entry.Name()) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		db, openErr := OpenCostDBRead(path)
		if openErr != nil {
			return nil, openErr
		}
		item, statErr := db.Stats()
		closeErr := db.Close()
		if statErr != nil {
			return nil, statErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		stats = append(stats, item)
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].Stack < stats[j].Stack })
	return stats, nil
}

func (d *CostDB) now() time.Time {
	if d != nil && d.clock != nil {
		return d.clock()
	}
	return time.Now()
}

func freeTimeKey(snaps, anns *bolt.Bucket, when time.Time) []byte {
	key := timeKey(when)
	for snaps.Get(key) != nil || anns.Get(key) != nil {
		binary.BigEndian.PutUint64(key, binary.BigEndian.Uint64(key)+1)
	}
	return key
}

func timeKey(when time.Time) []byte {
	key := make([]byte, timeKeyLen)
	unix := max(when.Unix(), 0)
	binary.BigEndian.PutUint64(key, uint64(unix))
	return key
}

func getUint64(bucket *bolt.Bucket, key string) uint64 {
	value := bucket.Get([]byte(key))
	if len(value) != timeKeyLen {
		return 0
	}
	return binary.BigEndian.Uint64(value)
}

func putUint64(bucket *bolt.Bucket, key string, value uint64) error {
	var buf [timeKeyLen]byte
	binary.BigEndian.PutUint64(buf[:], value)
	if err := bucket.Put([]byte(key), buf[:]); err != nil {
		return fmt.Errorf("writing %s: %w", key, err)
	}
	return nil
}
