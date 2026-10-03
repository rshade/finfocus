package engine

import (
	"fmt"
	"sort"
	"strings"
)

// GroupKeyNone groups rows that lack the requested dimension.
const GroupKeyNone = "<none>"

const labelGroupPrefix = "label:"

// pulumiURNSubjectKey is the usage subject copied from the annotation
// finfocus.dev/pulumi-urn. The label. prefix is required by stats validation.
const pulumiURNSubjectKey = "label.finfocus.dev/pulumi-urn"

const groupByPulumiStack = "pulumi-stack"

const groupByChoices = "namespace, controller, pod, node, pulumi-stack, label:<key>"

// ClusterGroup is the aggregate of cluster rows sharing a group key.
type ClusterGroup struct {
	Key        string   `json:"key"`
	CPUCost    float64  `json:"cpu_cost"`
	MemCost    float64  `json:"mem_cost"`
	TotalCost  float64  `json:"total_cost"`
	Rows       int      `json:"rows"`
	Notes      []string `json:"notes,omitempty"`
	PulumiURNs []string `json:"pulumi_urns,omitempty"`
}

// ValidateClusterGroupBy rejects unknown grouping dimensions.
func ValidateClusterGroupBy(groupBy string) error {
	switch groupBy {
	case "namespace", "controller", "pod", "node", groupByPulumiStack:
		return nil
	}
	if strings.HasPrefix(groupBy, labelGroupPrefix) && len(groupBy) > len(labelGroupPrefix) {
		return nil
	}
	return fmt.Errorf("invalid --group-by %q: use one of %s", groupBy, groupByChoices)
}

// GroupClusterRows aggregates rows by the requested dimension.
func GroupClusterRows(rows []ClusterRow, groupBy string) ([]ClusterGroup, error) {
	if err := ValidateClusterGroupBy(groupBy); err != nil {
		return nil, err
	}
	groups := map[string]*ClusterGroup{}
	notes := map[string]map[string]bool{}
	urns := map[string]map[string]bool{}
	for _, r := range rows {
		key := clusterGroupKey(r.Subject, groupBy)
		g := groups[key]
		if g == nil {
			g = &ClusterGroup{Key: key}
			groups[key] = g
			notes[key] = map[string]bool{}
			urns[key] = map[string]bool{}
		}
		g.CPUCost += r.CPUCost
		g.MemCost += r.MemCost
		g.TotalCost += r.TotalCost
		g.Rows++
		if r.Note != "" {
			notes[key][r.Note] = true
		}
		if urn := r.Subject[pulumiURNSubjectKey]; urn != "" {
			urns[key][urn] = true
		}
	}
	out := make([]ClusterGroup, 0, len(groups))
	for key, g := range groups {
		for n := range notes[key] {
			g.Notes = append(g.Notes, n)
		}
		sort.Strings(g.Notes)
		for urn := range urns[key] {
			g.PulumiURNs = append(g.PulumiURNs, urn)
		}
		sort.Strings(g.PulumiURNs)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TotalCost != out[j].TotalCost {
			return out[i].TotalCost > out[j].TotalCost
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

func clusterGroupKey(s map[string]string, groupBy string) string {
	switch s[subjectKeyKind] {
	case rowKindCluster:
		return rowKindCluster
	case rowKindIdle:
		if groupBy == subjectKeyNode {
			return orNone(s[subjectKeyNode])
		}
		return rowKindIdle
	}
	switch groupBy {
	case "namespace":
		return orNone(s["namespace"])
	case "controller":
		return orNone(s["namespace"]) + "/" + orNone(s["controller_kind"]) + "/" + orNone(s["controller"])
	case "pod":
		return orNone(s["namespace"]) + "/" + orNone(s["pod"])
	case subjectKeyNode:
		return orNone(s[subjectKeyNode])
	case groupByPulumiStack:
		return pulumiStackGroup(s[pulumiURNSubjectKey])
	default:
		return orNone(s["label."+strings.TrimPrefix(groupBy, labelGroupPrefix)])
	}
}

// pulumiStackGroup returns stack/project from a Pulumi URN. Anything that is
// not urn:pulumi:<stack>::<project>::<type>::<name> (parent types allowed in
// the type segment) groups under GroupKeyNone. The row is still counted.
func pulumiStackGroup(urn string) string {
	const prefix = "urn:pulumi:"
	if !strings.HasPrefix(urn, prefix) {
		return GroupKeyNone
	}
	stack, rest, ok := strings.Cut(urn[len(prefix):], "::")
	if !ok || stack == "" {
		return GroupKeyNone
	}
	project, rest, ok := strings.Cut(rest, "::")
	if !ok || project == "" || !strings.Contains(rest, "::") {
		return GroupKeyNone
	}
	return stack + "/" + project
}

func orNone(v string) string {
	if v == "" {
		return GroupKeyNone
	}
	return v
}
