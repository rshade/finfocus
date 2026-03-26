package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculatePluginTTL(t *testing.T) {
	defaultTTL := 3600

	tests := []struct {
		name        string
		expiresAtFn func() *time.Time
		defaultTTL  int
		wantTTL     int
		wantSkip    bool
		wantCapped  bool
		approxTTL   bool // when true, allow ±2s tolerance on wantTTL
	}{
		{
			name:        "nil expiresAt returns default",
			expiresAtFn: func() *time.Time { return nil },
			defaultTTL:  defaultTTL,
			wantTTL:     defaultTTL,
			wantSkip:    false,
		},
		{
			name:        "future timestamp returns remaining seconds",
			expiresAtFn: func() *time.Time { return timePtr(time.Now().Add(24 * time.Hour)) },
			defaultTTL:  defaultTTL,
			wantTTL:     86400,
			wantSkip:    false,
			approxTTL:   true,
		},
		{
			name:        "past timestamp returns skip=true",
			expiresAtFn: func() *time.Time { return timePtr(time.Now().Add(-1 * time.Hour)) },
			defaultTTL:  defaultTTL,
			wantTTL:     0,
			wantSkip:    true,
		},
		{
			name:        "current time returns skip=true",
			expiresAtFn: func() *time.Time { return timePtr(time.Now()) },
			defaultTTL:  defaultTTL,
			wantTTL:     0,
			wantSkip:    true,
		},
		{
			name:        "timestamp exceeding MaxTTLSeconds returns capped value",
			expiresAtFn: func() *time.Time { return timePtr(time.Now().Add(14 * 24 * time.Hour)) },
			defaultTTL:  defaultTTL,
			wantTTL:     MaxTTLSeconds,
			wantSkip:    false,
			wantCapped:  true,
		},
		{
			name:        "very short TTL under 60s is honored",
			expiresAtFn: func() *time.Time { return timePtr(time.Now().Add(30 * time.Second)) },
			defaultTTL:  defaultTTL,
			wantTTL:     30,
			wantSkip:    false,
			approxTTL:   true,
		},
		{
			name: "exactly MaxTTLSeconds is not capped",
			expiresAtFn: func() *time.Time {
				return timePtr(time.Now().Add(time.Duration(MaxTTLSeconds) * time.Second))
			},
			defaultTTL: defaultTTL,
			wantTTL:    MaxTTLSeconds,
			wantSkip:   false,
			wantCapped: false,
			approxTTL:  true,
		},
		{
			name:        "sub-second future timestamp rounds up to 1s",
			expiresAtFn: func() *time.Time { return timePtr(time.Now().Add(500 * time.Millisecond)) },
			defaultTTL:  defaultTTL,
			wantTTL:     1,
			wantSkip:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expiresAt := tt.expiresAtFn()
			gotTTL, gotSkip, gotCapped := CalculatePluginTTL(expiresAt, tt.defaultTTL)
			assert.Equal(t, tt.wantSkip, gotSkip, "skip mismatch")
			assert.Equal(t, tt.wantCapped, gotCapped, "capped mismatch")
			if tt.approxTTL {
				assert.InDelta(t, tt.wantTTL, gotTTL, 2, "TTL should be approximately %d", tt.wantTTL)
			} else {
				assert.Equal(t, tt.wantTTL, gotTTL, "TTL mismatch")
			}
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
