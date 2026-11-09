package stackload

import (
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// Descriptors returns plan resources followed by unchanged deployed resources.
// A state-only snapshot preserves historical plan-total membership.
// Properties take the region of the provider after the preview, except on a
// delete, whose properties are the state it removes. OldProperties take the
// region of the provider before the preview.
func (s *Snapshot) Descriptors() []engine.ResourceDescriptor {
	var result []engine.ResourceDescriptor
	seen := make(map[string]bool)
	scope := inventoryProviderScope(s.RawState, s.Plan)
	prior := priorProviderScope(s.RawState, s.Plan)
	if s.Plan != nil {
		result, _ = ingest.MapResources(s.Plan.GetResources())
	}
	for i := range result {
		seen[result[i].ID] = true
		copyDescriptorProperties(&result[i])
		current := scope
		if isDeleteOperation(result[i].Operation) {
			current = prior
		}
		result[i].Properties = current.withProviderRegion(result[i].Properties, result[i].ID, result[i].Type)
		if result[i].OldProperties != nil {
			result[i].OldProperties = prior.withProviderRegion(result[i].OldProperties, result[i].ID, result[i].Type)
		}
	}
	for _, raw := range s.RawState {
		if seen[raw.URN] {
			continue
		}
		desc, _ := ingest.MapStateResource(raw) // Mapping functions cannot currently fail.
		desc.Operation = "same"
		if s.PlanStatus.Loaded {
			desc.Origin = engine.OriginState
		}
		copyDescriptorProperties(&desc)
		desc.Properties = scope.withProviderRegion(desc.Properties, desc.ID, desc.Type)
		result = append(result, desc)
	}
	return result
}

// DeployedDescriptors returns exactly the deployed state mapping, as independent values.
// Provider regions come from deployed state alone: a preview's provider change
// has not been applied to what is deployed.
func (s *Snapshot) DeployedDescriptors() []engine.ResourceDescriptor {
	result, _ := ingest.MapStateResources(s.RawState) // Mapping functions cannot currently fail.
	scope := stateProviderScope(s.RawState)
	for i := range result {
		copyDescriptorProperties(&result[i])
		result[i].Properties = scope.withProviderRegion(result[i].Properties, result[i].ID, result[i].Type)
	}
	return result
}

// ActualDescriptors is the actual-cost inventory: the deployed resources when
// state was read, otherwise the plan's resources, as before the shared loader.
func (s *Snapshot) ActualDescriptors() []engine.ResourceDescriptor {
	if s.StateStatus.Loaded {
		return s.DeployedDescriptors()
	}
	return s.Descriptors()
}

func copyDescriptorProperties(desc *engine.ResourceDescriptor) {
	desc.Properties = deepCopyProperties(desc.Properties)
	desc.OldProperties = deepCopyProperties(desc.OldProperties)
}
