package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/engine"
)

const digestDisplayLen = 12

// percentMultiplier converts an idle/total ratio to a percentage.
const percentMultiplier = 100

// clusterPolicyOutput is the policy section of the cluster JSON contract.
type clusterPolicyOutput struct {
	Source    string          `json:"source"` // path or "built-in defaults"
	Digest    string          `json:"digest"`
	Effective json.RawMessage `json:"effective"`
}

// clusterOutput is the stable JSON contract for `cost cluster`.
type clusterOutput struct {
	Mode            string                 `json:"mode"`   // "run-rate"
	Period          string                 `json:"period"` // "monthly"
	Currency        string                 `json:"currency"`
	GroupBy         string                 `json:"group_by"`
	Total           float64                `json:"total"`
	Idle            *float64               `json:"idle,omitempty"` // nil when namespace- or selector-scoped
	NamespaceScoped bool                   `json:"namespace_scoped"`
	SelectorScoped  bool                   `json:"selector_scoped"`
	Incomplete      bool                   `json:"incomplete"`
	Groups          []engine.ClusterGroup  `json:"groups"`
	Capacity        []engine.CapacityGroup `json:"capacity"`
	Priced          []engine.PricedSummary `json:"priced"`
	Policy          clusterPolicyOutput    `json:"policy"`
	Warnings        []string               `json:"warnings,omitempty"`
}

func newClusterOutput(
	res *engine.ClusterResult, groups []engine.ClusterGroup, groupBy string, policy clusterPolicyOutput,
) clusterOutput {
	out := clusterOutput{
		Mode: res.Mode, Period: res.Period, Currency: res.Currency, GroupBy: groupBy, Total: res.Total,
		NamespaceScoped: res.NamespaceScoped, SelectorScoped: res.SelectorScoped, Incomplete: res.Incomplete,
		Groups: groups, Capacity: capacityOrEmpty(res.Capacity), Priced: res.Priced,
		Policy: policy, Warnings: res.Warnings,
	}
	if !res.Scoped() {
		idle := res.Idle
		out.Idle = &idle
	}
	return out
}

func capacityOrEmpty(c []engine.CapacityGroup) []engine.CapacityGroup {
	if c == nil {
		return []engine.CapacityGroup{}
	}
	return c
}

func renderClusterResult(cmd *cobra.Command, format string, out clusterOutput) error {
	switch format {
	case outputFormatJSON:
		return writeJSON(cmd, out)
	case outputFormatNDJSON:
		enc := json.NewEncoder(cmd.OutOrStdout())
		summary := struct {
			clusterOutput

			Type string `json:"type"`
			// Groups shadows the embedded field so the summary line omits it;
			// groups follow as one {"type":"group"} line each.
			Groups []engine.ClusterGroup `json:"groups,omitempty"`
		}{clusterOutput: out, Type: "summary"}
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

func renderClusterTable(cmd *cobra.Command, out clusterOutput) error {
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
	cmd.Printf("Mode:    %s\n", clusterModeLine(out))
	cmd.Printf("Total:   $%.2f %s\n", out.Total, out.Currency)
	if out.Idle == nil {
		cmd.Printf("Idle:    omitted (%s scoped)\n", clusterScopeFlags(out))
	} else {
		pct := 0.0
		if out.Total > 0 {
			pct = *out.Idle / out.Total * percentMultiplier
		}
		cmd.Printf("Idle:    $%.2f (%.1f%%)\n", *out.Idle, pct)
	}
	cmd.Printf("Policy:  %s · %s\n", out.Policy.Source, shortDigest(out.Policy.Digest))
	if out.Incomplete {
		renderIncompleteFooter(cmd, out)
	}
	return nil
}

func renderCapacityTable(cmd *cobra.Command, out clusterOutput) error {
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
		share := 0.0
		if out.Total > 0 {
			share = g.TotalCost / out.Total * percentMultiplier
		}
		idle := "-"
		if g.IdleCost != nil {
			idle = fmt.Sprintf("%.2f", *g.IdleCost)
		}
		fmt.Fprintf(w, "%s\t%d\t%.2f\t%.1f%%\t%s\t%s\n", g.CapacityType, g.Nodes, g.TotalCost, share,
			idle, strings.Join(g.Notes, "; "))
	}
	return w.Flush()
}

// clusterScopeFlags names the flags that scoped the report.
func clusterScopeFlags(out clusterOutput) string {
	var flags []string
	if out.NamespaceScoped {
		flags = append(flags, "--namespace")
	}
	if out.SelectorScoped {
		flags = append(flags, "--selector")
	}
	return strings.Join(flags, " and ")
}

func clusterModeLine(out clusterOutput) string {
	if out.Mode == engine.ModeHistorical {
		return fmt.Sprintf("%s (%s)", out.Mode, out.Period)
	}
	return fmt.Sprintf("%s (monthly, %d h)", out.Mode, engine.HoursPerMonth)
}

func renderPolicy(cmd *cobra.Command, format string, p clusterPolicyOutput) error {
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

func shortDigest(d string) string {
	if len(d) > digestDisplayLen {
		return d[:digestDisplayLen]
	}
	return d
}
