package proto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestParseDismissalReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    pbc.DismissalReason
		wantErr     bool
		errContains string
	}{
		{
			name:     "not-applicable",
			input:    "not-applicable",
			expected: pbc.DismissalReason_DISMISSAL_REASON_NOT_APPLICABLE,
		},
		{
			name:     "already-implemented",
			input:    "already-implemented",
			expected: pbc.DismissalReason_DISMISSAL_REASON_ALREADY_IMPLEMENTED,
		},
		{
			name:     "business-constraint",
			input:    "business-constraint",
			expected: pbc.DismissalReason_DISMISSAL_REASON_BUSINESS_CONSTRAINT,
		},
		{
			name:     "technical-constraint",
			input:    "technical-constraint",
			expected: pbc.DismissalReason_DISMISSAL_REASON_TECHNICAL_CONSTRAINT,
		},
		{
			name:     "deferred",
			input:    "deferred",
			expected: pbc.DismissalReason_DISMISSAL_REASON_DEFERRED,
		},
		{
			name:     "inaccurate",
			input:    "inaccurate",
			expected: pbc.DismissalReason_DISMISSAL_REASON_INACCURATE,
		},
		{
			name:     "other",
			input:    "other",
			expected: pbc.DismissalReason_DISMISSAL_REASON_OTHER,
		},
		{
			name:     "case insensitive uppercase",
			input:    "DEFERRED",
			expected: pbc.DismissalReason_DISMISSAL_REASON_DEFERRED,
		},
		{
			name:     "case insensitive mixed",
			input:    "Business-Constraint",
			expected: pbc.DismissalReason_DISMISSAL_REASON_BUSINESS_CONSTRAINT,
		},
		{
			name:     "whitespace trimmed",
			input:    "  deferred  ",
			expected: pbc.DismissalReason_DISMISSAL_REASON_DEFERRED,
		},
		{
			name:        "empty string",
			input:       "",
			wantErr:     true,
			errContains: "empty string",
		},
		{
			name:        "whitespace only",
			input:       "   ",
			wantErr:     true,
			errContains: "empty string",
		},
		{
			name:        "invalid reason",
			input:       "fake-reason",
			wantErr:     true,
			errContains: "invalid dismissal reason",
		},
		{
			name:        "unspecified not allowed",
			input:       "unspecified",
			wantErr:     true,
			errContains: "invalid dismissal reason",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := ParseDismissalReason(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDismissalReasonLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		reason   pbc.DismissalReason
		expected string
	}{
		{
			name:     "not applicable",
			reason:   pbc.DismissalReason_DISMISSAL_REASON_NOT_APPLICABLE,
			expected: "Not Applicable",
		},
		{
			name:     "already implemented",
			reason:   pbc.DismissalReason_DISMISSAL_REASON_ALREADY_IMPLEMENTED,
			expected: "Already Implemented",
		},
		{
			name:     "business constraint",
			reason:   pbc.DismissalReason_DISMISSAL_REASON_BUSINESS_CONSTRAINT,
			expected: "Business Constraint",
		},
		{
			name:     "technical constraint",
			reason:   pbc.DismissalReason_DISMISSAL_REASON_TECHNICAL_CONSTRAINT,
			expected: "Technical Constraint",
		},
		{
			name:     "deferred",
			reason:   pbc.DismissalReason_DISMISSAL_REASON_DEFERRED,
			expected: "Deferred",
		},
		{
			name:     "inaccurate",
			reason:   pbc.DismissalReason_DISMISSAL_REASON_INACCURATE,
			expected: "Inaccurate",
		},
		{
			name:     "other",
			reason:   pbc.DismissalReason_DISMISSAL_REASON_OTHER,
			expected: "Other",
		},
		{
			name:     "unspecified",
			reason:   pbc.DismissalReason_DISMISSAL_REASON_UNSPECIFIED,
			expected: "Unspecified",
		},
		{
			name:     "unknown future value",
			reason:   pbc.DismissalReason(999),
			expected: "Unknown (999)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, DismissalReasonLabel(tt.reason))
		})
	}
}

func TestValidDismissalReasons(t *testing.T) {
	t.Parallel()

	reasons := ValidDismissalReasons()

	// Must return exactly as many reasons as dismissalReasonNames map entries
	assert.Len(t, reasons, len(dismissalReasonNames))

	// All reasons must be parseable
	for _, reason := range reasons {
		_, err := ParseDismissalReason(reason)
		require.NoError(t, err, "valid reason %q should parse", reason)
	}

	// Must not include unspecified
	for _, reason := range reasons {
		assert.NotEqual(t, "unspecified", reason)
	}
}

func TestDismissalReasonFromProtoName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		protoName string
		expected  string
	}{
		{
			name:      "NOT_APPLICABLE",
			protoName: "NOT_APPLICABLE",
			expected:  "not-applicable",
		},
		{
			name:      "ALREADY_IMPLEMENTED",
			protoName: "ALREADY_IMPLEMENTED",
			expected:  "already-implemented",
		},
		{
			name:      "BUSINESS_CONSTRAINT",
			protoName: "BUSINESS_CONSTRAINT",
			expected:  "business-constraint",
		},
		{
			name:      "with prefix",
			protoName: "DISMISSAL_REASON_DEFERRED",
			expected:  "deferred",
		},
		{
			name:      "lowercase input",
			protoName: "inaccurate",
			expected:  "inaccurate",
		},
		{
			name:      "unknown value",
			protoName: "SOME_FUTURE_VALUE",
			expected:  "some-future-value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, DismissalReasonFromProtoName(tt.protoName))
		})
	}
}
