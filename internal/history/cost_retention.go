package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

// CostPrunePolicy selects snapshots to drop. Keep 0 means no count limit.
// A zero Cutoff means no age limit.
type CostPrunePolicy struct {
	Keep   int
	Cutoff time.Time
}

// Active reports whether the policy would drop anything.
func (p CostPrunePolicy) Active() bool {
	return p.Keep > 0 || !p.Cutoff.IsZero()
}

// CostPruneResult is the outcome of ApplyPrune.
type CostPruneResult struct {
	Total      int
	Pruned     int
	Kept       int
	Cutoff     time.Time
	SizeBefore int64
}

type costRecord struct {
	key      []byte
	snapshot CostSnapshot
}

// ParseOlderThan reads durations such as 365d, 6m, and 2y.
// An empty spec returns the zero time.
func ParseOlderThan(spec string, now time.Time) (time.Time, error) {
	spec = strings.TrimSpace(strings.ToLower(spec))
	if spec == "" {
		return time.Time{}, nil
	}
	const minSpec = 2
	if len(spec) < minSpec {
		return time.Time{}, invalidDuration(spec)
	}
	number, err := strconv.Atoi(spec[:len(spec)-1])
	if err != nil || number < 0 {
		return time.Time{}, invalidDuration(spec)
	}
	when := now.UTC()
	switch spec[len(spec)-1] {
	case 'd':
		return when.AddDate(0, 0, -number), nil
	case 'm':
		return when.AddDate(0, -number, 0), nil
	case 'y':
		return when.AddDate(-number, 0, 0), nil
	default:
		return time.Time{}, invalidDuration(spec)
	}
}

func invalidDuration(spec string) error {
	return fmt.Errorf("invalid duration %q (use 365d, 6m, or 2y)", spec)
}

// ApplyPrune drops snapshots that are older than the cutoff or beyond Keep.
// dryRun reports the plan and leaves the database unchanged.
func (d *CostDB) ApplyPrune(policy CostPrunePolicy, dryRun bool) (CostPruneResult, error) {
	if d == nil || d.db == nil {
		return CostPruneResult{}, errors.New("cost history is not open")
	}
	info, err := os.Stat(d.path)
	if err != nil {
		return CostPruneResult{}, fmt.Errorf("statting cost history: %w", err)
	}
	records, err := d.costRecords()
	if err != nil {
		return CostPruneResult{}, err
	}
	drop := pruneRecords(records, policy)
	result := CostPruneResult{
		Total:      len(records),
		Pruned:     len(drop),
		Kept:       len(records) - len(drop),
		Cutoff:     policy.Cutoff,
		SizeBefore: info.Size(),
	}
	if dryRun || len(drop) == 0 {
		return result, nil
	}
	if deleteErr := d.deleteRecords(drop, records); deleteErr != nil {
		return CostPruneResult{}, deleteErr
	}
	return result, nil
}

