package history

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollect_StoresPluginPrice(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	exp := &fakeExporter{
		history: historyRows(
			row(1, "update", "succeeded", "2025-01-15T00:00:00Z", "boot"),
			row(2, "update", "succeeded", "2025-02-01T00:00:00Z", "scale"),
		),
		exports: map[int][]byte{
			1: exportJSON("urn:web", "t3.micro"),
			2: exportJSON("urn:web", "m5.large"),
		},
	}
	var calls int
	result, err := Collect(context.Background(), db, exp, CollectOptions{
		Parallel: 1,
		Pricer: func(_ context.Context, resources []PriceResource) ([]PriceQuote, error) {
			calls++
			monthly := 10.0
			if resources[0].Properties["instanceType"] == "m5.large" {
				monthly = 40
			}
			return priced(resources, monthly, "aws-public"), nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Stored)
	assert.Equal(t, 2, calls)
	snaps, err := db.Snapshots(date(2025, 1, 1), date(2025, 3, 1))
	require.NoError(t, err)
	require.Len(t, snaps, 2)
	assert.InDelta(t, 10, snaps[0].TotalMonthly, 0.001)
	assert.InDelta(t, 40, snaps[1].TotalMonthly, 0.001)
	assert.Equal(t, "t3.micro", snaps[0].Resources[0].SKU)
	stats, err := db.Stats()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), stats.LastVersion)
}

