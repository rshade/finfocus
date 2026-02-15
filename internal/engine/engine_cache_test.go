package engine

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine/cache"
)

// mockCache implements cache.Cache for testing.
type mockCache struct {
	mu       sync.RWMutex
	store    map[string]*cache.CacheEntry
	enabled  bool
	setCalls int
	getCalls int
	setErr   error
}

func newMockCache(enabled bool) *mockCache {
	return &mockCache{
		store:   make(map[string]*cache.CacheEntry),
		enabled: enabled,
	}
}

func (m *mockCache) Get(key string) (*cache.CacheEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	m.getCalls++

	if !m.enabled {
		return nil, cache.ErrCacheDisabled
	}
	if key == "" {
		return nil, cache.ErrInvalidCacheKey
	}
	entry, ok := m.store[key]
	if !ok {
		return nil, cache.ErrCacheNotFound
	}
	return entry, nil
}

func (m *mockCache) Set(key string, data json.RawMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCalls++

	if !m.enabled {
		return cache.ErrCacheDisabled
	}
	if key == "" {
		return cache.ErrInvalidCacheKey
	}
	if m.setErr != nil {
		return m.setErr
	}
	m.store[key] = cache.NewCacheEntry(key, data, 3600)
	return nil
}

func (m *mockCache) IsEnabled() bool {
	return m.enabled
}

// preloadResults stores a slice of CostResults in the mock cache for a given key.
func (m *mockCache) preloadResults(key string, results []CostResult) {
	data, _ := json.Marshal(results)
	m.store[key] = cache.NewCacheEntry(key, data, 3600)
}

// --- T009: Tests for generateProjectedCostResourceKey ---

func TestGenerateProjectedCostResourceKey(t *testing.T) {
	t.Run("deterministic output", func(t *testing.T) {
		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
				"region":       "us-east-1",
			},
		}

		key1, err := generateProjectedCostResourceKey(resource)
		require.NoError(t, err)
		assert.NotEmpty(t, key1)

		key2, err := generateProjectedCostResourceKey(resource)
		require.NoError(t, err)
		assert.Equal(t, key1, key2, "same resource should produce same key")
	})

	t.Run("different properties produce different keys", func(t *testing.T) {
		resource1 := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}
		resource2 := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": "t3.large",
			},
		}

		key1, err := generateProjectedCostResourceKey(resource1)
		require.NoError(t, err)

		key2, err := generateProjectedCostResourceKey(resource2)
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2, "different properties should produce different keys")
	})

	t.Run("empty Type returns error", func(t *testing.T) {
		resource := ResourceDescriptor{
			Provider: "aws",
		}
		_, err := generateProjectedCostResourceKey(resource)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resource type is required")
	})

	t.Run("property order independence", func(t *testing.T) {
		resource1 := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"a": "1",
				"b": "2",
				"c": "3",
			},
		}
		resource2 := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"c": "3",
				"a": "1",
				"b": "2",
			},
		}

		key1, err := generateProjectedCostResourceKey(resource1)
		require.NoError(t, err)

		key2, err := generateProjectedCostResourceKey(resource2)
		require.NoError(t, err)

		assert.Equal(t, key1, key2, "property order should not affect key")
	})
}

// --- T010: Tests for projected cost cache integration ---

