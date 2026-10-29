package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotsAreCurrent(t *testing.T) {
	t.Parallel()
	for _, count := range []int{10, 50, 100, 500, 1000} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			t.Parallel()
			want, err := os.ReadFile(filepath.Join("..", "..", "snapshots", fmt.Sprintf("plan-%d.json", count)))
			require.NoError(t, err)

			var got bytes.Buffer
			require.NoError(t, write(&got, count))
			assert.Equal(t, string(want), got.String(), "run generate-snapshots.sh to refresh plan-%d.json", count)
		})
	}
}

func TestGeneratePlanCounts(t *testing.T) {
	t.Parallel()
	tests := []struct {
		instances, volumes int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{5, 2},
		{7, 4},
		{1000, 400},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.instances), func(t *testing.T) {
			t.Parallel()
			byType := map[string]int{}
			for _, step := range generatePlan(tt.instances).Steps {
				require.Equal(t, step.URN, step.NewState.URN)
				byType[step.NewState.Type]++
			}
			assert.Equal(t, tt.instances, byType["aws:ec2/instance:Instance"])
			assert.Equal(t, tt.volumes, byType["aws:ebs/volume:Volume"])
			assert.Equal(t, 1, byType["pulumi:pulumi:Stack"])
			assert.Equal(t, 1, byType["pulumi:providers:aws"])
		})
	}
}
