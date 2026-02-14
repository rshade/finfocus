package router

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestCompilePattern_Glob(t *testing.T) {
	pattern := config.ResourcePattern{
		Type:    "glob",
		Pattern: "aws:ec2:*",
	}

	compiled, err := CompilePattern(pattern)
	require.NoError(t, err)
	require.NotNil(t, compiled)
	assert.Nil(t, compiled.Regex, "glob patterns should not have compiled regex")
	assert.Equal(t, pattern, compiled.Original)
}

func TestCompilePattern_Regex(t *testing.T) {
	pattern := config.ResourcePattern{
		Type:    "regex",
		Pattern: "aws:(ec2|rds)/.*",
	}

	compiled, err := CompilePattern(pattern)
	require.NoError(t, err)
	require.NotNil(t, compiled)
	assert.NotNil(t, compiled.Regex, "regex patterns should have compiled regex")
	assert.Equal(t, pattern, compiled.Original)
}

func TestCompilePattern_InvalidRegex(t *testing.T) {
	pattern := config.ResourcePattern{
		Type:    "regex",
		Pattern: "aws:(ec2|rds", // Missing closing paren
	}

	compiled, err := CompilePattern(pattern)
	require.Error(t, err)
	assert.Nil(t, compiled)
	assert.Contains(t, err.Error(), "invalid regex")
}

func TestCompiledPattern_Match_Glob(t *testing.T) {
	tests := []struct {
		name         string
		pattern      string
		resourceType string
		wantMatch    bool
	}{
		{
			name:         "exact match",
			pattern:      "aws:ec2:Instance",
			resourceType: "aws:ec2:Instance",
			wantMatch:    true,
		},
		{
			name:         "wildcard matches any",
			pattern:      "aws:ec2:*",
			resourceType: "aws:ec2:Instance",
			wantMatch:    true,
		},
		{
			name:         "wildcard at end",
			pattern:      "aws:*",
			resourceType: "aws:ec2:Instance",
			wantMatch:    true,
		},
		{
			name:         "wildcard crosses slash",
			pattern:      "aws:*",
			resourceType: "aws:ec2/instance:Instance",
			wantMatch:    true,
		},
		{
			name:         "question mark single char",
			pattern:      "aws:ec?:Instance",
			resourceType: "aws:ec2:Instance",
			wantMatch:    true,
		},
		{
			name:         "no match different provider",
			pattern:      "aws:ec2:*",
			resourceType: "gcp:compute:Instance",
			wantMatch:    false,
		},
		{
			name:         "no cross-provider slash match",
			pattern:      "aws:ec2/*",
			resourceType: "aws:s3/bucket:Bucket",
			wantMatch:    false,
		},
		{
			name:         "character class",
			pattern:      "aws:ec[23]:Instance",
			resourceType: "aws:ec2:Instance",
			wantMatch:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := config.ResourcePattern{Type: "glob", Pattern: tt.pattern}
			compiled, err := CompilePattern(pattern)
			require.NoError(t, err)

			matched, err := compiled.Match(tt.resourceType)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMatch, matched)
		})
	}
}

func TestCompiledPattern_Match_Regex(t *testing.T) {
	tests := []struct {
		name         string
		pattern      string
		resourceType string
		wantMatch    bool
	}{
		{
			name:         "exact match",
			pattern:      "aws:ec2:Instance",
			resourceType: "aws:ec2:Instance",
			wantMatch:    true,
		},
		{
			name:         "dot star matches all",
			pattern:      "aws:ec2:.*",
			resourceType: "aws:ec2:Instance",
			wantMatch:    true,
		},
		{
			name:         "alternation",
			pattern:      "aws:(ec2|rds):.*",
			resourceType: "aws:rds:Instance",
			wantMatch:    true,
		},
		{
			name:         "nested path with slash",
			pattern:      "aws:ec2/instance:.*",
			resourceType: "aws:ec2/instance:Instance",
			wantMatch:    true,
		},
		{
			name:         "partial match at start",
			pattern:      "^aws:",
			resourceType: "aws:ec2:Instance",
			wantMatch:    true,
		},
		{
			name:         "regex matches substring without anchor",
			pattern:      "gcp:",
			resourceType: "aws:gcp:Instance",
			wantMatch:    true, // partial match
		},
		{
			name:         "no match",
			pattern:      "^gcp:.*",
			resourceType: "aws:ec2:Instance",
			wantMatch:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := config.ResourcePattern{Type: "regex", Pattern: tt.pattern}
			compiled, err := CompilePattern(pattern)
			require.NoError(t, err)

			matched, err := compiled.Match(tt.resourceType)
			require.NoError(t, err)
			assert.Equal(t, tt.wantMatch, matched)
		})
	}
}

