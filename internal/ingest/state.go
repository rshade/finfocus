package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/logging"
)

// Property keys for Pulumi metadata injected into ResourceDescriptor.Properties.
const (
	// PropertyPulumiCreated is the property key for resource creation timestamp.
	PropertyPulumiCreated = "pulumi:created"
	// PropertyPulumiModified is the property key for resource modification timestamp.
	PropertyPulumiModified = "pulumi:modified"
	// PropertyPulumiExternal indicates the resource was imported (not created by Pulumi).
	PropertyPulumiExternal = "pulumi:external"
	// PropertyPulumiCloudID is the cloud-provider resource ID (e.g., "i-0abc123", "db-instance-primary").
	PropertyPulumiCloudID = "pulumi:cloudId"
	// PropertyPulumiARN is the canonical cloud identifier from outputs (e.g., AWS ARN).
	PropertyPulumiARN = "pulumi:arn"
	// PropertyPulumiURN is the Pulumi URN preserved for correlation.
	PropertyPulumiURN = "pulumi:urn"
)

// StackExport represents the structure of `pulumi stack export` output.
type StackExport struct {
	Version    int                   `json:"version"`
	Deployment StackExportDeployment `json:"deployment"`
}

// StackExportDeployment contains the deployment manifest and resources.
type StackExportDeployment struct {
	Manifest  StackExportManifest   `json:"manifest"`
	Resources []StackExportResource `json:"resources"`
}

// StackExportManifest contains deployment metadata.
type StackExportManifest struct {
	Time    string `json:"time"`
	Magic   string `json:"magic"`
	Version string `json:"version"`
}

// StackExportResource represents a resource in Pulumi state (ResourceV3).
// Timestamps are available since Pulumi v3.60.0 (March 2023).
type StackExportResource struct {
	URN      string                 `json:"urn"`
	Type     string                 `json:"type"`
	ID       string                 `json:"id,omitempty"`
	Custom   bool                   `json:"custom,omitempty"`
	External bool                   `json:"external,omitempty"`
	Provider string                 `json:"provider,omitempty"`
	Inputs   map[string]interface{} `json:"inputs,omitempty"`
	Outputs  map[string]interface{} `json:"outputs,omitempty"`
	// Created tracks when the remote resource was first added to state.
	// Available since Pulumi v3.60.0 (March 2023).
	Created *time.Time `json:"created,omitempty"`
	// Modified tracks when the resource state was last altered.
	// Available since Pulumi v3.60.0 (March 2023).
	Modified *time.Time `json:"modified,omitempty"`
}

// LoadStackExport loads and parses a Pulumi state JSON file from the specified path.
// The state file is typically generated via `pulumi stack export > state.json`.
func LoadStackExport(path string) (*StackExport, error) {
	return LoadStackExportWithContext(context.Background(), path)
}