func TestProjectedCostCacheIntegration(t *testing.T) {
	t.Run("cache hit returns result with (cached) in Adapter", func(t *testing.T) {
		mc := newMockCache(true)

		// Preload a cached result
		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "test-instance",
			Provider: "aws",
			Properties: map[string]interface{}{
				"instanceType": "t3.micro",
			},
		}

		cachedResult := CostResult{
			ResourceType: resource.Type,
			ResourceID:   resource.ID,
			Adapter:      "aws-public",
			Currency:     "USD",
			Monthly:      10.0,
			Hourly:       0.014,
		}

		key, err := generateProjectedCostResourceKey(resource)
		require.NoError(t, err)
		mc.preloadResults(key, []CostResult{cachedResult})

		eng := New(nil, nil)
		eng.cache = mc

		results, err := eng.GetProjectedCostWithErrors(context.Background(), []ResourceDescriptor{resource})
		require.NoError(t, err)
		require.Len(t, results.Results, 1)
		assert.Contains(t, results.Results[0].Adapter, "(cached)")
		assert.Equal(t, 10.0, results.Results[0].Monthly)
		assert.Empty(t, results.Errors, "cache hit should produce no errors")
	})

	t.Run("cache disabled skips all cache operations", func(t *testing.T) {
		mc := newMockCache(false)

		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "test-instance",
			Provider: "aws",
		}

		eng := New(nil, nil)
		eng.cache = mc

		results, err := eng.GetProjectedCostWithErrors(context.Background(), []ResourceDescriptor{resource})
		require.NoError(t, err)
		// Should get a "no pricing" result, not a cached result
		require.Len(t, results.Results, 1)
		assert.NotContains(t, results.Results[0].Adapter, "(cached)")
	})

	t.Run("cache store failure logs WARN and returns live result", func(t *testing.T) {
		mc := newMockCache(true)
		mc.setErr = assert.AnError // Force Set to fail

		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "test-instance",
			Provider: "aws",
		}

		eng := New(nil, nil)
		eng.cache = mc

		results, err := eng.GetProjectedCostWithErrors(context.Background(), []ResourceDescriptor{resource})
		require.NoError(t, err)
		// Should still return a result (from spec fallback or placeholder)
		require.Len(t, results.Results, 1)
		assert.NotContains(t, results.Results[0].Adapter, "(cached)")
	})
}

// --- T015: Tests for generateActualCostCacheKey ---

func TestGenerateActualCostCacheKey(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	t.Run("deterministic output", func(t *testing.T) {
		request := ActualCostRequest{
			Resources: []ResourceDescriptor{
				{Type: "aws:ec2:Instance", ID: "i-1"},
			},
			From: baseTime,
			To:   endTime,
		}

		key1, err := generateActualCostCacheKey(request)
		require.NoError(t, err)
		assert.NotEmpty(t, key1)

		key2, err := generateActualCostCacheKey(request)
		require.NoError(t, err)
		assert.Equal(t, key1, key2, "same request should produce same key")
	})

	t.Run("different resource IDs produce different keys", func(t *testing.T) {
		request1 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:      baseTime,
			To:        endTime,
		}
		request2 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-2"}},
			From:      baseTime,
			To:        endTime,
		}

		key1, err := generateActualCostCacheKey(request1)
		require.NoError(t, err)

		key2, err := generateActualCostCacheKey(request2)
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2, "different resources should produce different keys")
	})

	t.Run("different time ranges produce different keys", func(t *testing.T) {
		request1 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:      baseTime,
			To:        endTime,
		}
		request2 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:      baseTime,
			To:        time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC),
		}

		key1, err := generateActualCostCacheKey(request1)
		require.NoError(t, err)

		key2, err := generateActualCostCacheKey(request2)
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2, "different time ranges should produce different keys")
	})

	t.Run("different tags produce different keys", func(t *testing.T) {
		request1 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:      baseTime,
			To:        endTime,
			Tags:      map[string]string{"env": "prod"},
		}
		request2 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:      baseTime,
			To:        endTime,
			Tags:      map[string]string{"env": "staging"},
		}

		key1, err := generateActualCostCacheKey(request1)
		require.NoError(t, err)

		key2, err := generateActualCostCacheKey(request2)
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2, "different tags should produce different keys")
	})

	t.Run("tag order independence", func(t *testing.T) {
		request1 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:      baseTime,
			To:        endTime,
			Tags:      map[string]string{"env": "prod", "team": "infra"},
		}
		request2 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:      baseTime,
			To:        endTime,
			Tags:      map[string]string{"team": "infra", "env": "prod"},
		}

		key1, err := generateActualCostCacheKey(request1)
		require.NoError(t, err)

		key2, err := generateActualCostCacheKey(request2)
		require.NoError(t, err)

		assert.Equal(t, key1, key2, "tag order should not affect key")
	})

	t.Run("adapter included in key", func(t *testing.T) {
		request1 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:      baseTime,
			To:        endTime,
			Adapter:   "aws-public",
		}
		request2 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:      baseTime,
			To:        endTime,
			Adapter:   "kubecost",
		}

		key1, err := generateActualCostCacheKey(request1)
		require.NoError(t, err)

		key2, err := generateActualCostCacheKey(request2)
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2, "different adapters should produce different keys")
	})

	t.Run("groupBy included in key", func(t *testing.T) {
		request1 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:      baseTime,
			To:        endTime,
			GroupBy:   "daily",
		}
		request2 := ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:      baseTime,
			To:        endTime,
			GroupBy:   "monthly",
		}

		key1, err := generateActualCostCacheKey(request1)
		require.NoError(t, err)

		key2, err := generateActualCostCacheKey(request2)
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2, "different groupBy should produce different keys")
	})

	t.Run("FallbackEstimate included in key", func(t *testing.T) {
		request1 := ActualCostRequest{
			Resources:        []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:             baseTime,
			To:               endTime,
			FallbackEstimate: true,
		}
		request2 := ActualCostRequest{
			Resources:        []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:             baseTime,
			To:               endTime,
			FallbackEstimate: false,
		}

		key1, err := generateActualCostCacheKey(request1)
		require.NoError(t, err)

		key2, err := generateActualCostCacheKey(request2)
		require.NoError(t, err)

		assert.NotEqual(t, key1, key2, "FallbackEstimate should affect key")
	})

	t.Run("EstimateConfidence excluded from key", func(t *testing.T) {
		request1 := ActualCostRequest{
			Resources:          []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:               baseTime,
			To:                 endTime,
			EstimateConfidence: true,
		}
		request2 := ActualCostRequest{
			Resources:          []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
			From:               baseTime,
			To:                 endTime,
			EstimateConfidence: false,
		}

		key1, err := generateActualCostCacheKey(request1)
		require.NoError(t, err)

		key2, err := generateActualCostCacheKey(request2)
		require.NoError(t, err)

		assert.Equal(t, key1, key2, "EstimateConfidence should not affect key")
	})
}