func TestPatternCache_MatchGlob(t *testing.T) {
	cache := NewPatternCache()

	matched, err := cache.MatchGlob("aws:*", "aws:ec2/instance:Instance")
	require.NoError(t, err)
	assert.True(t, matched)

	matched, err = cache.MatchGlob("aws:ec2:*", "aws:ec2:Instance")
	require.NoError(t, err)
	assert.True(t, matched)

	matched, err = cache.MatchGlob("aws:ec2:*", "gcp:compute:Instance")
	require.NoError(t, err)
	assert.False(t, matched)
}

func TestPatternCache_MatchRegex(t *testing.T) {
	cache := NewPatternCache()

	// First call compiles and caches
	matched, err := cache.MatchRegex("aws:(ec2|rds):.*", "aws:ec2:Instance")
	require.NoError(t, err)
	assert.True(t, matched)

	// Second call uses cache
	matched, err = cache.MatchRegex("aws:(ec2|rds):.*", "aws:rds:Instance")
	require.NoError(t, err)
	assert.True(t, matched)

	assert.Equal(t, 1, cache.Size(), "pattern should be cached")
}

func TestPatternCache_MatchRegex_InvalidPattern(t *testing.T) {
	cache := NewPatternCache()

	_, err := cache.MatchRegex("aws:(ec2|rds", "aws:ec2:Instance")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid regex")
}

func TestPatternCache_Match(t *testing.T) {
	cache := NewPatternCache()

	// Test glob
	globPattern := config.ResourcePattern{Type: "glob", Pattern: "aws:ec2:*"}
	matched, err := cache.Match(globPattern, "aws:ec2:Instance")
	require.NoError(t, err)
	assert.True(t, matched)

	// Test regex
	regexPattern := config.ResourcePattern{Type: "regex", Pattern: "aws:eks:.*"}
	matched, err = cache.Match(regexPattern, "aws:eks:Cluster")
	require.NoError(t, err)
	assert.True(t, matched)
}

func TestPatternCache_Clear(t *testing.T) {
	cache := NewPatternCache()

	// Populate cache
	_, _ = cache.MatchRegex("pattern1", "test")
	_, _ = cache.MatchRegex("pattern2", "test")
	assert.Equal(t, 2, cache.Size())

	// Clear
	cache.Clear()
	assert.Equal(t, 0, cache.Size())
}

func TestPatternCache_ConcurrentAccess(t *testing.T) {
	cache := NewPatternCache()
	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	errCh := make(chan error, goroutines*iterations)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				pattern := "aws:ec2:.*"
				if _, err := cache.MatchRegex(pattern, "aws:ec2:Instance"); err != nil {
					errCh <- err
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)

	// Collect and assert no errors occurred
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}
	require.Empty(t, errors, "concurrent cache access should not produce errors")

	// Should have exactly one cached pattern despite concurrent access
	assert.Equal(t, 1, cache.Size())
}

func BenchmarkCompiledPattern_Match_Glob(b *testing.B) {
	pattern := config.ResourcePattern{Type: "glob", Pattern: "aws:ec2:*"}
	compiled, _ := CompilePattern(pattern)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compiled.Match("aws:ec2:Instance")
	}
}

func BenchmarkCompiledPattern_Match_Regex(b *testing.B) {
	pattern := config.ResourcePattern{Type: "regex", Pattern: "aws:(ec2|rds)/.*:Instance"}
	compiled, _ := CompilePattern(pattern)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = compiled.Match("aws:ec2/instance:Instance")
	}
}

func BenchmarkPatternCache_MatchRegex_Cached(b *testing.B) {
	cache := NewPatternCache()
	// Pre-populate cache
	_, _ = cache.MatchRegex("aws:(ec2|rds)/.*:Instance", "test")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cache.MatchRegex("aws:(ec2|rds)/.*:Instance", "aws:ec2/instance:Instance")
	}
}
