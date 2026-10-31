package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

// DailyDB is the per-stack daily actual-cost file. It is not the projected
// timeline and it is not the resource-observation store.
type DailyDB struct {
	db        *bolt.DB
	path      string
	closeOnce sync.Once
	closeErr  error
}

type dailyRecord struct {
	Amount    float64   `json:"amount"`
	WrittenAt time.Time `json:"written_at"`
}

// OpenDailyDB creates or opens the daily actuals file for stack of project.
// An existing symlink or other non-regular file is rejected. A projected
// timeline is rejected. A database that already belongs to another project or
// stack is an error.
func OpenDailyDB(path, project, stack string) (*DailyDB, error) {
	if path == "" {
		return nil, errors.New("daily actuals path is required")
	}
	if err := rejectNonRegularDailyFile(path); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), dbDirPermissions); err != nil {
		return nil, fmt.Errorf("creating daily actuals directory: %w", err)
	}
	db, err := bolt.Open(path, dbFilePermissions, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, fmt.Errorf("opening daily actuals: %w", err)
	}
	store := &DailyDB{db: db, path: path}
	if initErr := store.init(project, stack); initErr != nil {
		_ = store.Close()
		return nil, initErr
	}
	return store, nil
}

func rejectNonRegularDailyFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("statting daily actuals file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return errors.New("daily actuals file is not a regular file")
	}
	return nil
}

// Close closes the file. A second call returns the first result.
func (d *DailyDB) Close() error {
	if d == nil || d.db == nil {
		return nil
	}
	d.closeOnce.Do(func() {
		d.closeErr = d.db.Close()
	})
	return d.closeErr
}

// Path returns the database file path.
func (d *DailyDB) Path() string {
	if d == nil {
		return ""
	}
	return d.path
}

func (d *DailyDB) init(project, stack string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		if err := rejectTimelineFile(tx); err != nil {
			return err
		}
		meta, err := tx.CreateBucketIfNotExists([]byte(costBucketMeta))
		if err != nil {
			return fmt.Errorf("creating daily actuals meta: %w", err)
		}
		if _, pointErr := tx.CreateBucketIfNotExists([]byte(dailyBucketPoints)); pointErr != nil {
			return fmt.Errorf("creating daily points bucket: %w", pointErr)
		}
		version := getUint64(meta, metaKeyDailySchema)
		switch {
		case version == 0:
			if err = putUint64(meta, metaKeyDailySchema, dailySchemaVersion); err != nil {
				return err
			}
		case version != dailySchemaVersion:
			return fmt.Errorf("unsupported daily actuals schema version %d", version)
		}
		return writeDailyOwner(meta, project, stack)
	})
}

func rejectTimelineFile(tx *bolt.Tx) error {
	meta := tx.Bucket([]byte(costBucketMeta))
	if meta != nil && len(meta.Get([]byte(metaKeyDailySchema))) > 0 {
		return nil
	}
	if meta != nil && len(meta.Get([]byte(metaKeyVersion))) > 0 {
		return ErrTimelineAsDaily
	}
	if tx.Bucket([]byte(costBucketSnapshots)) != nil {
		return ErrTimelineAsDaily
	}
	return nil
}

func writeDailyOwner(meta *bolt.Bucket, project, stack string) error {
	existing := string(meta.Get([]byte(metaKeyStack)))
	if existing == "" && stack != "" {
		if err := meta.Put([]byte(metaKeyStack), []byte(stack)); err != nil {
			return fmt.Errorf("writing stack name: %w", err)
		}
	} else if stack != "" && existing != "" && existing != stack {
		return fmt.Errorf("daily actuals database stack %q does not match %q", existing, stack)
	}
	return initProject(meta, project)
}

// Replace upserts points. Days and currencies absent from points stay stored.
func (d *DailyDB) Replace(points []DailyPoint) error {
	if d == nil || d.db == nil {
		return errors.New("daily actuals are not open")
	}
	return d.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(dailyBucketPoints))
		if bucket == nil {
			return errors.New("daily points bucket is missing")
		}
		for _, point := range points {
			if err := putDailyPoint(bucket, point); err != nil {
				return err
			}
		}
		return nil
	})
}