// --- T016: Tests for actual cost cache integration ---

func TestActualCostCacheIntegration(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	t.Run("cache hit returns results with (cached) markers", func(t *testing.T) {
		mc := newMockCache(true)

		request := ActualCostRequest{
			Resources: []ResourceDescriptor{
				{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"},
			},
			From: baseTime,
			To:   endTime,
		}

		cachedResults := []CostResult{
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-1",
				Adapter:      "aws-public",
				Currency:     "USD",
				TotalCost:    100.0,
			},
		}

		key, err := generateActualCostCacheKey(request)
		require.NoError(t, err)
		mc.preloadResults(key, cachedResults)

		eng := New(nil, nil)
		eng.cache = mc

		results, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), request)
		require.NoError(t, err)
		require.Len(t, results.Results, 1)
		assert.Contains(t, results.Results[0].Adapter, "(cached)")
		assert.Equal(t, 100.0, results.Results[0].TotalCost)
		assert.Empty(t, results.Errors, "cache hit should produce no errors")
	})

	t.Run("cache miss calls plugins", func(t *testing.T) {
		mc := newMockCache(true)
		// No preloaded data - cache miss

		request := ActualCostRequest{
			Resources: []ResourceDescriptor{
				{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"},
			},
			From:             baseTime,
			To:               endTime,
			FallbackEstimate: true, // Needed to get placeholder result with no plugins
		}

		eng := New(nil, nil) // No plugins either, so will get placeholder result
		eng.cache = mc

		results, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), request)
		require.NoError(t, err)
		require.Len(t, results.Results, 1)
		// Should NOT have "(cached)" since it's a miss
		assert.NotContains(t, results.Results[0].Adapter, "(cached)")
		// Should have stored the result in cache
		assert.Greater(t, mc.setCalls, 0, "cache miss should store result")
	})
}
