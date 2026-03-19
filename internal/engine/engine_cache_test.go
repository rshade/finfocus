package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine/cache"
)

// mockCache implements cache.Cache for testing.
type mockCache struct {
	mu         sync.RWMutex
	store      map[string]*cache.CacheEntry
	enabled    bool
	setCalls   int
	setWithTTL int
	getCalls   int
	setErr     error
	lastTTL    int
}

func newMockCache(enabled bool) *mockCache {
	return &mockCache{
		store:   make(map[string]*cache.CacheEntry),
		enabled: enabled,
	}
}

func (m *mockCache) Get(key string) (*cache.CacheEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func (m *mockCache) SetWithTTL(key string, data json.RawMessage, ttlSeconds int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setWithTTL++
	m.lastTTL = ttlSeconds

	if !m.enabled {
		return cache.ErrCacheDisabled
	}
	if key == "" {
		return cache.ErrInvalidCacheKey
	}
	if m.setErr != nil {
		return m.setErr
	}
	m.store[key] = cache.NewCacheEntry(key, data, ttlSeconds)
	return nil
}

func (m *mockCache) IsEnabled() bool {
	return m.enabled
}

func (m *mockCache) Close() error {
	return nil
}

func (m *mockCache) InvalidateByPrefix(_ string) (int, error) {
	return 0, nil
}

// preloadResults stores a slice of CostResults in the mock cache for a given key.
func (m *mockCache) preloadResults(key string, results []CostResult) {
	data, _ := json.Marshal(results)
	m.store[key] = cache.NewCacheEntry(key, data, 3600)
}

// --- Tests for generateProjectedCostResourceKey (structured keys) ---

func TestGenerateProjectedCostResourceKey(t *testing.T) {
	tests := []struct {
		name            string
		resource        ResourceDescriptor
		wantErr         bool
		errContains     string
		wantContains    string
		expectEqual     *ResourceDescriptor
		expectDifferent *ResourceDescriptor
	}{
		{
			name: "structured format",
			resource: ResourceDescriptor{
				Type: "aws:ec2:Instance", Provider: "aws",
				Properties: map[string]interface{}{"instanceType": "t3.micro", "availabilityZone": "us-east-1"},
			},
			wantContains: "projected/aws/aws:ec2:Instance/us-east-1/t3.micro",
		},
		{
			name: "different properties produce different keys",
			resource: ResourceDescriptor{
				Type: "aws:ec2:Instance", Provider: "aws",
				Properties: map[string]interface{}{"instanceType": "t3.large"},
			},
			expectDifferent: &ResourceDescriptor{
				Type: "aws:ec2:Instance", Provider: "aws",
				Properties: map[string]interface{}{"instanceType": "t3.micro"},
			},
		},
		{
			name:        "empty Type returns error",
			resource:    ResourceDescriptor{Provider: "aws"},
			wantErr:     true,
			errContains: "resource type is required",
		},
		{
			name: "same region and sku produce same key",
			resource: ResourceDescriptor{
				Type: "aws:ec2:Instance", Provider: "aws",
				Properties: map[string]interface{}{"instanceType": "t3.micro", "region": "us-east-1"},
			},
			expectEqual: &ResourceDescriptor{
				Type: "aws:ec2:Instance", Provider: "aws",
				Properties: map[string]interface{}{"instanceType": "t3.micro", "region": "us-east-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := generateProjectedCostResourceKey(tt.resource)
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, key)

			if tt.wantContains != "" {
				assert.Equal(t, tt.wantContains, key)
			}

			// Determinism: same input produces same key
			key2, err := generateProjectedCostResourceKey(tt.resource)
			require.NoError(t, err)
			assert.Equal(t, key, key2, "same resource should produce same key")

			if tt.expectEqual != nil {
				otherKey, otherErr := generateProjectedCostResourceKey(*tt.expectEqual)
				require.NoError(t, otherErr)
				assert.Equal(t, key, otherKey)
			}
			if tt.expectDifferent != nil {
				otherKey, otherErr := generateProjectedCostResourceKey(*tt.expectDifferent)
				require.NoError(t, otherErr)
				assert.NotEqual(t, key, otherKey)
			}
		})
	}
}

// --- Tests for projected cost cache integration ---

func TestProjectedCostCacheIntegration(t *testing.T) {
	t.Run("cache hit returns result with (cached) in Adapter", func(t *testing.T) {
		mc := newMockCache(true)

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
		require.Len(t, results.Results, 1)
		assert.NotContains(t, results.Results[0].Adapter, "(cached)")
	})

	t.Run("cache store failure logs WARN and returns live result", func(t *testing.T) {
		mc := newMockCache(true)
		mc.setErr = assert.AnError

		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "test-instance",
			Provider: "aws",
		}

		eng := New(nil, nil)
		eng.cache = mc

		results, err := eng.GetProjectedCostWithErrors(context.Background(), []ResourceDescriptor{resource})
		require.NoError(t, err)
		require.Len(t, results.Results, 1)
		assert.NotContains(t, results.Results[0].Adapter, "(cached)")
	})
}

