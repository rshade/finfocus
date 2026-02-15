package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetWorkerCount_WithJobsOverride(t *testing.T) {
	tests := []struct {
		name     string
		jobs     int
		jobCount int
		want     int
		autoMode bool // When true, just verify result > 0 (NumCPU-based)
	}{
		{
			name:     "zero job count returns 0 regardless of jobs setting",
			jobs:     4,
			jobCount: 0,
			want:     0,
		},
		{
			name:     "jobs override used when set and less than job count",
			jobs:     4,
			jobCount: 10,
			want:     4,
		},
		{
			name:     "jobs capped at resource count when jobs exceeds it",
			jobs:     10,
			jobCount: 3,
			want:     3,
		},
		{
			name:     "jobs equal to job count returns job count",
			jobs:     5,
			jobCount: 5,
			want:     5,
		},
		{
			name:     "jobs of 1 enforces serial execution",
			jobs:     1,
			jobCount: 100,
			want:     1,
		},
		{
			name:     "auto mode when jobs is 0",
			jobs:     0,
			jobCount: 10,
			autoMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng := New(nil, nil).WithJobs(tt.jobs)
			got := eng.getWorkerCount(tt.jobCount)
			if tt.autoMode {
				// Auto mode: result should be > 0 when jobCount > 0
				assert.Greater(t, got, 0, "auto mode should return positive worker count")
				assert.LessOrEqual(t, got, tt.jobCount, "worker count should not exceed job count")
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestWithJobs_BuilderPattern(t *testing.T) {
	// Verify WithJobs returns the same engine for chaining
	eng := New(nil, nil)
	result := eng.WithJobs(4)
	assert.Same(t, eng, result, "WithJobs should return the same engine instance")
	assert.Equal(t, 4, eng.jobs, "jobs field should be set")
}

func TestWithJobs_ZeroIsDefault(t *testing.T) {
	eng := New(nil, nil)
	assert.Equal(t, 0, eng.jobs, "default jobs should be 0 (auto)")
}

func TestWithJobs_ChainedWithOtherOptions(t *testing.T) {
	// Verify WithJobs works correctly when chained with other builder methods
	eng := New(nil, nil).
		WithJobs(8).
		WithCache(nil).
		WithRouter(nil)

	assert.Equal(t, 8, eng.jobs)
	// Verify getWorkerCount respects the jobs override
	got := eng.getWorkerCount(20)
	assert.Equal(t, 8, got)
}
