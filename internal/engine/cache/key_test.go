package cache_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/engine/cache"
)

// TestBuildProjectedKey verifies structured projected cost key generation.
func TestBuildProjectedKey(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		resourceType string
		region       string
		sku          string
		want         string
	}{
		{
			name:         "full key",
			provider:     "aws",
			resourceType: "aws:ec2:Instance",
			region:       "us-east-1",
			sku:          "t3.micro",
			want:         "projected/aws/aws:ec2:Instance/us-east-1/t3.micro",
		},
		{
			name:         "minimal key",
			provider:     "aws",
			resourceType: "aws:ec2:Instance",
			want:         "projected/aws/aws:ec2:Instance/_/_",
		},
		{
			name: "empty provider",
			want: "projected/_/_/_/_",
		},
		{
			name:     "provider only",
			provider: "gcp",
			want:     "projected/gcp/_/_/_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := cache.BuildProjectedKey(tt.provider, tt.resourceType, tt.region, tt.sku)
			assert.Equal(t, tt.want, key)
		})
	}
}

// TestBuildProjectedKey_Deterministic verifies same inputs produce same key.
func TestBuildProjectedKey_Deterministic(t *testing.T) {
	key1 := cache.BuildProjectedKey("aws", "aws:ec2:Instance", "us-east-1", "t3.micro")
	key2 := cache.BuildProjectedKey("aws", "aws:ec2:Instance", "us-east-1", "t3.micro")
	assert.Equal(t, key1, key2)
}

// TestBuildProjectedKey_DifferentInputs verifies different inputs produce different keys.
func TestBuildProjectedKey_DifferentInputs(t *testing.T) {
	key1 := cache.BuildProjectedKey("aws", "aws:ec2:Instance", "us-east-1", "t3.micro")
	key2 := cache.BuildProjectedKey("aws", "aws:ec2:Instance", "us-east-1", "t3.large")
	assert.NotEqual(t, key1, key2)
}

// TestBuildActualKey verifies structured actual cost key generation.
func TestBuildActualKey(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	key := cache.BuildActualKey("aws", []string{"aws:ec2:Instance"}, from, to, nil)
	assert.Contains(t, key, "actual/aws/aws:ec2:Instance/2025-01-01/2025-01-31")
}

// TestBuildActualKey_ResourceTypeSorting verifies resource types are sorted for determinism.
func TestBuildActualKey_ResourceTypeSorting(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	key1 := cache.BuildActualKey("aws", []string{"rds", "ec2", "s3"}, from, to, nil)
	key2 := cache.BuildActualKey("aws", []string{"ec2", "rds", "s3"}, from, to, nil)
	assert.Equal(t, key1, key2, "resource type order should not affect key")
}

// TestBuildActualKey_FiltersDeterministic verifies filter order independence.
func TestBuildActualKey_FiltersDeterministic(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	filters1 := map[string]string{"region": "us-east-1", "env": "prod"}
	filters2 := map[string]string{"env": "prod", "region": "us-east-1"}

	key1 := cache.BuildActualKey("aws", []string{"ec2"}, from, to, filters1)
	key2 := cache.BuildActualKey("aws", []string{"ec2"}, from, to, filters2)
	assert.Equal(t, key1, key2, "filter order should not affect key")
}

// TestBuildActualKey_PositionalAmbiguity verifies that empty provider with non-empty
// resource types produces a different key than non-empty provider with empty types.
func TestBuildActualKey_PositionalAmbiguity(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	// provider="aws", no resource types
	key1 := cache.BuildActualKey("aws", nil, from, to, nil)
	// no provider, resourceTypes=["aws"]
	key2 := cache.BuildActualKey("", []string{"aws"}, from, to, nil)

	assert.NotEqual(t, key1, key2, "empty provider + types vs provider + empty types must produce distinct keys")
}

// TestBuildActualKey_DifferentFiltersProduceDifferentKeys verifies filter sensitivity.
func TestBuildActualKey_DifferentFiltersProduceDifferentKeys(t *testing.T) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	key1 := cache.BuildActualKey("aws", []string{"ec2"}, from, to, map[string]string{"env": "prod"})
	key2 := cache.BuildActualKey("aws", []string{"ec2"}, from, to, map[string]string{"env": "staging"})
	assert.NotEqual(t, key1, key2)
}

// TestBuildRecommendationsKey verifies recommendation key generation.
func TestBuildRecommendationsKey(t *testing.T) {
	key := cache.BuildRecommendationsKey([]string{"ec2", "rds", "s3"})
	assert.Equal(t, "recommendations/multi/ec2+rds+s3", key)
}

// TestBuildRecommendationsKey_Sorting verifies resource type sorting.
func TestBuildRecommendationsKey_Sorting(t *testing.T) {
	key1 := cache.BuildRecommendationsKey([]string{"s3", "ec2", "rds"})
	key2 := cache.BuildRecommendationsKey([]string{"ec2", "rds", "s3"})
	assert.Equal(t, key1, key2, "resource type order should not affect key")
}

// TestBucketFromKey verifies bucket extraction from structured keys.
func TestBucketFromKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"projected/aws/ec2:Instance/us-east-1/t3.micro", "projected"},
		{"actual/aws/ec2/2025-01-01/2025-01-31", "actual"},
		{"recommendations/multi/ec2+rds", "recommendations"},
		{"nobucket", "nobucket"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.want, cache.BucketFromKey(tt.key))
		})
	}
}

// TestStripBucket verifies bucket prefix removal.
func TestStripBucket(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"projected/aws/ec2:Instance", "aws/ec2:Instance"},
		{"actual/aws/ec2", "aws/ec2"},
		{"nobucket", "nobucket"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			assert.Equal(t, tt.want, cache.StripBucket(tt.key))
		})
	}
}

// BenchmarkBuildProjectedKey benchmarks projected key generation.
func BenchmarkBuildProjectedKey(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		cache.BuildProjectedKey("aws", "aws:ec2:Instance", "us-east-1", "t3.micro")
	}
}

// BenchmarkBuildActualKey benchmarks actual cost key generation.
func BenchmarkBuildActualKey(b *testing.B) {
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)
	filters := map[string]string{"region": "us-west-2", "env": "prod"}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		cache.BuildActualKey("aws", []string{"ec2", "rds", "s3"}, from, to, filters)
	}
}
