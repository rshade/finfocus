package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
)

const retentionWarning = "incomplete: stored data starts at 2026-10-27T06:00:20Z, after the window start " +
	"2026-10-01T00:00:00Z; Prometheus retention may be shorter than the window"

func gapWarnings(pods int) []string {
	var out []string
	for _, metric := range []string{"cpu_usage", "mem_usage"} {
		for i := range pods {
			out = append(out, fmt.Sprintf("incomplete: ns/pod-%02d: gap in %s", i, metric))
		}
	}
	return out
}

func incompleteOutput(warnings []string, unpriced ...engine.PricedSummary) clusterOutput {
	out := sampleClusterOutput(false)
	out.Mode, out.Period = engine.ModeHistorical, "2026-10-01 to 2026-11-11, 41 days"
	out.Incomplete = true
	out.Warnings = warnings
	out.Priced = append([]engine.PricedSummary{{Kind: "node", ID: "n1", Priced: true}}, unpriced...)
	return out
}

func footer(t *testing.T, out clusterOutput) string {
	t.Helper()
	table := renderTo(t, outputFormatTable, out)
	_, after, found := strings.Cut(table, "Policy:")
	require.True(t, found, table)
	return after
}

func TestRenderCluster_IncompleteFooter(t *testing.T) {
	t.Parallel()

	unpriced := engine.PricedSummary{Kind: "node", ID: "n2", Note: "no price for m9.huge"}
	tests := []struct {
		name     string
		out      clusterOutput
		want     []string
		wantNone []string
	}{
		{
			name: "retention only",
			out:  incompleteOutput([]string{retentionWarning}),
			want: []string{
				"Incomplete: the usage source reported partial data\n",
				"  - stored data starts at 2026-10-27T06:00:20Z, after the window start 2026-10-01T00:00:00Z;",
			},
			wantNone: []string{"could not be priced", "()"},
		},
		{
			name: "gaps only",
			out:  incompleteOutput(gapWarnings(20)),
			want: []string{
				"Incomplete: the usage source reported partial data\n",
				"  - gap in cpu_usage for 20 resources\n",
				"  - gap in mem_usage for 20 resources\n",
			},
			wantNone: []string{"could not be priced", "ns/pod-05", "()"},
		},
		{
			name:     "unpriced only",
			out:      incompleteOutput(nil, unpriced),
			want:     []string{"Incomplete: 1 resource could not be priced (n2: no price for m9.huge)\n"},
			wantNone: []string{"partial data"},
		},
		{
			name: "mixed",
			out:  incompleteOutput(append([]string{retentionWarning}, gapWarnings(3)...), unpriced),
			want: []string{
				"Incomplete: the usage source reported partial data\n",
				"  - stored data starts at 2026-10-27T06:00:20Z",
				"  - gap in cpu_usage for 3 resources\n",
				"Incomplete: 1 resource could not be priced (n2: no price for m9.huge)\n",
			},
		},
		{
			name: "unpriced without a note",
			out:  incompleteOutput(nil, engine.PricedSummary{Kind: "node", ID: "n3"}),
			want: []string{"Incomplete: 1 resource could not be priced (n3)\n"},
		},
		{
			name:     "no stated cause",
			out:      incompleteOutput([]string{"allocator: rounding applied"}),
			want:     []string{"Incomplete: see the warnings on stderr\n"},
			wantNone: []string{"could not be priced", "()", "rounding"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := footer(t, tt.out)
			for _, w := range tt.want {
				assert.Contains(t, got, w)
			}
			for _, w := range tt.wantNone {
				assert.NotContains(t, got, w)
			}
		})
	}
}

func TestRenderCluster_CompleteHasNoIncompleteFooter(t *testing.T) {
	t.Parallel()

	out := incompleteOutput([]string{"allocator: rounding applied"})
	out.Incomplete = false
	assert.NotContains(t, renderTo(t, outputFormatTable, out), "Incomplete")
}

