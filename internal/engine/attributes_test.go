// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	protobuf "google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/internal/proto"
)

const testPulumiSecretSig = "4dabf18193072939515e22adb298388d"

func TestBuildAttributes(t *testing.T) {
	t.Parallel()

	secret := map[string]any{testPulumiSecretSig: "1", "ciphertext": "AAAA"}
	tests := []struct {
		name  string
		props map[string]any
		want  map[string]any
	}{
		{
			name:  "nil input",
			props: nil,
			want:  nil,
		},
		{
			name:  "empty input",
			props: map[string]any{},
			want:  nil,
		},
		{
			name:  "everything redacted",
			props: map[string]any{"password": "p", "__defaults": []any{}},
			want:  nil,
		},
		{
			name: "nested maps and lists kept",
			props: map[string]any{
				"spec": map[string]any{
					"replicas": float64(3),
					"template": map[string]any{"spec": map[string]any{
						"containers": []any{
							map[string]any{"name": "app", "resources": map[string]any{
								"requests": map[string]any{"cpu": "500m", "memory": "1Gi"},
							}},
						},
					}},
				},
			},
			want: map[string]any{
				"spec": map[string]any{
					"replicas": float64(3),
					"template": map[string]any{"spec": map[string]any{
						"containers": []any{
							map[string]any{"name": "app", "resources": map[string]any{
								"requests": map[string]any{"cpu": "500m", "memory": "1Gi"},
							}},
						},
					}},
				},
			},
		},
		{
			name: "credential-like keys dropped at any depth",
			props: map[string]any{
				"password":   "p",
				"dbPassword": "p",
				"api_key":    "k",
				"env": []any{map[string]any{
					"name":      "DB",
					"secretRef": map[string]any{"token": "t"},
					"apiKey":    "k",
				}},
				"instanceType": "m5.large",
			},
			want: map[string]any{
				"env":          []any{map[string]any{"name": "DB"}},
				"instanceType": "m5.large",
			},
		},
		{
			name: "double underscore keys dropped at any depth",
			props: map[string]any{
				"__defaults": []any{"a"},
				"__provider": "urn:provider",
				"spec":       map[string]any{"__inner": "x", "kept": "y"},
			},
			want: map[string]any{"spec": map[string]any{"kept": "y"}},
		},
		{
			name: "pulumi secret values dropped at any depth",
			props: map[string]any{
				"userData": secret,
				"spec":     map[string]any{"value": secret, "kept": "y"},
				"list":     []any{secret, "z"},
			},
			want: map[string]any{
				"spec": map[string]any{"kept": "y"},
				"list": []any{"z"},
			},
		},
		{
			name: "top-level ref keys dropped, nested ref kept",
			props: map[string]any{
				"ref":            "x",
				"ref.subnet.urn": "urn:subnet",
				"spec":           map[string]any{"ref": "kept"},
			},
			want: map[string]any{"spec": map[string]any{"ref": "kept"}},
		},
		{
			name: "tag and label containers kept",
			props: map[string]any{
				"tags":        map[string]any{"env": "prod"},
				"tagsAll":     map[string]any{"env": "prod"},
				"labels":      map[string]any{"app": "web"},
				"annotations": map[string]any{"note": "n"},
			},
			want: map[string]any{
				"tags":        map[string]any{"env": "prod"},
				"tagsAll":     map[string]any{"env": "prod"},
				"labels":      map[string]any{"app": "web"},
				"annotations": map[string]any{"note": "n"},
			},
		},
		{
			name: "pulumi unknown kept",
			props: map[string]any{
				"spec": map[string]any{"replicas": proto.UnknownPulumiValue},
			},
			want: map[string]any{
				"spec": map[string]any{"replicas": proto.UnknownPulumiValue},
			},
		},
		{
			name: "unconvertible values omitted",
			props: map[string]any{
				"bad":  make(chan int),
				"spec": map[string]any{"bad": make(chan int), "kept": "y"},
				"list": []any{make(chan int), "z"},
				"ok":   true,
			},
			want: map[string]any{
				"spec": map[string]any{"kept": "y"},
				"list": []any{"z"},
				"ok":   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := BuildAttributes(context.Background(), tt.props)
			if tt.want == nil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.AsMap())
		})
	}
}