// --- Tests for generateActualCostCacheKey (structured keys) ---

func TestGenerateActualCostCacheKey(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	baseRequest := func() ActualCostRequest {
		return ActualCostRequest{
			Resources: []ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"}},
			From:      baseTime,
			To:        endTime,
		}
	}

	tests := []struct {
		name      string
		request1  ActualCostRequest
		request2  ActualCostRequest
		wantEqual bool
		reason    string
	}{
		{
			name:      "deterministic output",
			request1:  baseRequest(),
			wantEqual: true,
			reason:    "same request should produce same key",
		},
		{
			name:     "different time ranges produce different keys",
			request1: baseRequest(),
			request2: func() ActualCostRequest {
				r := baseRequest()
				r.To = time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC)
				return r
			}(),
			reason: "different time ranges should produce different keys",
		},
		{
			name: "different tags produce different keys",
			request1: func() ActualCostRequest {
				r := baseRequest()
				r.Tags = map[string]string{"env": "prod"}
				return r
			}(),
			request2: func() ActualCostRequest {
				r := baseRequest()
				r.Tags = map[string]string{"env": "staging"}
				return r
			}(),
			reason: "different tags should produce different keys",
		},
		{
			name: "tag order independence",
			request1: func() ActualCostRequest {
				r := baseRequest()
				r.Tags = map[string]string{"env": "prod", "team": "infra"}
				return r
			}(),
			request2: func() ActualCostRequest {
				r := baseRequest()
				r.Tags = map[string]string{"team": "infra", "env": "prod"}
				return r
			}(),
			wantEqual: true,
			reason:    "tag order should not affect key",
		},
		{
			name: "adapter included in key",
			request1: func() ActualCostRequest {
				r := baseRequest()
				r.Adapter = "aws-public"
				return r
			}(),
			request2: func() ActualCostRequest {
				r := baseRequest()
				r.Adapter = "kubecost"
				return r
			}(),
			reason: "different adapters should produce different keys",
		},
		{
			name: "groupBy included in key",
			request1: func() ActualCostRequest {
				r := baseRequest()
				r.GroupBy = "daily"
				return r
			}(),
			request2: func() ActualCostRequest {
				r := baseRequest()
				r.GroupBy = "monthly"
				return r
			}(),
			reason: "different groupBy should produce different keys",
		},
		{
			name: "key contains structured format",
			request1: func() ActualCostRequest {
				r := baseRequest()
				return r
			}(),
			wantEqual: true,
			reason:    "key should contain structured date format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := generateActualCostCacheKey(tt.request1)
			assert.NotEmpty(t, key1)

			// Verify structured format
			assert.Contains(t, key1, "actual/")

			// Determinism: same input produces same key
			key1Again := generateActualCostCacheKey(tt.request1)
			assert.Equal(t, key1, key1Again, "same request should produce same key")

			if len(tt.request2.Resources) > 0 {
				key2 := generateActualCostCacheKey(tt.request2)

				if tt.wantEqual {
					assert.Equal(t, key1, key2, tt.reason)
				} else {
					assert.NotEqual(t, key1, key2, tt.reason)
				}
			}
		})
	}
}

