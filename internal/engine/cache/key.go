package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Bucket names for the BoltDB cache.
const (
	BucketProjected       = "projected"
	BucketActual          = "actual"
	BucketRecommendations = "recommendations"
	BucketResolveTypes    = "resolve_types"
	BucketScores          = "scores"
)

// isValidBucket reports whether the given name is a recognized top-level bucket.
func isValidBucket(name string) bool {
	switch name {
	case BucketProjected, BucketActual, BucketRecommendations, BucketResolveTypes, BucketScores:
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

// RecommendationInput carries the identity and plugin-visible properties of one
// resource requested for recommendations. It is the input to HashRecommendationInputs.
type RecommendationInput struct {
	ID         string
	Provider   string
	Type       string
	Properties map[string]string
}

// BuildRecommendationsKey constructs a cache key for recommendation results.
// The key has the format "recommendations/multi/{sorted-types-joined-by-+}/{inputs-hash}",
// where inputs-hash comes from HashRecommendationInputs. Empty segments use "_" as a
// placeholder (consistent with sibling builders). Entries written under the previous
// "recommendations/multi/{sorted-types}" format are never matched by this format and
// simply expire.
func BuildRecommendationsKey(resourceTypes []string, inputsHash string) string {
	sorted := make([]string, len(resourceTypes))
	copy(sorted, resourceTypes)
	sort.Strings(sorted)

	return strings.Join([]string{
		BucketRecommendations,
		"multi",
		placeholder(strings.Join(sorted, "+")),
		placeholder(inputsHash),
	}, "/")
}

// HashRecommendationInputs returns a stable hex digest of the requested resources
// (identity, provider, type and properties) together with the excluded recommendation
// IDs. Resource order and excluded ID order do not affect the result; duplicate excluded
// IDs are ignored. Every field is length-prefixed so adjacent values cannot collide.
// It returns the first 16 bytes of the SHA-256 digest as 32 lowercase hex characters.
func HashRecommendationInputs(resources []RecommendationInput, excludedIDs []string) string {
	records := make([]string, 0, len(resources))
	for _, r := range resources {
		var sb strings.Builder
		writeField(&sb, r.ID)
		writeField(&sb, r.Provider)
		writeField(&sb, r.Type)

		keys := make([]string, 0, len(r.Properties))
		for k := range r.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			writeField(&sb, k)
			writeField(&sb, r.Properties[k])
		}
		records = append(records, sb.String())
	}
	sort.Strings(records)

	excluded := make([]string, len(excludedIDs))
	copy(excluded, excludedIDs)
	sort.Strings(excluded)
	excluded = slices.Compact(excluded)

	h := sha256.New()
	writeHashField(h, "resources")
	writeHashField(h, strconv.Itoa(len(records)))
	for _, rec := range records {
		writeHashField(h, rec)
	}
	writeHashField(h, "excluded")
	writeHashField(h, strconv.Itoa(len(excluded)))
	for _, id := range excluded {
		writeHashField(h, id)
	}
	return hex.EncodeToString(h.Sum(nil)[:16])
}

func writeField(sb *strings.Builder, v string) {
	sb.WriteString(strconv.Itoa(len(v)))
	sb.WriteByte(':')
	sb.WriteString(v)
}

func writeHashField(h hash.Hash, v string) {
	_, _ = io.WriteString(h, strconv.Itoa(len(v)))
	_, _ = io.WriteString(h, ":")
	_, _ = io.WriteString(h, v)
}

// BucketFromKey returns the leading bucket name from a cache key by taking the substring
// before the first '/'. If the key contains no '/', the entire key is returned.
func BucketFromKey(key string) string {
	before, _, ok := strings.Cut(key, "/")
	if !ok {
		return key
	}
	return before
}

// StripBucket returns the portion of key after the first "/" separator.
// If key contains no "/", the original key is returned unchanged.
func StripBucket(key string) string {
	_, after, ok := strings.Cut(key, "/")
	if !ok {
		return key
	}
	return after
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

// BuildResolveTypesKey constructs a cache key for IaC type-resolution results.
// The key has the form "resolve_types/{format}/{source_type}".
// Empty segments use "_" as a placeholder (consistent with sibling builders).
func BuildResolveTypesKey(sourceFormat, sourceType string) string {
	return strings.Join([]string{
		BucketResolveTypes,
		placeholder(sourceFormat),
		placeholder(sourceType),
	}, "/")
}

// segment makes a value safe to use as one path segment of a cache key.
func segment(s string) string {
	return placeholder(strings.ReplaceAll(s, "/", "_"))
}

// BuildScoreKey constructs the cache key for one recommendation's scores.
// The key has the form "scores/{scorer}/{plugin-version}/{model}/{content-hash}".
// Changing the scorer, its plugin version or its model therefore never reuses an entry.
func BuildScoreKey(scorer, pluginVersion, model, contentHash string) string {
	return strings.Join([]string{
		BucketScores, segment(scorer), segment(pluginVersion), segment(model), segment(contentHash),
	}, "/")
}

// BuildScoreModelKey constructs the key holding the model a scorer reported last,
// which lets a later run build score keys before it calls the scorer.
// The key has the form "scores/{scorer}/{plugin-version}/_model".
func BuildScoreModelKey(scorer, pluginVersion string) string {
	return strings.Join([]string{BucketScores, segment(scorer), segment(pluginVersion), "_model"}, "/")
}
