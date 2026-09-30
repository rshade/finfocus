package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractRunID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		body     string
		expected string
		wantErr  bool
	}{
		{
			name:     "valid run id",
			body:     "Workflow run https://github.com/rshade/finfocus/actions/runs/1234567890 failed",
			expected: "1234567890",
			wantErr:  false,
		},
		{
			name:     "no run id",
			body:     "Some other issue description",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "empty body",
			body:     "",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "partial url without run id",
			body:     "Check https://github.com/rshade/finfocus/actions for details",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := extractRunID(tt.body)
			if tt.wantErr {
				require.Error(t, err, "expected error for input: %q", tt.body)
			} else {
				require.NoError(t, err, "unexpected error for input: %q", tt.body)
			}
			assert.Equal(t, tt.expected, got, "extractRunID() result mismatch")
		})
	}
}
