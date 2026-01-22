package cache_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine/cache"
)

// TestGenerateKey verifies deterministic cache key generation.
func TestGenerateKey(t *testing.T) {
	tests := []struct {
		name   string
		params cache.KeyParams
	}{
		{
			name: "simple params",
			params: cache.KeyParams{
				Operation: "projected_cost",
				Provider:  "aws",
			},
		},
		{
			name: "with resource types",
			params: cache.KeyParams{
				Operation:     "projected_cost",
				Provider:      "aws",
				ResourceTypes: []string{"ec2", "rds", "s3"},
			},
		},
		{
			name: "with filters",
			params: cache.KeyParams{
				Operation: "actual_cost",
				Provider:  "gcp",
				Filters: map[string]string{
					"region": "us-east-1",
					"env":    "prod",
				},
			},
		},
		{
			name: "with pagination",
			params: cache.KeyParams{
				Operation: "recommendations",
				Provider:  "azure",
				Pagination: &cache.PaginationKeyParams{
					Limit:     20,
					Offset:    0,
					SortField: "savings",
					SortOrder: "desc",
				},
			},
		},
		{
			name: "complex params",
			params: cache.KeyParams{
				Operation:     "recommendations",
				Provider:      "aws",
				ResourceTypes: []string{"ec2", "rds", "s3", "elasticache"},
				Filters: map[string]string{
					"region": "us-west-2",
					"env":    "prod",
					"team":   "platform",
				},
				Pagination: &cache.PaginationKeyParams{
					Limit:     50,
					Offset:    100,
					SortField: "cost",
					SortOrder: "asc",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1, err1 := cache.GenerateKey(tt.params)
			require.NoError(t, err1)
			require.NotEmpty(t, key1)

			// Generate key again with same params
			key2, err2 := cache.GenerateKey(tt.params)
			require.NoError(t, err2)
			require.NotEmpty(t, key2)

			// Verify keys are identical (deterministic)
			assert.Equal(t, key1, key2)

			// Verify key is SHA256 hash (64 hex characters)
			assert.Len(t, key1, 64)
		})
	}
}

// TestGenerateKey_Normalization verifies param normalization.
func TestGenerateKey_Normalization(t *testing.T) {
	// Test case sensitivity normalization
	params1 := cache.KeyParams{
		Operation: "PROJECTED_COST",
		Provider:  "AWS",
	}

	params2 := cache.KeyParams{
		Operation: "projected_cost",
		Provider:  "aws",
	}

	key1, err := cache.GenerateKey(params1)
	require.NoError(t, err)

	key2, err := cache.GenerateKey(params2)
	require.NoError(t, err)

	// Keys should be identical after normalization
	assert.Equal(t, key1, key2)
}

// TestGenerateKey_ResourceTypeOrdering verifies resource type sorting.
func TestGenerateKey_ResourceTypeOrdering(t *testing.T) {
	// Different order, same resource types
	params1 := cache.KeyParams{
		Operation:     "projected_cost",
		Provider:      "aws",
		ResourceTypes: []string{"s3", "ec2", "rds"},
	}

	params2 := cache.KeyParams{
		Operation:     "projected_cost",
		Provider:      "aws",
		ResourceTypes: []string{"ec2", "rds", "s3"},
	}

	key1, err := cache.GenerateKey(params1)
	require.NoError(t, err)

	key2, err := cache.GenerateKey(params2)
	require.NoError(t, err)

	// Keys should be identical after sorting
	assert.Equal(t, key1, key2)
}

// TestGenerateKey_FilterOrdering verifies filter key sorting.
func TestGenerateKey_FilterOrdering(t *testing.T) {
	// Same filters, different insertion order
	params1 := cache.KeyParams{
		Operation: "actual_cost",
		Provider:  "aws",
		Filters: map[string]string{
			"region": "us-east-1",
			"env":    "prod",
			"team":   "platform",
		},
	}

	params2 := cache.KeyParams{
		Operation: "actual_cost",
		Provider:  "aws",
		Filters: map[string]string{
			"team":   "platform",
			"env":    "prod",
			"region": "us-east-1",
		},
	}

	key1, err := cache.GenerateKey(params1)
	require.NoError(t, err)

	key2, err := cache.GenerateKey(params2)
	require.NoError(t, err)

	// Keys should be identical (filter order doesn't matter)
	assert.Equal(t, key1, key2)
}

// TestGenerateKey_DifferentParams verifies different params produce different keys.
func TestGenerateKey_DifferentParams(t *testing.T) {
	baseParams := cache.KeyParams{
		Operation: "projected_cost",
		Provider:  "aws",
	}

	tests := []struct {
		name   string
		modify func(cache.KeyParams) cache.KeyParams
	}{
		{
			name: "different operation",
			modify: func(p cache.KeyParams) cache.KeyParams {
				p.Operation = "actual_cost"
				return p
			},
		},
		{
			name: "different provider",
			modify: func(p cache.KeyParams) cache.KeyParams {
				p.Provider = "gcp"
				return p
			},
		},
		{
			name: "added resource types",
			modify: func(p cache.KeyParams) cache.KeyParams {
				p.ResourceTypes = []string{"ec2"}
				return p
			},
		},
		{
			name: "added filters",
			modify: func(p cache.KeyParams) cache.KeyParams {
				p.Filters = map[string]string{"region": "us-east-1"}
				return p
			},
		},
		{
			name: "added pagination",
			modify: func(p cache.KeyParams) cache.KeyParams {
				p.Pagination = &cache.PaginationKeyParams{Limit: 20}
				return p
			},
		},
	}

	baseKey, err := cache.GenerateKey(baseParams)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modifiedParams := tt.modify(baseParams)
			modifiedKey, err := cache.GenerateKey(modifiedParams)
			require.NoError(t, err)

			// Modified params should produce different key
			assert.NotEqual(t, baseKey, modifiedKey)
		})
	}
}