// --- Tests for actual cost cache integration ---

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

		key := generateActualCostCacheKey(request)
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

		request := ActualCostRequest{
			Resources: []ResourceDescriptor{
				{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"},
			},
			From:             baseTime,
			To:               endTime,
			FallbackEstimate: true,
		}

		eng := New(nil, nil)
		eng.cache = mc

		results, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), request)
		require.NoError(t, err)
		require.Len(t, results.Results, 1)
		assert.NotContains(t, results.Results[0].Adapter, "(cached)")
		assert.Greater(t, mc.setCalls, 0, "cache miss should store result")
	})

	t.Run("cache disabled skips all cache operations", func(t *testing.T) {
		mc := newMockCache(false)

		request := ActualCostRequest{
			Resources: []ResourceDescriptor{
				{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"},
			},
			From:             baseTime,
			To:               endTime,
			FallbackEstimate: true,
		}

		eng := New(nil, nil)
		eng.cache = mc

		results, err := eng.GetActualCostWithOptionsAndErrors(context.Background(), request)
		require.NoError(t, err)
		require.Len(t, results.Results, 1)
		assert.NotContains(t, results.Results[0].Adapter, "(cached)")
		assert.Equal(t, 0, mc.getCalls, "cache disabled should skip Get")
		assert.Equal(t, 0, mc.setCalls, "cache disabled should skip Set")
	})
}

// T008: Tests for storeProjectedCostCache TTL override when ExpiresAt is set.
func TestStoreProjectedCostCache_TTLOverride(t *testing.T) {
	t.Run("SetWithTTL called when ExpiresAt is set", func(t *testing.T) {
		mc := newMockCache(true)
		eng := New(nil, nil)
		eng.cache = mc

		futureTime := time.Now().Add(2 * time.Hour)
		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "i-ttl-test",
			Provider: "aws",
		}
		results := []CostResult{
			{
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				Currency:     "USD",
				Monthly:      15.0,
				ExpiresAt:    &futureTime,
			},
		}

		eng.storeProjectedCostCache(context.Background(), resource, results)

		assert.Equal(t, 1, mc.setWithTTL, "SetWithTTL should be called when ExpiresAt is set")
		assert.Equal(t, 0, mc.setCalls, "Set should not be called when ExpiresAt is set")
		assert.InDelta(t, 7200, mc.lastTTL, 5, "TTL should be ~2 hours in seconds")
	})

	t.Run("Set called with default TTL when ExpiresAt is nil", func(t *testing.T) {
		mc := newMockCache(true)
		eng := New(nil, nil)
		eng.cache = mc

		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "i-no-ttl",
			Provider: "aws",
		}
		results := []CostResult{
			{
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				Currency:     "USD",
				Monthly:      20.0,
			},
		}

		eng.storeProjectedCostCache(context.Background(), resource, results)

		assert.Equal(t, 1, mc.setCalls, "Set should be called when ExpiresAt is nil")
		assert.Equal(t, 0, mc.setWithTTL, "SetWithTTL should not be called when ExpiresAt is nil")
	})

	t.Run("SetWithTTL error logs warning without panic", func(t *testing.T) {
		mc := newMockCache(true)
		mc.setErr = assert.AnError
		eng := New(nil, nil)
		eng.cache = mc

		ctx, buf := ctxWithLogBuffer(zerolog.WarnLevel)
		futureTime := time.Now().Add(2 * time.Hour)
		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "i-err-test",
			Provider: "aws",
		}
		results := []CostResult{
			{
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				Currency:     "USD",
				Monthly:      15.0,
				ExpiresAt:    &futureTime,
			},
		}

		eng.storeProjectedCostCache(ctx, resource, results)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "failed to cache cost results with plugin TTL")
		assert.Equal(t, 1, mc.setWithTTL, "SetWithTTL should have been called")
	})
}

// T019: Tests for skip-caching when projected cost ExpiresAt is in the past.
func TestStoreProjectedCostCache_SkipPastExpiry(t *testing.T) {
	t.Run("neither Set nor SetWithTTL called when ExpiresAt is in the past", func(t *testing.T) {
		mc := newMockCache(true)
		eng := New(nil, nil)
		eng.cache = mc

		pastTime := time.Now().Add(-1 * time.Hour)
		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "i-past",
			Provider: "aws",
		}
		results := []CostResult{
			{
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				Currency:     "USD",
				Monthly:      15.0,
				ExpiresAt:    &pastTime,
			},
		}

		eng.storeProjectedCostCache(context.Background(), resource, results)

		assert.Equal(t, 0, mc.setCalls, "Set should not be called for past ExpiresAt")
		assert.Equal(t, 0, mc.setWithTTL, "SetWithTTL should not be called for past ExpiresAt")
	})
}

