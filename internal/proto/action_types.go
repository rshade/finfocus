package proto

import (
	"errors"
	"fmt"
	"strings"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// actionTypeLabels maps RecommendationActionType enum values to human-readable labels.
// These labels are used for display in TUI and table output.
//
//nolint:gochecknoglobals // Intentional: static lookup table, avoiding allocation per call
var actionTypeLabels = map[pbc.RecommendationActionType]string{
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_UNSPECIFIED:         "Unspecified",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE:           "Rightsize",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_TERMINATE:           "Terminate",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_PURCHASE_COMMITMENT: "Purchase Commitment",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_ADJUST_REQUESTS:     "Adjust Requests",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MODIFY:              "Modify",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_DELETE_UNUSED:       "Delete Unused",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE:             "Migrate",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_CONSOLIDATE:         "Consolidate",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_SCHEDULE:            "Schedule",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_REFACTOR:            "Refactor",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_INVESTIGATE:         "Investigate",
	pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_OTHER:               "Other",
}

// actionTypeNames maps short names (for filter parsing) to proto enum values.
// UNSPECIFIED is excluded as it's not a valid filter value.
//
//nolint:gochecknoglobals // Intentional: static lookup table, avoiding allocation per call
var actionTypeNames = map[string]pbc.RecommendationActionType{
	"RIGHTSIZE":           pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_RIGHTSIZE,
	"TERMINATE":           pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_TERMINATE,
	"PURCHASE_COMMITMENT": pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_PURCHASE_COMMITMENT,
	"ADJUST_REQUESTS":     pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_ADJUST_REQUESTS,
	"MODIFY":              pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MODIFY,
	"DELETE_UNUSED":       pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_DELETE_UNUSED,
	"MIGRATE":             pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_MIGRATE,
	"CONSOLIDATE":         pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_CONSOLIDATE,
	"SCHEDULE":            pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_SCHEDULE,
	"REFACTOR":            pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_REFACTOR,
	"INVESTIGATE":         pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_INVESTIGATE,
	"OTHER":               pbc.RecommendationActionType_RECOMMENDATION_ACTION_TYPE_OTHER,
}

// stringLabels maps short names to human-readable labels for string-based lookups.
//
//nolint:gochecknoglobals // Intentional: static lookup table, avoiding allocation per call
var stringLabels = map[string]string{
	"RIGHTSIZE":           "Rightsize",
	"TERMINATE":           "Terminate",
	"PURCHASE_COMMITMENT": "Purchase Commitment",
	"ADJUST_REQUESTS":     "Adjust Requests",
	"MODIFY":              "Modify",
	"DELETE_UNUSED":       "Delete Unused",
	"MIGRATE":             "Migrate",
	"CONSOLIDATE":         "Consolidate",
	"SCHEDULE":            "Schedule",
	"REFACTOR":            "Refactor",
	"INVESTIGATE":         "Investigate",
	"OTHER":               "Other",
}

// ActionTypeLabel returns the human-readable label for a RecommendationActionType.
// For unknown/future enum values, it returns "Unknown (N)" where N is the integer value.
func ActionTypeLabel(at pbc.RecommendationActionType) string {
	if label, ok := actionTypeLabels[at]; ok {
		return label
	}
	return fmt.Sprintf("Unknown (%d)", int32(at))
}

// ActionTypeLabelFromString returns the human-readable label for an action type string.
// For unknown types, it returns the input string unchanged.
// This is useful for converting stored/serialized action type strings to display labels.
func ActionTypeLabelFromString(actionType string) string {
	if actionType == "" {
		return ""
	}
	upperType := strings.ToUpper(strings.TrimSpace(actionType))
	if label, ok := stringLabels[upperType]; ok {
		return label
	}
	return actionType
}

// ParseActionType parses a string into a RecommendationActionType enum value.
// Matching is case-insensitive and whitespace is trimmed.
// Returns an error for unknown type names, listing all valid options.
// UNSPECIFIED is not allowed as a filter value.
func ParseActionType(s string) (pbc.RecommendationActionType, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, fmt.Errorf("invalid action type %q: empty string. Valid types: %s",
			s, strings.Join(ValidActionTypes(), ", "))
	}

	upperType := strings.ToUpper(trimmed)

	// Check if it's UNSPECIFIED (not allowed in filters)
	if upperType == "UNSPECIFIED" {
		return 0, fmt.Errorf("invalid action type %q: UNSPECIFIED is not allowed. Valid types: %s",
			s, strings.Join(ValidActionTypes(), ", "))
	}

	if at, ok := actionTypeNames[upperType]; ok {
		return at, nil
	}

	return 0, fmt.Errorf("invalid action type %q. Valid types: %s",
		s, strings.Join(ValidActionTypes(), ", "))
}

// ParseActionTypeFilter parses a comma-separated list of action types.
// Each type is validated and converted to a RecommendationActionType enum value.
// Returns an error if any type is invalid or if the input is empty.
func ParseActionTypeFilter(filter string) ([]pbc.RecommendationActionType, error) {
	trimmed := strings.TrimSpace(filter)
	if trimmed == "" {
		return nil, errors.New("invalid action type filter: empty string")
	}

	parts := strings.Split(trimmed, ",")
	result := make([]pbc.RecommendationActionType, 0, len(parts))

	for _, part := range parts {
		trimmedPart := strings.TrimSpace(part)
		if trimmedPart == "" {
			continue
		}

		at, err := ParseActionType(trimmedPart)
		if err != nil {
			return nil, err
		}
		result = append(result, at)
	}

	if len(result) == 0 {
		return nil, errors.New("invalid action type filter: no valid types found")
	}

	return result, nil
}

// ValidActionTypes returns a list of valid action type short names for filter expressions.
// UNSPECIFIED is excluded as it's not a valid filter value.
func ValidActionTypes() []string {
	types := make([]string, 0, len(actionTypeNames))
	for name := range actionTypeNames {
		types = append(types, name)
	}
	return types
}

// MatchesActionType checks if a recommendation's action type matches any of the given types.
// The rec parameter should have a Type field containing the action type string.
// Matching is case-insensitive.
func MatchesActionType(recType string, types []pbc.RecommendationActionType) bool {
	if len(types) == 0 {
		return true // No filter means match all
	}

	upperRecType := strings.ToUpper(strings.TrimSpace(recType))

	for _, filterType := range types {
		// Get the short name for the filter type
		for name, at := range actionTypeNames {
			if at == filterType && name == upperRecType {
				return true
			}
		}
	}
	return false
}