// TestGenerateSimpleKey verifies simple key generation.
func TestGenerateSimpleKey(t *testing.T) {
	tests := []struct {
		name             string
		operation        string
		provider         string
		additionalParams []string
	}{
		{
			name:      "basic params",
			operation: "projected_cost",
			provider:  "aws",
		},
		{
			name:             "with additional params",
			operation:        "actual_cost",
			provider:         "gcp",
			additionalParams: []string{"us-east-1", "prod"},
		},
		{
			name:             "many additional params",
			operation:        "recommendations",
			provider:         "azure",
			additionalParams: []string{"region1", "env1", "team1", "service1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := cache.GenerateSimpleKey(tt.operation, tt.provider, tt.additionalParams...)
			require.NotEmpty(t, key1)

			// Generate again with same params
			key2 := cache.GenerateSimpleKey(tt.operation, tt.provider, tt.additionalParams...)
			require.NotEmpty(t, key2)

			// Verify deterministic
			assert.Equal(t, key1, key2)

			// Verify SHA256 hash
			assert.Len(t, key1, 64)
		})
	}
}

// TestGenerateSimpleKey_OrderMatters verifies order matters in simple keys.
func TestGenerateSimpleKey_OrderMatters(t *testing.T) {
	// Different order of additional params
	key1 := cache.GenerateSimpleKey("recommendations", "aws", "param1", "param2")
	key2 := cache.GenerateSimpleKey("recommendations", "aws", "param2", "param1")

	// Order matters in simple keys
	assert.NotEqual(t, key1, key2)
}

