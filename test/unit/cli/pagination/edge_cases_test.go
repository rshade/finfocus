package pagination_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli/pagination"
)

// TestZeroResultsWithPagination tests that pagination metadata is correct
// when the result set is empty.
func TestZeroResultsWithPagination(t *testing.T) {
	t.Run("zero results with limit", func(t *testing.T) {
		params := pagination.PaginationParams{
			Limit:  10,
			Offset: 0,
		}

		// Simulate empty result set
		totalItems := 0
		meta := pagination.NewPaginationMeta(params, totalItems)

		// Verify metadata for empty results
		assert.Equal(t, 0, meta.TotalItems, "total_items should be 0")
		assert.Equal(t, 0, meta.TotalPages, "total_pages should be 0")
		assert.Equal(t, 1, meta.CurrentPage, "current_page should be 1")
		assert.Equal(t, 10, meta.PageSize, "page_size should match limit")
		assert.False(t, meta.HasNext, "has_next should be false")
		assert.False(t, meta.HasPrevious, "has_previous should be false")
	})

	t.Run("zero results with page-based pagination", func(t *testing.T) {
		params := pagination.PaginationParams{
			Page:     1,
			PageSize: 20,
		}

		totalItems := 0
		meta := pagination.NewPaginationMeta(params, totalItems)

		// Verify metadata structure
		assert.Equal(t, 0, meta.TotalItems)
		assert.Equal(t, 0, meta.TotalPages)
		assert.Equal(t, 1, meta.CurrentPage, "current_page should be 1 (requested page)")
		assert.Equal(t, 20, meta.PageSize)
		assert.False(t, meta.HasNext)
		assert.False(t, meta.HasPrevious)
	})

	t.Run("zero results without pagination", func(t *testing.T) {
		params := pagination.PaginationParams{
			Limit:  0, // unlimited
			Offset: 0,
		}

		totalItems := 0
		meta := pagination.NewPaginationMeta(params, totalItems)

		// Verify unlimited mode with zero results
		assert.Equal(t, 0, meta.TotalItems)
		assert.Equal(t, 0, meta.TotalPages)
		assert.Equal(t, 1, meta.CurrentPage)
		assert.Equal(t, 0, meta.PageSize, "page_size should be 0 for unlimited")
		assert.False(t, meta.HasNext)
		assert.False(t, meta.HasPrevious)
	})
}

// TestOutOfBoundsPage tests that requesting a page beyond the available
// pages returns empty results with correct metadata.
func TestOutOfBoundsPage(t *testing.T) {
	t.Run("page beyond total pages", func(t *testing.T) {
		params := pagination.PaginationParams{
			Page:     10,
			PageSize: 20,
		}

		// Dataset has 50 items (3 pages total with page_size=20)
		totalItems := 50
		meta := pagination.NewPaginationMeta(params, totalItems)

		// Verify metadata shows requested page is out of bounds
		assert.Equal(t, 50, meta.TotalItems)
		assert.Equal(t, 3, meta.TotalPages, "50 items / 20 per page = 3 pages")
		assert.Equal(t, 10, meta.CurrentPage, "current_page should reflect requested page")
		assert.Equal(t, 20, meta.PageSize)
		assert.False(t, meta.HasNext, "no next page when beyond bounds")
		assert.True(t, meta.HasPrevious, "has_previous should be true")
	})

	t.Run("offset beyond total items", func(t *testing.T) {
		params := pagination.PaginationParams{
			Limit:  10,
			Offset: 100,
		}

		// Dataset has 50 items
		totalItems := 50
		meta := pagination.NewPaginationMeta(params, totalItems)

		// Calculate effective page from offset
		effectivePage := (params.Offset / params.Limit) + 1

		assert.Equal(t, 50, meta.TotalItems)
		assert.Equal(t, 5, meta.TotalPages, "50 items / 10 per page = 5 pages")
		assert.Equal(t, effectivePage, meta.CurrentPage, "current_page calculated from offset")
		assert.Equal(t, 10, meta.PageSize)
		assert.False(t, meta.HasNext, "no next page when offset > total")
		assert.True(t, meta.HasPrevious, "has previous page")
	})

	t.Run("page 1 with zero items", func(t *testing.T) {
		params := pagination.PaginationParams{
			Page:     1,
			PageSize: 10,
		}

		totalItems := 0
		meta := pagination.NewPaginationMeta(params, totalItems)

		// First page of empty results is valid, not "out of bounds"
		assert.Equal(t, 0, meta.TotalItems)
		assert.Equal(t, 0, meta.TotalPages)
		assert.Equal(t, 1, meta.CurrentPage)
		assert.False(t, meta.HasNext)
		assert.False(t, meta.HasPrevious)
	})

	t.Run("negative page number", func(t *testing.T) {
		params := pagination.PaginationParams{
			Page:     -1,
			PageSize: 10,
		}

		// Validation should catch this before metadata generation
		err := params.Validate()
		require.Error(t, err, "negative page should be invalid")
		assert.Contains(t, err.Error(), "page cannot be negative", "error should mention page constraint")
	})

	t.Run("zero page number with page-size", func(t *testing.T) {
		params := pagination.PaginationParams{
			Page:     0,
			PageSize: 10,
		}

		// Page 0 with page-size is invalid (must specify page)
		err := params.Validate()
		require.Error(t, err, "page 0 with page-size should be invalid")
		assert.Contains(t, err.Error(), "page must be")
	})
}

