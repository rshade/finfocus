package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDeriveActualCostWindowNoteUsesOneTimeZone pins the note's dates to a single
// time zone. A bare --from date parses as UTC midnight while --to defaults to
// [time.Now] in the local zone, so formatting each bound in its own location
// printed a "to" date that fell before the "from" date near a UTC day boundary.
func TestDeriveActualCostWindowNoteUsesOneTimeZone(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		from time.Time
		to   time.Time
		want string
	}{
		{
			name: "local end just past UTC midnight keeps both dates equal",
			from: time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2026, 11, 15, 20, 9, 0, 0, time.FixedZone("CST", -6*60*60)),
			want: "Actual cost from 2026-11-16 to 2026-11-16",
		},
		{
			name: "local end on the previous UTC date keeps the order",
			from: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2026, 2, 28, 23, 0, 0, 0, time.FixedZone("CST", -6*60*60)),
			want: "Actual cost from 2026-03-01 to 2026-03-01",
		},
		{
			name: "UTC bounds render unchanged",
			from: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			to:   time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC),
			want: "Actual cost from 2026-03-01 to 2026-03-08",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, _, notes := deriveActualCostWindow(1, tt.from, tt.to)
			assert.Equal(t, tt.want, notes)
		})
	}
}
