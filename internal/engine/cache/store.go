package cache

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

// Common cache errors.
var (
	ErrCacheNotFound   = errors.New("cache entry not found")
	ErrCacheExpired    = errors.New("cache entry expired")
	ErrInvalidCacheKey = errors.New("cache key cannot be empty")
	ErrCacheDisabled   = errors.New("cache is disabled")
	ErrCacheLocked     = errors.New("cache database locked by another process")
)

// Cache defines the interface for cache operations used by the engine.
// All implementations must be safe for concurrent use by multiple goroutines.
type Cache interface {
	Get(key string) (*CacheEntry, error)
	Set(key string, data json.RawMessage) error
	IsEnabled() bool
	Close() error
	InvalidateByPrefix(prefix string) (int, error)
}

// allBucketNames returns the top-level bucket names used by the cache database.
// The returned slice contains BucketProjected, BucketActual, and BucketRecommendations.
func allBucketNames() []string {
	return []string{BucketProjected, BucketActual, BucketRecommendations}
}

// Compile-time check that BoltStore implements Cache.
var _ Cache = (*BoltStore)(nil)

// dbLockTimeout is the time to wait for the database file lock before giving up.
const dbLockTimeout = 500 * time.Millisecond

// dbFileName is the name of the BoltDB database file within the cache directory.
const dbFileName = "cache.db"

// compactPageSize is the page size used when compacting the database.
const compactPageSize = 65536

// dbFilePermissions is the file mode used when creating or opening the database.
const dbFilePermissions = 0o600

// bytesPerMB is the number of bytes in a megabyte.
const bytesPerMB = 1024 * 1024

// BoltStore provides BoltDB-backed caching with TTL expiration.
// It stores cache entries as JSON values in named buckets.
// Thread-safe: uses bbolt's internal MVCC for concurrency.
type BoltStore struct {
	db         *bolt.DB
	dbPath     string
	ttlSeconds int
	maxSizeMB  int
	enabled    bool
	logger     zerolog.Logger
	closeOnce  sync.Once
	closeErr   error
}

// NewBoltStore creates a new BoltDB-backed cache store.
// The database file is stored at {directory}/cache.db.
// If enabled is false, returns a disabled store where Get returns
// ErrCacheDisabled and Set is a no-op.
// If the database file is locked by another process (timeout 500ms),
// returns nil, ErrCacheLocked so the caller can degrade gracefully.
// NewBoltStore creates and returns a BoltStore backed by a BoltDB file located
// in the provided directory.
//
// NewBoltStore will:
// - return a disabled BoltStore when `enabled` is false.
// - create the directory if it does not exist.
// - open or create the BoltDB file at "<directory>/cache.db"; if the database is
//   locked by another process an error is returned, and if the file is detected
//   as corrupted it will be deleted and recreated.
// - initialize the required top-level buckets.
// - run a startup cleanup of expired entries and perform a size check that may
//   trigger compaction if the DB exceeds `maxSizeMB`.
//
// Parameters:
// - ctx: context used to derive a logger.
// - directory: filesystem directory to contain the BoltDB file (must be
//   non-empty).
// - enabled: if false, returns a disabled store without touching the filesystem.
// - ttlSeconds: default time-to-live for new cache entries, in seconds.
// - maxSizeMB: maximum database size in megabytes used to decide compaction.
//
// Returns:
// - *BoltStore on success, or an error if the directory is invalid, directory
//   creation fails, the database cannot be opened/created, or bucket
//   initialization fails.
func NewBoltStore(ctx context.Context, directory string, enabled bool, ttlSeconds, maxSizeMB int) (*BoltStore, error) {
	logger := logging.FromContext(ctx).With().
		Str("component", "cache").
		Str("backend", "boltdb").
		Logger()

	if !enabled {
		return &BoltStore{enabled: false, logger: logger}, nil
	}

	if directory == "" {
		return nil, errors.New("cache directory cannot be empty")
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	dbPath := filepath.Join(directory, dbFileName)

	db, err := openBoltDB(dbPath, &logger)
	if err != nil {
		return nil, err
	}

	store := &BoltStore{
		db:         db,
		dbPath:     dbPath,
		ttlSeconds: ttlSeconds,
		maxSizeMB:  maxSizeMB,
		enabled:    true,
		logger:     logger,
	}

	// Initialize buckets
	if bucketErr := store.initBuckets(); bucketErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize cache buckets: %w", bucketErr)
	}

	// Startup cleanup of expired entries
	if cleaned, cleanErr := store.CleanupExpired(); cleanErr != nil {
		logger.Warn().Err(cleanErr).Msg("startup cache cleanup failed")
	} else if cleaned > 0 {
		logger.Debug().Int("cleaned", cleaned).Msg("startup cache cleanup completed")
	}

	// Startup size check: compact if over maxSizeMB.
	store.compactIfOversized()

	return store, nil
}