// T020: Tests for skip-caching when all actual cost results have past ExpiresAt.
func TestStoreActualCostCache_SkipPastExpiry(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	t.Run("neither Set nor SetWithTTL called when all results have past ExpiresAt", func(t *testing.T) {
		mc := newMockCache(true)
		eng := New(nil, nil)
		eng.cache = mc

		pastTime1 := time.Now().Add(-2 * time.Hour)
		pastTime2 := time.Now().Add(-30 * time.Minute)
		request := ActualCostRequest{
			Resources: []ResourceDescriptor{
				{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"},
			},
			From: baseTime,
			To:   endTime,
		}
		results := []CostResult{
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-1",
				Currency:     "USD",
				TotalCost:    100.0,
				ExpiresAt:    &pastTime1,
			},
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-1",
				Currency:     "USD",
				TotalCost:    50.0,
				ExpiresAt:    &pastTime2,
			},
		}

		eng.storeActualCostCache(context.Background(), request, results)

		assert.Equal(t, 0, mc.setCalls, "Set should not be called for past ExpiresAt")
		assert.Equal(t, 0, mc.setWithTTL, "SetWithTTL should not be called for past ExpiresAt")
	})
}

// T021: Tests for cache-disabled edge case — expires_at hints are irrelevant.
func TestStoreCache_DisabledIgnoresExpiresAt(t *testing.T) {
	t.Run("projected: cache disabled ignores ExpiresAt entirely", func(t *testing.T) {
		mc := newMockCache(false)
		eng := New(nil, nil)
		eng.cache = mc

		futureTime := time.Now().Add(2 * time.Hour)
		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "i-disabled",
			Provider: "aws",
		}
		results := []CostResult{
			{
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				Currency:     "USD",
				Monthly:      15.0,
				ExpiresAt:    &futureTime,
			},
		}

		eng.storeProjectedCostCache(context.Background(), resource, results)

		assert.Equal(t, 0, mc.setCalls, "Set should not be called when cache disabled")
		assert.Equal(t, 0, mc.setWithTTL, "SetWithTTL should not be called when cache disabled")
	})

	t.Run("actual: cache disabled ignores ExpiresAt entirely", func(t *testing.T) {
		mc := newMockCache(false)
		eng := New(nil, nil)
		eng.cache = mc

		futureTime := time.Now().Add(2 * time.Hour)
		request := ActualCostRequest{
			Resources: []ResourceDescriptor{
				{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"},
			},
			From: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			To:   time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		}
		results := []CostResult{
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-1",
				Currency:     "USD",
				TotalCost:    100.0,
				ExpiresAt:    &futureTime,
			},
		}

		eng.storeActualCostCache(context.Background(), request, results)

		assert.Equal(t, 0, mc.setCalls, "Set should not be called when cache disabled")
		assert.Equal(t, 0, mc.setWithTTL, "SetWithTTL should not be called when cache disabled")
	})
}

// T009: Tests for storeActualCostCache TTL override when ExpiresAt is set.
func TestStoreActualCostCache_TTLOverride(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	t.Run("SetWithTTL called with earliest TTL from batch", func(t *testing.T) {
		mc := newMockCache(true)
		eng := New(nil, nil)
		eng.cache = mc

		earlierExpiry := time.Now().Add(30 * time.Minute)
		laterExpiry := time.Now().Add(12 * time.Hour)
		request := ActualCostRequest{
			Resources: []ResourceDescriptor{
				{Type: "aws:ec2:Instance", ID: "i-1", Provider: "aws"},
			},
			From: baseTime,
			To:   endTime,
		}
		results := []CostResult{
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-1",
				Currency:     "USD",
				TotalCost:    100.0,
				ExpiresAt:    &laterExpiry,
			},
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-1",
				Currency:     "USD",
				TotalCost:    50.0,
				ExpiresAt:    &earlierExpiry,
			},
		}

		eng.storeActualCostCache(context.Background(), request, results)

		assert.Equal(t, 1, mc.setWithTTL, "SetWithTTL should be called when ExpiresAt is set")
		assert.Equal(t, 0, mc.setCalls, "Set should not be called when ExpiresAt is set")
		assert.InDelta(t, 1800, mc.lastTTL, 5, "TTL should be ~30 minutes (earliest)")
	})

	t.Run("Set called with default when no ExpiresAt", func(t *testing.T) {
		mc := newMockCache(true)
		eng := New(nil, nil)
		eng.cache = mc

		request := ActualCostRequest{
			Resources: []ResourceDescriptor{
				{Type: "aws:ec2:Instance", ID: "i-2", Provider: "aws"},
			},
			From: baseTime,
			To:   endTime,
		}
		results := []CostResult{
			{
				ResourceType: "aws:ec2:Instance",
				ResourceID:   "i-2",
				Currency:     "USD",
				TotalCost:    75.0,
			},
		}

		eng.storeActualCostCache(context.Background(), request, results)

		assert.Equal(t, 1, mc.setCalls, "Set should be called when no ExpiresAt")
		assert.Equal(t, 0, mc.setWithTTL, "SetWithTTL should not be called when no ExpiresAt")
	})
}

