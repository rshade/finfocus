package pagination_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli/pagination"
)

// TestPaginationParams_Validate verifies pagination parameter validation.
func TestPaginationParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  pagination.PaginationParams
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid limit only",
			params: pagination.PaginationParams{
				Limit: 10,
			},
			wantErr: false,
		},
		{
			name: "valid page and page-size",
			params: pagination.PaginationParams{
				Page:     2,
				PageSize: 20,
			},
			wantErr: false,
		},
		{
			name: "valid offset only",
			params: pagination.PaginationParams{
				Offset: 50,
			},
			wantErr: false,
		},
		{
			name: "valid limit with page and page-size",
			params: pagination.PaginationParams{
				Limit:    100,
				Page:     1,
				PageSize: 25,
			},
			wantErr: false,
		},
		{
			name: "page and offset mutual exclusion",
			params: pagination.PaginationParams{
				Page:   2,
				Offset: 20,
			},
			wantErr: true,
			errMsg:  "page and offset",
		},
		{
			name: "page without page-size",
			params: pagination.PaginationParams{
				Page: 2,
			},
			wantErr: true,
			errMsg:  "page-size must be specified",
		},
		{
			name: "page-size without page",
			params: pagination.PaginationParams{
				PageSize: 20,
			},
			wantErr: true,
			errMsg:  "page must be specified",
		},
		{
			name: "negative limit",
			params: pagination.PaginationParams{
				Limit: -1,
			},
			wantErr: true,
			errMsg:  "limit cannot be negative",
		},
		{
			name: "zero page",
			params: pagination.PaginationParams{
				Page:     0,
				PageSize: 20,
			},
			wantErr: true,
			errMsg:  "page must be specified",
		},
		{
			name: "negative page",
			params: pagination.PaginationParams{
				Page:     -1,
				PageSize: 20,
			},
			wantErr: true,
			errMsg:  "page cannot be negative",
		},
		{
			name: "zero page-size",
			params: pagination.PaginationParams{
				Page:     1,
				PageSize: 0,
			},
			wantErr: true,
			errMsg:  "page-size must be specified",
		},
		{
			name: "negative page-size",
			params: pagination.PaginationParams{
				Page:     1,
				PageSize: -1,
			},
			wantErr: true,
			errMsg:  "page-size cannot be negative",
		},
		{
			name: "negative offset",
			params: pagination.PaginationParams{
				Offset: -1,
			},
			wantErr: true,
			errMsg:  "offset cannot be negative",
		},
		{
			name: "all zeros (no pagination)",
			params: pagination.PaginationParams{
				Limit:    0,
				Page:     0,
				PageSize: 0,
				Offset:   0,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestPaginationParams_IsEnabled verifies pagination enabled detection.
func TestPaginationParams_IsEnabled(t *testing.T) {
	tests := []struct {
		name   string
		params pagination.PaginationParams
		want   bool
	}{
		{
			name:   "no pagination",
			params: pagination.PaginationParams{},
			want:   false,
		},
		{
			name: "limit only",
			params: pagination.PaginationParams{
				Limit: 10,
			},
			want: true,
		},
		{
			name: "page and page-size",
			params: pagination.PaginationParams{
				Page:     2,
				PageSize: 20,
			},
			want: true,
		},
		{
			name: "offset only",
			params: pagination.PaginationParams{
				Offset: 50,
			},
			want: true,
		},
		{
			name: "limit with page",
			params: pagination.PaginationParams{
				Limit:    100,
				Page:     3,
				PageSize: 25,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.params.IsEnabled())
		})
	}
}

// TestPaginationParams_CalculateOffsetLimit verifies offset/limit calculation from page/page-size.
func TestPaginationParams_CalculateOffsetLimit(t *testing.T) {
	tests := []struct {
		name       string
		params     pagination.PaginationParams
		wantOffset int
		wantLimit  int
	}{
		{
			name: "page 1 with page-size 20",
			params: pagination.PaginationParams{
				Page:     1,
				PageSize: 20,
			},
			wantOffset: 0,
			wantLimit:  20,
		},
		{
			name: "page 2 with page-size 20",
			params: pagination.PaginationParams{
				Page:     2,
				PageSize: 20,
			},
			wantOffset: 20,
			wantLimit:  20,
		},
		{
			name: "page 5 with page-size 10",
			params: pagination.PaginationParams{
				Page:     5,
				PageSize: 10,
			},
			wantOffset: 40,
			wantLimit:  10,
		},
		{
			name: "page 3 with page-size 50 and limit override",
			params: pagination.PaginationParams{
				Page:     3,
				PageSize: 50,
				Limit:    25,
			},
			wantOffset: 100,
			wantLimit:  25,
		},
		{
			name: "explicit offset with limit",
			params: pagination.PaginationParams{
				Offset: 30,
				Limit:  15,
			},
			wantOffset: 30,
			wantLimit:  15,
		},
		{
			name: "limit only (no offset)",
			params: pagination.PaginationParams{
				Limit: 10,
			},
			wantOffset: 0,
			wantLimit:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			offset, limit := tt.params.CalculateOffsetLimit()
			assert.Equal(t, tt.wantOffset, offset, "offset mismatch")
			assert.Equal(t, tt.wantLimit, limit, "limit mismatch")
		})
	}
}

// TestPaginationParams_ApplyToSlice verifies slice pagination.
func TestPaginationParams_ApplyToSlice(t *testing.T) {
	// Create test data slice
	items := make([]int, 100)
	for i := range items {
		items[i] = i
	}

	tests := []struct {
		name       string
		params     pagination.PaginationParams
		wantStart  int
		wantEnd    int
		wantLength int
	}{
		{
			name: "first page of 20",
			params: pagination.PaginationParams{
				Page:     1,
				PageSize: 20,
			},
			wantStart:  0,
			wantEnd:    20,
			wantLength: 20,
		},
		{
			name: "second page of 20",
			params: pagination.PaginationParams{
				Page:     2,
				PageSize: 20,
			},
			wantStart:  20,
			wantEnd:    40,
			wantLength: 20,
		},
		{
			name: "last page partial",
			params: pagination.PaginationParams{
				Page:     5,
				PageSize: 30,
			},
			wantStart:  90, // (5-1)*30 = 120, but capped at 90
			wantEnd:    100,
			wantLength: 10,
		},
		{
			name: "offset 50 limit 10",
			params: pagination.PaginationParams{
				Offset: 50,
				Limit:  10,
			},
			wantStart:  50,
			wantEnd:    60,
			wantLength: 10,
		},
		{
			name: "limit only",
			params: pagination.PaginationParams{
				Limit: 15,
			},
			wantStart:  0,
			wantEnd:    15,
			wantLength: 15,
		},
		{
			name: "offset beyond end",
			params: pagination.PaginationParams{
				Offset: 150,
				Limit:  10,
			},
			wantStart:  100, // Capped at length
			wantEnd:    100,
			wantLength: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.params.ApplyToSlice(items)
			assert.Len(t, result, tt.wantLength)
			if tt.wantLength > 0 {
				assert.Equal(t, tt.wantStart, result[0])
				assert.Equal(t, tt.wantEnd-1, result[len(result)-1])
			}
		})
	}
}
