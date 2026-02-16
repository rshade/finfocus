package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Bucket names for the BoltDB cache.
const (
	BucketProjected       = "projected"
	BucketActual          = "actual"
	BucketRecommendations = "recommendations"
)

// isValidBucket reports whether the given name is a recognized top-level bucket.
func isValidBucket(name string) bool {
	switch name {
	case BucketProjected, BucketActual, BucketRecommendations:
		return true
	default:
		return false
	}
}

// BuildProjectedKey constructs a human-readable key for per-resource
// projected cost caching.
// Format: projected/{provider}/{type}/{region}/{sku}.
// Empty segments use "_" as a placeholder to ensure fixed-position keys
// and avoid ambiguity (e.g., ("aws","","us-east-1","") vs ("aws","us-east-1","","")).
func BuildProjectedKey(provider, resourceType, region, sku string) string {
	placeholder := func(s string) string {
		if s == "" {
			return "_"
		}
		return s
	}
	return strings.Join([]string{
		BucketProjected,
		placeholder(provider),
		placeholder(resourceType),
		placeholder(region),
		placeholder(sku),
	}, "/")
}

// BuildActualKey constructs a key for whole-query actual cost caching.
// Format: actual/{provider}/{types}/{from}/{to}/{filter-hash}
// Empty segments use "_" as a placeholder to ensure fixed-position keys
// and avoid ambiguity (e.g., provider="aws" vs resourceTypes=["aws"]).
// The filter-hash is a deterministic SHA256 prefix of sorted filter key-value pairs.
func BuildActualKey(provider string, resourceTypes []string, from, to time.Time, filters map[string]string) string {
	placeholder := func(s string) string {
		if s == "" {
			return "_"
		}
		return s
	}

	// Sort resource types for determinism
	sorted := make([]string, len(resourceTypes))
	copy(sorted, resourceTypes)
	sort.Strings(sorted)

	typesSegment := "_"
	if len(sorted) > 0 {
		typesSegment = strings.Join(sorted, "+")
	}

	filterSegment := "_"
	if len(filters) > 0 {
		filterSegment = hashFilters(filters)
	}

	return strings.Join([]string{
		BucketActual,
		placeholder(provider),
		typesSegment,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"),
		filterSegment,
	}, "/")
}

// BuildRecommendationsKey constructs a key for recommendation result caching.
// Format: recommendations/multi/{sorted-types-hash}.
func BuildRecommendationsKey(resourceTypes []string) string {
	sorted := make([]string, len(resourceTypes))
	copy(sorted, resourceTypes)
	sort.Strings(sorted)

	combined := strings.Join(sorted, "+")
	return fmt.Sprintf("%s/multi/%s", BucketRecommendations, combined)
}

// BucketFromKey extracts the bucket name from a structured cache key.
// Returns the first path segment before the first "/".
// Returns the entire key if there is no "/" separator.
func BucketFromKey(key string) string {
	idx := strings.Index(key, "/")
	if idx < 0 {
		return key
	}
	return key[:idx]
}

// StripBucket removes the bucket prefix from a key, returning the portion
// after the first "/". If the key has no "/" separator, returns the key as-is.
func StripBucket(key string) string {
	idx := strings.Index(key, "/")
	if idx < 0 {
		return key
	}
	return key[idx+1:]
}

// hashFilters produces a short deterministic hash string from a map of filters.
func hashFilters(filters map[string]string) string {
	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(filters[k])
		sb.WriteString(";")
	}

	h := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(h[:8]) // 16 hex chars
}
