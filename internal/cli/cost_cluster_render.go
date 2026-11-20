package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/viewmodel"
)

func renderClusterResult(cmd *cobra.Command, format string, out engine.ClusterReport) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(cmd, out)
	case outputFormatNDJSON:
		enc := json.NewEncoder(cmd.OutOrStdout())
		summary := struct {
			engine.ClusterReport

			Type string `json:"type"`
			// Groups shadows the embedded field so the summary line omits it;
			// groups follow as one {"type":"group"} line each.
			Groups []engine.ClusterGroup `json:"groups,omitempty"`
		}{ClusterReport: out, Type: "summary"}
		if err := enc.Encode(summary); err != nil {
			return err
		}
		for _, g := range out.Groups {
			if err := enc.Encode(struct {
				engine.ClusterGroup

				Type string `json:"type"`
			}{g, "group"}); err != nil {
				return err
			}
		}
		return nil
	default:
		return renderClusterTable(cmd, out)
	}
}

func renderClusterTable(cmd *cobra.Command, out engine.ClusterReport) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, tabPadding, ' ', 0)
	fmt.Fprintln(w, "GROUP\tCPU\tMEMORY\tTOTAL\tNOTES")
	for _, g := range out.Groups {
		fmt.Fprintf(w, "%s\t%.2f\t%.2f\t%.2f\t%s\n", g.Key, g.CPUCost, g.MemCost, g.TotalCost,
			strings.Join(g.Notes, "; "))
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := renderCapacityTable(cmd, out); err != nil {
		return err
	}
	cmd.Println()
	cmd.Printf("Mode:    %s\n", viewmodel.ClusterModeLine(out.Mode, out.Period))
	cmd.Printf("Total:   $%.2f %s\n", out.Total, out.Currency)
	if out.Idle == nil {
		cmd.Printf("Idle:    omitted (%s scoped)\n", clusterScopeFlags(out))
	} else {
		cmd.Printf("Idle:    $%.2f (%s)\n", *out.Idle, viewmodel.ClusterShare(*out.Idle, out.Total))
	}
	cmd.Printf("Policy:  %s\n", viewmodel.ClusterPolicyLine(out.Policy.Source, out.Policy.Digest))
	if out.Incomplete {
		renderIncompleteFooter(cmd, out)
	}
	return nil
}

func renderCapacityTable(cmd *cobra.Command, out engine.ClusterReport) error {
	labeled := false
	for _, g := range out.Capacity {
		if g.CapacityType != engine.CapacityUnlabeled {
			labeled = true
			break
		}
	}
	if !labeled {
		return nil
	}
	cmd.Println()
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, tabPadding, ' ', 0)
	fmt.Fprintln(w, "CAPACITY TYPE\tNODES\tTOTAL\tSHARE\tIDLE\tNOTES")
	for _, g := range out.Capacity {
		idle := "-"
		if g.IdleCost != nil {
			idle = fmt.Sprintf("%.2f", *g.IdleCost)
		}
		fmt.Fprintf(w, "%s\t%d\t%.2f\t%s\t%s\t%s\n", g.CapacityType, g.Nodes, g.TotalCost,
			viewmodel.ClusterShare(g.TotalCost, out.Total),
			idle, strings.Join(g.Notes, "; "))
	}
	return w.Flush()
}

// clusterScopeFlags names the flags that scoped the report.
func clusterScopeFlags(out engine.ClusterReport) string {
	var flags []string
	if out.NamespaceScoped {
		flags = append(flags, "--namespace")
	}
	if out.SelectorScoped {
		flags = append(flags, "--selector")
	}
	return strings.Join(flags, " and ")
}

func renderPolicy(cmd *cobra.Command, format string, p engine.ClusterPolicy) error {
	if format != outputFormatTable {
		return writeJSON(cmd, p)
	}
	cmd.Printf("# source: %s\n# digest: %s\n", p.Source, p.Digest)
	var pretty map[string]any
	if err := json.Unmarshal(p.Effective, &pretty); err != nil {
		return fmt.Errorf("decode effective policy: %w", err)
	}
	return writeJSON(cmd, pretty)
}