func putDailyPoint(bucket *bolt.Bucket, point DailyPoint) error {
	key, err := dailyPointKey(point)
	if err != nil {
		return err
	}
	body, err := json.Marshal(dailyRecord{
		Amount:    point.Amount,
		WrittenAt: point.WrittenAt.UTC(),
	})
	if err != nil {
		return fmt.Errorf("encoding daily point: %w", err)
	}
	if err = bucket.Put(key, body); err != nil {
		return fmt.Errorf("writing daily point: %w", err)
	}
	return nil
}

// Points returns every stored point, ordered by resource, day, then currency.
func (d *DailyDB) Points() ([]DailyPoint, error) {
	if d == nil || d.db == nil {
		return nil, errors.New("daily actuals are not open")
	}
	var points []DailyPoint
	err := d.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(dailyBucketPoints))
		if bucket == nil {
			return errors.New("daily points bucket is missing")
		}
		return bucket.ForEach(func(key, value []byte) error {
			point, decodeErr := decodeDailyPoint(key, value)
			if decodeErr != nil {
				return decodeErr
			}
			points = append(points, point)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(points, compareDailyPoints)
	return points, nil
}

func decodeDailyPoint(key, value []byte) (DailyPoint, error) {
	parts := strings.SplitN(string(key), "\x00", dailyKeyParts)
	if len(parts) != dailyKeyParts {
		return DailyPoint{}, errors.New("daily point key is malformed")
	}
	day, err := time.Parse(time.DateOnly, parts[1])
	if err != nil {
		return DailyPoint{}, fmt.Errorf("parsing daily point day: %w", err)
	}
	var record dailyRecord
	if err = json.Unmarshal(value, &record); err != nil {
		return DailyPoint{}, fmt.Errorf("decoding daily point: %w", err)
	}
	return DailyPoint{
		ResourceID: parts[0],
		Day:        day.UTC(),
		Currency:   parts[2],
		Amount:     record.Amount,
		WrittenAt:  record.WrittenAt.UTC(),
	}, nil
}

func compareDailyPoints(a, b DailyPoint) int {
	if a.ResourceID != b.ResourceID {
		if a.ResourceID < b.ResourceID {
			return -1
		}
		return 1
	}
	if !a.Day.Equal(b.Day) {
		if a.Day.Before(b.Day) {
			return -1
		}
		return 1
	}
	if a.Currency != b.Currency {
		if a.Currency < b.Currency {
			return -1
		}
		return 1
	}
	return 0
}

// PruneBefore deletes points whose UTC day is strictly before day.
// Resource ids in keep are left stored, including days older than day.
func (d *DailyDB) PruneBefore(day time.Time, keep []string) (int, error) {
	if d == nil || d.db == nil {
		return 0, errors.New("daily actuals are not open")
	}
	cutoff := day.UTC().Format(time.DateOnly)
	kept := make(map[string]struct{}, len(keep))
	for _, id := range keep {
		kept[id] = struct{}{}
	}
	deleted := 0
	err := d.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(dailyBucketPoints))
		if bucket == nil {
			return errors.New("daily points bucket is missing")
		}
		var drop [][]byte
		scanErr := bucket.ForEach(func(key, _ []byte) error {
			parts := strings.SplitN(string(key), "\x00", dailyKeyParts)
			if len(parts) != dailyKeyParts || parts[1] >= cutoff {
				return nil
			}
			if _, skip := kept[parts[0]]; skip {
				return nil
			}
			drop = append(drop, append([]byte(nil), key...))
			return nil
		})
		if scanErr != nil {
			return scanErr
		}
		for _, key := range drop {
			if err := bucket.Delete(key); err != nil {
				return fmt.Errorf("deleting daily point: %w", err)
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
