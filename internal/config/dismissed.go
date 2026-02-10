package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// ErrStoreCorrupted indicates the dismissal state file exists but contains invalid data.
// Callers should abort unless the user explicitly forces a reset.
var ErrStoreCorrupted = errors.New("dismissal state file corrupted")

// DismissalStoreVersion is the current schema version for the dismissal state file.
const DismissalStoreVersion = 1

// DismissalStatus represents the lifecycle state of a dismissal record.
type DismissalStatus string

const (
	// StatusDismissed indicates a permanent dismissal (no expiry).
	StatusDismissed DismissalStatus = "dismissed"
	// StatusSnoozed indicates a temporary dismissal with an expiry date.
	StatusSnoozed DismissalStatus = "snoozed"
	// StatusActive indicates a previously dismissed recommendation that was re-enabled.
	// The record is preserved for audit trail / history purposes.
	StatusActive DismissalStatus = "active"
)

// LifecycleAction represents the type of lifecycle event.
type LifecycleAction string

const (
	// ActionDismissed indicates the recommendation was permanently dismissed.
	ActionDismissed LifecycleAction = "dismissed"
	// ActionSnoozed indicates the recommendation was snoozed with an expiry.
	ActionSnoozed LifecycleAction = "snoozed"
	// ActionUndismissed indicates the recommendation was re-enabled.
	ActionUndismissed LifecycleAction = "undismissed"
)

// DismissalRecord represents a single recommendation's dismissal state.
type DismissalRecord struct {
	RecommendationID string                   `json:"recommendation_id"`
	Status           DismissalStatus          `json:"status"`
	Reason           string                   `json:"reason"`
	CustomReason     string                   `json:"custom_reason,omitempty"`
	DismissedAt      time.Time                `json:"dismissed_at"`
	DismissedBy      string                   `json:"dismissed_by,omitempty"`
	ExpiresAt        *time.Time               `json:"expires_at"`
	LastKnown        *LastKnownRecommendation `json:"last_known,omitempty"`
	History          []LifecycleEvent         `json:"history"`
}

// LastKnownRecommendation captures recommendation details at the time of dismissal.
type LastKnownRecommendation struct {
	Description      string  `json:"description"`
	EstimatedSavings float64 `json:"estimated_savings"`
	Currency         string  `json:"currency"`
	Type             string  `json:"type"`
	ResourceID       string  `json:"resource_id"`
}

// LifecycleEvent is a timestamped action in a recommendation's history.
type LifecycleEvent struct {
	Action       LifecycleAction `json:"action"`
	Reason       string          `json:"reason"`
	CustomReason string          `json:"custom_reason,omitempty"`
	Timestamp    time.Time       `json:"timestamp"`
	ExpiresAt    *time.Time      `json:"expires_at,omitempty"`
}

// dismissalStoreData is the serialized form of the dismissal store.
type dismissalStoreData struct {
	Version    int                         `json:"version"`
	Dismissals map[string]*DismissalRecord `json:"dismissals"`
}

// DismissalStore manages dismissal state persisted as a JSON file.
type DismissalStore struct {
	mu         sync.RWMutex
	filePath   string
	version    int
	dismissals map[string]*DismissalRecord
}

// NewDismissalStore creates a new DismissalStore backed by the given file path.
// If filePath is empty, it defaults to ~/.finfocus/dismissed.json.
func NewDismissalStore(filePath string) (*DismissalStore, error) {
	if filePath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("determining home directory: %w", err)
		}
		filePath = filepath.Join(homeDir, ".finfocus", "dismissed.json")
	}

	store := &DismissalStore{
		filePath:   filePath,
		version:    DismissalStoreVersion,
		dismissals: make(map[string]*DismissalRecord),
	}

	return store, nil
}

// lockFilePath returns the path to the lockfile for cross-process coordination.
func (s *DismissalStore) lockFilePath() string {
	return s.filePath + ".lock"
}

