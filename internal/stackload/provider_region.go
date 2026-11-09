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
		scope.setRegion(r.URN, awsProviderRegion(r.Type, r.Inputs, r.Outputs))
	}
	return scope
}

// inventoryProviderScope covers deployed state with the preview applied over
// it, so a provider the preview creates or updates decides the region, even
// when its new region is unknown. A delete step's old reference is used only
// for a resource no other source names, so it never replaces the reference a
// replacement creates.
func inventoryProviderScope(state []ingest.StackExportResource, plan *ingest.PulumiPlan) providerScope {
	scope := stateProviderScope(state)
	if plan == nil {
		return scope
	}
	for _, step := range plan.Steps {
		if step.URN == "" {
			continue
		}
		if ref := stepNewReference(step); ref != "" {
			scope.refs[step.URN] = ref
		} else if _, known := scope.refs[step.URN]; !known {
			scope.refs[step.URN] = stepOldReference(step)
		}
		if next := step.NewState; next != nil {
			scope.setRegion(step.URN, awsProviderRegion(step.ResourceType(), next.Inputs, next.Outputs))
		}
	}
	return scope
}

// priorProviderScope covers providers as they are before the preview: deployed
// state, then the old state a preview step reports for a resource the state
// does not hold. It prices the before side of an update and a delete.
func priorProviderScope(state []ingest.StackExportResource, plan *ingest.PulumiPlan) providerScope {
	scope := stateProviderScope(state)
	if plan == nil {
		return scope
	}
	known := make(map[string]bool, len(state))
	for _, r := range state {
		known[r.URN] = true
	}
	for _, step := range plan.Steps {
		if step.URN == "" || step.OldState == nil || known[step.URN] {
			continue
		}
		known[step.URN] = true
		scope.refs[step.URN] = stepOldReference(step)
		old := step.OldState
		scope.setRegion(step.URN, awsProviderRegion(step.ResourceType(), old.Inputs, old.Outputs))
	}
	return scope
}

func (p providerScope) setRegion(urn, region string) {
	if region == "" {
		delete(p.regions, urn)
		return
	}
	p.regions[urn] = region
}

// stepNewReference reads the provider reference of the state a step creates.
// A step without new state names one at the top level, unless it deletes.
func stepNewReference(step ingest.PulumiStep) string {
	if step.NewState != nil {
		return step.NewState.Provider
	}
	if isDeleteOperation(step.Op) {
		return ""
	}
	return step.Provider
}

// stepOldReference reads the provider reference of the state a step leaves.
func stepOldReference(step ingest.PulumiStep) string {
	if step.OldState != nil && step.OldState.Provider != "" {
		return step.OldState.Provider
	}
	return step.Provider
}

func isDeleteOperation(op string) bool {
	return op == engine.DiffOperationDelete || op == opDeleteReplaced
}

// awsProviderRegion returns the region an AWS provider resource configures.
// A region input decides when the key is present, so a secret, unknown,
// non-string, or blank input gives no region rather than a stale output. The
// output is read only when the inputs have no region key. Other resources give
// no region.
func awsProviderRegion(resourceType string, inputs, outputs map[string]any) string {
	pkg, isProvider := strings.CutPrefix(resourceType, pulumiProviderTypePrefix)
	if !isProvider || resourcetype.NormalizeProvider(pkg) != "aws" {
		return ""
	}
	if value, ok := inputs["region"]; ok {
		return regionValue(value)
	}
	return regionValue(outputs["region"])
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