func TestRenderCluster_MachineOutputKeepsEveryWarning(t *testing.T) {
	t.Parallel()

	warnings := append(gapWarnings(20), retentionWarning)
	out := incompleteOutput(warnings)

	var doc clusterOutput
	require.NoError(t, json.Unmarshal([]byte(renderTo(t, outputFormatJSON, out)), &doc))
	assert.Equal(t, warnings, doc.Warnings)

	first, _, _ := strings.Cut(renderTo(t, outputFormatNDJSON, out), "\n")
	var summary clusterOutput
	require.NoError(t, json.Unmarshal([]byte(first), &summary))
	assert.Equal(t, warnings, summary.Warnings)
}

func warningsTo(t *testing.T, format string, warnings []string) []string {
	t.Helper()
	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetErr(&buf)
	printClusterWarnings(cmd, format, warnings)
	s := strings.TrimSuffix(buf.String(), "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func TestPrintClusterWarnings(t *testing.T) {
	t.Parallel()

	t.Run("store-wide first, gaps summarized", func(t *testing.T) {
		t.Parallel()

		lines := warningsTo(t, outputFormatTable, append(gapWarnings(20), retentionWarning))
		assert.Equal(t, []string{
			"warning: " + retentionWarning,
			"warning: incomplete: gap in cpu_usage for 20 resources (ns/pod-00, ns/pod-01, ns/pod-02, and 17 more)",
			"warning: incomplete: gap in mem_usage for 20 resources (ns/pod-00, ns/pod-01, ns/pod-02, and 17 more)",
			"warning: 40 warnings summarized above; --output json lists each one under \"warnings\"",
		}, lines)
	})

	t.Run("machine formats point at the warnings field", func(t *testing.T) {
		t.Parallel()

		for _, format := range []string{outputFormatJSON, outputFormatNDJSON} {
			lines := warningsTo(t, format, gapWarnings(4))
			require.Len(t, lines, 3, format)
			assert.Equal(t, "warning: 8 warnings summarized above; the \"warnings\" field lists each one", lines[2])
		}
	})

	t.Run("a short group lists every subject", func(t *testing.T) {
		t.Parallel()

		lines := warningsTo(t, outputFormatTable, []string{
			"incomplete: node b: cannot determine provider, instance type, or region",
			"incomplete: node a: cannot determine provider, instance type, or region",
		})
		assert.Equal(t, []string{
			"warning: incomplete: cannot determine provider, instance type, or region for 2 resources (node a, node b)",
			"warning: 2 warnings summarized above; --output json lists each one under \"warnings\"",
		}, lines)
	})

	t.Run("a repeated subject counts once", func(t *testing.T) {
		t.Parallel()

		in := []string{
			"incomplete: ns/worker-0: gap in cpu_usage",
			"incomplete: ns/worker-0: gap in cpu_usage",
			"incomplete: ns/worker-1: gap in cpu_usage",
			"incomplete: ns/solo: gap in mem_usage",
			"incomplete: ns/solo: gap in mem_usage",
		}
		lines := warningsTo(t, outputFormatTable, in)
		assert.Equal(t, []string{
			"warning: incomplete: ns/solo: gap in mem_usage",
			"warning: incomplete: ns/solo: gap in mem_usage",
			"warning: incomplete: gap in cpu_usage for 2 resources (ns/worker-0, ns/worker-1)",
			"warning: 3 warnings summarized above; --output json lists each one under \"warnings\"",
		}, lines)
		assert.Contains(t, footer(t, incompleteOutput(in)), "  - gap in cpu_usage for 2 resources\n")
	})

	t.Run("single and unprefixed warnings stay as written", func(t *testing.T) {
		t.Parallel()

		in := []string{
			"allocator: rounding applied",
			"incomplete: kubeconfig: no such file",
			"incomplete: ns/pod-00: gap in cpu_usage",
		}
		lines := warningsTo(t, outputFormatTable, in)
		assert.Equal(t, []string{
			"warning: allocator: rounding applied",
			"warning: incomplete: kubeconfig: no such file",
			"warning: incomplete: ns/pod-00: gap in cpu_usage",
		}, lines)
	})

	t.Run("no warnings print nothing", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, warningsTo(t, outputFormatTable, nil))
	})
}
