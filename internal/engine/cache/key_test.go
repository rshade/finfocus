package cache_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine/cache"
)

// TestBuildProjectedKey verifies structured projected cost key generation.
func TestBuildProjectedKey(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
			key := cache.BuildProjectedKey(tt.provider, tt.resourceType, tt.region, tt.sku)
			assert.Equal(t, tt.want, key)
		})
	}
}

// TestBuildProjectedKey_Deterministic verifies same inputs produce same key.
func TestBuildProjectedKey_Deterministic(t *testing.T) {
	t.Parallel()

	key1 := cache.BuildProjectedKey("aws", "aws:ec2:Instance", "us-east-1", "t3.micro")
	key2 := cache.BuildProjectedKey("aws", "aws:ec2:Instance", "us-east-1", "t3.micro")
	assert.Equal(t, key1, key2)
}

// TestBuildProjectedKey_DifferentInputs verifies different inputs produce different keys.
func TestBuildProjectedKey_DifferentInputs(t *testing.T) {
	t.Parallel()

	key1 := cache.BuildProjectedKey("aws", "aws:ec2:Instance", "us-east-1", "t3.micro")
	key2 := cache.BuildProjectedKey("aws", "aws:ec2:Instance", "us-east-1", "t3.large")
	assert.NotEqual(t, key1, key2)
}

// TestBuildActualKey verifies structured actual cost key generation.
func TestBuildActualKey(t *testing.T) {
	t.Parallel()

	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	key := cache.BuildActualKey("aws", []string{"aws:ec2:Instance"}, from, to, nil)
	assert.Contains(t, key, "actual/aws/aws:ec2:Instance/2025-01-01/2025-01-31")
}

// TestBuildActualKey_ResourceTypeSorting verifies resource types are sorted for determinism.
func TestBuildActualKey_ResourceTypeSorting(t *testing.T) {
	t.Parallel()

	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	key1 := cache.BuildActualKey("aws", []string{"rds", "ec2", "s3"}, from, to, nil)
	key2 := cache.BuildActualKey("aws", []string{"ec2", "rds", "s3"}, from, to, nil)
	assert.Equal(t, key1, key2, "resource type order should not affect key")
}

// TestBuildActualKey_FiltersDeterministic verifies filter order independence.
func TestBuildActualKey_FiltersDeterministic(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	key1 := cache.BuildActualKey("aws", []string{"ec2"}, from, to, map[string]string{"env": "prod"})
	key2 := cache.BuildActualKey("aws", []string{"ec2"}, from, to, map[string]string{"env": "staging"})
	assert.NotEqual(t, key1, key2)
}

// TestBuildRecommendationsKey verifies recommendation key generation.
func TestBuildRecommendationsKey(t *testing.T) {
	t.Parallel()

	key := cache.BuildRecommendationsKey([]string{"ec2", "rds", "s3"}, "abc123")
	assert.Equal(t, "recommendations/multi/ec2+rds+s3/abc123", key)
}

// TestBuildRecommendationsKey_EmptyPlaceholders verifies fixed segment positions.
func TestBuildRecommendationsKey_EmptyPlaceholders(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "recommendations/multi/_/_", cache.BuildRecommendationsKey(nil, ""))
}

// TestBuildRecommendationsKey_Sorting verifies resource type sorting.
func TestBuildRecommendationsKey_Sorting(t *testing.T) {
	t.Parallel()

	key1 := cache.BuildRecommendationsKey([]string{"s3", "ec2", "rds"}, "h")
	key2 := cache.BuildRecommendationsKey([]string{"ec2", "rds", "s3"}, "h")
	assert.Equal(t, key1, key2, "resource type order should not affect key")
}

// TestBuildRecommendationsKey_HashDistinguishes verifies the input hash is part of the key.
func TestBuildRecommendationsKey_HashDistinguishes(t *testing.T) {
	t.Parallel()

	key1 := cache.BuildRecommendationsKey([]string{"ec2"}, "h1")
	key2 := cache.BuildRecommendationsKey([]string{"ec2"}, "h2")
	assert.NotEqual(t, key1, key2)
}

