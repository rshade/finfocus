package engine

import (
	"slices"
)

// DependencyGraph holds the relationships Pulumi records between the resources
// of one stack load. It is built outside this package because ingest imports
// engine, and it is read-only once built.
type DependencyGraph struct {
	// Available is false when the source carries no dependency data, as with a
	// Terraform plan or a state in which no resource depends on another.
	Available bool
	// Nodes maps each resource URN, components included, to its node.
	Nodes map[string]DependencyNode
}

// DependencyNode is one resource in a DependencyGraph.
type DependencyNode struct {
	// URN is the resource identity.
	URN string
	// Type is the resource type, so a dependent with no overview row (a
	// component, or a create the overview has not priced) still shows one.
	Type string
	// DependsOn lists the URNs this resource depends on: the union of Pulumi's
	// dependencies, every propertyDependencies list, deletedWith, replaceWith,
	// and the provider URN, sorted and without duplicates. Pulumi records a
	// dependency on a component as its members.
	DependsOn []string
	// Component is true for a resource that is not a custom resource.
	Component bool
	// Status is the lifecycle status a preview gives the resource, so a
	// dependent with no overview row (filtered out, or a create the overview has
	// not priced) still shows it. StatusActive when no preview step names it.
	Status ResourceStatus
}

// BlastRadius is the set of resources that depend, directly or through other
// resources, on one selected resource.
type BlastRadius struct {
	// URN is the selected resource.
	URN string `json:"urn"`
	// Available mirrors DependencyGraph.Available. When false, no other field
	// is set.
	Available bool `json:"available"`
	// Dependents lists the tree rows ordered by distance, then URN.
	Dependents []BlastDependent `json:"dependents"`
	// Count is the number of dependents, components excluded.
	Count int `json:"count"`
	// MonthlyTotal is the summed projected monthly cost of the priced
	// dependents that will exist. It is nil when withheld.
	MonthlyTotal *float64 `json:"monthlyTotal,omitempty"`
	// Currency is the currency of MonthlyTotal; blank means USD.
	Currency string `json:"currency"`
	// MixedCurrency is true when the counted dependents disagree on currency,
	// in which case MonthlyTotal is withheld.
	MixedCurrency bool `json:"mixedCurrency"`
	// LeftOut counts the dependents the total does not include, by reason.
	LeftOut BlastLeftOut `json:"leftOut"`
	// SummaryDisplay is the count and total line, such as
	// "2 dependents, $10.00/month".
	SummaryDisplay string `json:"summaryDisplay"`
	// NoteDisplay says which dependents the total leaves out and why; blank
	// when it leaves none out.
	NoteDisplay string `json:"noteDisplay"`
}

// BlastDependent is one resource in a BlastRadius tree.
type BlastDependent struct {
	// URN is the dependent's identity.
	URN string `json:"urn"`
	// Type is the resource type.
	Type string `json:"type"`
	// DisplayName is the rightmost URN component.
	DisplayName string `json:"displayName"`
	// Status is the resource lifecycle status.
	Status ResourceStatus `json:"status"`
	// Depth is the shortest distance from the selected resource; 1 is direct.
	Depth int `json:"depth"`
	// Via is the URN of the tree parent: the selected resource at depth 1, or an
	// earlier entry of Dependents.
	Via string `json:"via"`
	// MonthlyCost is the projected monthly cost; nil when unpriced, deleting or
	// a component.
	MonthlyCost *float64 `json:"monthlyCost"`
	// Priced is true when the row has a projected cost that did not fail.
	Priced bool `json:"priced"`
	// Component marks a resource listed for tree continuity only.
	Component bool `json:"component"`
	// MonthlyDisplay is the cost cell: the formatted cost, "no price", or "-"
	// for a deleting dependent or a component.
	MonthlyDisplay string `json:"monthlyDisplay"`
	// StatusLabel is the status marker (new, deleting, replacing, updating,
	// component); blank for an active dependent.
	StatusLabel string `json:"statusLabel"`
}

// BlastLeftOut counts dependents that the blast-radius total does not include.
type BlastLeftOut struct {
	// Deleting counts dependents a preview deletes.
	Deleting int `json:"deleting"`
	// Unpriced counts non-component, non-deleting dependents with no price.
	Unpriced int `json:"unpriced"`
	// Unresolved counts references to resources missing from the graph.
	Unresolved int `json:"unresolved"`
}

