// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package viewmodel

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/resourcetype"
)

const (
	provenanceSKUValue    = "sentinel-sku"
	provenanceRegionValue = "us-east-1"
	provenanceZoneValue   = "us-east-1a"
)

func TestInputMappingsProvenanceMatchesExtraction(t *testing.T) {
	t.Parallel()

	mappings := reviewedInputMappings()
	require.NotEmpty(t, mappings)

	for _, mapped := range mappings {
		for _, input := range mapped.Inputs {
			name := mapped.Provider + "/" + mapped.ResourceSegment + "/" + input.Key
			switch input.Provenance {
			case ProvenanceSKU:
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					sku, _ := proto.ResolveSKUAndRegion(
						context.Background(), mapped.Provider, "", map[string]string{input.Key: provenanceSKUValue},
					)

					assert.Equal(t, provenanceSKUValue, sku, "extraction does not read %q for a SKU", input.Key)
				})
			case ProvenanceRegion:
				t.Run(name, func(t *testing.T) {
					t.Parallel()

					value, want := provenanceRegionValue, provenanceRegionValue
					if input.Key == "availabilityZone" {
						value = provenanceZoneValue
					}

					_, region := proto.ResolveSKUAndRegion(
						context.Background(), mapped.Provider, "", map[string]string{input.Key: value},
					)

					assert.Equal(t, want, region, "extraction does not read %q for a region", input.Key)
				})
			case ProvenancePluginInput, ProvenanceUsage:
			}
		}
	}
}

func TestInputMappingsAreWellFormed(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, mapped := range reviewedInputMappings() {
		name := mapped.Provider + "/" + mapped.ResourceSegment

		assert.False(t, seen[name], "duplicate mapping %s", name)
		seen[name] = true
		assert.Equal(t, resourcetype.NormalizeProvider(mapped.Provider), mapped.Provider,
			"%s: provider must already be the normalized billing cloud", name)
		assert.Equal(t, mapped.ResourceSegment, strings.ToLower(mapped.ResourceSegment),
			"%s: segment must be lowercase", name)
		require.NotEmpty(t, mapped.Inputs, name)

		keys := map[string]bool{}
		for _, input := range mapped.Inputs {
			assert.False(t, keys[input.Key], "%s: duplicate key %q", name, input.Key)
			keys[input.Key] = true
			assert.NotEqual(t, ProvenanceUsage, input.Provenance, "%s: usage comes from discovery, not data", name)
			assert.NotEmpty(t, input.Provenance, "%s: %q has no provenance", name, input.Key)
		}
	}
}

func TestInputMappingsResolveFromResourceTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		resourceType string
		wantSegment  string
	}{
		{"aws:ec2/instance:Instance", "ec2/instance"},
		{"aws:ebs/volume:Volume", "ebs/volume"},
		{"aws:lambda/function:Function", "lambda/function"},
		{"aws:dynamodb/table:Table", "dynamodb/table"},
		{"aws:lb/loadBalancer:LoadBalancer", "lb/loadbalancer"},
		{"aws:alb/loadBalancer:LoadBalancer", "alb/loadbalancer"},
		{"aws:elasticache/cluster:Cluster", "elasticache/cluster"},
		{"aws:s3/bucket:Bucket", "s3/bucket"},
		{"aws:ec2/natGateway:NatGateway", "ec2/natgateway"},
		{"aws-native:ec2:Instance", ""},
	}

	for _, tt := range tests {
		t.Run(tt.resourceType, func(t *testing.T) {
			t.Parallel()

			mapped, ok := mappingFor(tt.resourceType)

			if tt.wantSegment == "" {
				assert.False(t, ok)
				return
			}
			require.True(t, ok)
			assert.Equal(t, tt.wantSegment, mapped.ResourceSegment)
			assert.Equal(t, "aws", mapped.Provider)
		})
	}
}
