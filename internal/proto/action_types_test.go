package proto_test

import (
	"testing"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T011: Table-driven tests for ActionTypeLabel() covering all 11 types.
func TestActionTypeLabel(t *testing.T) {
	tests := []struct {
		name     string
		input    pbc.RecommendationActionType
		expected string
	}{
		{
			name:     "UNSPECIFIED",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_UNSPECIFIED,
			expected: "Unspecified",
		},
		{
			name:     "RIGHTSIZE",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
			expected: "Rightsize",
		},
		{
			name:     "TERMINATE",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_TERMINATE,
			expected: "Terminate",
		},
		{
			name:     "PURCHASE_COMMITMENT",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_PURCHASE_COMMITMENT,
			expected: "Purchase Commitment",
		},
		{
			name:     "ADJUST_REQUESTS",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_ADJUST_REQUESTS,
			expected: "Adjust Requests",
		},
		{
			name:     "MODIFY",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MODIFY,
			expected: "Modify",
		},
		{
			name:     "DELETE_UNUSED",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_DELETE_UNUSED,
			expected: "Delete Unused",
		},
		{
			name:     "MIGRATE",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
			expected: "Migrate",
		},
		{
			name:     "CONSOLIDATE",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_CONSOLIDATE,
			expected: "Consolidate",
		},
		{
			name:     "SCHEDULE",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_SCHEDULE,
			expected: "Schedule",
		},
		{
			name:     "REFACTOR",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_REFACTOR,
			expected: "Refactor",
		},
		{
			name:     "INVESTIGATE",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_INVESTIGATE,
			expected: "Investigate",
		},
		{
			name:     "OTHER",
			input:    pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_OTHER,
			expected: "Other",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := proto.ActionTypeLabel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// T012: Table-driven tests for ParseActionType() with valid/invalid inputs.
func TestParseActionType(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    pbc.RecommendationActionType
		expectError bool
	}{
		// Valid inputs - uppercase
		{
			name:     "RIGHTSIZE uppercase",
			input:    "RIGHTSIZE",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
		},
		{
			name:     "TERMINATE uppercase",
			input:    "TERMINATE",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_TERMINATE,
		},
		{
			name:     "PURCHASE_COMMITMENT uppercase",
			input:    "PURCHASE_COMMITMENT",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_PURCHASE_COMMITMENT,
		},
		{
			name:     "ADJUST_REQUESTS uppercase",
			input:    "ADJUST_REQUESTS",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_ADJUST_REQUESTS,
		},
		{
			name:     "MODIFY uppercase",
			input:    "MODIFY",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MODIFY,
		},
		{
			name:     "DELETE_UNUSED uppercase",
			input:    "DELETE_UNUSED",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_DELETE_UNUSED,
		},
		{
			name:     "MIGRATE uppercase",
			input:    "MIGRATE",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
		},
		{
			name:     "CONSOLIDATE uppercase",
			input:    "CONSOLIDATE",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_CONSOLIDATE,
		},
		{
			name:     "SCHEDULE uppercase",
			input:    "SCHEDULE",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_SCHEDULE,
		},
		{
			name:     "REFACTOR uppercase",
			input:    "REFACTOR",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_REFACTOR,
		},
		{
			name:     "INVESTIGATE uppercase",
			input:    "INVESTIGATE",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_INVESTIGATE,
		},
		{
			name:     "OTHER uppercase",
			input:    "OTHER",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_OTHER,
		},
		// Valid inputs - lowercase (case-insensitive)
		{
			name:     "rightsize lowercase",
			input:    "rightsize",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
		},
		{
			name:     "migrate lowercase",
			input:    "migrate",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
		},
		{
			name:     "purchase_commitment lowercase",
			input:    "purchase_commitment",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_PURCHASE_COMMITMENT,
		},
		// Valid inputs - mixed case
		{
			name:     "Migrate mixed case",
			input:    "Migrate",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
		},
		{
			name:     "Delete_Unused mixed case",
			input:    "Delete_Unused",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_DELETE_UNUSED,
		},
		// Valid with whitespace
		{
			name:     "with leading/trailing spaces",
			input:    "  RIGHTSIZE  ",
			expected: pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
		},
		// Invalid inputs
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "unknown type",
			input:       "UNKNOWN",
			expectError: true,
		},
		{
			name:        "invalid random string",
			input:       "not-a-valid-type",
			expectError: true,
		},
		{
			name:        "UNSPECIFIED not allowed in filter",
			input:       "UNSPECIFIED",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := proto.ParseActionType(tt.input)
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "invalid action type")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// T013: Table-driven tests for ParseActionTypeFilter() with comma-separated values.
func TestParseActionTypeFilter(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []pbc.RecommendationActionType
		expectError bool
	}{
		{
			name:  "single type",
			input: "MIGRATE",
			expected: []pbc.RecommendationActionType{
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
			},
		},
		{
			name:  "two types",
			input: "MIGRATE,RIGHTSIZE",
			expected: []pbc.RecommendationActionType{
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
			},
		},
		{
			name:  "three types",
			input: "CONSOLIDATE,SCHEDULE,REFACTOR",
			expected: []pbc.RecommendationActionType{
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_CONSOLIDATE,
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_SCHEDULE,
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_REFACTOR,
			},
		},
		{
			name:  "with spaces",
			input: "MIGRATE , RIGHTSIZE , TERMINATE",
			expected: []pbc.RecommendationActionType{
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_TERMINATE,
			},
		},
		{
			name:  "lowercase mixed",
			input: "migrate,Consolidate,SCHEDULE",
			expected: []pbc.RecommendationActionType{
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_CONSOLIDATE,
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_SCHEDULE,
			},
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "contains invalid type",
			input:       "MIGRATE,INVALID,RIGHTSIZE",
			expectError: true,
		},
		{
			name:        "only commas",
			input:       ",,,",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := proto.ParseActionTypeFilter(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// T014: Tests for ValidActionTypes() excluding UNSPECIFIED.
func TestValidActionTypes(t *testing.T) {
	types := proto.ValidActionTypes()

	// Should have exactly 12 types (excluding UNSPECIFIED)
	assert.Len(t, types, 12)

	// Should not contain UNSPECIFIED
	for _, at := range types {
		assert.NotEqual(t, "UNSPECIFIED", at)
	}

	// Should contain all the valid types
	expectedTypes := []string{
		"RIGHTSIZE",
		"TERMINATE",
		"PURCHASE_COMMITMENT",
		"ADJUST_REQUESTS",
		"MODIFY",
		"DELETE_UNUSED",
		"MIGRATE",
		"CONSOLIDATE",
		"SCHEDULE",
		"REFACTOR",
		"INVESTIGATE",
		"OTHER",
	}
	for _, expected := range expectedTypes {
		assert.Contains(t, types, expected)
	}
}

// T015: Tests for unknown/future enum value handling (display as "Unknown (N)").
func TestActionTypeLabel_UnknownValue(t *testing.T) {
	// Test with a hypothetical future value (high integer)
	unknownType := pbc.RecommendationActionType(999)
	result := proto.ActionTypeLabel(unknownType)

	assert.Contains(t, result, "Unknown")
	assert.Contains(t, result, "999")
}

// Test ActionTypeLabelFromString for string-based lookups.
func TestActionTypeLabelFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"RIGHTSIZE", "RIGHTSIZE", "Rightsize"},
		{"MIGRATE", "MIGRATE", "Migrate"},
		{"PURCHASE_COMMITMENT", "PURCHASE_COMMITMENT", "Purchase Commitment"},
		{"lowercase", "migrate", "Migrate"},
		{"unknown", "UNKNOWN", "UNKNOWN"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := proto.ActionTypeLabelFromString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test MatchesActionType for recommendation filtering.
func TestMatchesActionType(t *testing.T) {
	tests := []struct {
		name     string
		recType  string
		types    []pbc.RecommendationActionType
		expected bool
	}{
		{
			name:     "empty filter matches all",
			recType:  "MIGRATE",
			types:    []pbc.RecommendationActionType{},
			expected: true,
		},
		{
			name:    "matches single type",
			recType: "MIGRATE",
			types: []pbc.RecommendationActionType{
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
			},
			expected: true,
		},
		{
			name:    "matches in multiple types",
			recType: "RIGHTSIZE",
			types: []pbc.RecommendationActionType{
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
			},
			expected: true,
		},
		{
			name:    "no match in multiple types",
			recType: "TERMINATE",
			types: []pbc.RecommendationActionType{
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
			},
			expected: false,
		},
		{
			name:    "case insensitive match",
			recType: "migrate",
			types: []pbc.RecommendationActionType{
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
			},
			expected: true,
		},
		{
			name:    "with whitespace",
			recType: "  MIGRATE  ",
			types: []pbc.RecommendationActionType{
				pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := proto.MatchesActionType(tt.recType, tt.types)
			assert.Equal(t, tt.expected, result)
		})
	}
}
