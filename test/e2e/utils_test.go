package e2e_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/test/e2e"
)

func TestParseTimeRange(t *testing.T) {
	tests := []struct {
		name     string
		start    string
		end      string
		wantErr  bool
		expected time.Duration // duration between start and end
	}{
		{
			name:     "RFC3339",
			start:    "2023-01-01T00:00:00Z",
			end:      "2023-01-01T01:00:00Z",
			wantErr:  false,
			expected: 1 * time.Hour,
		},
		{
			name:     "YYYY-MM-DD",
			start:    "2023-01-01",
			end:      "2023-01-02",
			wantErr:  false,
			expected: 24 * time.Hour,
		},
		{
			name:    "InvalidFormat",
			start:   "invalid",
			end:     "2023-01-02",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := e2e.ParseTimeRange(tt.start, tt.end)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, end.Sub(start))
			}
		})
	}
}
