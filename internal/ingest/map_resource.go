package ingest

import (
	"fmt"
	"maps"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/resourcetype"
)

// MergeProperties merges two property maps into a single map. Keys from the
// inputs map override keys from the outputs map when they conflict. If both
// inputs and outputs are nil, MergeProperties returns nil.
func MergeProperties(outputs, inputs map[string]any) map[string]any {
	if outputs == nil && inputs == nil {
		return nil
	}
	result := make(map[string]any, len(outputs)+len(inputs))
	maps.Copy(result, outputs)
	maps.Copy(result, inputs)
	return result
}

// MapResource converts a PulumiResource into an engine.ResourceDescriptor.
// The returned descriptor contains the resource Type, URN as ID, the provider
// derived from the resource type, and Properties produced by merging the
// resource's outputs with its inputs (inputs take precedence). OldProperties is
// built the same way from the outputs and the old inputs, so a diff prices both
// sides from the same kinds of property; it stays nil when the step has no old
// inputs.
// The function does not currently produce an error; the returned error is nil.
func MapResource(pulumiResource PulumiResource) (engine.ResourceDescriptor, error) {
	provider := resourcetype.ExtractProvider(pulumiResource.Type)

	var oldProperties map[string]any
	if pulumiResource.OldInputs != nil {
		oldProperties = MergeProperties(pulumiResource.Outputs, pulumiResource.OldInputs)
	}

	return engine.ResourceDescriptor{
		Type:          pulumiResource.Type,
		ID:            pulumiResource.URN,
		Provider:      provider,
		Properties:    MergeProperties(pulumiResource.Outputs, pulumiResource.Inputs),
		Refs:          refsFromPropertyDependencies(pulumiResource.PropertyDependencies),
		Operation:     pulumiResource.Operation,
		OldProperties: oldProperties,
	}, nil
}

// refsFromPropertyDependencies copies property-to-URN entries that name at
// least one resource. Empty lists mean the property is not a reference and are
// dropped. A nil or all-empty map returns nil.
func refsFromPropertyDependencies(in map[string][]string) map[string][]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string][]string)
	for key, urns := range in {
		if len(urns) == 0 {
			continue
		}
		out[key] = append([]string(nil), urns...)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// MapResources converts multiple Pulumi resources to ResourceDescriptors.
func MapResources(resources []PulumiResource) ([]engine.ResourceDescriptor, error) {
	var descriptors []engine.ResourceDescriptor
	for _, r := range resources {
		desc, err := MapResource(r)
		if err != nil {
			return nil, fmt.Errorf("mapping resource %s: %w", r.URN, err)
		}
		descriptors = append(descriptors, desc)
	}
	return descriptors, nil
}
