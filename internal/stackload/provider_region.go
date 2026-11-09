package stackload

import (
	"maps"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/history"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/resourcetype"
)

const pulumiProviderTypePrefix = "pulumi:providers:"

// providerScope finds the AWS region of a resource through the provider it
// names. refs maps a resource URN to its provider reference, which is
// <provider-urn>::<provider-id>, and regions maps a provider URN to the region
// that provider configures.
type providerScope struct {
	refs    map[string]string
	regions map[string]string
}

// stateProviderScope covers the deployed inventory only.
func stateProviderScope(resources []ingest.StackExportResource) providerScope {
	scope := providerScope{refs: make(map[string]string), regions: make(map[string]string)}
	for _, r := range resources {
		if r.URN == "" {
			continue
		}
		scope.refs[r.URN] = r.Provider
		if region := awsProviderRegion(r.Type, r.Inputs, r.Outputs); region != "" {
			scope.regions[r.URN] = region
		}
	}
	return scope
}

// inventoryProviderScope covers deployed state with the preview applied over
// it, so a provider the preview creates or updates decides the region.
func inventoryProviderScope(state []ingest.StackExportResource, plan *ingest.PulumiPlan) providerScope {
	scope := stateProviderScope(state)
	if plan == nil {
		return scope
	}
	for _, step := range plan.Steps {
		if step.URN == "" {
			continue
		}
		if ref := stepProviderReference(step); ref != "" {
			scope.refs[step.URN] = ref
		}
		if step.NewState == nil {
			continue
		}
		if region := awsProviderRegion(step.ResourceType(), step.NewState.Inputs, step.NewState.Outputs); region != "" {
			scope.regions[step.URN] = region
		}
	}
	return scope
}

// stepProviderReference reads the provider reference a step carries, from the
// state it leaves or the state it creates, before the step-level field.
func stepProviderReference(step ingest.PulumiStep) string {
	switch {
	case step.NewState != nil && step.NewState.Provider != "":
		return step.NewState.Provider
	case step.OldState != nil && step.OldState.Provider != "":
		return step.OldState.Provider
	default:
		return step.Provider
	}
}

// awsProviderRegion returns the region an AWS provider resource configures,
// taking its inputs before its outputs. Other resources, Pulumi secrets, blank
// values, and the unknown sentinel give no region.
func awsProviderRegion(resourceType string, inputs, outputs map[string]any) string {
	pkg, isProvider := strings.CutPrefix(resourceType, pulumiProviderTypePrefix)
	if !isProvider || resourcetype.NormalizeProvider(pkg) != "aws" {
		return ""
	}
	for _, values := range []map[string]any{inputs, outputs} {
		if region := regionValue(values["region"]); region != "" {
			return region
		}
	}
	return ""
}

func regionValue(value any) string {
	if history.IsPulumiSecret(value) {
		return ""
	}
	region, ok := value.(string)
	if !ok || region == proto.UnknownPulumiValue {
		return ""
	}
	return region
}

func (p providerScope) annotateStates(states []engine.StateResource) {
	for i := range states {
		states[i].Properties = p.withProviderRegion(states[i].Properties, states[i].URN, states[i].Type)
	}
}

// annotateSteps leaves a step without projected properties alone, so a delete
// is not turned into a create-like pricing input.
func (p providerScope) annotateSteps(steps []engine.PlanStep) {
	for i := range steps {
		if steps[i].ProjectedProperties == nil {
			continue
		}
		steps[i].ProjectedProperties = p.withProviderRegion(steps[i].ProjectedProperties, steps[i].URN, steps[i].Type)
	}
}

// withProviderRegion returns props with the region of the resource's provider
// under proto.ProviderRegionProperty. It applies only to AWS resources whose
// provider configures a region. A copy is returned, so props is never
// modified, and the adapter uses the value only after the resource's own
// region sources.
func (p providerScope) withProviderRegion(props map[string]any, urn, resourceType string) map[string]any {
	if resourcetype.ExtractProvider(resourceType) != "aws" {
		return props
	}
	region := p.regions[providerURN(p.refs[urn])]
	if region == "" {
		return props
	}
	out := make(map[string]any, len(props)+1)
	maps.Copy(out, props)
	out[proto.ProviderRegionProperty] = region
	return out
}