func TestCollect_IncrementalSkipsAndDestroy(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	require.NoError(
		t,
		db.Put(ZeroSnapshot(date(2025, 1, 1), 2), AnnotationFrom(StackUpdate{Version: 2, Kind: "update"})),
	)
	var pricedCalls int
	exp := &fakeExporter{
		history: historyRows(
			row(2, "update", "succeeded", "2025-01-01T00:00:00Z", "old"),
			row(3, "destroy", "succeeded", "2025-02-01T00:00:00Z", "gone"),
			row(4, "update", "failed", "2025-03-01T00:00:00Z", "nope"),
		),
		exports: map[int][]byte{3: exportJSON("urn:web", "t3.micro")},
	}
	result, err := Collect(context.Background(), db, exp, CollectOptions{
		Parallel: 1,
		Pricer: func(context.Context, []PriceResource) ([]PriceQuote, error) {
			pricedCalls++
			return nil, errors.New("should not price a destroy")
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Stored)
	assert.Equal(t, 0, pricedCalls)
	snaps, err := db.Snapshots(date(2025, 2, 1), date(2025, 2, 2))
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	assert.InDelta(t, 0, snaps[0].TotalMonthly, 0.001)
	anns, err := db.Annotations(date(2025, 2, 1), date(2025, 2, 2))
	require.NoError(t, err)
	assert.Equal(t, "Stack destroyed", anns[0].Message)
}

func TestCollect_FailFastMissingPlugin(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	exp := &fakeExporter{
		history: historyRows(
			row(1, "update", "succeeded", "2025-01-01T00:00:00Z", "ok"),
			row(2, "update", "succeeded", "2025-02-01T00:00:00Z", "bad"),
		),
		exports: map[int][]byte{
			1: exportJSON("urn:ok", "t3.micro"),
			2: exportJSON("urn:bad", "n1-standard-1"),
		},
	}
	_, err := Collect(context.Background(), db, exp, CollectOptions{
		Parallel: 1,
		Pricer: func(_ context.Context, resources []PriceResource) ([]PriceQuote, error) {
			adapter := "aws-public"
			if resources[0].ID == "urn:bad" {
				adapter = "none"
			}
			return priced(resources, 5, adapter), nil
		},
	})
	require.ErrorIs(t, err, ErrNoPlugin)
	require.ErrorContains(t, err, "Install the required plugin first.")
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	assert.Equal(t, 1, snaps[0].Version)

	fixed, err := Collect(context.Background(), db, exp, CollectOptions{
		Parallel: 1,
		Pricer: func(_ context.Context, resources []PriceResource) ([]PriceQuote, error) {
			return priced(resources, 5, "aws-public"), nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, fixed.Stored)
	snaps, err = db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, snaps, 2)
	assert.Equal(t, []int{1, 2}, []int{snaps[0].Version, snaps[1].Version})
}

func TestCollect_SkipsPulumiProviderResources(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	exp := &fakeExporter{
		history: historyRows(row(1, "update", "succeeded", "2025-01-15T00:00:00Z", "boot")),
		exports: map[int][]byte{1: exportWithProviderJSON("urn:web", "t3.micro")},
	}
	var sent []string
	result, err := Collect(context.Background(), db, exp, CollectOptions{
		Parallel: 1,
		Pricer: func(_ context.Context, resources []PriceResource) ([]PriceQuote, error) {
			var quotes []PriceQuote
			for _, resource := range resources {
				sent = append(sent, resource.Type)
				if strings.HasPrefix(resource.Type, "pulumi:") {
					continue
				}
				quotes = append(quotes, priced([]PriceResource{resource}, 10, "aws-public")...)
			}
			return quotes, nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Stored)
	assert.Equal(t, []string{"aws:ec2/instance:Instance"}, sent)
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	assert.Equal(t, 1, snaps[0].ResourceCount)
	assert.InDelta(t, 10, snaps[0].TotalMonthly, 0.001)
}

func TestCollect_ProviderOnlyStackStoresZero(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	body, err := json.Marshal(map[string]any{
		"version": 3,
		"deployment": map[string]any{
			"resources": []any{providerResource()},
		},
	})
	require.NoError(t, err)
	exp := &fakeExporter{
		history: historyRows(row(1, "update", "succeeded", "2025-01-15T00:00:00Z", "boot")),
		exports: map[int][]byte{1: body},
	}
	result, err := Collect(context.Background(), db, exp, CollectOptions{
		Parallel: 1,
		Pricer: func(_ context.Context, resources []PriceResource) ([]PriceQuote, error) {
			assert.Empty(t, resources)
			return nil, nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Stored)
}

func TestCollect_RetryThenSkip(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	exp := &fakeExporter{
		history: historyRows(
			row(1, "update", "succeeded", "2025-01-01T00:00:00Z", "flaky"),
			row(2, "update", "succeeded", "2025-02-01T00:00:00Z", "ok"),
		),
		exports: map[int][]byte{
			1: exportJSON("urn:web", "t3.micro"),
			2: exportJSON("urn:web", "t3.micro"),
		},
		fail: map[int]int{1: 3},
	}
	result, err := Collect(context.Background(), db, exp, CollectOptions{
		Parallel: 1,
		Pricer: func(_ context.Context, resources []PriceResource) ([]PriceQuote, error) {
			return priced(resources, 8, "aws-public"), nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Stored)
	require.Len(t, result.Skipped, 1)
	assert.Equal(t, 1, result.Skipped[0].Version)
	assert.Equal(t, 3, exp.calls[1])
	snaps, err := db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	assert.Equal(t, 2, snaps[0].Version)

	again, err := Collect(context.Background(), db, exp, CollectOptions{
		Parallel: 1,
		Pricer: func(_ context.Context, resources []PriceResource) ([]PriceQuote, error) {
			return priced(resources, 8, "aws-public"), nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, again.Stored)
	snaps, err = db.Snapshots(time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Len(t, snaps, 2)
	assert.Equal(t, 1, snaps[0].Version)
	assert.Equal(t, 2, snaps[1].Version)
}

func TestCollect_EmptyAndEncrypted(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
		exp := &fakeExporter{
			history: historyRows(row(1, "update", "succeeded", "2025-01-01T00:00:00Z", "empty")),
			exports: map[int][]byte{
				1: []byte(`{"version":3,"deployment":{"resources":[` +
					`{"urn":"urn:stack","type":"pulumi:pulumi:Stack","custom":false}]}}`),
			},
		}
		called := false
		_, err := Collect(context.Background(), db, exp, CollectOptions{
			Pricer: func(context.Context, []PriceResource) ([]PriceQuote, error) {
				called = true
				return nil, errors.New("unused")
			},
		})
		require.NoError(t, err)
		assert.False(t, called)
		snaps, err := db.Snapshots(time.Time{}, time.Time{})
		require.NoError(t, err)
		require.Len(t, snaps, 1)
		assert.Equal(t, 0, snaps[0].ResourceCount)
	})
	t.Run("encrypted", func(t *testing.T) {
		t.Parallel()
		db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
		body := []byte(`{"version":3,"deployment":{"resources":[{"urn":"urn:web","custom":true,` +
			`"type":"aws:ec2/instance:Instance","inputs":{"instanceType":{"` +
			secretSignature + `":"1"}}}]}}`)
		exp := &fakeExporter{
			history: historyRows(row(1, "update", "succeeded", "2025-01-01T00:00:00Z", "secret")),
			exports: map[int][]byte{1: body},
		}
		_, err := Collect(
			context.Background(),
			db,
			exp,
			CollectOptions{Pricer: func(context.Context, []PriceResource) ([]PriceQuote, error) {
				return nil, errors.New("should not price")
			}},
		)
		require.ErrorIs(t, err, ErrEncryptedProperty)
		require.ErrorContains(t, err, "required property 'instanceType' is encrypted")
		snaps, err := db.Snapshots(time.Time{}, time.Time{})
		require.NoError(t, err)
		assert.Empty(t, snaps)
	})
}

func TestCollect_VersionReset(t *testing.T) {
	t.Parallel()
	db := openCostDB(t, filepath.Join(t.TempDir(), "dev.history.db"))
	require.NoError(
		t,
		db.Put(ZeroSnapshot(date(2024, 1, 1), 50), AnnotationFrom(StackUpdate{Version: 50, Kind: "update"})),
	)
	exp := &fakeExporter{
		history: historyRows(row(1, "update", "succeeded", "2025-04-01T00:00:00Z", "recreated")),
		exports: map[int][]byte{1: []byte(`{"version":3,"deployment":{"resources":[]}}`)},
	}
	_, err := Collect(context.Background(), db, exp, CollectOptions{Confirm: func(string) bool { return false }})
	require.ErrorIs(t, err, ErrVersionResetDeclined)
	stats, err := db.Stats()
	require.NoError(t, err)
	assert.Equal(t, uint64(50), stats.LastVersion)

	result, err := Collect(context.Background(), db, exp, CollectOptions{Confirm: func(warning string) bool {
		assert.Contains(t, warning, "v1 after v50")
		return true
	}})
	require.NoError(t, err)
	assert.True(t, result.Plan.Reset)
	assert.Equal(t, 1, result.Stored)
	stats, err = db.Stats()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), stats.LastVersion)
}

type fakeExporter struct {
	history []byte
	exports map[int][]byte
	fail    map[int]int
	calls   map[int]int
	mu      sync.Mutex
}

func (f *fakeExporter) History(context.Context) ([]byte, error) {
	return f.history, nil
}

func (f *fakeExporter) Export(_ context.Context, version int) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[int]int{}
	}
	f.calls[version]++
	if f.fail[version] > 0 {
		f.fail[version]--
		return nil, errors.New("export failed")
	}
	body, ok := f.exports[version]
	if !ok {
		return nil, errors.New("missing export")
	}
	return body, nil
}

func historyRows(rows ...map[string]any) []byte {
	body, err := json.Marshal(rows)
	if err != nil {
		panic(err)
	}
	return body
}

func row(version int, kind, result, start, message string) map[string]any {
	return map[string]any{
		"version": version, "kind": kind, "result": result, "startTime": start, "message": message,
		"resourceChanges": map[string]int{"same": 1},
	}
}

func exportJSON(urn, instance string) []byte {
	body, err := json.Marshal(map[string]any{
		"version": 3,
		"deployment": map[string]any{
			"resources": []any{map[string]any{
				"urn": urn, "custom": true, "type": "aws:ec2/instance:Instance",
				"inputs": map[string]any{"instanceType": instance, "region": "us-east-1"},
			}},
		},
	})
	if err != nil {
		panic(err)
	}
	return body
}

func providerResource() map[string]any {
	return map[string]any{
		"urn": "urn:pulumi:dev::app::pulumi:providers:aws::default", "custom": true,
		"type":   "pulumi:providers:aws",
		"inputs": map[string]any{"region": "us-east-1"},
	}
}

func exportWithProviderJSON(urn, instance string) []byte {
	body, err := json.Marshal(map[string]any{
		"version": 3,
		"deployment": map[string]any{
			"resources": []any{
				providerResource(),
				map[string]any{
					"urn": urn, "custom": true, "type": "aws:ec2/instance:Instance",
					"inputs": map[string]any{"instanceType": instance, "region": "us-east-1"},
				},
			},
		},
	})
	if err != nil {
		panic(err)
	}
	return body
}

func priced(resources []PriceResource, monthly float64, adapter string) []PriceQuote {
	results := make([]PriceQuote, 0, len(resources))
	for _, resource := range resources {
		results = append(results, PriceQuote{
			ResourceID:   resource.ID,
			ResourceType: resource.Type,
			Adapter:      adapter,
			Currency:     "USD",
			Monthly:      monthly,
		})
	}
	return results
}