// compactIfOversized checks whether the DB file exceeds maxSizeMB and compacts it.
func (s *BoltStore) compactIfOversized() {
	if s.maxSizeMB <= 0 {
		return
	}
	sz, szErr := s.Size()
	if szErr != nil {
		return
	}
	maxBytes := int64(s.maxSizeMB) * bytesPerMB
	if sz <= maxBytes {
		return
	}
	s.logger.Warn().
		Int64("size_bytes", sz).
		Int("max_size_mb", s.maxSizeMB).
		Msg("cache file exceeds max size, compacting")
	if compactErr := s.Compact(); compactErr != nil {
		s.logger.Warn().Err(compactErr).Msg("startup compaction failed")
	}
}

// openBoltDB opens the BoltDB file at dbPath, handling lock and corruption scenarios.
// It attempts to open the database with a configured timeout. If the database file
// is locked by another process it logs a warning and returns ErrCacheLocked. If the
// open fails due to detected corruption the file is removed (unless already missing)
// and a fresh database is created and returned.
// Parameters:
//  - dbPath: filesystem path to the BoltDB file.
//  - logger: optional logger used to report lock or corruption events.
// Returns the opened *bolt.DB on success, or a non-nil error describing the failure.
func openBoltDB(dbPath string, logger *zerolog.Logger) (*bolt.DB, error) {
	db, err := bolt.Open(dbPath, dbFilePermissions, &bolt.Options{
		Timeout: dbLockTimeout,
	})
	if err == nil {
		return db, nil
	}

	// Check for lock timeout → graceful degradation
	if errors.Is(err, berrors.ErrTimeout) {
		logger.Warn().Str("path", dbPath).Msg("cache database locked by another process, proceeding without cache")
		return nil, ErrCacheLocked
	}

	// Check for corruption → delete and retry
	if isCorruptionError(err) {
		logger.Warn().Err(err).Str("path", dbPath).Msg("corrupt cache database detected, recreating")
		if removeErr := os.Remove(dbPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return nil, fmt.Errorf("failed to remove corrupt cache database: %w", removeErr)
		}
		freshDB, retryErr := bolt.Open(dbPath, dbFilePermissions, &bolt.Options{
			Timeout: dbLockTimeout,
		})
		if retryErr != nil {
			return nil, fmt.Errorf("failed to create fresh cache database: %w", retryErr)
		}
		return freshDB, nil
	}

	return nil, fmt.Errorf("failed to open cache database: %w", err)
}

// isCorruptionError reports whether err represents a BoltDB corruption condition
// (specifically `berrors.ErrInvalid`, `berrors.ErrChecksum`, or
// `berrors.ErrVersionMismatch`). It returns false for a nil error.
func isCorruptionError(err error) bool {
	return errors.Is(err, berrors.ErrInvalid) ||
		errors.Is(err, berrors.ErrChecksum) ||
		errors.Is(err, berrors.ErrVersionMismatch)
}

// initBuckets creates the required top-level buckets if they don't exist.
func (s *BoltStore) initBuckets() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, name := range allBucketNames() {
			if _, err := tx.CreateBucketIfNotExists([]byte(name)); err != nil {
				return fmt.Errorf("creating bucket %q: %w", name, err)
			}
		}
		return nil
	})
}

// Get retrieves a cache entry by key.
// Returns ErrCacheNotFound if the key does not exist.
// Returns ErrCacheExpired if the entry exists but has expired (lazily deleted).
// Returns ErrCacheDisabled if the store is disabled.
func (s *BoltStore) Get(key string) (*CacheEntry, error) {
	if !s.enabled {
		return nil, ErrCacheDisabled
	}
	if key == "" {
		return nil, ErrInvalidCacheKey
	}

	bucketName := BucketFromKey(key)
	innerKey := StripBucket(key)

	var valueCopy []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return ErrCacheNotFound
		}
		v := b.Get([]byte(innerKey))
		if v == nil {
			return ErrCacheNotFound
		}
		// Copy value bytes: bbolt values are only valid during the transaction
		valueCopy = make([]byte, len(v))
		copy(valueCopy, v)
		return nil
	})
	if err != nil {
		return nil, err
	}

	var entry CacheEntry
	if unmarshalErr := json.Unmarshal(valueCopy, &entry); unmarshalErr != nil {
		// Corrupt entry: delete it and return not found
		s.deleteKeyAsync(bucketName, innerKey)
		return nil, ErrCacheNotFound
	}

	if entry.IsExpired() {
		// Lazy delete via async batch
		s.deleteKeyAsync(bucketName, innerKey)
		return nil, ErrCacheExpired
	}

	return &entry, nil
}

