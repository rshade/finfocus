package cli_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/engine"
)

func TestApplyFilters(t *testing.T) {
	t.Parallel()

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2:Instance", ID: "i-123", Provider: "aws"},
		{Type: "aws:rds:Instance", ID: "db-456", Provider: "aws"},
		{Type: "azure:compute:VirtualMachine", ID: "vm-789", Provider: "azure"},
	}

	tests := []struct {
		name          string
		resources     []engine.ResourceDescriptor
		filters       []string
		wantCount     int
		wantErr       bool
		wantErrSubstr string
	}{
		{
			name:      "no filters returns all resources",
			resources: resources,
			filters:   []string{},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "nil filters returns all resources",
			resources: resources,
			filters:   nil,
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "empty string filter is ignored",
			resources: resources,
			filters:   []string{""},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "filter by provider",
			resources: resources,
			filters:   []string{"provider=aws"},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "filter by type substring",
			resources: resources,
			filters:   []string{"type=ec2"},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "multiple filters applied sequentially",
			resources: resources,
			filters:   []string{"provider=aws", "type=rds"},
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "filter with no matches returns empty",
			resources: resources,
			filters:   []string{"provider=gcp"},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:          "invalid filter syntax returns error",
			resources:     resources,
			filters:       []string{"invalid-filter"},
			wantErr:       true,
			wantErrSubstr: "invalid filter syntax",
		},
		{
			name:          "empty key returns error",
			resources:     resources,
			filters:       []string{"=value"},
			wantErr:       true,
			wantErrSubstr: "key and value must be non-empty",
		},
		{
			name:          "empty value returns error",
			resources:     resources,
			filters:       []string{"key="},
			wantErr:       true,
			wantErrSubstr: "key and value must be non-empty",
		},
		{
			name:      "empty resources returns empty",
			resources: []engine.ResourceDescriptor{},
			filters:   []string{"provider=aws"},
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "filter with mixed empty and valid",
			resources: resources,
			filters:   []string{"", "provider=aws", ""},
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			result, err := cli.ApplyFilters(ctx, tt.resources, tt.filters)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrSubstr)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.wantCount)
		})
	}
}

func TestApplyFilters_ValidationBeforeApplication(t *testing.T) {
	t.Parallel()

	resources := []engine.ResourceDescriptor{
		{Type: "aws:ec2:Instance", ID: "i-123", Provider: "aws"},
		{Type: "aws:rds:Instance", ID: "db-456", Provider: "aws"},
	}

	// First filter is valid, second is invalid
	// Validation should fail before any filtering is applied
	filters := []string{"provider=aws", "invalid-no-equals"}

	ctx := context.Background()
	result, err := cli.ApplyFilters(ctx, resources, filters)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid filter syntax")
}

// =============================================================================
// Budget Filter Parsing Tests (T010, T023, T027)
// =============================================================================

func TestParseBudgetFilters_ValidTagKeyValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		filters      []string
		wantTags     map[string]string
		wantProvider []string
	}{
		{
			name:         "single tag filter",
			filters:      []string{"tag:namespace=production"},
			wantTags:     map[string]string{"namespace": "production"},
			wantProvider: nil,
		},
		{
			name:         "multiple tag filters (AND logic)",
			filters:      []string{"tag:namespace=production", "tag:cluster=us-east-1"},
			wantTags:     map[string]string{"namespace": "production", "cluster": "us-east-1"},
			wantProvider: nil,
		},
		{
			name:         "provider filter only",
			filters:      []string{"provider=kubecost"},
			wantTags:     map[string]string{},
			wantProvider: []string{"kubecost"},
		},
		{
			name:         "mixed provider and tag filters",
			filters:      []string{"provider=kubecost", "tag:namespace=staging"},
			wantTags:     map[string]string{"namespace": "staging"},
			wantProvider: []string{"kubecost"},
		},
		{
			name:         "multiple providers (OR logic)",
			filters:      []string{"provider=kubecost", "provider=aws-budgets"},
			wantTags:     map[string]string{},
			wantProvider: []string{"kubecost", "aws-budgets"},
		},
		{
			name:         "empty filters returns empty options",
			filters:      []string{},
			wantTags:     map[string]string{},
			wantProvider: nil,
		},
		{
			name:         "tag with glob pattern",
			filters:      []string{"tag:namespace=prod-*"},
			wantTags:     map[string]string{"namespace": "prod-*"},
			wantProvider: nil,
		},
		{
			name:         "tag with empty value (matches empty metadata)",
			filters:      []string{"tag:namespace="},
			wantTags:     map[string]string{"namespace": ""},
			wantProvider: nil,
		},
		{
			name:         "duplicate tag key (later overwrites)",
			filters:      []string{"tag:namespace=staging", "tag:namespace=production"},
			wantTags:     map[string]string{"namespace": "production"},
			wantProvider: nil,
		},
		{
			name:         "tag key with special characters",
			filters:      []string{"tag:kubernetes.io/name=my-app"},
			wantTags:     map[string]string{"kubernetes.io/name": "my-app"},
			wantProvider: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			opts, err := cli.ParseBudgetFilters(ctx, tt.filters)

			require.NoError(t, err)
			require.NotNil(t, opts)
			assert.Equal(t, tt.wantTags, opts.Tags)
			assert.Equal(t, tt.wantProvider, opts.Providers)
		})
	}
}

