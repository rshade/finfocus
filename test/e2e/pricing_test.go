package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFixtureMonthlyCost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dir  string
		want float64
	}{
		// t2.micro plus a 10 GB gp2 volume; the nightly E2E log measured
		// finfocus.summary.totalMonthly 9.468 for this fixture.
		{dir: "fixtures/ec2", want: 9.468},
		{dir: "projects/ec2", want: 7.59},
	}
	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			t.Parallel()
			assert.InDelta(t, tt.want, FixtureMonthlyCost(t, tt.dir), 0.001)
		})
	}
}
