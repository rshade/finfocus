package history

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rs/zerolog"
	bolt "go.etcd.io/bbolt"
	berrors "go.etcd.io/bbolt/errors"

	"github.com/rshade/finfocus/internal/logging"
)

// Common history store errors.
var (
	ErrHistoryDisabled = errors.New("history store is disabled")
	ErrHistoryLocked   = errors.New("history database locked by another process")
)

// Bucket names for the history database.
const (
	BucketResourceHistory = "resource_history"
	BucketResourceTags    = "resource_tags"
)

// Database configuration constants.
const (
	dbLockTimeout     = 500 * time.Millisecond
	dbFileName        = "history.db"
	dbFilePermissions = 0o600
	dbDirPermissions  = 0o750
)

// Store defines the interface for resource history persistence.
// It follows the same optional-dependency pattern as cache.Cache.
// When nil or disabled, callers degrade gracefully to current behavior.
type Store interface {
	Upsert(entry ResourceHistoryEntry) error
	UpsertBatch(entries []ResourceHistoryEntry) error
	GetCloudIDsForURN(stackHash, urnHash string, from, to int64) ([]ResourceHistoryEntry, error)
	GetAllForStack(stackHash string, from, to int64) ([]ResourceHistoryEntry, error)
	GetDeletedResources(
		stackHash string, currentURNHashes map[string]bool, from, to int64,
	) ([]ResourceHistoryEntry, error)
	CleanupExpired(retentionDays int) (int, error)
	IsEnabled() bool
	Close() error
}

// Compile-time check that BoltStore implements Store.
var _ Store = (*BoltStore)(nil)

// BoltStore provides BoltDB-backed resource history storage.
// Thread-safe: uses bbolt's internal MVCC for concurrency.
type BoltStore struct {
	db        *bolt.DB
	dbPath    string
	enabled   bool
	logger    zerolog.Logger
	closeOnce sync.Once
	closeErr  error
}

// NewBoltStore creates a BoltStore backed by a BoltDB file in the provided directory.
//
// When enabled is false, returns a disabled store where all operations are no-ops.
// Handles corruption by deleting and recreating the database file.
// Runs startup cleanup of expired entries.
func NewBoltStore(ctx context.Context, directory string, enabled bool, retentionDays int) (*BoltStore, error) {
	logger := logging.FromContext(ctx).With().
		Str("component", "history").
		Str("backend", "boltdb").
		Logger()

	if !enabled {
		return &BoltStore{enabled: false, logger: logger}, nil
	}

	if directory == "" {
		return nil, errors.New("history directory cannot be empty")
	}

	if err := os.MkdirAll(directory, dbDirPermissions); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	dbPath := filepath.Join(directory, dbFileName)

	db, err := openHistoryDB(dbPath, &logger)
	if err != nil {
		return nil, err
	}

	store := &BoltStore{
		db:      db,
		dbPath:  dbPath,
		enabled: true,
		logger:  logger,
	}

	if bucketErr := store.initBuckets(); bucketErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize history buckets: %w", bucketErr)
	}

	if cleaned, cleanErr := store.CleanupExpired(retentionDays); cleanErr != nil {
		logger.Warn().Err(cleanErr).Msg("startup history cleanup failed")
	} else if cleaned > 0 {
		logger.Debug().Int("cleaned", cleaned).Msg("startup history cleanup completed")
	}

	return store, nil
}

// openHistoryDB opens the BoltDB file, handling lock and corruption scenarios.
func openHistoryDB(dbPath string, logger *zerolog.Logger) (*bolt.DB, error) {
	db, err := bolt.Open(dbPath, dbFilePermissions, &bolt.Options{
		Timeout: dbLockTimeout,
	})
	if err == nil {
		return db, nil
	}

	if errors.Is(err, berrors.ErrTimeout) {
		logger.Warn().Str("path", dbPath).Msg("history database locked by another process")
		return nil, ErrHistoryLocked
	}

	if isCorruptionError(err) {
		logger.Warn().Err(err).Str("path", dbPath).Msg("corrupt history database detected, recreating")
		if removeErr := os.Remove(dbPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, fmt.Errorf("failed to remove corrupt history database: %w", removeErr)
		}
		freshDB, retryErr := bolt.Open(dbPath, dbFilePermissions, &bolt.Options{
			Timeout: dbLockTimeout,
		})
		if retryErr != nil {
			return nil, fmt.Errorf("failed to create fresh history database: %w", retryErr)
		}
		logger.Info().Str("path", dbPath).Msg("successfully recreated history database")
		return freshDB, nil
	}

	return nil, fmt.Errorf("failed to open history database: %w", err)
}