// ctxWithLogBuffer returns a context with a zerolog logger writing to a buffer,
// and the buffer for inspection.
func ctxWithLogBuffer(level zerolog.Level) (context.Context, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(level).With().Logger()
	return logger.WithContext(context.Background()), &buf
}

// T023: Tests for debug logging when plugin TTL differs from default.
func TestStoreProjectedCostCache_DebugLog_TTLOverride(t *testing.T) {
	t.Run("debug log emitted when plugin TTL differs from default", func(t *testing.T) {
		mc := newMockCache(true)
		eng := New(nil, nil)
		eng.cache = mc

		ctx, buf := ctxWithLogBuffer(zerolog.DebugLevel)
		futureTime := time.Now().Add(2 * time.Hour)
		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "i-log-test",
			Provider: "aws",
		}
		results := []CostResult{
			{
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				Currency:     "USD",
				Monthly:      15.0,
				ExpiresAt:    &futureTime,
			},
		}

		eng.storeProjectedCostCache(ctx, resource, results)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "using plugin TTL hint", "should log TTL override")
		assert.Contains(t, logOutput, "storeProjectedCostCache", "should include operation")
	})
}

// T024: Tests for debug logging when caching is skipped due to past ExpiresAt.
func TestStoreProjectedCostCache_DebugLog_SkipCache(t *testing.T) {
	t.Run("debug log emitted when caching skipped for past ExpiresAt", func(t *testing.T) {
		mc := newMockCache(true)
		eng := New(nil, nil)
		eng.cache = mc

		ctx, buf := ctxWithLogBuffer(zerolog.DebugLevel)
		pastTime := time.Now().Add(-1 * time.Hour)
		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "i-skip-test",
			Provider: "aws",
		}
		results := []CostResult{
			{
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				Currency:     "USD",
				Monthly:      15.0,
				ExpiresAt:    &pastTime,
			},
		}

		eng.storeProjectedCostCache(ctx, resource, results)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "caching skipped", "should log skip reason")
		assert.Contains(t, logOutput, "past", "should mention past expiration")
	})
}

// T025: Tests for warning logging when CalculatePluginTTL caps at MaxTTLSeconds.
func TestStoreProjectedCostCache_WarnLog_TTLCapped(t *testing.T) {
	t.Run("warn log emitted when plugin TTL is capped at max", func(t *testing.T) {
		mc := newMockCache(true)
		eng := New(nil, nil)
		eng.cache = mc

		ctx, buf := ctxWithLogBuffer(zerolog.DebugLevel)
		// Exceed MaxTTLSeconds (604800 = 7 days)
		farFuture := time.Now().Add(30 * 24 * time.Hour)
		resource := ResourceDescriptor{
			Type:     "aws:ec2:Instance",
			ID:       "i-cap-test",
			Provider: "aws",
		}
		results := []CostResult{
			{
				ResourceType: resource.Type,
				ResourceID:   resource.ID,
				Currency:     "USD",
				Monthly:      15.0,
				ExpiresAt:    &farFuture,
			},
		}

		eng.storeProjectedCostCache(ctx, resource, results)

		logOutput := buf.String()
		assert.Contains(t, logOutput, "plugin TTL capped", "should warn about cap")
		assert.Contains(t, logOutput, "warn", "should be warn level")
	})
}
