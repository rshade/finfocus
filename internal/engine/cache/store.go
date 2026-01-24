package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// cacheFileExtension is the file extension used for cache entries.
const cacheFileExtension = ".json"

// Common cache errors.
var (
	ErrCacheNotFound   = errors.New("cache entry not found")
	ErrCacheExpired    = errors.New("cache entry expired")
	ErrInvalidCacheKey = errors.New("cache key cannot be empty")
	ErrCacheDisabled   = errors.New("cache is disabled")
)

// FileStore provides file-based caching with TTL expiration.
// It stores cache entries as JSON files in a directory structure.
// Thread-safe for concurrent access.
type FileStore struct {
	// directory is the cache directory path.
	directory string

	// enabled controls whether caching is active.
	enabled bool

	// ttlSeconds is the default TTL for cache entries.
	ttlSeconds int

	// maxSizeMB is the maximum cache size in megabytes (0 = unlimited).
	maxSizeMB int

	// mu protects concurrent access to file operations.
	mu sync.RWMutex
}

// NewFileStore creates a new file-based cache store.
// The directory will be created if it doesn't exist.
func NewFileStore(directory string, enabled bool, ttlSeconds, maxSizeMB int) (*FileStore, error) {
	if !enabled {
		return &FileStore{enabled: false}, nil
	}

	if directory == "" {
		return nil, errors.New("cache directory cannot be empty")
	}

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(directory, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &FileStore{
		directory:  directory,
		enabled:    true,
		ttlSeconds: ttlSeconds,
		maxSizeMB:  maxSizeMB,
	}, nil
}

// Get retrieves a cache entry by key.
// Returns ErrCacheNotFound if the entry doesn't exist.
// Returns ErrCacheExpired if the entry has expired.
func (s *FileStore) Get(key string) (*CacheEntry, error) {
	if !s.enabled {
		return nil, ErrCacheDisabled
	}

	if key == "" {
		return nil, ErrInvalidCacheKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := s.keyToFilePath(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCacheNotFound
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var entry CacheEntry
	if unmarshalErr := json.Unmarshal(data, &entry); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal cache entry: %w", unmarshalErr)
	}

	if entry.IsExpired() {
		// Delete expired entry asynchronously
		go func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			_ = os.Remove(filePath)
		}()
		return nil, ErrCacheExpired
	}

	return &entry, nil
}

// Set stores a cache entry with the given key and data.
// If the entry already exists, it will be overwritten.
func (s *FileStore) Set(key string, data json.RawMessage) error {
	if !s.enabled {
		return ErrCacheDisabled
	}

	if key == "" {
		return ErrInvalidCacheKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry := NewCacheEntry(key, data, s.ttlSeconds)
	entryData, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache entry: %w", err)
	}

	filePath := s.keyToFilePath(key)

	// Write to temporary file first, then rename for atomicity
	tempPath := filePath + ".tmp"
	if writeErr := os.WriteFile(tempPath, entryData, 0600); writeErr != nil {
		return fmt.Errorf("failed to write cache file: %w", writeErr)
	}

	if renameErr := os.Rename(tempPath, filePath); renameErr != nil {
		_ = os.Remove(tempPath) // Clean up temp file on error
		return fmt.Errorf("failed to rename cache file: %w", renameErr)
	}

	return nil
}

// Delete removes a cache entry by key.
// Returns nil if the entry doesn't exist (idempotent).
func (s *FileStore) Delete(key string) error {
	if !s.enabled {
		return ErrCacheDisabled
	}

	if key == "" {
		return ErrInvalidCacheKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.keyToFilePath(key)
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete cache file: %w", err)
	}

	return nil
}

// Clear removes all cache entries from the store.
func (s *FileStore) Clear() error {
	if !s.enabled {
		return ErrCacheDisabled
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only delete cache files
		if filepath.Ext(entry.Name()) == cacheFileExtension {
			filePath := filepath.Join(s.directory, entry.Name())
			if removeErr := os.Remove(filePath); removeErr != nil {
				return fmt.Errorf("failed to remove cache file %s: %w", entry.Name(), removeErr)
			}
		}
	}

	return nil
}

// CleanupExpired removes all expired cache entries.
// This is useful for periodic maintenance.
func (s *FileStore) CleanupExpired() error {
	if !s.enabled {
		return ErrCacheDisabled
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, dirEntry := range entries {
		if dirEntry.IsDir() || filepath.Ext(dirEntry.Name()) != cacheFileExtension {
			continue
		}

		filePath := filepath.Join(s.directory, dirEntry.Name())
		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			continue // Skip files we can't read
		}

		var entry CacheEntry
		if unmarshalErr := json.Unmarshal(data, &entry); unmarshalErr != nil {
			continue // Skip invalid entries
		}

		if entry.IsExpired() {
			_ = os.Remove(filePath)
		}
	}

	return nil
}

// Size returns the total size of the cache in bytes.
func (s *FileStore) Size() (int64, error) {
	if !s.enabled {
		return 0, ErrCacheDisabled
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	var totalSize int64
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return 0, fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if filepath.Ext(entry.Name()) == cacheFileExtension {
			info, infoErr := entry.Info()
			if infoErr != nil {
				continue
			}
			totalSize += info.Size()
		}
	}

	return totalSize, nil
}

// Count returns the number of cache entries (including expired ones).
func (s *FileStore) Count() (int, error) {
	if !s.enabled {
		return 0, ErrCacheDisabled
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return 0, fmt.Errorf("failed to read cache directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == cacheFileExtension {
			count++
		}
	}

	return count, nil
}

// IsEnabled returns true if caching is enabled.
func (s *FileStore) IsEnabled() bool {
	return s.enabled
}

// GetDirectory returns the cache directory path.
func (s *FileStore) GetDirectory() string {
	return s.directory
}

// GetTTL returns the default TTL in seconds.
func (s *FileStore) GetTTL() int {
	return s.ttlSeconds
}

// keyToFilePath converts a cache key to a file path.
// The key is sanitized to ensure filesystem safety.
func (s *FileStore) keyToFilePath(key string) string {
	// Sanitize key for filesystem safety
	safeKey := strings.ReplaceAll(key, "/", "_")
	safeKey = strings.ReplaceAll(safeKey, "\\", "_")
	safeKey = strings.ReplaceAll(safeKey, ":", "_")
	return filepath.Join(s.directory, safeKey+cacheFileExtension)
}