// TestGenerateKeyFromQuery verifies query-based key generation.
func TestGenerateKeyFromQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "simple query",
			query: "SELECT id FROM costs WHERE date > '2024-01-01'",
		},
		{
			name:  "complex query",
			query: "SELECT provider, SUM(cost) FROM costs WHERE date BETWEEN '2024-01-01' AND '2024-01-31' GROUP BY provider ORDER BY cost DESC",
		},
		{
			name:  "json query",
			query: `{"operation":"projected_cost","provider":"aws","filters":{"region":"us-east-1"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key1 := cache.GenerateKeyFromQuery(tt.query)
			require.NotEmpty(t, key1)

			// Generate again with same query
			key2 := cache.GenerateKeyFromQuery(tt.query)
			require.NotEmpty(t, key2)

			// Verify deterministic
			assert.Equal(t, key1, key2)

			// Verify SHA256 hash
			assert.Len(t, key1, 64)
		})
	}
}

// TestGenerateKeyFromQuery_SensitiveToWhitespace verifies whitespace sensitivity.
func TestGenerateKeyFromQuery_SensitiveToWhitespace(t *testing.T) {
	query1 := "SELECT id FROM costs WHERE date > '2024-01-01'"
	query2 := "SELECT id FROM costs  WHERE date > '2024-01-01'" // Extra space

	key1 := cache.GenerateKeyFromQuery(query1)
	key2 := cache.GenerateKeyFromQuery(query2)

	// Different whitespace produces different keys (no normalization)
	assert.NotEqual(t, key1, key2)
}

// TestKeyParamsBuilder verifies fluent builder interface.
func TestKeyParamsBuilder(t *testing.T) {
	key, err := cache.NewKeyParamsBuilder("recommendations", "aws").
		WithResourceTypes("ec2", "rds", "s3").
		WithFilter("region", "us-east-1").
		WithFilter("env", "prod").
		WithPagination(20, 0, "savings", "desc").
		Build()

	require.NoError(t, err)
	require.NotEmpty(t, key)
	assert.Len(t, key, 64)
}

// TestKeyParamsBuilder_BuildParams verifies BuildParams method.
func TestKeyParamsBuilder_BuildParams(t *testing.T) {
	builder := cache.NewKeyParamsBuilder("projected_cost", "gcp").
		WithResourceTypes("compute", "storage").
		WithFilters(map[string]string{
			"zone":    "us-central1-a",
			"project": "my-project",
		})

	params := builder.BuildParams()

	assert.Equal(t, "projected_cost", params.Operation)
	assert.Equal(t, "gcp", params.Provider)
	assert.ElementsMatch(t, []string{"compute", "storage"}, params.ResourceTypes)
	assert.Equal(t, "us-central1-a", params.Filters["zone"])
	assert.Equal(t, "my-project", params.Filters["project"])
}

// TestKeyParamsBuilder_Chaining verifies method chaining.
func TestKeyParamsBuilder_Chaining(t *testing.T) {
	builder := cache.NewKeyParamsBuilder("actual_cost", "azure")

	// All methods should return the builder for chaining
	result1 := builder.WithResourceTypes("vm")
	assert.Equal(t, builder, result1)

	result2 := builder.WithFilter("location", "eastus")
	assert.Equal(t, builder, result2)

	result3 := builder.WithFilters(map[string]string{"tier": "premium"})
	assert.Equal(t, builder, result3)

	result4 := builder.WithPagination(50, 0, "cost", "asc")
	assert.Equal(t, builder, result4)
}

// TestKeyParamsBuilder_EmptyFilters verifies empty filters handling.
func TestKeyParamsBuilder_EmptyFilters(t *testing.T) {
	key, err := cache.NewKeyParamsBuilder("projected_cost", "aws").
		Build()

	require.NoError(t, err)
	require.NotEmpty(t, key)

	// Should succeed even without filters
	assert.Len(t, key, 64)
}

// TestKeyParamsBuilder_WithFiltersMultipleCalls verifies multiple WithFilters calls.
func TestKeyParamsBuilder_WithFiltersMultipleCalls(t *testing.T) {
	builder := cache.NewKeyParamsBuilder("recommendations", "aws").
		WithFilters(map[string]string{"region": "us-east-1"}).
		WithFilters(map[string]string{"env": "prod"})

	params := builder.BuildParams()

	// Both filters should be present
	assert.Equal(t, "us-east-1", params.Filters["region"])
	assert.Equal(t, "prod", params.Filters["env"])
	assert.Len(t, params.Filters, 2)
}

// TestKeyCollision verifies different inputs don't produce same key.
func TestKeyCollision(t *testing.T) {
	// Generate 100 unique keys with different parameters
	keys := make(map[string]bool)

	for i := range 100 {
		params := cache.KeyParams{
			Operation: "test_operation",
			Provider:  "test_provider",
			ResourceTypes: []string{
				string(rune('a' + i/10)),
				string(rune('a' + i%10)),
			},
			Filters: map[string]string{
				"iteration": string(rune('0' + i/10)),
			},
		}

		key, err := cache.GenerateKey(params)
		require.NoError(t, err)

		// Check for collision
		if keys[key] {
			t.Fatalf("Key collision detected for params %+v", params)
		}
		keys[key] = true
	}

	// All keys should be unique
	assert.Len(t, keys, 100)
}

// TestPaginationKeyParams verifies pagination parameter handling.
func TestPaginationKeyParams(t *testing.T) {
	params1 := cache.KeyParams{
		Operation: "recommendations",
		Provider:  "aws",
		Pagination: &cache.PaginationKeyParams{
			Limit:     20,
			Offset:    0,
			SortField: "savings",
			SortOrder: "desc",
		},
	}

	params2 := cache.KeyParams{
		Operation: "recommendations",
		Provider:  "aws",
		Pagination: &cache.PaginationKeyParams{
			Limit:     20,
			Offset:    20, // Different offset
			SortField: "savings",
			SortOrder: "desc",
		},
	}

	key1, err := cache.GenerateKey(params1)
	require.NoError(t, err)

	key2, err := cache.GenerateKey(params2)
	require.NoError(t, err)

	// Different pagination params should produce different keys
	assert.NotEqual(t, key1, key2)
}

// TestSortOrderNormalization verifies sort order normalization.
func TestSortOrderNormalization(t *testing.T) {
	params1 := cache.KeyParams{
		Operation: "recommendations",
		Provider:  "aws",
		Pagination: &cache.PaginationKeyParams{
			SortOrder: "DESC",
		},
	}

	params2 := cache.KeyParams{
		Operation: "recommendations",
		Provider:  "aws",
		Pagination: &cache.PaginationKeyParams{
			SortOrder: "desc",
		},
	}

	key1, err := cache.GenerateKey(params1)
	require.NoError(t, err)

	key2, err := cache.GenerateKey(params2)
	require.NoError(t, err)

	// Sort order should be normalized (case-insensitive)
	assert.Equal(t, key1, key2)
}

// BenchmarkGenerateKey benchmarks key generation.
func BenchmarkGenerateKey(b *testing.B) {
	params := cache.KeyParams{
		Operation:     "recommendations",
		Provider:      "aws",
		ResourceTypes: []string{"ec2", "rds", "s3", "elasticache"},
		Filters: map[string]string{
			"region": "us-west-2",
			"env":    "prod",
			"team":   "platform",
		},
		Pagination: &cache.PaginationKeyParams{
			Limit:     50,
			Offset:    100,
			SortField: "cost",
			SortOrder: "asc",
		},
	}

	b.ResetTimer()
	for range b.N {
		_, _ = cache.GenerateKey(params)
	}
}

// BenchmarkGenerateSimpleKey benchmarks simple key generation.
func BenchmarkGenerateSimpleKey(b *testing.B) {
	b.ResetTimer()
	for range b.N {
		_ = cache.GenerateSimpleKey("recommendations", "aws", "us-east-1", "prod", "platform")
	}
}

// BenchmarkGenerateKeyFromQuery benchmarks query-based key generation.
func BenchmarkGenerateKeyFromQuery(b *testing.B) {
	query := `SELECT provider, SUM(cost) FROM costs WHERE date BETWEEN '2024-01-01' AND '2024-01-31' GROUP BY provider ORDER BY cost DESC`

	b.ResetTimer()
	for range b.N {
		_ = cache.GenerateKeyFromQuery(query)
	}
}