func TestBuildAttributesReadableByPath(t *testing.T) {
	t.Parallel()

	got := BuildAttributes(context.Background(), map[string]any{
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
			"containers": []any{
				map[string]any{"resources": map[string]any{"requests": map[string]any{"cpu": "250m"}}},
				map[string]any{"resources": map[string]any{"requests": map[string]any{"cpu": "500m"}}},
			},
		}}},
	})

	value, ok := pluginsdk.AttributeValue(got, "spec.template.spec.containers.1.resources.requests.cpu")
	require.True(t, ok)
	assert.Equal(t, "500m", value.GetStringValue())
}

func TestBuildAttributesSizeCap(t *testing.T) {
	t.Parallel()

	t.Run("exactly at the cap is kept", func(t *testing.T) {
		t.Parallel()
		ctx, logs := ctxWithLogBuffer(zerolog.DebugLevel)
		props := propsOfEncodedSize(t, pluginsdk.MaxAttributesBytes)

		got := BuildAttributes(ctx, props)

		require.NotNil(t, got)
		assert.Equal(t, pluginsdk.MaxAttributesBytes, protobuf.Size(got))
		assert.NotContains(t, logs.String(), `"level":"warn"`)
	})

	t.Run("over the cap returns nil and warns", func(t *testing.T) {
		t.Parallel()
		ctx, logs := ctxWithLogBuffer(zerolog.DebugLevel)
		props := propsOfEncodedSize(t, pluginsdk.MaxAttributesBytes+1)

		got := BuildAttributes(ctx, props)

		assert.Nil(t, got)
		assert.Contains(t, logs.String(), `"level":"warn"`)
		assert.Contains(t, logs.String(), "attributes")
	})
}

// propsOfEncodedSize returns a one-key property map whose structpb encoding is
// exactly target bytes, so the cap boundary is tested on the wire size.
func propsOfEncodedSize(t *testing.T, target int) map[string]any {
	t.Helper()
	for n := target - 32; n <= target; n++ {
		props := map[string]any{"k": strings.Repeat("a", n)}
		encoded, err := structpb.NewStruct(props)
		require.NoError(t, err)
		if protobuf.Size(encoded) == target {
			return props
		}
	}
	require.FailNow(t, "no string length gives the target encoded size", "target %d", target)
	return nil
}

func TestAttrsCacheSuffix(t *testing.T) {
	t.Parallel()

	deep := func(leaf string) map[string]any {
		return map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{
			"d": map[string]any{"e": map[string]any{"f": map[string]any{"g": map[string]any{
				"h": leaf,
			}}}},
		}}}}
	}
	mustStruct := func(t *testing.T, m map[string]any) *structpb.Struct {
		t.Helper()
		s, err := structpb.NewStruct(m)
		require.NoError(t, err)
		return s
	}

	t.Run("nil is empty", func(t *testing.T) {
		t.Parallel()
		assert.Empty(t, attrsCacheSuffix(nil))
	})

	t.Run("deterministic across runs and map orders", func(t *testing.T) {
		t.Parallel()
		first := map[string]any{}
		second := map[string]any{}
		keys := []string{"zeta", "alpha", "mid", "beta", "omega", "gamma", "delta"}
		for i, key := range keys {
			first[key] = map[string]any{"i": float64(i), "k": key}
		}
		for i := len(keys) - 1; i >= 0; i-- {
			second[keys[i]] = map[string]any{"k": keys[i], "i": float64(i)}
		}

		want := attrsCacheSuffix(mustStruct(t, first))
		assert.Regexp(t, `^[0-9a-f]{16}$`, want)
		for range 20 {
			assert.Equal(t, want, attrsCacheSuffix(mustStruct(t, first)))
			assert.Equal(t, want, attrsCacheSuffix(mustStruct(t, second)))
		}
	})

	t.Run("differs when a value 8 levels deep differs", func(t *testing.T) {
		t.Parallel()
		assert.NotEqual(t,
			attrsCacheSuffix(mustStruct(t, deep("one"))),
			attrsCacheSuffix(mustStruct(t, deep("two"))),
		)
	})
}

func TestAttrsCacheSuffixUnencodableStruct(t *testing.T) {
	t.Parallel()

	// Invalid UTF-8 cannot be encoded as a proto3 string. BuildAttributes never
	// produces this, so such a struct gets no digest rather than a wrong one.
	invalid := &structpb.Struct{Fields: map[string]*structpb.Value{"k": structpb.NewStringValue("\xff")}}

	assert.Empty(t, attrsCacheSuffix(invalid))
}