// TestInvalidSortField tests that invalid sort fields are detected
// using the existing sorter and parse functions.
func TestInvalidSortField(t *testing.T) {
	// Create a sorter with known valid fields
	sorter := pagination.NewRecommendationSorter()
	validFields := sorter.GetValidFields()

	t.Run("completely invalid field", func(t *testing.T) {
		invalidField := "invalidField"

		// Check using sorter's IsValidField
		isValid := sorter.IsValidField(invalidField)
		assert.False(t, isValid, "invalid field should return false")

		// Verify valid fields are known
		assert.Contains(t, validFields, "savings")
		assert.Contains(t, validFields, "cost")
		assert.Contains(t, validFields, "name")
		assert.NotContains(t, validFields, invalidField)
	})

	t.Run("empty sort field", func(t *testing.T) {
		// Empty field is valid (no sorting) - ParseSort handles this
		field, order, err := pagination.ParseSort("")
		assert.NoError(t, err, "empty sort field should be valid (no sorting)")
		assert.Equal(t, "", field)
		assert.Equal(t, "asc", order, "default order should be asc")
	})

	t.Run("case sensitivity", func(t *testing.T) {
		upperField := "SAVINGS" // uppercase

		// Field names are case-sensitive
		isValid := sorter.IsValidField(upperField)
		assert.False(t, isValid, "uppercase field should be invalid (case-sensitive)")

		// Lower case should be valid
		lowerField := "savings"
		isValid = sorter.IsValidField(lowerField)
		assert.True(t, isValid, "lowercase field should be valid")
	})

	t.Run("invalid sort order", func(t *testing.T) {
		sortExpr := "savings:invalid"

		// ParseSort should return error for invalid order
		_, _, err := pagination.ParseSort(sortExpr)
		require.Error(t, err, "invalid sort order should return error")
		assert.Contains(t, err.Error(), "sort order", "error should mention sort order")
	})

	t.Run("valid field and order", func(t *testing.T) {
		sortExpr := "savings:desc"

		// ParseSort should succeed
		field, order, err := pagination.ParseSort(sortExpr)
		require.NoError(t, err)
		assert.Equal(t, "savings", field)
		assert.Equal(t, "desc", order)

		// Field should be valid in sorter
		isValid := sorter.IsValidField(field)
		assert.True(t, isValid)
	})

	t.Run("valid field without explicit order", func(t *testing.T) {
		sortExpr := "cost"

		// ParseSort should default to asc
		field, order, err := pagination.ParseSort(sortExpr)
		require.NoError(t, err)
		assert.Equal(t, "cost", field)
		assert.Equal(t, "asc", order, "should default to asc")

		// Field should be valid
		isValid := sorter.IsValidField(field)
		assert.True(t, isValid)
	})
}

// TestPaginationParamsValidation tests comprehensive validation of pagination parameters.
func TestPaginationParamsValidation(t *testing.T) {
	t.Run("page and offset are mutually exclusive", func(t *testing.T) {
		params := pagination.PaginationParams{
			Page:     2,
			PageSize: 10,
			Offset:   20,
			Limit:    10,
		}

		err := params.Validate()
		require.Error(t, err, "page and offset should be mutually exclusive")
		assert.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("page requires page_size", func(t *testing.T) {
		params := pagination.PaginationParams{
			Page:     2,
			PageSize: 0,
		}

		err := params.Validate()
		require.Error(t, err, "page requires page_size")
		assert.Contains(t, err.Error(), "page-size must be")
	})

	t.Run("negative limit", func(t *testing.T) {
		params := pagination.PaginationParams{
			Limit:  -1,
			Offset: 0,
		}

		err := params.Validate()
		require.Error(t, err, "negative limit should be invalid")
		assert.Contains(t, err.Error(), "limit cannot be negative")
	})

	t.Run("negative offset", func(t *testing.T) {
		params := pagination.PaginationParams{
			Limit:  10,
			Offset: -1,
		}

		err := params.Validate()
		require.Error(t, err, "negative offset should be invalid")
		assert.Contains(t, err.Error(), "offset cannot be negative")
	})

	t.Run("valid offset-based pagination", func(t *testing.T) {
		params := pagination.PaginationParams{
			Limit:  10,
			Offset: 20,
		}

		err := params.Validate()
		assert.NoError(t, err, "valid offset-based pagination")
	})

	t.Run("valid page-based pagination", func(t *testing.T) {
		params := pagination.PaginationParams{
			Page:     2,
			PageSize: 20,
		}

		err := params.Validate()
		assert.NoError(t, err, "valid page-based pagination")
	})

	t.Run("unlimited mode", func(t *testing.T) {
		params := pagination.PaginationParams{
			Limit:  0,
			Offset: 0,
		}

		err := params.Validate()
		assert.NoError(t, err, "unlimited mode (limit=0) should be valid")
	})
}
