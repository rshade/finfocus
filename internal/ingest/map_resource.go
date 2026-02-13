package ingest

import (
	"fmt"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
)

const unknownProvider = "unknown"

// MergeProperties merges two property maps into a single map. Keys from the
// inputs map override keys from the outputs map when they conflict. If both
// inputs and outputs are nil, MergeProperties returns nil.
func MergeProperties(outputs, inputs map[string]interface{}) map[string]interface{} {
	if outputs == nil && inputs == nil {
		return nil
	}
	result := make(map[string]interface{}, len(outputs)+len(inputs))
	for k, v := range outputs {
		result[k] = v
	}
	for k, v := range inputs {
		result[k] = v
	}
	return result
}

// MapResource converts a PulumiResource into an engine.ResourceDescriptor.
// The returned descriptor contains the resource Type, URN as ID, the provider
// derived from the resource type, and Properties produced by merging the
// resource's outputs with its inputs (inputs take precedence).
// The function does not currently produce an error; the returned error is nil.
func MapResource(pulumiResource PulumiResource) (engine.ResourceDescriptor, error) {
	provider := extractProvider(pulumiResource.Type)

	return engine.ResourceDescriptor{
		Type:       pulumiResource.Type,
		ID:         pulumiResource.URN,
		Provider:   provider,
		Properties: MergeProperties(pulumiResource.Outputs, pulumiResource.Inputs),
	}, nil
}

func extractProvider(resourceType string) string {
	parts := strings.Split(resourceType, ":")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return unknownProvider
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
