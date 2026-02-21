package pagination_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/internal/cli/pagination"
)

// TestPaginationMeta_NewPaginationMeta verifies pagination metadata creation.
func TestPaginationMeta_NewPaginationMeta(t *testing.T) {
	tests := []struct {
		name         string
		params       pagination.PaginationParams
		totalCount   int
		wantPage     int
		wantPageSize int
		wantTotal    int
		wantHasPrev  bool
		wantHasNext  bool
	}{
		{
			name: "first page of 100 items",
			params: pagination.PaginationParams{
				Page:     1,
				PageSize: 20,
			},
			totalCount:   100,
			wantPage:     1,
			wantPageSize: 20,
			wantTotal:    5, // 100/20 = 5 pages
			wantHasPrev:  false,
			wantHasNext:  true,
		},
		{
			name: "middle page",
			params: pagination.PaginationParams{
				Page:     3,
				PageSize: 25,
			},
			totalCount:   100,
			wantPage:     3,
			wantPageSize: 25,
			wantTotal:    4, // 100/25 = 4 pages
			wantHasPrev:  true,
			wantHasNext:  true,
		},
		{
			name: "last page full",
			params: pagination.PaginationParams{
				Page:     5,
				PageSize: 20,
			},
			totalCount:   100,
			wantPage:     5,
			wantPageSize: 20,
			wantTotal:    5,
			wantHasPrev:  true,
			wantHasNext:  false,
		},
		{
			name: "last page partial",
			params: pagination.PaginationParams{
				Page:     4,
				PageSize: 30,
			},
			totalCount:   100,
			wantPage:     4,
			wantPageSize: 30,
			wantTotal:    4, // ceil(100/30) = 4
			wantHasPrev:  true,
			wantHasNext:  false,
		},
		{
			name: "single page",
			params: pagination.PaginationParams{
				Page:     1,
				PageSize: 100,
			},
			totalCount:   50,
			wantPage:     1,
			wantPageSize: 100,
			wantTotal:    1,
			wantHasPrev:  false,
			wantHasNext:  false,
		},
		{
			name: "exact page boundary",
			params: pagination.PaginationParams{
				Page:     2,
				PageSize: 50,
			},
			totalCount:   100,
			wantPage:     2,
			wantPageSize: 50,
			wantTotal:    2,
			wantHasPrev:  true,
			wantHasNext:  false,
		},
		{
			name: "offset-based pagination",
			params: pagination.PaginationParams{
				Offset: 40,
				Limit:  20,
			},
			totalCount:   100,
			wantPage:     3, // offset 40 with limit 20 = page 3
			wantPageSize: 20,
			wantTotal:    5, // 100/20 = 5 pages
			wantHasPrev:  true,
			wantHasNext:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := pagination.NewPaginationMeta(tt.params, tt.totalCount)

			assert.Equal(t, tt.wantPage, meta.CurrentPage, "current_page mismatch")
			assert.Equal(t, tt.wantPageSize, meta.PageSize, "page_size mismatch")
			assert.Equal(t, tt.wantTotal, meta.TotalPages, "total_pages mismatch")
			assert.Equal(t, tt.totalCount, meta.TotalItems, "total_items mismatch")
			assert.Equal(t, tt.wantHasPrev, meta.HasPrevious, "has_previous mismatch")
			assert.Equal(t, tt.wantHasNext, meta.HasNext, "has_next mismatch")
		})
	}
}

// TestPaginationMeta_OutOfBounds verifies out-of-bounds page handling.
func TestPaginationMeta_OutOfBounds(t *testing.T) {
	tests := []struct {
		name        string
		params      pagination.PaginationParams
		totalCount  int
		wantPage    int
		wantTotal   int
		wantHasPrev bool
		wantHasNext bool
	}{
		{
			name: "page beyond last",
			params: pagination.PaginationParams{
				Page:     10,
				PageSize: 20,
			},
			totalCount:  100,
			wantPage:    10,
			wantTotal:   5, // actual total pages
			wantHasPrev: true,
			wantHasNext: false,
		},
		{
			name: "large offset",
			params: pagination.PaginationParams{
				Offset: 200,
				Limit:  20,
			},
			totalCount:  100,
			wantPage:    11, // offset 200 / limit 20 + 1 = 11
			wantTotal:   5,  // 100/20 = 5 pages
			wantHasPrev: true,
			wantHasNext: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := pagination.NewPaginationMeta(tt.params, tt.totalCount)

			assert.Equal(t, tt.wantPage, meta.CurrentPage)
			assert.Equal(t, tt.wantTotal, meta.TotalPages)
			assert.Equal(t, tt.wantHasPrev, meta.HasPrevious)
			assert.Equal(t, tt.wantHasNext, meta.HasNext)
		})
	}
}

// TestPaginationMeta_EmptyResults verifies metadata for empty result sets.
func TestPaginationMeta_EmptyResults(t *testing.T) {
	tests := []struct {
		name         string
		params       pagination.PaginationParams
		totalCount   int
		wantPage     int
		wantPageSize int
		wantTotal    int
		wantHasPrev  bool
		wantHasNext  bool
	}{
		{
			name: "no results page 1",
			params: pagination.PaginationParams{
				Page:     1,
				PageSize: 20,
			},
			totalCount:   0,
			wantPage:     1,
			wantPageSize: 20,
			wantTotal:    0,
			wantHasPrev:  false,
			wantHasNext:  false,
		},
		{
			name: "no results beyond page 1",
			params: pagination.PaginationParams{
				Page:     5,
				PageSize: 20,
			},
			totalCount:   0,
			wantPage:     5,
			wantPageSize: 20,
			wantTotal:    0,
			wantHasPrev:  true, // Requesting page 5 means previous pages exist conceptually
			wantHasNext:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := pagination.NewPaginationMeta(tt.params, tt.totalCount)

			assert.Equal(t, tt.wantPage, meta.CurrentPage)
			assert.Equal(t, tt.wantPageSize, meta.PageSize)
			assert.Equal(t, tt.wantTotal, meta.TotalPages)
			assert.Equal(t, tt.totalCount, meta.TotalItems)
			assert.Equal(t, tt.wantHasPrev, meta.HasPrevious)
			assert.Equal(t, tt.wantHasNext, meta.HasNext)
		})
	}
}

// TestPaginationMeta_EdgeCases verifies edge case handling.
func TestPaginationMeta_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		params     pagination.PaginationParams
		totalCount int
		wantTotal  int
	}{
		{
			name: "one item total",
			params: pagination.PaginationParams{
				Page:     1,
				PageSize: 10,
			},
			totalCount: 1,
			wantTotal:  1,
		},
		{
			name: "page size equals total",
			params: pagination.PaginationParams{
				Page:     1,
				PageSize: 50,
			},
			totalCount: 50,
			wantTotal:  1,
		},
		{
			name: "page size larger than total",
			params: pagination.PaginationParams{
				Page:     1,
				PageSize: 100,
			},
			totalCount: 50,
			wantTotal:  1,
		},
		{
			name: "page size of 1",
			params: pagination.PaginationParams{
				Page:     1,
				PageSize: 1,
			},
			totalCount: 100,
			wantTotal:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := pagination.NewPaginationMeta(tt.params, tt.totalCount)
			assert.Equal(t, tt.wantTotal, meta.TotalPages)
		})
	}
}