// acquireFileLock acquires a cross-process advisory lockfile.
// Returns a cleanup function that releases the lock.
func (s *DismissalStore) acquireFileLock() (func(), error) {
	lockPath := s.lockFilePath()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o750); err != nil {
		return nil, fmt.Errorf("creating lock directory: %w", err)
	}

	// Try to create lockfile exclusively; retry with stale lock detection
	const maxRetries = 10
	const retryDelay = 100 * time.Millisecond
	const staleLockAge = 30 * time.Second

	for range maxRetries {
		f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			// Write PID for stale lock detection
			_, _ = fmt.Fprintf(f, "%d", os.Getpid())
			_ = f.Close()
			return func() { _ = os.Remove(lockPath) }, nil
		}

		if removeStaleLock(lockPath, staleLockAge) {
			continue
		}
		time.Sleep(retryDelay)
	}

	return nil, fmt.Errorf("could not acquire lock on %s after retries", lockPath)
}

// removeStaleLock checks if a lock file is stale and removes it if so.
// Returns true if the lock was removed (caller should retry), false otherwise.
func removeStaleLock(lockPath string, staleLockAge time.Duration) bool {
	info, statErr := os.Stat(lockPath)
	if statErr != nil || time.Since(info.ModTime()) <= staleLockAge {
		return false
	}

	// Lock is old enough to be stale — check if owning process is alive
	if isLockHeldByLiveProcess(lockPath) {
		return false
	}

	// PID not readable, not parseable, or process dead — remove stale lock
	_ = os.Remove(lockPath)
	return true
}

// isLockHeldByLiveProcess reads the PID from a lock file and checks if that
// process is still alive. Returns true if the process exists.
func isLockHeldByLiveProcess(lockPath string) bool {
	pidData, readErr := os.ReadFile(lockPath)
	if readErr != nil || len(pidData) == 0 {
		return false
	}
	var pid int
	if _, scanErr := fmt.Sscanf(string(pidData), "%d", &pid); scanErr != nil || pid <= 0 {
		return false
	}
	return processExists(pid) == nil
}

// processExists checks whether a process with the given PID is still alive.
// Returns nil if the process exists, an error otherwise.
func processExists(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	// Signal 0 tests process existence without actually sending a signal
	return proc.Signal(syscall.Signal(0))
}

// Load reads the dismissal state from the JSON file.
// If the file does not exist, the store starts empty.
// If the file is corrupted, ErrStoreCorrupted is returned.
func (s *DismissalStore) Load() error {
	unlock, lockErr := s.acquireFileLock()
	if lockErr != nil {
		return fmt.Errorf("acquiring file lock: %w", lockErr)
	}
	defer unlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet; start with empty store
			s.dismissals = make(map[string]*DismissalRecord)
			return nil
		}
		return fmt.Errorf("reading dismissal state file: %w", err)
	}

	var storeData dismissalStoreData
	if unmarshalErr := json.Unmarshal(data, &storeData); unmarshalErr != nil {
		// Corrupted file: do NOT start fresh — callers must handle explicitly
		s.dismissals = make(map[string]*DismissalRecord)
		return fmt.Errorf("%w: %w", ErrStoreCorrupted, unmarshalErr)
	}

	// Version check
	if storeData.Version != DismissalStoreVersion {
		s.dismissals = make(map[string]*DismissalRecord)
		return fmt.Errorf("%w: unsupported version %d (expected %d)",
			ErrStoreCorrupted, storeData.Version, DismissalStoreVersion)
	}

	if storeData.Dismissals == nil {
		storeData.Dismissals = make(map[string]*DismissalRecord)
	}

	s.dismissals = storeData.Dismissals
	s.version = storeData.Version

	return nil
}

