package pagination

import (
	"math"
)

// PaginationMeta contains metadata about paginated results.
//
//nolint:revive // PaginationMeta is the canonical name for this exported type.
type PaginationMeta struct {
	CurrentPage int  `json:"current_page" yaml:"current_page"`
	PageSize    int  `json:"page_size"    yaml:"page_size"`
	TotalPages  int  `json:"total_pages"  yaml:"total_pages"`
	TotalItems  int  `json:"total_items"  yaml:"total_items"`
	HasPrevious bool `json:"has_previous" yaml:"has_previous"`
	HasNext     bool `json:"has_next"     yaml:"has_next"`
}

// NewPaginationMeta creates pagination metadata from parameters and total count.
func NewPaginationMeta(params PaginationParams, totalCount int) PaginationMeta {
	// Calculate page size (prefer explicit page-size, fall back to limit)
	pageSize := params.PageSize
	if pageSize == 0 && params.Limit > 0 {
		pageSize = params.Limit
	}
	if pageSize == 0 {
		pageSize = totalCount // If no page size specified, treat as single page
	}

	// Calculate current page
	currentPage := params.Page
	if currentPage == 0 && params.Offset > 0 && pageSize > 0 {
		// Convert offset to page number
		currentPage = (params.Offset / pageSize) + 1
	}
	if currentPage == 0 {
		currentPage = 1
	}

	// Calculate total pages
	totalPages := 0
	if pageSize > 0 {
		totalPages = int(math.Ceil(float64(totalCount) / float64(pageSize)))
	}

	// Calculate has_previous and has_next
	hasPrevious := currentPage > 1
	hasNext := currentPage < totalPages

	return PaginationMeta{
		CurrentPage: currentPage,
		PageSize:    pageSize,
		TotalPages:  totalPages,
		TotalItems:  totalCount,
		HasPrevious: hasPrevious,
		HasNext:     hasNext,
	}
}
