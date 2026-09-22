package cache

import (
	"encoding/json"
	"time"
)

// CacheEntry represents a single cached value with TTL metadata.
// It wraps arbitrary JSON-serializable data with expiration information.
//
//nolint:revive // CacheEntry is the canonical name for this exported type.
type CacheEntry struct {
	// Key is the cache key (structured, human-readable, `/`-separated).
	Key string `json:"key"`

	// Data is the cached value (JSON-serializable).
	Data json.RawMessage `json:"data"`

	// CreatedAt is the timestamp when the entry was created.
	CreatedAt time.Time `json:"created_at"`

	// ExpiresAt is the timestamp when the entry expires.
	ExpiresAt time.Time `json:"expires_at"`

	// TTLSeconds is the time-to-live in seconds (for reference).
	TTLSeconds int `json:"ttl_seconds"`
}

// NewCacheEntry creates a new cache entry with the given TTL.
// The entry is created with the current time and calculates expiration based on TTL.
func NewCacheEntry(key string, data json.RawMessage, ttlSeconds int) *CacheEntry {
	now := time.Now()
	return &CacheEntry{
		Key:        key,
		Data:       data,
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Duration(ttlSeconds) * time.Second),
		TTLSeconds: ttlSeconds,
	}
}

// IsExpired checks if the cache entry has expired based on current time.
// Returns true if the current time is after the expiration time.
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// IsValid checks if the cache entry is valid (not expired).
// This is the inverse of IsExpired() and is provided for readability.
func (e *CacheEntry) IsValid() bool {
	return !e.IsExpired()
}

// Age returns the duration since the entry was created.
func (e *CacheEntry) Age() time.Duration {
	return time.Since(e.CreatedAt)
}

// TimeUntilExpiration returns the duration until the entry expires.
// Returns 0 if already expired.
func (e *CacheEntry) TimeUntilExpiration() time.Duration {
	remaining := time.Until(e.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Touch updates the entry's expiration time by extending it by the original TTL.
// This is useful for implementing "refresh on access" caching strategies.
func (e *CacheEntry) Touch() {
	now := time.Now()
	e.ExpiresAt = now.Add(time.Duration(e.TTLSeconds) * time.Second)
}

// cacheEntryJSON is the wire format for CacheEntry stored in BoltDB.
// Times are stored as Unix timestamps (int64) for efficient storage and comparison.
type cacheEntryJSON struct {
	Key        string          `json:"key"`
	Data       json.RawMessage `json:"data"`
	CreatedAt  int64           `json:"created_at"`
	ExpiresAt  int64           `json:"expires_at"`
	TTLSeconds int             `json:"ttl_seconds"`
}

// unixSecondsCutoff distinguishes nanosecond timestamps from legacy
// second-precision timestamps when decoding. Any realistic date expressed in
// seconds (through the year 5138) is below this value; the same date expressed
// in nanoseconds (after 1973) is above it.
const unixSecondsCutoff = 100_000_000_000 // 1e11

// decodeUnixTime converts a stored int64 timestamp back into a time.Time.
// Values below unixSecondsCutoff are legacy second-precision timestamps and
// are scaled up; anything at or above the cutoff is already nanoseconds.
func decodeUnixTime(v int64) time.Time {
	if v < unixSecondsCutoff {
		return time.Unix(v, 0)
	}
	return time.Unix(0, v)
}

// MarshalJSON implements json.Marshaler for CacheEntry.
// Times are stored as Unix nanosecond timestamps (int64) for bbolt storage
// efficiency. Sub-second precision is required: truncating to whole seconds
// can make an entry appear expired up to a second before its true TTL elapses.
func (e *CacheEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(&cacheEntryJSON{
		Key:        e.Key,
		Data:       e.Data,
		CreatedAt:  e.CreatedAt.UnixNano(),
		ExpiresAt:  e.ExpiresAt.UnixNano(),
		TTLSeconds: e.TTLSeconds,
	})
}

// UnmarshalJSON implements json.Unmarshaler for CacheEntry.
// Parses Unix timestamps from stored data, accepting both the current
// nanosecond-precision format and the legacy second-precision format.
func (e *CacheEntry) UnmarshalJSON(data []byte) error {
	var aux cacheEntryJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	e.Key = aux.Key
	e.Data = aux.Data
	e.CreatedAt = decodeUnixTime(aux.CreatedAt)
	e.ExpiresAt = decodeUnixTime(aux.ExpiresAt)
	e.TTLSeconds = aux.TTLSeconds

	return nil
}
