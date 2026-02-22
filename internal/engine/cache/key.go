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

// placeholder returns "_" for empty strings to preserve fixed segment positions in cache keys.
func placeholder(s string) string {
	if s == "" {
		return "_"
	}
	return s
}

// BuildProjectedKey constructs a human-readable cache key for per-resource projected costs.
// The key has the form "projected/{provider}/{type}/{region}/{sku}".
// Any empty segment is replaced with "_" to preserve fixed segment positions.
func BuildProjectedKey(provider, resourceType, region, sku string) string {
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
// The resulting key is safe for use as a top-level bucketed cache key.
func BuildActualKey(provider string, resourceTypes []string, from, to time.Time, filters map[string]string) string {
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

// BuildRecommendationsKey constructs a cache key for recommendation results.
// The key has the format "recommendations/multi/{sorted-types-joined-by-+}".
// When resourceTypes is empty, the types segment uses "_" as a placeholder
// (consistent with sibling builders) to avoid a trailing slash.
func BuildRecommendationsKey(resourceTypes []string) string {
	sorted := make([]string, len(resourceTypes))
	copy(sorted, resourceTypes)
	sort.Strings(sorted)

	combined := strings.Join(sorted, "+")
	if combined == "" {
		combined = "_"
	}
	return fmt.Sprintf("%s/multi/%s", BucketRecommendations, combined)
}

// BucketFromKey returns the leading bucket name from a cache key by taking the substring
// before the first '/'. If the key contains no '/', the entire key is returned.
func BucketFromKey(key string) string {
	idx := strings.Index(key, "/")
	if idx < 0 {
		return key
	}
	return key[:idx]
}

// StripBucket returns the portion of key after the first "/" separator.
// If key contains no "/", the original key is returned unchanged.
func StripBucket(key string) string {
	idx := strings.Index(key, "/")
	if idx < 0 {
		return key
	}
	return key[idx+1:]
}

// hashFilters produces a short deterministic hex string representing the given filters.
// The filters are canonicalized by sorting keys and concatenating "key=value;" pairs.
// It returns the first 8 bytes of the SHA-256 digest encoded as 16 lowercase hex characters.
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