// Set stores a cache entry with the given key and data.
// The key format determines which bucket the entry is stored in.
// Concurrent calls are batched for efficiency via db.Batch().
// Returns ErrCacheDisabled if caching is disabled.
// Returns ErrInvalidCacheKey if key is empty.
func (s *BoltStore) Set(key string, data json.RawMessage) error {
	if !s.enabled {
		return nil
	}
	if key == "" {
		return ErrInvalidCacheKey
	}

	bucketName := BucketFromKey(key)
	innerKey := StripBucket(key)

	entry := NewCacheEntry(key, data, s.ttlSeconds)
	entryData, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", marshalErr)
	}

	err := s.db.Batch(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return fmt.Errorf("bucket %q does not exist", bucketName)
		}
		return b.Put([]byte(innerKey), entryData)
	})
	if err != nil {
		return fmt.Errorf("cache write failed: %w", err)
	}

	return nil
}

// IsEnabled returns true if caching is enabled.
func (s *BoltStore) IsEnabled() bool {
	return s.enabled
}

// Close releases the database file handle and flushes pending writes.
// Safe to call multiple times; only the first call closes the database.
func (s *BoltStore) Close() error {
	s.closeOnce.Do(func() {
		if s.db != nil {
			s.closeErr = s.db.Close()
		}
	})
	return s.closeErr
}

// InvalidateByPrefix removes all cache entries whose keys start with
// the given prefix. Returns the count of entries removed.
// An empty prefix clears the entire cache.
//
//nolint:gocognit // Prefix-based deletion requires bucket routing with cursor iteration.
func (s *BoltStore) InvalidateByPrefix(prefix string) (int, error) {
	if !s.enabled {
		return 0, ErrCacheDisabled
	}

	// Empty prefix: clear all buckets
	if prefix == "" {
		return s.clearAllBuckets()
	}

	bucketName := BucketFromKey(prefix)
	if !isValidBucket(bucketName) {
		return 0, nil
	}

	innerPrefix := StripBucket(prefix)

	count := 0
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}

		// If the prefix is just the bucket name (no inner prefix), delete all in bucket
		if innerPrefix == "" || innerPrefix == bucketName {
			c := b.Cursor()
			for k, _ := c.First(); k != nil; k, _ = c.Next() {
				if delErr := b.Delete(k); delErr != nil {
					return delErr
				}
				count++
			}
			return nil
		}

		// Prefix scan within bucket
		prefixBytes := []byte(innerPrefix)
		c := b.Cursor()
		for k, _ := c.Seek(prefixBytes); k != nil && bytes.HasPrefix(k, prefixBytes); k, _ = c.Next() {
			if delErr := b.Delete(k); delErr != nil {
				return delErr
			}
			count++
		}
		return nil
	})

	return count, err
}

// Delete removes a single cache entry by exact key.
// Idempotent: no error if key doesn't exist.
func (s *BoltStore) Delete(key string) error {
	if !s.enabled {
		return ErrCacheDisabled
	}
	if key == "" {
		return ErrInvalidCacheKey
	}

	bucketName := BucketFromKey(key)
	innerKey := StripBucket(key)

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(innerKey))
	})
}

// Clear removes all entries from all buckets.
func (s *BoltStore) Clear() error {
	if !s.enabled {
		return ErrCacheDisabled
	}

	_, err := s.clearAllBuckets()
	return err
}

// CleanupExpired removes all expired entries across all buckets.
// Returns the number of entries removed.
func (s *BoltStore) CleanupExpired() (int, error) {
	if !s.enabled {
		return 0, ErrCacheDisabled
	}

	now := time.Now()
	totalCleaned := 0

	for _, bucketName := range allBucketNames() {
		cleaned, err := s.cleanupBucketExpired(bucketName, now)
		if err != nil {
			s.logger.Warn().Err(err).Str("bucket", bucketName).Msg("cleanup failed for bucket")
			continue
		}
		totalCleaned += cleaned
	}

	return totalCleaned, nil
}

