package config_test

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/rshade/finfocus/internal/config"
)

// T036: Benchmark DismissalStore with 1,000+ records.
// SC-008: Load/Save, GetDismissedIDs, CleanExpiredSnoozes must complete in <100ms.

// benchmarkStoreSize is the number of records used in benchmarks.
const benchmarkStoreSize = 1000

// createBenchmarkStore creates a DismissalStore with benchmarkStoreSize dismissal records for benchmarking.
func createBenchmarkStore(b *testing.B, dir string) *config.DismissalStore {
	b.Helper()

	storePath := filepath.Join(dir, "dismissed.json")
	store, err := config.NewDismissalStore(storePath)
	if err != nil {
		b.Fatalf("creating store: %v", err)
	}

	now := time.Now()
	future := now.Add(30 * 24 * time.Hour)
	past := now.Add(-1 * time.Hour)

	for i := range benchmarkStoreSize {
		var status config.DismissalStatus
		var expiresAt *time.Time

		switch i % 3 {
		case 0:
			status = config.StatusDismissed
		case 1:
			status = config.StatusSnoozed
			expiresAt = &future
		case 2:
			status = config.StatusSnoozed
			expiresAt = &past // expired snooze
		}

		recID := fmt.Sprintf("rec-%06d", i)
		record := &config.DismissalRecord{
			RecommendationID: recID,
			Status:           status,
			Reason:           "BUSINESS_CONSTRAINT",
			DismissedAt:      now,
			ExpiresAt:        expiresAt,
			LastKnown: &config.LastKnownRecommendation{
				Description:      fmt.Sprintf("Recommendation %d", i),
				EstimatedSavings: float64(i) * 10.0,
				Currency:         "USD",
				Type:             "RIGHTSIZE",
				ResourceID:       fmt.Sprintf("aws:ec2:instance-%06d", i),
			},
			History: []config.LifecycleEvent{
				{
					Action:    config.ActionDismissed,
					Reason:    "BUSINESS_CONSTRAINT",
					Timestamp: now,
					ExpiresAt: expiresAt,
				},
			},
		}

		if err := store.Set(record); err != nil {
			b.Fatalf("setting record %d: %v", i, err)
		}
	}

	if err := store.Save(); err != nil {
		b.Fatalf("saving store: %v", err)
	}

	return store
}

func BenchmarkDismissalStore_Load_1000(b *testing.B) {
	dir := b.TempDir()
	store := createBenchmarkStore(b, dir)
	storePath := store.FilePath()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		s, err := config.NewDismissalStore(storePath)
		if err != nil {
			b.Fatal(err)
		}
		if err := s.Load(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDismissalStore_Save_1000(b *testing.B) {
	dir := b.TempDir()
	store := createBenchmarkStore(b, dir)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := store.Save(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDismissalStore_GetDismissedIDs_1000(b *testing.B) {
	dir := b.TempDir()
	store := createBenchmarkStore(b, dir)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		ids := store.GetDismissedIDs()
		_ = ids
	}
}

func BenchmarkDismissalStore_CleanExpiredSnoozes_1000(b *testing.B) {
	dir := b.TempDir()

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		b.StopTimer()
		store := createBenchmarkStore(b, dir)
		b.StartTimer()

		if _, err := store.CleanExpiredSnoozes(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDismissalStore_GetAllRecords_1000(b *testing.B) {
	dir := b.TempDir()
	store := createBenchmarkStore(b, dir)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		records := store.GetAllRecords()
		_ = records
	}
}