//nolint:paralleltest // subtests share the parent-scoped fixture base (composite value mutated by a subtest)
func TestHashRecommendationInputs(t *testing.T) {
	base := []cache.RecommendationInput{
		{
			ID:         "i-1",
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			Properties: map[string]string{"instanceType": "t3.micro"},
		},
		{
			ID:         "i-2",
			Provider:   "aws",
			Type:       "aws:ec2/instance:Instance",
			Properties: map[string]string{"instanceType": "t3.large"},
		},
	}

	t.Run("deterministic and order independent", func(t *testing.T) {
		reordered := []cache.RecommendationInput{base[1], base[0]}
		hash := cache.HashRecommendationInputs(base, nil)
		assert.Equal(t, hash, cache.HashRecommendationInputs(reordered, nil))
		assert.Len(t, hash, 32)
	})

	t.Run("resource identity changes hash", func(t *testing.T) {
		other := []cache.RecommendationInput{
			{ID: "i-3", Provider: "aws", Type: base[0].Type, Properties: base[0].Properties},
			{ID: "i-4", Provider: "aws", Type: base[1].Type, Properties: base[1].Properties},
		}
		assert.NotEqual(t, cache.HashRecommendationInputs(base, nil), cache.HashRecommendationInputs(other, nil))
	})

	t.Run("property change changes hash", func(t *testing.T) {
		changed := []cache.RecommendationInput{base[0], base[1]}
		changed[1].Properties = map[string]string{"instanceType": "t3.xlarge"}
		assert.NotEqual(t, cache.HashRecommendationInputs(base, nil), cache.HashRecommendationInputs(changed, nil))
	})

	t.Run("provider change changes hash", func(t *testing.T) {
		changed := []cache.RecommendationInput{base[0]}
		changed[0].Provider = "azure"
		assert.NotEqual(t, cache.HashRecommendationInputs(base[:1], nil), cache.HashRecommendationInputs(changed, nil))
	})

	t.Run("excluded ids change hash and are order independent", func(t *testing.T) {
		none := cache.HashRecommendationInputs(base, nil)
		one := cache.HashRecommendationInputs(base, []string{"rec-1"})
		ab := cache.HashRecommendationInputs(base, []string{"rec-a", "rec-b"})
		ba := cache.HashRecommendationInputs(base, []string{"rec-b", "rec-a", "rec-a"})
		assert.NotEqual(t, none, one)
		assert.Equal(t, ab, ba)
	})

	t.Run("field boundaries are unambiguous", func(t *testing.T) {
		a := []cache.RecommendationInput{{ID: "ab", Type: "c"}}
		b := []cache.RecommendationInput{{ID: "a", Type: "bc"}}
		assert.NotEqual(t, cache.HashRecommendationInputs(a, nil), cache.HashRecommendationInputs(b, nil))

		p1 := []cache.RecommendationInput{{ID: "x", Properties: map[string]string{"a": "b=c"}}}
		p2 := []cache.RecommendationInput{{ID: "x", Properties: map[string]string{"a=b": "c"}}}
		assert.NotEqual(t, cache.HashRecommendationInputs(p1, nil), cache.HashRecommendationInputs(p2, nil))
	})

	t.Run("empty inputs are stable", func(t *testing.T) {
		assert.Equal(
			t,
			cache.HashRecommendationInputs(nil, nil),
			cache.HashRecommendationInputs([]cache.RecommendationInput{}, []string{}),
		)
		assert.NotEmpty(t, cache.HashRecommendationInputs(nil, nil))
	})
}

// TestBuildScoreKeys verifies score cache key generation.
func TestBuildScoreKeys(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "scores/jev/1.2.0/jev-1.13/abc", cache.BuildScoreKey("jev", "1.2.0", "jev-1.13", "abc"))
	assert.Equal(t, "scores/jev/1.2.0/_model", cache.BuildScoreModelKey("jev", "1.2.0"))
	assert.Equal(t, "scores/_/_/_/_", cache.BuildScoreKey("", "", "", ""))

	key := cache.BuildScoreKey("a/b", "1", "m/1", "h")
	assert.Equal(t, "scores/a_b/1/m_1/h", key)
	assert.Equal(t, cache.BucketScores, cache.BucketFromKey(key))
}

// TestBucketFromKey verifies bucket extraction from structured keys.
func TestBucketFromKey(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
			assert.Equal(t, tt.want, cache.BucketFromKey(tt.key))
		})
	}
}

// TestStripBucket verifies bucket prefix removal.
func TestStripBucket(t *testing.T) {
	t.Parallel()

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
			t.Parallel()
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

func TestBuildResolveTypesKey(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "resolve_types/terraform/aws_instance",
		cache.BuildResolveTypesKey("terraform", "aws_instance"))
	assert.Equal(t, "resolve_types/cloudformation/AWS::EC2::Instance",
		cache.BuildResolveTypesKey("cloudformation", "AWS::EC2::Instance"))
}

func TestResolveTypesBucketRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := cache.NewBoltStore(context.Background(), t.TempDir(), true, 3600, 0)
	require.NoError(t, err)
	defer store.Close()

	key := cache.BuildResolveTypesKey("terraform", "aws_instance")
	require.NoError(t, store.SetWithTTL(key,
		json.RawMessage(`{"pulumi_token":"aws:ec2/instance:Instance","supported":true}`), cache.MaxTTLSeconds))

	entry, err := store.Get(key)
	require.NoError(t, err)
	assert.Equal(t, key, entry.Key)
	assert.Contains(t, string(entry.Data), "aws:ec2/instance:Instance")
}
