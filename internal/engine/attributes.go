// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode/utf8"

	protobuf "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/logging"
)

// BuildAttributes converts a resource's declared properties into the
// ResourceDescriptor.attributes finfocus-spec v0.7.3 defines. Hosts must
// redact before sending, so at any depth it drops keys that look like
// credentials or start with "__" (the dotted-tag rule, skipDottedSegment),
// Pulumi secret values, and values structpb cannot represent. At the top level
// it drops "ref" and "ref.*", which core writes for cross-resource references.
// Pulumi unknown values and tag or label maps are kept. It returns nil when
// nothing survives or when the encoding exceeds pluginsdk.MaxAttributesBytes.
func BuildAttributes(ctx context.Context, props map[string]any) *structpb.Struct {
	attrs, oversize := sendableAttributes(ctx, props)
	if oversize > 0 {
		logging.FromContext(ctx).Warn().
			Ctx(ctx).
			Str("component", "engine").
			Int("attributes_bytes", oversize).
			Int("limit_bytes", pluginsdk.MaxAttributesBytes).
			Msg("resource attributes exceed the size limit and are not sent")
	}
	return attrs
}

// sendableAttributes is BuildAttributes without the warning, for cache keys,
// which are computed more often than requests are sent. When the attributes
// are over the limit it returns nil and their encoded size.
func sendableAttributes(ctx context.Context, props map[string]any) (*structpb.Struct, int) {
	attrs := attributeStruct(ctx, props, true)
	if len(attrs.GetFields()) == 0 {
		return nil, 0
	}
	if size := protobuf.Size(attrs); size > pluginsdk.MaxAttributesBytes {
		return nil, size
	}
	return attrs, 0
}

// attributesCacheSuffix digests the attributes a request for props would send.
func attributesCacheSuffix(props map[string]any) string {
	attrs, _ := sendableAttributes(context.Background(), props)
	return attrsCacheSuffix(attrs)
}

// attrsCacheSuffix hashes the deterministic protobuf encoding of attrs and
// returns the first 8 bytes in hex, the width of tagCacheSuffix. It is empty
// for nil attributes. Deterministic marshaling sorts map keys, so Go map
// iteration order cannot change the digest.
func attrsCacheSuffix(attrs *structpb.Struct) string {
	if attrs == nil {
		return ""
	}
	encoded, err := protobuf.MarshalOptions{Deterministic: true}.Marshal(attrs)
	if err != nil {
		// attributeStruct only admits valid UTF-8 keys and structpb-checked
		// values, so this is a struct from elsewhere; it gets no digest.
		return ""
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:8])
}

func attributeStruct(ctx context.Context, values map[string]any, top bool) *structpb.Struct {
	fields := make(map[string]*structpb.Value, len(values))
	for key, value := range values {
		if !utf8.ValidString(key) || skipDottedSegment(key) {
			continue
		}
		if top && (key == "ref" || strings.HasPrefix(key, "ref.")) {
			continue
		}
		if converted, ok := attributeValue(ctx, key, value); ok {
			fields[key] = converted
		}
	}
	return &structpb.Struct{Fields: fields}
}

func attributeValue(ctx context.Context, key string, value any) (*structpb.Value, bool) {
	if history.IsPulumiSecret(value) {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		return structpb.NewStructValue(attributeStruct(ctx, typed, false)), true
	case []any:
		items := make([]*structpb.Value, 0, len(typed))
		for _, item := range typed {
			if converted, ok := attributeValue(ctx, key, item); ok {
				items = append(items, converted)
			}
		}
		return structpb.NewListValue(&structpb.ListValue{Values: items}), true
	default:
		converted, err := structpb.NewValue(value)
		if err != nil {
			logging.FromContext(ctx).Debug().
				Ctx(ctx).
				Str("component", "engine").
				Str("attribute", key).
				Err(err).
				Msg("attribute value cannot be represented, omitted")
			return nil, false
		}
		return converted, true
	}
}