// Save writes the dismissal state to the JSON file atomically.
func (s *DismissalStore) Save() error {
	unlock, lockErr := s.acquireFileLock()
	if lockErr != nil {
		return fmt.Errorf("acquiring file lock: %w", lockErr)
	}
	defer unlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	storeData := dismissalStoreData{
		Version:    s.version,
		Dismissals: s.dismissals,
	}

	data, err := json.MarshalIndent(storeData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling dismissal state: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if mkdirErr := os.MkdirAll(dir, 0o750); mkdirErr != nil {
		return fmt.Errorf("creating dismissal state directory: %w", mkdirErr)
	}

	// Write atomically via temp file
	tmpPath := s.filePath + ".tmp"
	if writeErr := os.WriteFile(tmpPath, data, 0o600); writeErr != nil {
		return fmt.Errorf("writing dismissal state temp file: %w", writeErr)
	}

	if renameErr := os.Rename(tmpPath, s.filePath); renameErr != nil {
		// Clean up temp file on rename failure
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming dismissal state temp file: %w", renameErr)
	}

	return nil
}

// Get retrieves a dismissal record by recommendation ID.
// Returns a copy of the record to prevent callers from mutating internal state.
// Returns nil and false if the ID is not found.
func (s *DismissalStore) Get(recommendationID string) (*DismissalRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.dismissals[recommendationID]
	if !ok {
		return nil, false
	}

	return copyDismissalRecord(record), true
}

// Set adds or updates a dismissal record.
func (s *DismissalStore) Set(record *DismissalRecord) error {
	if record == nil {
		return errors.New("dismissal record cannot be nil")
	}
	if record.RecommendationID == "" {
		return errors.New("recommendation ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.dismissals[record.RecommendationID] = copyDismissalRecord(record)
	return nil
}

// Delete removes a dismissal record by recommendation ID.
func (s *DismissalStore) Delete(recommendationID string) error {
	if recommendationID == "" {
		return errors.New("recommendation ID cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.dismissals, recommendationID)
	return nil
}

// GetDismissedIDs returns all recommendation IDs that are currently dismissed or snoozed
// (excluding expired snoozes). This is used to populate ExcludedRecommendationIds.
func (s *DismissalStore) GetDismissedIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var ids []string

	for id, record := range s.dismissals {
		// Skip expired snoozes
		if record.Status == StatusSnoozed && record.ExpiresAt != nil && record.ExpiresAt.Before(now) {
			continue
		}
		// Skip active (undismissed) records — they are kept only for history
		if record.Status == StatusActive {
			continue
		}
		ids = append(ids, id)
	}

	return ids
}

// GetAllRecords returns all dismissal records (including expired snoozes).
// Returns a deep copy to prevent concurrent modification of internal state.
func (s *DismissalStore) GetAllRecords() map[string]*DismissalRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*DismissalRecord, len(s.dismissals))
	for k, v := range s.dismissals {
		result[k] = copyDismissalRecord(v)
	}

	return result
}

// GetExpiredSnoozes returns records that have snoozed status with an expired ExpiresAt.
func (s *DismissalStore) GetExpiredSnoozes() []*DismissalRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var expired []*DismissalRecord

	for _, record := range s.dismissals {
		if record.Status == StatusSnoozed && record.ExpiresAt != nil && record.ExpiresAt.Before(now) {
			expired = append(expired, copyDismissalRecord(record))
		}
	}

	return expired
}

// CleanExpiredSnoozes transitions snoozed records whose ExpiresAt has passed to active status.
// Returns the number of snoozes that were cleaned.
func (s *DismissalStore) CleanExpiredSnoozes() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for _, record := range s.dismissals {
		if record.Status == StatusSnoozed && record.ExpiresAt != nil && record.ExpiresAt.Before(now) {
			// Mark as active with undismissed lifecycle event (preserves history)
			record.History = append(record.History, LifecycleEvent{
				Action:    ActionUndismissed,
				Reason:    record.Reason,
				Timestamp: now,
			})
			record.Status = StatusActive
			record.ExpiresAt = nil
			cleaned++
		}
	}

	return cleaned, nil
}

// FilePath returns the file path of the dismissal store.
func (s *DismissalStore) FilePath() string {
	return s.filePath
}

// Count returns the number of dismissal records.
func (s *DismissalStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.dismissals)
}

// copyDismissalRecord returns a deep copy of a DismissalRecord.
func copyDismissalRecord(r *DismissalRecord) *DismissalRecord {
	c := *r
	if r.ExpiresAt != nil {
		t := *r.ExpiresAt
		c.ExpiresAt = &t
	}
	if r.LastKnown != nil {
		lk := *r.LastKnown
		c.LastKnown = &lk
	}
	if r.History != nil {
		c.History = make([]LifecycleEvent, len(r.History))
		for i, evt := range r.History {
			c.History[i] = evt
			if evt.ExpiresAt != nil {
				t := *evt.ExpiresAt
				c.History[i].ExpiresAt = &t
			}
		}
	}
	return &c
}
