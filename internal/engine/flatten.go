// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/proto"
)

const (
	// maxFlattenDepth is the maximum number of path segments in a dotted key.
	// A scalar at this depth is kept. A deeper leaf is not emitted.
	maxFlattenDepth = 6
	// maxFlattenedTags is the conformance MaxTagCount. Collapsed keys are kept
	// even when they already exceed it. Dotted keys fill only the remaining room.
	maxFlattenedTags = 50
	// refTagNamespace is the prefix of the ref.<property>.* tags that cross-
	// resource resolution writes. A user input named "ref" must not flatten into
	// that namespace, where it would read as a resolved reference.
	refTagNamespace   = "ref"
	maxDottedKeyLen   = 128
	maxDottedValueLen = 256
)

type dottedTag struct {
	key   string
	value string
	depth int
}

// addDottedTags appends scalar leaves of nested maps and arrays. Existing keys
// are left unchanged, including when a dotted path would reuse one of them.
func addDottedTags(result map[string]string, properties map[string]any) {
	if len(properties) == 0 {
		return
	}
	candidates := collectDottedTags(properties)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].depth != candidates[j].depth {
			return candidates[i].depth < candidates[j].depth
		}
		return candidates[i].key < candidates[j].key
	})

	kept, dropped := 0, 0
	for _, candidate := range candidates {
		if _, exists := result[candidate.key]; exists {
			continue
		}
		if len(result) >= maxFlattenedTags {
			dropped++
			continue
		}
		result[candidate.key] = candidate.value
		kept++
	}
	if dropped == 0 {
		return
	}
	logging.FromContext(context.Background()).Debug().
		Int("dotted_kept", kept).
		Int("dotted_dropped", dropped).
		Msg("flattened resource tags truncated")
}

func collectDottedTags(properties map[string]any) []dottedTag {
	var out []dottedTag
	for _, key := range sortedPropertyKeys(properties) {
		if key == refTagNamespace {
			continue
		}
		walkDotted(properties[key], []string{key}, &out)
	}
	return out
}

func walkDotted(value any, path []string, out *[]dottedTag) {
	if skipDottedSegment(path[len(path)-1]) || history.IsPulumiSecret(value) {
		return
	}
	switch typed := value.(type) {
	case map[string]any:
		walkDottedMap(typed, path, out)
	case []any:
		walkDottedSlice(typed, path, out)
	default:
		appendDottedLeaf(value, path, out)
	}
}

func walkDottedMap(value map[string]any, path []string, out *[]dottedTag) {
	if len(value) == 0 || skipFlattenContainer(path[len(path)-1]) || len(path) >= maxFlattenDepth {
		return
	}
	for _, key := range sortedPropertyKeys(value) {
		next := append(append([]string{}, path...), key)
		walkDotted(value[key], next, out)
	}
}

func walkDottedSlice(value []any, path []string, out *[]dottedTag) {
	if len(value) == 0 || skipFlattenContainer(path[len(path)-1]) || len(path) >= maxFlattenDepth {
		return
	}
	for i, elem := range value {
		next := append(append([]string{}, path...), strconv.Itoa(i))
		walkDotted(elem, next, out)
	}
}

func appendDottedLeaf(value any, path []string, out *[]dottedTag) {
	if value == nil || len(path) < 2 || len(path) > maxFlattenDepth {
		return
	}
	text := ConvertValueToString(value)
	if text == proto.UnknownPulumiValue {
		return
	}
	key := strings.Join(path, ".")
	if len(key) > maxDottedKeyLen || len(text) > maxDottedValueLen {
		return
	}
	*out = append(*out, dottedTag{key: key, value: text, depth: len(path)})
}

func skipDottedSegment(segment string) bool {
	return strings.HasPrefix(segment, "__") || isCredentialKey(segment)
}

// isCredentialKey reports whether a property name looks like it holds a
// credential. It is a substring match, not a configurable list, and it is the
// one rule for tags, collapsed values, attributes, and EstimateCost.
func isCredentialKey(key string) bool {
	lower := strings.ToLower(key)
	for _, fragment := range []string{
		"password", "passwd", "passphrase", "secret", "token", "credential", "ciphertext",
		"privatekey", "private_key", "apikey", "api_key", "accesskey", "access_key",
		"connectionstring", "connection_string",
	} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func skipFlattenContainer(segment string) bool {
	switch segment {
	case "tags", "tagsAll", "labels", "annotations":
		return true
	default:
		return false
	}
}

func sortedPropertyKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// tagCacheSuffix hashes the flattened tag map so projected cache keys change
// when any tag changes, including dotted leaves. Each key and value is
// length-prefixed, matching cache.writeField, so "=" or "|" inside a value
// cannot alias a different map. The prefix is always appended.
func tagCacheSuffix(tags map[string]string) string {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var encoded strings.Builder
	for _, key := range keys {
		writeTagField(&encoded, key)
		writeTagField(&encoded, tags[key])
	}
	sum := sha256.Sum256([]byte(encoded.String()))
	return hex.EncodeToString(sum[:8])
}

func writeTagField(b *strings.Builder, value string) {
	b.WriteString(strconv.Itoa(len(value)))
	b.WriteByte(':')
	b.WriteString(value)
}