// ComputeBlastRadius returns the resources that depend on the resource at urn,
// directly or through other resources, on one basis: projected monthly cost.
// rows supplies type, status and cost; graph supplies the edges. It performs no
// I/O, never fails, and terminates on a cycle. An unavailable graph returns an
// unavailable result, and an unknown urn returns an empty available one.
func ComputeBlastRadius(graph DependencyGraph, rows []OverviewRow, urn string) BlastRadius {
	if !graph.Available {
		return BlastRadius{URN: urn}
	}

	rowByURN := make(map[string]*OverviewRow, len(rows))
	for i := range rows {
		rowByURN[rows[i].URN] = &rows[i]
	}

	result := BlastRadius{URN: urn, Available: true, Dependents: []BlastDependent{}}
	visited := map[string]bool{urn: true}
	index := reverseIndex(graph)
	totals := blastTotals{}
	unresolved := map[string]bool{}
	noteUnresolved(graph, graph.Nodes[urn], unresolved)

	level := []string{urn}
	for depth := 1; len(level) > 0; depth++ {
		var next []string
		for _, via := range level {
			for _, dependent := range index[via] {
				if visited[dependent] {
					continue
				}
				visited[dependent] = true
				next = append(next, dependent)
				node := graph.Nodes[dependent]
				noteUnresolved(graph, node, unresolved)
				row := rowByURN[dependent]
				entry := blastDependent(node, row, depth, via)
				result.Dependents = append(result.Dependents, entry)
				if entry.Component {
					continue
				}
				result.Count++
				switch {
				case entry.Status == StatusDeleting:
					result.LeftOut.Deleting++
				case !entry.Priced:
					result.LeftOut.Unpriced++
				default:
					totals.add(*entry.MonthlyCost, row.ProjectedCost.Currency)
				}
			}
		}
		slices.Sort(next)
		level = next
	}

	result.LeftOut.Unresolved = len(unresolved)
	totals.apply(&result)
	applyBlastDisplay(&result)
	return result
}

// blastTotals accumulates the summed cost and the currency of the counted rows.
type blastTotals struct {
	sum      float64
	currency string
	priced   bool
	mixed    bool
}

func (t *blastTotals) add(monthly float64, currency string) {
	if currency == "" {
		currency = "USD"
	}
	if t.priced && currency != t.currency {
		t.mixed = true
	}
	t.currency = currency
	t.sum += monthly
	t.priced = true
}

func (t *blastTotals) apply(result *BlastRadius) {
	result.MixedCurrency = t.mixed
	if !t.priced || t.mixed {
		return
	}
	total := t.sum
	result.MonthlyTotal = &total
	result.Currency = t.currency
}

// noteUnresolved records each reference of node that has no entry in the graph.
func noteUnresolved(graph DependencyGraph, node DependencyNode, unresolved map[string]bool) {
	for _, target := range node.DependsOn {
		if _, ok := graph.Nodes[target]; !ok {
			unresolved[target] = true
		}
	}
}

// reverseIndex maps each URN to the sorted URNs that list it in DependsOn.
func reverseIndex(graph DependencyGraph) map[string][]string {
	index := make(map[string][]string, len(graph.Nodes))
	for dependent, node := range graph.Nodes {
		for _, target := range node.DependsOn {
			index[target] = append(index[target], dependent)
		}
	}
	for target := range index {
		slices.Sort(index[target])
	}
	return index
}

func blastDependent(node DependencyNode, row *OverviewRow, depth int, via string) BlastDependent {
	entry := BlastDependent{
		URN:         node.URN,
		Type:        node.Type,
		DisplayName: ExtractResourceDisplayName(node.URN),
		Depth:       depth,
		Via:         via,
		Component:   node.Component,
	}
	if row != nil {
		if row.Type != "" {
			entry.Type = row.Type
		}
		entry.Status = row.Status
	}
	if node.Status == StatusDeleting || row == nil {
		entry.Status = node.Status
	}
	if row == nil || entry.Status == StatusDeleting {
		return entry
	}
	if !entry.Component && row.ProjectedCost != nil && !row.ProjectedPricingFailed {
		cost := row.ProjectedCost.MonthlyCost
		entry.MonthlyCost = &cost
		entry.Priced = true
	}
	return entry
}