func TestValidateBudgetFilter_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		filter      string
		wantErr     bool
		errContains string
	}{
		{
			name:        "missing equals in tag filter",
			filter:      "tag:namespace",
			wantErr:     true,
			errContains: "missing '='",
		},
		{
			name:        "empty key after tag:",
			filter:      "tag:=production",
			wantErr:     true,
			errContains: "empty key",
		},
		{
			name:        "unknown filter type",
			filter:      "unknown:key=value",
			wantErr:     true,
			errContains: "unknown filter type",
		},
		{
			name:        "empty filter string",
			filter:      "",
			wantErr:     true,
			errContains: "empty filter",
		},
		{
			name:        "missing provider value",
			filter:      "provider=",
			wantErr:     true,
			errContains: "missing provider value",
		},
		{
			name:        "invalid glob pattern (unclosed bracket)",
			filter:      "tag:env=[invalid",
			wantErr:     true,
			errContains: "invalid glob pattern",
		},
		{
			name:    "valid tag filter",
			filter:  "tag:namespace=production",
			wantErr: false,
		},
		{
			name:    "valid provider filter",
			filter:  "provider=kubecost",
			wantErr: false,
		},
		{
			name:    "valid tag with glob pattern",
			filter:  "tag:env=prod-*",
			wantErr: false,
		},
		{
			name:    "valid tag with empty value",
			filter:  "tag:key=",
			wantErr: false,
		},
		{
			name:    "valid glob with character class",
			filter:  "tag:env=[a-z]*",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := cli.ValidateBudgetFilter(tt.filter)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// Input Limit Tests (DoS Prevention)
// =============================================================================

func TestParseBudgetFilters_ExceedsLimits(t *testing.T) {
	t.Parallel()

	t.Run("exceeds max filter count", func(t *testing.T) {
		t.Parallel()

		// Create 101 filters (exceeds MaxBudgetFilters = 100)
		filters := make([]string, 101)
		for i := range filters {
			filters[i] = fmt.Sprintf("tag:key%d=value%d", i, i)
		}

		ctx := context.Background()
		_, err := cli.ParseBudgetFilters(ctx, filters)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "too many filters")
	})

	t.Run("exceeds max tag count", func(t *testing.T) {
		t.Parallel()

		// Create 51 unique tag filters (exceeds MaxBudgetTags = 50)
		filters := make([]string, 51)
		for i := range filters {
			filters[i] = fmt.Sprintf("tag:key%d=value%d", i, i)
		}

		ctx := context.Background()
		_, err := cli.ParseBudgetFilters(ctx, filters)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "too many")
	})

	t.Run("at limit succeeds", func(t *testing.T) {
		t.Parallel()

		// Create exactly 50 tag filters (at MaxBudgetTags limit)
		filters := make([]string, 50)
		for i := range filters {
			filters[i] = fmt.Sprintf("tag:key%d=value%d", i, i)
		}

		ctx := context.Background()
		opts, err := cli.ParseBudgetFilters(ctx, filters)

		require.NoError(t, err)
		assert.Len(t, opts.Tags, 50)
	})
}
