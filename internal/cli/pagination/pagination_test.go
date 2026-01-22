package pagination

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

func TestPaginationParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  PaginationParams
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid default",
			params:  *NewPaginationParams(),
			wantErr: false,
		},
		{
			name: "valid offset mode",
			params: PaginationParams{
				Limit:  10,
				Offset: 20,
			},
			wantErr: false,
		},
		{
			name: "valid page mode",
			params: PaginationParams{
				Page:     2,
				PageSize: 10,
			},
			wantErr: false,
		},
		{
			name: "negative limit",
			params: PaginationParams{
				Limit: -1,
			},
			wantErr: true,
			errMsg:  "limit cannot be negative",
		},
		{
			name: "negative offset",
			params: PaginationParams{
				Offset: -1,
			},
			wantErr: true,
			errMsg:  "offset cannot be negative",
		},
		{
			name: "negative page",
			params: PaginationParams{
				Page: -1,
			},
			wantErr: true,
			errMsg:  "page cannot be negative",
		},
		{
			name: "negative page-size",
			params: PaginationParams{
				PageSize: -1,
			},
			wantErr: true,
			errMsg:  "page-size cannot be negative",
		},
		{
			name: "mixed modes",
			params: PaginationParams{
				Page:   1,
				Offset: 10,
			},
			wantErr: true,
			errMsg:  "page and offset parameters are mutually exclusive",
		},
		{
			name: "page-size without page",
			params: PaginationParams{
				PageSize: 10,
			},
			wantErr: true,
			errMsg:  "page must be specified when using page-size",
		},
		{
			name: "page without page-size",
			params: PaginationParams{
				Page: 1,
			},
			wantErr: true,
			errMsg:  "page-size must be specified when using page",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseSort(t *testing.T) {
	tests := []struct {
		name      string
		sortStr   string
		wantField string
		wantOrder string
		wantErr   bool
	}{
		{
			name:      "empty",
			sortStr:   "",
			wantField: DefaultSortField,
			wantOrder: DefaultSortOrder,
		},
		{
			name:      "field only",
			sortStr:   "savings",
			wantField: "savings",
			wantOrder: "asc",
		},
		{
			name:      "field and order asc",
			sortStr:   "savings:asc",
			wantField: "savings",
			wantOrder: "asc",
		},
		{
			name:      "field and order desc",
			sortStr:   "savings:desc",
			wantField: "savings",
			wantOrder: "desc",
		},
		{
			name:    "invalid format",
			sortStr: "field:order:extra",
			wantErr: true,
		},
		{
			name:    "empty field",
			sortStr: ":asc",
			wantErr: true,
		},
		{
			name:    "invalid order",
			sortStr: "savings:invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, order, err := ParseSort(tt.sortStr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantField, field)
				assert.Equal(t, tt.wantOrder, order)
			}
		})
	}
}

func TestPaginationParams_Calculations(t *testing.T) {
	t.Run("OffsetBased", func(t *testing.T) {
		p := PaginationParams{Limit: 10, Offset: 20}
		assert.False(t, p.IsPageBased())
		assert.True(t, p.IsOffsetBased())
		assert.Equal(t, 10, p.GetEffectiveLimit())
		assert.Equal(t, 20, p.GetEffectiveOffset())
		assert.Equal(t, 0, p.CalculateTotalPages(100))
	})

	t.Run("PageBased", func(t *testing.T) {
		p := PaginationParams{Page: 3, PageSize: 10}
		assert.True(t, p.IsPageBased())
		assert.False(t, p.IsOffsetBased())
		assert.Equal(t, 10, p.GetEffectiveLimit())
		assert.Equal(t, 20, p.GetEffectiveOffset()) // (3-1) * 10
		assert.Equal(t, 10, p.CalculateTotalPages(100))
		assert.Equal(t, 11, p.CalculateTotalPages(101))
		assert.Equal(t, 0, p.CalculateTotalPages(0))
	})

	t.Run("IsEnabled", func(t *testing.T) {
		assert.False(t, PaginationParams{}.IsEnabled())
		assert.True(t, PaginationParams{Limit: 10}.IsEnabled())
		assert.True(t, PaginationParams{Page: 1}.IsEnabled())
		assert.True(t, PaginationParams{Offset: 1}.IsEnabled())
		assert.True(t, PaginationParams{PageSize: 1}.IsEnabled())
	})
}

