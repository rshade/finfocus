//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/router"
)

// TestPatternRouting_GlobOverridesAutomatic tests US3:
// Pattern-based routing overrides automatic provider matching.
func TestPatternRouting_GlobOverridesAutomatic(t *testing.T) {
	ctx := context.Background()

	// Create mock plugin clients
	awsPublicClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			Name:               "aws-public",
			Version:            "1.0.0",
			SupportedProviders: []string{"aws"},
		},
	}
	eksClient := &pluginhost.Client{
		Name: "eks-costs",
		Metadata: &proto.PluginMetadata{
			Name:               "eks-costs",
			Version:            "1.0.0",
			SupportedProviders: []string{"aws"}, // Also supports AWS
		},
	}

	// Configure pattern override: EKS resources go to eks-costs
	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name: "eks-costs",
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "aws:eks:*"},
				},
				Priority: 20,
			},
			{
				Name:     "aws-public",
				Priority: 10,
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{awsPublicClient, eksClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	t.Run("EKS resource routes to eks-costs via pattern", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "aws:eks:Cluster", ID: "eks-cluster-1"}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		// Should match eks-costs via pattern (plus aws-public via provider)
		require.GreaterOrEqual(t, len(matches), 1)
		// eks-costs should be first due to pattern match + higher priority
		assert.Equal(t, "eks-costs", matches[0].Client.Name)
		assert.Equal(t, router.MatchReasonPattern, matches[0].MatchReason)
		assert.Equal(t, "config", matches[0].Source)
	})

	t.Run("EC2 resource routes to aws-public via automatic", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance", ID: "i-12345"}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		// Both should match via provider
		require.GreaterOrEqual(t, len(matches), 1)
		// Both aws-public and eks-costs support aws provider, but eks-costs
		// doesn't have a pattern match for EC2, so check we get the expected plugins
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Client.Name
		}
		assert.Contains(t, names, "aws-public")
	})
}

// TestPatternRouting_RegexMatching tests regex pattern matching.
func TestPatternRouting_RegexMatching(t *testing.T) {
	ctx := context.Background()

	kubecostClient := &pluginhost.Client{
		Name: "kubecost",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"kubernetes"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name: "kubecost",
				Patterns: []config.ResourcePattern{
					{Type: "regex", Pattern: "kubernetes:apps/v1:(Deployment|StatefulSet|DaemonSet)"},
				},
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{kubecostClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	t.Run("Deployment matches regex", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "kubernetes:apps/v1:Deployment"}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.Len(t, matches, 1)
		assert.Equal(t, "kubecost", matches[0].Client.Name)
		assert.Equal(t, router.MatchReasonPattern, matches[0].MatchReason)
	})

	t.Run("StatefulSet matches regex", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "kubernetes:apps/v1:StatefulSet"}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.Len(t, matches, 1)
		assert.Equal(t, "kubecost", matches[0].Client.Name)
	})

	t.Run("DaemonSet matches regex", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "kubernetes:apps/v1:DaemonSet"}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		require.Len(t, matches, 1)
		assert.Equal(t, "kubecost", matches[0].Client.Name)
	})

	t.Run("Pod does NOT match regex (not in alternation)", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "kubernetes:apps/v1:Pod"}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		// Should still match via provider
		require.Len(t, matches, 1)
		// But the match reason should be automatic (provider), not pattern
		assert.Equal(t, router.MatchReasonAutomatic, matches[0].MatchReason)
	})
}

// TestPatternRouting_MultiplePatterns tests multiple patterns on a single plugin.
func TestPatternRouting_MultiplePatterns(t *testing.T) {
	ctx := context.Background()

	multiClient := &pluginhost.Client{
		Name: "multi-pattern",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"custom"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name: "multi-pattern",
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "aws:s3:*"},
					{Type: "glob", Pattern: "gcp:storage:*"},
					{Type: "regex", Pattern: "azure:storage/.*"},
				},
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{multiClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	testCases := []struct {
		name         string
		resourceType string
		wantMatch    bool
	}{
		{"AWS S3", "aws:s3:Bucket", true},
		{"GCP Storage", "gcp:storage:Bucket", true},
		{"Azure Storage", "azure:storage/account:StorageAccount", true},
		{"AWS EC2 no match", "aws:ec2:Instance", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resource := engine.ResourceDescriptor{Type: tc.resourceType}
			matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

			if tc.wantMatch {
				require.Len(t, matches, 1)
				assert.Equal(t, "multi-pattern", matches[0].Client.Name)
				assert.Equal(t, router.MatchReasonPattern, matches[0].MatchReason)
			} else {
				assert.Empty(t, matches)
			}
		})
	}
}

// TestPatternRouting_PatternPrecedence tests FR-009:
// Patterns take precedence over automatic provider matching.
func TestPatternRouting_PatternPrecedence(t *testing.T) {
	ctx := context.Background()

	// Two plugins both support AWS
	awsGenericClient := &pluginhost.Client{
		Name: "aws-generic",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}
	awsSpecificClient := &pluginhost.Client{
		Name: "aws-specific",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name: "aws-specific",
				Patterns: []config.ResourcePattern{
					{Type: "glob", Pattern: "aws:rds:*"},
				},
				Priority: 20,
			},
			{
				Name:     "aws-generic",
				Priority: 10,
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{awsGenericClient, awsSpecificClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	t.Run("RDS matches pattern first, then automatic", func(t *testing.T) {
		resource := engine.ResourceDescriptor{Type: "aws:rds:Instance"}
		matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

		// Both should match - specific via pattern, generic via provider
		require.Len(t, matches, 2)

		// First should be aws-specific with pattern match (higher priority + pattern)
		assert.Equal(t, "aws-specific", matches[0].Client.Name)
		assert.Equal(t, router.MatchReasonPattern, matches[0].MatchReason)
		assert.Equal(t, "config", matches[0].Source)

		// Second should be aws-generic with automatic match
		assert.Equal(t, "aws-generic", matches[1].Client.Name)
		assert.Equal(t, router.MatchReasonAutomatic, matches[1].MatchReason)
	})
}

// TestPatternRouting_NoPatternFallbackToAutomatic tests that plugins without
// patterns still participate in automatic routing.
func TestPatternRouting_NoPatternFallbackToAutomatic(t *testing.T) {
	ctx := context.Background()

	awsClient := &pluginhost.Client{
		Name: "aws-public",
		Metadata: &proto.PluginMetadata{
			SupportedProviders: []string{"aws"},
		},
	}

	cfg := &config.RoutingConfig{
		Plugins: []config.PluginRouting{
			{
				Name:     "aws-public",
				Priority: 10,
				// No patterns configured
			},
		},
	}

	r, err := router.NewRouter(
		router.WithClients([]*pluginhost.Client{awsClient}),
		router.WithConfig(cfg),
	)
	require.NoError(t, err)

	resource := engine.ResourceDescriptor{Type: "aws:ec2:Instance"}
	matches := r.SelectPlugins(ctx, resource, "ProjectedCosts")

	require.Len(t, matches, 1)
	assert.Equal(t, "aws-public", matches[0].Client.Name)
	assert.Equal(t, router.MatchReasonAutomatic, matches[0].MatchReason)
	assert.Equal(t, "automatic", matches[0].Source)
}