// Size returns the current database file size in bytes.
func (s *BoltStore) Size() (int64, error) {
	if !s.enabled {
		return 0, ErrCacheDisabled
	}

	info, err := os.Stat(s.dbPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat cache database: %w", err)
	}
	return info.Size(), nil
}

// Count returns the total number of entries across all buckets.
func (s *BoltStore) Count() (int, error) {
	if !s.enabled {
		return 0, ErrCacheDisabled
	}

	total := 0
	err := s.db.View(func(tx *bolt.Tx) error {
		for _, name := range allBucketNames() {
			b := tx.Bucket([]byte(name))
			if b == nil {
				continue
			}
			total += b.Stats().KeyN
		}
		return nil
	})
	return total, err
}

// Compact rewrites the database to reclaim free pages.
// It must only be called when no concurrent operations are running (e.g., at startup).
func (s *BoltStore) Compact() error {
	if !s.enabled {
		return ErrCacheDisabled
	}

	tmpPath := s.dbPath + ".compact"

	// Open destination database
	dst, err := bolt.Open(tmpPath, dbFilePermissions, &bolt.Options{Timeout: dbLockTimeout})
	if err != nil {
		return fmt.Errorf("failed to open temporary database for compaction: %w", err)
	}

	// Compact: copy live data from source to destination
	if compactErr := bolt.Compact(dst, s.db, compactPageSize); compactErr != nil {
		_ = dst.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("compaction failed: %w", compactErr)
	}

	if closeErr := dst.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to close compacted database: %w", closeErr)
	}

	// Close source database
	if closeErr := s.db.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to close source database: %w", closeErr)
	}

	// Replace source with compacted file
	if renameErr := os.Rename(tmpPath, s.dbPath); renameErr != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to replace database with compacted version: %w", renameErr)
	}

	// Reopen database
	db, openErr := bolt.Open(s.dbPath, dbFilePermissions, &bolt.Options{Timeout: dbLockTimeout})
	if openErr != nil {
		return fmt.Errorf("failed to reopen compacted database: %w", openErr)
	}
	s.db = db

	// Reinitialize buckets
	return s.initBuckets()
}

// GetDirectory returns the cache directory path.
func (s *BoltStore) GetDirectory() string {
	return filepath.Dir(s.dbPath)
}

// GetTTL returns the default TTL in seconds.
func (s *BoltStore) GetTTL() int {
	return s.ttlSeconds
}

// clearAllBuckets deletes and recreates all buckets within a single transaction.
func (s *BoltStore) clearAllBuckets() (int, error) {
	count := 0
	err := s.db.Update(func(tx *bolt.Tx) error {
		for _, name := range allBucketNames() {
			b := tx.Bucket([]byte(name))
			if b != nil {
				count += b.Stats().KeyN
				if delErr := tx.DeleteBucket([]byte(name)); delErr != nil {
					return delErr
				}
			}
			if _, createErr := tx.CreateBucket([]byte(name)); createErr != nil {
				return createErr
			}
		}
		return nil
	})
	return count, err
}

// cleanupBucketExpired removes expired entries from a single bucket.
func (s *BoltStore) cleanupBucketExpired(bucketName string, now time.Time) (int, error) {
	// First, collect expired keys via a read transaction
	var expiredKeys [][]byte
	if err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var entry CacheEntry
			if unmarshalErr := json.Unmarshal(v, &entry); unmarshalErr != nil {
				// Corrupt entry: mark for deletion
				keyCopy := make([]byte, len(k))
				copy(keyCopy, k)
				expiredKeys = append(expiredKeys, keyCopy)
				continue
			}
			if now.After(entry.ExpiresAt) {
				keyCopy := make([]byte, len(k))
				copy(keyCopy, k)
				expiredKeys = append(expiredKeys, keyCopy)
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}

	if len(expiredKeys) == 0 {
		return 0, nil
	}

	// Delete expired keys in a write transaction
	deleted := 0
	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		for _, k := range expiredKeys {
			if delErr := b.Delete(k); delErr != nil {
				return delErr
			}
			deleted++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	return deleted, nil
}

// deleteKeyAsync schedules a lazy delete of an expired/corrupt entry via db.Batch().
func (s *BoltStore) deleteKeyAsync(bucketName, innerKey string) {
	if err := s.db.Batch(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucketName))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(innerKey))
	}); err != nil {
		s.logger.Debug().
			Err(err).
			Str("bucket", bucketName).
			Str("key", innerKey).
			Msg("async cache key deletion failed")
	}
}