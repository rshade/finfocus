package history

import (
	"encoding/json"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

const secondsPerDay = 24 * 3600

// cleanupExpiredEntries removes entries with LastSeen older than the retention
// window from both resource_history and resource_tags buckets.
// Uses a single db.Update() transaction to prevent TOCTOU races.
// Returns count of removed resource_history entries.
func (s *BoltStore) cleanupExpiredEntries(retentionDays int) (int, error) {
	cutoff := time.Now().Unix() - int64(retentionDays)*secondsPerDay
	deleted := 0

	err := s.db.Update(func(tx *bolt.Tx) error {
		historyBucket := tx.Bucket([]byte(BucketResourceHistory))
		if historyBucket == nil {
			return nil
		}

		expiredURNHashes := make(map[string]bool)
		deleted = s.deleteExpiredFromBucket(historyBucket, cutoff, expiredURNHashes)

		if len(expiredURNHashes) > 0 {
			s.cleanupExpiredTags(tx, expiredURNHashes)
		}

		return nil
	})

	if deleted > 0 {
		s.logger.Debug().Int("deleted", deleted).Int("retention_days", retentionDays).
			Msg("history cleanup completed")
	}

	return deleted, err
}

// deleteExpiredFromBucket iterates the history bucket and removes entries
// whose LastSeen is before the cutoff timestamp. Returns the count of
// deleted entries and populates expiredURNHashes with their URN hashes.
func (s *BoltStore) deleteExpiredFromBucket(
	bucket *bolt.Bucket,
	cutoff int64,
	expiredURNHashes map[string]bool,
) int {
	deleted := 0
	c := bucket.Cursor()

	for k, v := c.First(); k != nil; k, v = c.Next() {
		var entry ResourceHistoryEntry
		if err := json.Unmarshal(v, &entry); err != nil {
			if delErr := bucket.Delete(k); delErr != nil {
				s.logger.Debug().Err(delErr).Str("key", string(k)).
					Msg("failed to delete corrupt history entry")
			}
			deleted++

			continue
		}

		if entry.LastSeen < cutoff {
			expiredURNHashes[URNHash(entry.URN)] = true
			if delErr := bucket.Delete(k); delErr != nil {
				s.logger.Debug().Err(delErr).Str("key", string(k)).Str("urn", entry.URN).
					Msg("failed to delete expired history entry")
			}
			deleted++
		}
	}

	return deleted
}

// cleanupExpiredTags removes tag entries whose URN hash is in the expired set.
// Tag keys have format: {tag_key}:{tag_value}/{urn_hash}.
func (s *BoltStore) cleanupExpiredTags(tx *bolt.Tx, expiredURNHashes map[string]bool) {
	tagBucket := tx.Bucket([]byte(BucketResourceTags))
	if tagBucket == nil {
		return
	}

	c := tagBucket.Cursor()
	for k, _ := c.First(); k != nil; k, _ = c.Next() {
		keyStr := string(k)
		lastSlash := strings.LastIndex(keyStr, "/")
		if lastSlash < 0 {
			continue
		}
		urnHash := keyStr[lastSlash+1:]
		if expiredURNHashes[urnHash] {
			_ = tagBucket.Delete(k)
		}
	}
}
