//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scalePoint struct {
	instances   int
	maxWallTime time.Duration
}

func (sp scalePoint) planPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(fmt.Sprintf("fixtures/scalability/snapshots/plan-%d.json", sp.instances))
	require.NoError(t, err)
	return path
}

// pricedResources counts the plan's steps that cost projected reports: every
// step except the pulumi: stack and provider.
func pricedResources(t *testing.T, planPath string) int {
	t.Helper()
	data, err := os.ReadFile(planPath)
	require.NoError(t, err)
	var plan struct {
		Steps []struct {
			NewState struct {
				Type string `json:"type"`
			} `json:"newState"`
		} `json:"steps"`
	}
	require.NoError(t, json.Unmarshal(data, &plan))
	count := 0
	for _, step := range plan.Steps {
		if !strings.HasPrefix(step.NewState.Type, "pulumi:") {
			count++
		}
	}
	return count
}

// outputFormatTimeout bounds each output-format run, which only checks shape.
const outputFormatTimeout = 60 * time.Second

// runProjected runs cost projected with the cache disabled, so every run
// prices each resource instead of reusing an earlier scale point's results,
// and kills the CLI once timeout passes.
func runProjected(t *testing.T, binary, planPath, format string, timeout time.Duration) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := newCommand(ctx, binary, "cost", "projected",
		"--pulumi-json", planPath, "--output", format, "--cache-ttl", "0")
	out, err := cmd.Output()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		require.FailNow(t, fmt.Sprintf("cost projected did not finish within %v", timeout))
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			require.NoError(t, err, "command failed with stderr: %s", string(exitErr.Stderr))
		}
		require.NoError(t, err)
	}
	return out
}

//nolint:paralleltest // Wall-time bounds need each CLI run to have the machine to itself.
func TestScalability_ProjectedCost(t *testing.T) {
	binary := findFinFocusBinary()
	require.NotEmpty(t, binary)

	scalePoints := []scalePoint{
		{instances: 10, maxWallTime: 5 * time.Second},
		{instances: 50, maxWallTime: 10 * time.Second},
		{instances: 100, maxWallTime: 20 * time.Second},
		{instances: 500, maxWallTime: 60 * time.Second},
		{instances: 1000, maxWallTime: 120 * time.Second},
	}

	for _, sp := range scalePoints {
		t.Run(fmt.Sprintf("instances-%d", sp.instances), func(t *testing.T) {
			planPath := sp.planPath(t)
			want := pricedResources(t, planPath)

			start := time.Now()
			out := runProjected(t, binary, planPath, "json", sp.maxWallTime)
			elapsed := time.Since(start)

			t.Logf("%d instances, %d resources: %v total, %v/instance, %v/resource",
				sp.instances, want, elapsed,
				elapsed/time.Duration(sp.instances), elapsed/time.Duration(want))
			assert.LessOrEqual(t, elapsed, sp.maxWallTime, "wall time bound exceeded")

			var result struct {
				FinFocus struct {
					Summary   map[string]any   `json:"summary"`
					Resources []map[string]any `json:"resources"`
					Diff      struct {
						Creates int `json:"creates"`
					} `json:"diff"`
				} `json:"finfocus"`
			}
			require.NoError(t, json.Unmarshal(out, &result))
			assert.NotNil(t, result.FinFocus.Summary)
			assert.Len(t, result.FinFocus.Resources, want)
			assert.Equal(t, want, result.FinFocus.Diff.Creates)
		})
	}
}

func TestScalability_OutputFormats(t *testing.T) {
	t.Parallel()
	binary := findFinFocusBinary()
	require.NotEmpty(t, binary)

	sp := scalePoint{instances: 100}
	planPath := sp.planPath(t)
	want := pricedResources(t, planPath)

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		var result struct {
			FinFocus struct {
				Resources []json.RawMessage `json:"resources"`
			} `json:"finfocus"`
		}
		require.NoError(t, json.Unmarshal(runProjected(t, binary, planPath, "json", outputFormatTimeout), &result))
		assert.Len(t, result.FinFocus.Resources, want)
	})

	t.Run("ndjson", func(t *testing.T) {
		t.Parallel()
		out := runProjected(t, binary, planPath, "ndjson", outputFormatTimeout)
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		require.Len(t, lines, want)
		for _, line := range lines {
			var row map[string]any
			require.NoError(t, json.Unmarshal([]byte(line), &row), "invalid NDJSON line: %s", line)
			assert.Contains(t, row, "resourceType")
			assert.Contains(t, row, "resourceId")
			assert.Equal(t, "create", row["operation"])
		}
	})

	t.Run("table", func(t *testing.T) {
		t.Parallel()
		out := string(runProjected(t, binary, planPath, "table", outputFormatTimeout))
		assert.Contains(t, out, "COST DIFF")
		assert.Contains(t, out, fmt.Sprintf("Resources  %d (%d create,", want, want))
		assert.Equal(t, sp.instances, strings.Count(out, "aws:ec2/instance:Instance"))
	})
}
