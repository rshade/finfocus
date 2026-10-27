package stackload

import (
	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
)

// Descriptors returns plan resources followed by unchanged deployed resources.
// A state-only snapshot preserves historical plan-total membership.
func (s *Snapshot) Descriptors() []engine.ResourceDescriptor {
	var result []engine.ResourceDescriptor
	seen := make(map[string]bool)
	if s.Plan != nil {
		result, _ = ingest.MapResources(s.Plan.GetResources())
	}
	for i := range result {
		seen[result[i].ID] = true
		copyDescriptorProperties(&result[i])
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
		result = append(result, desc)
	}
	return result
}

// DeployedDescriptors returns exactly the deployed state mapping, as independent values.
func (s *Snapshot) DeployedDescriptors() []engine.ResourceDescriptor {
	result, _ := ingest.MapStateResources(s.RawState) // Mapping functions cannot currently fail.
	for i := range result {
		copyDescriptorProperties(&result[i])
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
