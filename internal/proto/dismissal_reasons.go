package proto

import (
	"fmt"
	"strings"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// dismissalReasonLabels maps DismissalReason enum values to human-readable labels.
//
//nolint:gochecknoglobals // Intentional: static lookup table, avoiding allocation per call
var dismissalReasonLabels = map[pbc.DismissalReason]string{
	pbc.DismissalReason_DISMISSAL_REASON_UNSPECIFIED:          "Unspecified",
	pbc.DismissalReason_DISMISSAL_REASON_NOT_APPLICABLE:       "Not Applicable",
	pbc.DismissalReason_DISMISSAL_REASON_ALREADY_IMPLEMENTED:  "Already Implemented",
	pbc.DismissalReason_DISMISSAL_REASON_BUSINESS_CONSTRAINT:  "Business Constraint",
	pbc.DismissalReason_DISMISSAL_REASON_TECHNICAL_CONSTRAINT: "Technical Constraint",
	pbc.DismissalReason_DISMISSAL_REASON_DEFERRED:             "Deferred",
	pbc.DismissalReason_DISMISSAL_REASON_INACCURATE:           "Inaccurate",
	pbc.DismissalReason_DISMISSAL_REASON_OTHER:                "Other",
}

// dismissalReasonNames maps CLI flag values to proto enum values.
// UNSPECIFIED is excluded as it's not a valid user-selectable reason.
//
//nolint:gochecknoglobals // Intentional: static lookup table, avoiding allocation per call
var dismissalReasonNames = map[string]pbc.DismissalReason{
	"not-applicable":       pbc.DismissalReason_DISMISSAL_REASON_NOT_APPLICABLE,
	"already-implemented":  pbc.DismissalReason_DISMISSAL_REASON_ALREADY_IMPLEMENTED,
	"business-constraint":  pbc.DismissalReason_DISMISSAL_REASON_BUSINESS_CONSTRAINT,
	"technical-constraint": pbc.DismissalReason_DISMISSAL_REASON_TECHNICAL_CONSTRAINT,
	"deferred":             pbc.DismissalReason_DISMISSAL_REASON_DEFERRED,
	"inaccurate":           pbc.DismissalReason_DISMISSAL_REASON_INACCURATE,
	"other":                pbc.DismissalReason_DISMISSAL_REASON_OTHER,
}

// ParseDismissalReason parses a CLI flag value into a DismissalReason enum value.
// Matching is case-insensitive and whitespace is trimmed.
// Returns an error for unknown reason values, listing all valid options.
func ParseDismissalReason(s string) (pbc.DismissalReason, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, fmt.Errorf("invalid dismissal reason %q: empty string. Valid reasons: %s",
			s, strings.Join(ValidDismissalReasons(), ", "))
	}

	lowerReason := strings.ToLower(trimmed)

	if reason, ok := dismissalReasonNames[lowerReason]; ok {
		return reason, nil
	}

	return 0, fmt.Errorf("invalid dismissal reason %q. Valid reasons: %s",
		s, strings.Join(ValidDismissalReasons(), ", "))
}

// DismissalReasonLabel returns the human-readable label for a DismissalReason.
// For unknown/future enum values, it returns "Unknown (N)" where N is the integer value.
func DismissalReasonLabel(reason pbc.DismissalReason) string {
	if label, ok := dismissalReasonLabels[reason]; ok {
		return label
	}
	return fmt.Sprintf("Unknown (%d)", int32(reason))
}

// ValidDismissalReasons returns a sorted list of valid CLI flag values for dismissal reasons.
// UNSPECIFIED is excluded as it's not a valid user-selectable reason.
func ValidDismissalReasons() []string {
	// Return in a stable, logical order matching proto enum values
	return []string{
		"not-applicable",
		"already-implemented",
		"business-constraint",
		"technical-constraint",
		"deferred",
		"inaccurate",
		"other",
	}
}

// DismissalReasonFromProtoName converts a proto enum name (e.g., "BUSINESS_CONSTRAINT")
// to a CLI flag value (e.g., "business-constraint"). This is used when reading
// stored dismissal reasons from the local state file.
func DismissalReasonFromProtoName(protoName string) string {
	upper := strings.ToUpper(strings.TrimSpace(protoName))
	// Remove DISMISSAL_REASON_ prefix if present
	upper = strings.TrimPrefix(upper, "DISMISSAL_REASON_")

	switch upper {
	case "NOT_APPLICABLE":
		return "not-applicable"
	case "ALREADY_IMPLEMENTED":
		return "already-implemented"
	case "BUSINESS_CONSTRAINT":
		return "business-constraint"
	case "TECHNICAL_CONSTRAINT":
		return "technical-constraint"
	case "DEFERRED":
		return "deferred"
	case "INACCURATE":
		return "inaccurate"
	case "OTHER":
		return "other"
	default:
		return strings.ToLower(strings.ReplaceAll(upper, "_", "-"))
	}
}
