package cli_test

import (
	"context"
	"testing"

	"github.com/rshade/finfocus/internal/cli"
	"github.com/rshade/finfocus/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