func isCorruptionError(err error) bool {
	return errors.Is(err, berrors.ErrInvalid) ||
		errors.Is(err, berrors.ErrChecksum) ||
		errors.Is(err, berrors.ErrVersionMismatch)
}

func (s *BoltStore) initBuckets() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, name := range []string{BucketResourceHistory, BucketResourceTags} {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("creating bucket %q: %w", name, err)
			}
		}
		return nil
	})
}

// Upsert records a single resource observation.
// If the (URN, CloudID) pair already exists for this stack, only LastSeen is updated.
func (s *BoltStore) Upsert(entry ResourceHistoryEntry) error {
	if !s.enabled {
		return nil
	}

	return s.db.Batch(func(tx *bolt.Tx) error {
		return s.upsertInTx(tx, entry)
	})
}

// UpsertBatch records multiple resource observations atomically using db.Batch()
// for coalesced writes.
func (s *BoltStore) UpsertBatch(entries []ResourceHistoryEntry) error {
	if !s.enabled {
		return nil
	}

	return s.db.Batch(func(tx *bolt.Tx) error {
		for i := range entries {
			if err := s.upsertInTx(tx, entries[i]); err != nil {
				return err
			}
		}
		return nil
	})
}

// upsertInTx performs a single upsert within a transaction.
// Key format: {urn_hash}/{cloud_id}. Stack-scoping is handled by the writer layer.
func (s *BoltStore) upsertInTx(tx *bolt.Tx, entry ResourceHistoryEntry) error {
	b := tx.Bucket([]byte(BucketResourceHistory))
	if b == nil {
		return fmt.Errorf("bucket %q does not exist", BucketResourceHistory)
	}

	urnHash := URNHash(entry.URN)
	key := urnHash + "/" + entry.CloudID

	if updated, err := s.tryUpdateExisting(b, key, entry); err != nil {
		return err
	} else if updated {
		return nil
	}

	data, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal new entry: %w", marshalErr)
	}

	if putErr := b.Put([]byte(key), data); putErr != nil {
		return putErr
	}

	s.upsertTags(tx, entry, urnHash)
	return nil
}

// tryUpdateExisting attempts to update an existing entry's LastSeen.
// Returns true if an existing entry was found and updated.
func (s *BoltStore) tryUpdateExisting(
	b *bolt.Bucket, key string, entry ResourceHistoryEntry,
) (bool, error) {
	existing := b.Get([]byte(key))
	if existing == nil {
		return false, nil
	}

	var existingEntry ResourceHistoryEntry
	if err := json.Unmarshal(existing, &existingEntry); err != nil {
		// Corrupt entry — overwrite by returning false so the caller creates a fresh one
		return false, nil //nolint:nilerr // intentional: corrupt entries are overwritten
	}

	if entry.LastSeen > existingEntry.LastSeen {
		existingEntry.LastSeen = entry.LastSeen
	}

	// Merge metadata: update Type/Provider/Source when incoming values are non-empty
	if entry.Type != "" {
		existingEntry.Type = entry.Type
	}
	if entry.Provider != "" {
		existingEntry.Provider = entry.Provider
	}
	if entry.Source != "" {
		existingEntry.Source = entry.Source
	}

	// Merge tags (incoming tags override existing for matching keys)
	if len(entry.Tags) > 0 {
		if existingEntry.Tags == nil {
			existingEntry.Tags = make(map[string]string, len(entry.Tags))
		}
		for k, v := range entry.Tags {
			existingEntry.Tags[k] = v
		}
	}

	data, marshalErr := json.Marshal(existingEntry)
	if marshalErr != nil {
		return false, fmt.Errorf("failed to marshal updated entry: %w", marshalErr)
	}

	return true, b.Put([]byte(key), data)
}

// upsertTags updates the resource_tags bucket for tag-based lookups.
func (s *BoltStore) upsertTags(tx *bolt.Tx, entry ResourceHistoryEntry, urnHash string) {
	if len(entry.Tags) == 0 {
		return
	}

	tagBucket := tx.Bucket([]byte(BucketResourceTags))
	if tagBucket == nil {
		return
	}

	for tagKey, tagValue := range entry.Tags {
		tagKeyStr := tagKey + ":" + tagValue + "/" + urnHash
		tagEntry := map[string]any{
			"tag_key":    tagKey,
			"tag_value":  tagValue,
			"urn_hash":   urnHash,
			"cloud_id":   entry.CloudID,
			"first_seen": entry.FirstSeen,
			"last_seen":  entry.LastSeen,
		}
		tagData, tagMarshalErr := json.Marshal(tagEntry)
		if tagMarshalErr != nil {
			continue
		}
		_ = tagBucket.Put([]byte(tagKeyStr), tagData)
	}
}