// LoadStackExportWithContext loads and parses a Pulumi state JSON file with logging context.
func LoadStackExportWithContext(ctx context.Context, path string) (*StackExport, error) {
	log := logging.FromContext(ctx)
	log.Debug().
		Str("component", "ingest").
		Str("operation", "load_state").
		Str("state_path", path).
		Msg("loading Pulumi state")

	data, err := os.ReadFile(path)
	if err != nil {
		log.Error().
			Str("component", "ingest").
			Err(err).
			Str("state_path", path).
			Msg("failed to read state file")
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	log.Debug().
		Str("component", "ingest").
		Int("file_size_bytes", len(data)).
		Msg("state file read successfully")

	var state StackExport
	if unmarshalErr := json.Unmarshal(data, &state); unmarshalErr != nil {
		log.Error().
			Str("component", "ingest").
			Err(unmarshalErr).
			Str("state_path", path).
			Msg("failed to parse state JSON")
		return nil, fmt.Errorf("parsing state JSON: %w", unmarshalErr)
	}

	log.Debug().
		Str("component", "ingest").
		Int("version", state.Version).
		Int("resource_count", len(state.Deployment.Resources)).
		Msg("state parsed successfully")

	return &state, nil
}

// GetCustomResources returns only custom resources (cloud resources) from state.
// Component resources and providers are filtered out.
func (s *StackExport) GetCustomResources() []StackExportResource {
	return s.GetCustomResourcesWithContext(context.Background())
}

// GetCustomResourcesWithContext returns custom resources with logging context.
func (s *StackExport) GetCustomResourcesWithContext(ctx context.Context) []StackExportResource {
	log := logging.FromContext(ctx)
	// Pre-allocate with estimate; most resources in typical stacks are custom
	resources := make([]StackExportResource, 0, len(s.Deployment.Resources))

	for _, r := range s.Deployment.Resources {
		if r.Custom {
			resources = append(resources, r)
		}
	}

	log.Debug().
		Str("component", "ingest").
		Int("total_resources", len(s.Deployment.Resources)).
		Int("custom_resources", len(resources)).
		Msg("filtered custom resources from state")

	return resources
}

// MapStateResource converts a StackExportResource to a ResourceDescriptor.
// Timestamps, cloud identifiers (ID, ARN), and outputs are injected into Properties.
func MapStateResource(resource StackExportResource) (engine.ResourceDescriptor, error) {
	provider := extractProvider(resource.Type)

	// Copy inputs to properties, then inject Pulumi metadata
	properties := make(map[string]interface{})
	for k, v := range resource.Inputs {
		properties[k] = v
	}

	// Inject timestamps as RFC3339 strings
	if resource.Created != nil {
		properties[PropertyPulumiCreated] = resource.Created.Format(time.RFC3339)
	}
	if resource.Modified != nil {
		properties[PropertyPulumiModified] = resource.Modified.Format(time.RFC3339)
	}
	if resource.External {
		properties[PropertyPulumiExternal] = "true"
	}

	// Inject cloud identifiers for actual cost lookups
	if resource.ID != "" {
		properties[PropertyPulumiCloudID] = resource.ID
	}
	properties[PropertyPulumiURN] = resource.URN

	// Extract ARN from outputs if available
	if arn, ok := extractStringFromOutputs(resource.Outputs, "arn"); ok {
		properties[PropertyPulumiARN] = arn
	}

	// Extract tags from outputs (prefer tagsAll over tags for AWS completeness)
	if outputTags, ok := extractTagsFromMap(resource.Outputs, "tagsAll"); ok {
		properties["tagsAll"] = outputTags
	} else if inputTags, hasInputTags := extractTagsFromMap(resource.Inputs, "tags"); hasInputTags {
		// Fallback: use tags from inputs if outputs don't have tagsAll
		if _, alreadyHasTags := properties["tags"]; !alreadyHasTags {
			properties["tags"] = inputTags
		}
	}

	return engine.ResourceDescriptor{
		Type:       resource.Type,
		ID:         resource.URN,
		Provider:   provider,
		Properties: properties,
	}, nil
}

// extractStringFromOutputs extracts a string value from a map by key.
func extractStringFromOutputs(m map[string]interface{}, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, found := m[key]
	if !found {
		return "", false
	}
	s, isStr := v.(string)
	if !isStr || s == "" {
		return "", false
	}
	return s, true
}

// extractTagsFromMap extracts a map[string]interface{} from a nested map value.
func extractTagsFromMap(m map[string]interface{}, key string) (map[string]interface{}, bool) {
	if m == nil {
		return nil, false
	}
	v, found := m[key]
	if !found {
		return nil, false
	}
	tags, isMap := v.(map[string]interface{})
	if !isMap || len(tags) == 0 {
		return nil, false
	}
	return tags, true
}

// MapStateResources converts multiple StackExportResource to ResourceDescriptors.
func MapStateResources(resources []StackExportResource) ([]engine.ResourceDescriptor, error) {
	var descriptors []engine.ResourceDescriptor
	for _, r := range resources {
		desc, err := MapStateResource(r)
		if err != nil {
			return nil, fmt.Errorf("mapping state resource %s: %w", r.URN, err)
		}
		descriptors = append(descriptors, desc)
	}
	return descriptors, nil
}

// HasTimestamps checks if the state contains resources with timestamp data.
// Returns true if at least one resource has Created or Modified timestamps.
func (s *StackExport) HasTimestamps() bool {
	for _, r := range s.Deployment.Resources {
		if r.Created != nil || r.Modified != nil {
			return true
		}
	}
	return false
}

// GetResourceByURN finds a resource by its URN.
func (s *StackExport) GetResourceByURN(urn string) *StackExportResource {
	for i := range s.Deployment.Resources {
		if s.Deployment.Resources[i].URN == urn {
			return &s.Deployment.Resources[i]
		}
	}
	return nil
}
