// Package cache provides file-based caching with TTL expiration for query results.
//
// This package implements persistent caching to improve CLI performance by avoiding
// redundant plugin calls for recently-fetched data. Key features:
//   - File-based storage in ~/.finfocus/cache/ (cross-platform, no external dependencies)
//   - Configurable TTL (default 1 hour) via config file, environment variable, or CLI flag
//   - Automatic expiration and cleanup of stale entries
//   - SHA256-based cache keys for deterministic lookups
//
// The cache is designed for CLI workflows where queries may be repeated within a short
// time window (e.g., iterating on commands during development or automation).
package cache