func (d *CostDB) costRecords() ([]costRecord, error) {
	var records []costRecord
	err := d.db.View(func(tx *bolt.Tx) error {
		snaps := tx.Bucket([]byte(costBucketSnapshots))
		if snaps == nil {
			return errors.New("cost history buckets are missing")
		}
		return snaps.ForEach(func(key, value []byte) error {
			var snapshot CostSnapshot
			if err := json.Unmarshal(value, &snapshot); err != nil {
				return fmt.Errorf("decoding snapshot: %w", err)
			}
			records = append(records, costRecord{
				key:      append([]byte(nil), key...),
				snapshot: snapshot,
			})
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(records, func(i, j int) bool {
		return newerSnapshot(records[i].snapshot, records[j].snapshot)
	})
	return records, nil
}

func newerSnapshot(left, right CostSnapshot) bool {
	if !left.Timestamp.Equal(right.Timestamp) {
		return left.Timestamp.After(right.Timestamp)
	}
	return left.Version > right.Version
}

func pruneRecords(records []costRecord, policy CostPrunePolicy) []costRecord {
	drop := make([]costRecord, 0)
	kept := 0
	for _, record := range records {
		tooOld := !policy.Cutoff.IsZero() && record.snapshot.Timestamp.Before(policy.Cutoff)
		beyond := policy.Keep > 0 && kept >= policy.Keep
		if tooOld || beyond {
			drop = append(drop, record)
			continue
		}
		kept++
	}
	return drop
}

func (d *CostDB) deleteRecords(drop, records []costRecord) error {
	dropped := make(map[string]struct{}, len(drop))
	for _, record := range drop {
		dropped[string(record.key)] = struct{}{}
	}
	newest, hasKept := newestKept(records, dropped)
	return d.db.Update(func(tx *bolt.Tx) error {
		snaps := tx.Bucket([]byte(costBucketSnapshots))
		anns := tx.Bucket([]byte(costBucketAnnotations))
		meta := tx.Bucket([]byte(costBucketMeta))
		if snaps == nil || anns == nil || meta == nil {
			return errors.New("cost history buckets are missing")
		}
		if err := deleteSnapshotKeys(snaps, anns, drop); err != nil {
			return err
		}
		last := uint64(0)
		if hasKept && newest.Version > 0 {
			last = uint64(newest.Version)
		}
		return putUint64(meta, metaKeyLastVersion, last)
	})
}

func newestKept(records []costRecord, dropped map[string]struct{}) (CostSnapshot, bool) {
	var newest CostSnapshot
	hasKept := false
	for _, record := range records {
		if _, ok := dropped[string(record.key)]; ok {
			continue
		}
		if !hasKept || newerSnapshot(record.snapshot, newest) {
			newest = record.snapshot
			hasKept = true
		}
	}
	return newest, hasKept
}

func deleteSnapshotKeys(snaps, anns *bolt.Bucket, drop []costRecord) error {
	for _, record := range drop {
		if err := snaps.Delete(record.key); err != nil {
			return fmt.Errorf("deleting snapshot: %w", err)
		}
		if err := anns.Delete(record.key); err != nil {
			return fmt.Errorf("deleting annotation: %w", err)
		}
	}
	return nil
}

// CompactCostFile rewrites path so deleted bbolt pages are not left on disk.
// The database must be closed. It returns the size before and after the swap.
func CompactCostFile(path string) (int64, int64, error) {
	before, err := os.Stat(path)
	if err != nil {
		return 0, 0, fmt.Errorf("statting cost history: %w", err)
	}
	tmp := path + ".compact"
	src, err := bolt.Open(path, dbFilePermissions, &bolt.Options{ReadOnly: true, Timeout: time.Second})
	if err != nil {
		return 0, 0, fmt.Errorf("opening cost history for compact: %w", err)
	}
	dst, err := bolt.Open(tmp, dbFilePermissions, &bolt.Options{Timeout: time.Second})
	if err != nil {
		_ = src.Close()
		return 0, 0, fmt.Errorf("creating compacted cost history: %w", err)
	}
	compactErr := bolt.Compact(dst, src, 0)
	srcErr := src.Close()
	dstErr := dst.Close()
	if compactErr != nil || srcErr != nil || dstErr != nil {
		_ = os.Remove(tmp)
		if compactErr != nil {
			return 0, 0, fmt.Errorf("compacting cost history: %w", compactErr)
		}
		if srcErr != nil {
			return 0, 0, srcErr
		}
		return 0, 0, dstErr
	}
	if renameErr := os.Rename(tmp, path); renameErr != nil {
		_ = os.Remove(tmp)
		return 0, 0, fmt.Errorf("replacing cost history: %w", renameErr)
	}
	after, err := os.Stat(path)
	if err != nil {
		return before.Size(), 0, fmt.Errorf("statting compacted cost history: %w", err)
	}
	return before.Size(), after.Size(), nil
}
