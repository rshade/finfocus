package stackload

import (
	"maps"
	"slices"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// BuildDependencyGraph converts deployed resources, components included, into
// an engine.DependencyGraph. A resource depends on everything its
// dependencies, propertyDependencies, deletedWith and replaceWith name, and on
// the provider its provider reference names: the links Pulumi's own dependents
// query follows, except parent. The graph is available only when at least one
// resource depends on another.
func BuildDependencyGraph(resources []ingest.StackExportResource) engine.DependencyGraph {
	graph := engine.DependencyGraph{Nodes: make(map[string]engine.DependencyNode, len(resources))}
	for _, r := range resources {
		if r.URN == "" {
			continue
		}
		deps := resourceLinks{
			dependencies:         r.Dependencies,
			propertyDependencies: r.PropertyDependencies,
			provider:             r.Provider,
			deletedWith:          r.DeletedWith,
			replaceWith:          r.ReplaceWith,
		}.dependsOn()
		graph.Nodes[r.URN] = engine.DependencyNode{
			URN:       r.URN,
			Type:      r.Type,
			DependsOn: deps,
			Component: !r.Custom,
		}
		if len(deps) > 0 {
			graph.Available = true
		}
	}
	return graph
}

// resourceLinks holds the links Pulumi records on one resource that make it
// depend on another.
type resourceLinks struct {
	dependencies         []string
	propertyDependencies map[string][]string
	provider             string
	deletedWith          string
	replaceWith          []string
}

// dependsOn returns every URN the links name, sorted and without duplicates
// or empty values, or nil when there is none.
func (l resourceLinks) dependsOn() []string {
	merged := slices.Concat(l.dependencies, l.replaceWith, []string{l.deletedWith, providerURN(l.provider)})
	for _, urns := range l.propertyDependencies {
		merged = append(merged, urns...)
	}
	merged = slices.DeleteFunc(merged, func(urn string) bool { return urn == "" })
	if len(merged) == 0 {
		return nil
	}
	slices.Sort(merged)
	return slices.Compact(merged)
}

// providerURN returns the URN in a Pulumi provider reference, which has the
// form <provider-urn>::<provider-id>. URNs contain "::" themselves, so the
// reference is cut at its last one; a reference without one names nothing.
func providerURN(reference string) string {
	if i := strings.LastIndex(reference, "::"); i > 0 {
		return reference[:i]
	}
	return ""
}

// OverlayPlanGraph returns base with the preview's changes applied: a create,
// update or replace step sets the node's edges from the step's new state, and
// every step sets the node's Status from its operation, and a delete step keeps
// the node. Unchanged and omitted resources keep their state edges. base is not
// modified, and a nil plan returns base.
func OverlayPlanGraph(base engine.DependencyGraph, plan *ingest.PulumiPlan) engine.DependencyGraph {
	if plan == nil {
		return base
	}
	graph := engine.DependencyGraph{
		Available: base.Available,
		Nodes:     make(map[string]engine.DependencyNode, len(base.Nodes)+len(plan.Steps)),
	}
	maps.Copy(graph.Nodes, base.Nodes)
	for _, step := range plan.Steps {
		if step.URN == "" {
			continue
		}
		switch step.Op {
		case engine.DiffOperationCreate, engine.DiffOperationUpdate, opReplace, opCreateReplacement:
			node := graph.Nodes[step.URN]
			node.URN = step.URN
			node.Type = step.ResourceType()
			node.DependsOn = stepDependencies(step, step.NewState)
			node.Status = engine.MapOperationToStatus(step.Op)
			if _, known := base.Nodes[step.URN]; !known {
				node.Component = isStepComponent(step, step.NewState)
			}
			graph.Nodes[step.URN] = node
		case engine.DiffOperationDelete:
			node := graph.Nodes[step.URN]
			node.URN = step.URN
			if _, known := base.Nodes[step.URN]; !known {
				node.Type = step.ResourceType()
				node.DependsOn = stepDependencies(step, step.OldState)
				node.Component = isStepComponent(step, step.OldState)
			}
			node.Status = engine.StatusDeleting
			graph.Nodes[step.URN] = node
		}
	}
	for _, node := range graph.Nodes {
		if len(node.DependsOn) > 0 {
			graph.Available = true
			break
		}
	}
	return graph
}

func stepDependencies(step ingest.PulumiStep, state *ingest.PulumiState) []string {
	if state != nil {
		return resourceLinks{
			dependencies:         state.Dependencies,
			propertyDependencies: state.PropertyDependencies,
			provider:             state.Provider,
			deletedWith:          state.DeletedWith,
			replaceWith:          state.ReplaceWith,
		}.dependsOn()
	}
	return resourceLinks{
		dependencies:         step.Dependencies,
		propertyDependencies: step.PropertyDependencies,
		provider:             step.Provider,
	}.dependsOn()
}

// isStepComponent reports whether a plan step names a component. Pulumi
// records custom on each step state; a state without it falls back to the
// built-in pulumi: types, which are never custom resources except providers.
func isStepComponent(step ingest.PulumiStep, state *ingest.PulumiState) bool {
	if state != nil && state.Custom != nil {
		return !*state.Custom
	}
	resourceType := step.ResourceType()
	return strings.HasPrefix(resourceType, "pulumi:") && !strings.HasPrefix(resourceType, "pulumi:providers:")
}
