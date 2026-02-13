package ingest

import (
	"fmt"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
)

const unknownProvider = "unknown"

// MergeProperties merges Outputs (base) with Inputs (overlay).
// Inputs take precedence on key conflicts so user-declared values win over
// provider-computed defaults. Returns nil when both maps are nil.
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

// MapResource converts a single Pulumi resource to a ResourceDescriptor.
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