// GetCloudIDsForURN returns all cloud IDs ever observed for a URN,
// filtered to entries where [FirstSeen, LastSeen] overlaps [from, to].
// The stackHash parameter is accepted for interface compatibility but
// not used as a key prefix — stack-scoping is handled by the writer layer.
func (s *BoltStore) GetCloudIDsForURN(_, urnHash string, from, to int64) ([]ResourceHistoryEntry, error) {
	if !s.enabled {
		return nil, nil
	}

	var results []ResourceHistoryEntry
	prefix := []byte(urnHash + "/")

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BucketResourceHistory))
		if b == nil {
			return nil
		}

		c := b.Cursor()
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var entry ResourceHistoryEntry
			if unmarshalErr := json.Unmarshal(v, &entry); unmarshalErr != nil {
				s.logger.Debug().Err(unmarshalErr).Str("key", string(k)).Msg("skipping corrupt entry")
				continue
			}

			if overlaps(entry.FirstSeen, entry.LastSeen, from, to) {
				results = append(results, entry)
			}
		}
		return nil
	})

	return results, err
}

// GetAllForStack returns all history entries for a stack,
// filtered to entries where [FirstSeen, LastSeen] overlaps [from, to].
// The stackHash parameter is accepted for interface compatibility.
func (s *BoltStore) GetAllForStack(_ string, from, to int64) ([]ResourceHistoryEntry, error) {
	if !s.enabled {
		return nil, nil
	}

	var results []ResourceHistoryEntry

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BucketResourceHistory))
		if b == nil {
			return nil
		}

		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var entry ResourceHistoryEntry
			if unmarshalErr := json.Unmarshal(v, &entry); unmarshalErr != nil {
				s.logger.Debug().Err(unmarshalErr).Str("key", string(k)).Msg("skipping corrupt entry")
				continue
			}

			if overlaps(entry.FirstSeen, entry.LastSeen, from, to) {
				results = append(results, entry)
			}
		}
		return nil
	})

	return results, err
}

// GetDeletedResources returns history entries that exist in the store
// but are NOT in the provided set of current URN hashes.
// The stackHash parameter is accepted for interface compatibility.
func (s *BoltStore) GetDeletedResources(
	_ string, currentURNHashes map[string]bool, from, to int64,
) ([]ResourceHistoryEntry, error) {
	if !s.enabled {
		return nil, nil
	}

	var results []ResourceHistoryEntry
	seen := make(map[string]bool)

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BucketResourceHistory))
		if b == nil {
			return nil
		}

		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var entry ResourceHistoryEntry
			if unmarshalErr := json.Unmarshal(v, &entry); unmarshalErr != nil {
				continue
			}

			entryURNHash := URNHash(entry.URN)
			if currentURNHashes[entryURNHash] {
				continue
			}

			if !overlaps(entry.FirstSeen, entry.LastSeen, from, to) {
				continue
			}

			if !seen[entryURNHash] {
				seen[entryURNHash] = true
				results = append(results, entry)
			}
		}
		return nil
	})

	return results, err
}

// CleanupExpired removes entries with LastSeen older than the retention window.
// Also cleans corresponding resource_tags entries.
// Returns count of removed resource_history entries.
func (s *BoltStore) CleanupExpired(retentionDays int) (int, error) {
	if !s.enabled {
		return 0, nil
	}

	return s.cleanupExpiredEntries(retentionDays)
}

// IsEnabled returns whether the store is active.
func (s *BoltStore) IsEnabled() bool {
	return s.enabled
}

// Close releases resources. Safe to call multiple times.
func (s *BoltStore) Close() error {
	s.closeOnce.Do(func() {
		if s.db != nil {
			s.closeErr = s.db.Close()
		}
	})
	return s.closeErr
}

// overlaps checks if time range [aFrom, aTo] overlaps with [bFrom, bTo].
func overlaps(aFrom, aTo, bFrom, bTo int64) bool {
	return aFrom <= bTo && aTo >= bFrom
}