func TestPaginationParams_ApplyToSlice(t *testing.T) {
	items := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}

	tests := []struct {
		name   string
		params PaginationParams
		want   []int
	}{
		{
			name:   "limit only",
			params: PaginationParams{Limit: 3},
			want:   []int{0, 1, 2},
		},
		{
			name:   "offset only",
			params: PaginationParams{Offset: 7},
			want:   []int{7, 8, 9},
		},
		{
			name:   "offset and limit",
			params: PaginationParams{Offset: 2, Limit: 3},
			want:   []int{2, 3, 4},
		},
		{
			name:   "page 1",
			params: PaginationParams{Page: 1, PageSize: 3},
			want:   []int{0, 1, 2},
		},
		{
			name:   "page 2",
			params: PaginationParams{Page: 2, PageSize: 3},
			want:   []int{3, 4, 5},
		},
		{
			name:   "out of bounds offset",
			params: PaginationParams{Offset: 20},
			want:   []int{},
		},
		{
			name:   "out of bounds page",
			params: PaginationParams{Page: 10, PageSize: 3},
			want:   []int{9}, // Caps to last page [9]
		},
		{
			name:   "empty items",
			params: PaginationParams{Limit: 5},
			want:   []int{}, // Returns empty/same
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input []int
			if tt.name == "empty items" {
				input = []int{}
			} else {
				input = items
			}
			got := tt.params.ApplyToSlice(input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewPaginationMeta(t *testing.T) {
	tests := []struct {
		name       string
		params     PaginationParams
		totalCount int
		want       PaginationMeta
	}{
		{
			name:       "first page",
			params:     PaginationParams{Page: 1, PageSize: 10},
			totalCount: 25,
			want: PaginationMeta{
				CurrentPage: 1,
				PageSize:    10,
				TotalPages:  3,
				TotalItems:  25,
				HasPrevious: false,
				HasNext:     true,
			},
		},
		{
			name:       "middle page",
			params:     PaginationParams{Page: 2, PageSize: 10},
			totalCount: 25,
			want: PaginationMeta{
				CurrentPage: 2,
				PageSize:    10,
				TotalPages:  3,
				TotalItems:  25,
				HasPrevious: true,
				HasNext:     true,
			},
		},
		{
			name:       "last page",
			params:     PaginationParams{Page: 3, PageSize: 10},
			totalCount: 25,
			want: PaginationMeta{
				CurrentPage: 3,
				PageSize:    10,
				TotalPages:  3,
				TotalItems:  25,
				HasPrevious: true,
				HasNext:     false,
			},
		},
		{
			name:       "offset conversion",
			params:     PaginationParams{Offset: 10, Limit: 10},
			totalCount: 25,
			want: PaginationMeta{
				CurrentPage: 2,
				PageSize:    10,
				TotalPages:  3,
				TotalItems:  25,
				HasPrevious: true,
				HasNext:     true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewPaginationMeta(tt.params, tt.totalCount)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRecommendationSorter(t *testing.T) {
	sorter := NewRecommendationSorter()
	recs := []engine.Recommendation{
		{ResourceID: "aws:s3:Bucket/b1", EstimatedSavings: 10.0, Type: "MODIFY"},
		{ResourceID: "gcp:storage:Bucket/b2", EstimatedSavings: 5.0, Type: "TERMINATE"},
		{ResourceID: "aws:ec2:Instance/i1", EstimatedSavings: 20.0, Type: "RIGHTSIZE"},
	}

	t.Run("SortBySavingsAsc", func(t *testing.T) {
		sorted := sorter.Sort(recs, "savings", "asc")
		assert.Equal(t, 5.0, sorted[0].EstimatedSavings)
		assert.Equal(t, 10.0, sorted[1].EstimatedSavings)
		assert.Equal(t, 20.0, sorted[2].EstimatedSavings)
	})

	t.Run("SortBySavingsDesc", func(t *testing.T) {
		sorted := sorter.Sort(recs, "savings", "desc")
		assert.Equal(t, 20.0, sorted[0].EstimatedSavings)
		assert.Equal(t, 10.0, sorted[1].EstimatedSavings)
		assert.Equal(t, 5.0, sorted[2].EstimatedSavings)
	})

	t.Run("SortByName", func(t *testing.T) {
		sorted := sorter.Sort(recs, "name", "asc")
		assert.Equal(t, "aws:ec2:Instance/i1", sorted[0].ResourceID)
		assert.Equal(t, "aws:s3:Bucket/b1", sorted[1].ResourceID)
		assert.Equal(t, "gcp:storage:Bucket/b2", sorted[2].ResourceID)
	})

	t.Run("SortByProvider", func(t *testing.T) {
		sorted := sorter.Sort(recs, "provider", "asc")
		assert.Equal(t, "aws", extractProvider(sorted[0].ResourceID))
		assert.Equal(t, "aws", extractProvider(sorted[1].ResourceID))
		assert.Equal(t, "gcp", extractProvider(sorted[2].ResourceID))
	})

	t.Run("InvalidField", func(t *testing.T) {
		sorted := sorter.Sort(recs, "invalid", "asc")
		assert.Equal(t, recs, sorted)
	})

	t.Run("GetValidFields", func(t *testing.T) {
		fields := sorter.GetValidFields()
		assert.Contains(t, fields, "savings")
		assert.Contains(t, fields, "provider")
	})
}

func TestParseSortExpression(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		wantField string
		wantOrder string
		wantErr   bool
	}{
		{"valid asc", "savings:asc", "savings", "asc", false},
		{"valid desc", "savings:desc", "savings", "desc", false},
		{"field only", "savings", "savings", "desc", false},
		{"empty", "", "", "", true},
		{"too many parts", "a:b:c", "", "", true},
		{"invalid order", "savings:foo", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, order, err := ParseSortExpression(tt.expr)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantField, field)
				assert.Equal(t, tt.wantOrder, order)
			}
		})
	}
}
