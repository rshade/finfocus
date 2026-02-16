// Package cache provides BoltDB-backed caching with TTL expiration for cost query results.
//
// This package implements persistent caching using bbolt (go.etcd.io/bbolt) to improve
// CLI performance by avoiding redundant plugin calls for recently-fetched data.
//
// # Storage
//
// The cache is stored in a single BoltDB file (cache.db) within the project or global
// cache directory. BoltDB provides atomic transactions, indexed lookups, and reduced disk
// I/O compared to individual JSON files.
//
// # Bucket Layout
//
// Data is organized into three top-level buckets:
//
//   - projected: Per-resource projected cost results
//   - actual: Whole-query actual cost results
//   - recommendations: Recommendation query results
//
// # Key Format
//
// Keys use human-readable slash-separated paths for easy debugging and prefix scanning:
//
//   - projected/{provider}/{type}/{region}/{sku}
//   - actual/{provider}/{types}/{from}/{to}/{filter-hash}
//   - recommendations/multi/{sorted-types}
//
// # Concurrency
//
// BoltDB supports concurrent reads via read-only transactions. Writes are serialized
// through DB.Batch() for automatic coalescing of concurrent write operations. The
// database file is protected by an OS-level file lock (flock/LockFileEx).
//
// # TTL Expiration
//
// Entries are checked for expiration on read (lazy expiration). A startup cleanup
// pass removes expired entries in bulk. TTL is configurable via CLI flag
// (--cache-ttl), environment variable (FINFOCUS_CACHE_TTL), or config file.
package cache
