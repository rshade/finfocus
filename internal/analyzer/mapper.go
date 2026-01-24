package analyzer

import (
	"errors"
	"fmt"
	"strings"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rshade/finfocus/internal/engine"
)

// URN parsing constants.
const (
	// minURNParts is the minimum number of parts in a URN to extract the resource name.
	// URN format: urn:pulumi:stack::project::type::name (6 parts minimum, we need at least 2).
	minURNParts = 2

	// minProviderTypeParts is the minimum number of parts in a provider type.
	// Format: pulumi:providers:aws (3 parts).
	minProviderTypeParts = 3
)

// MappingError represents an error that occurred during resource mapping.
type MappingError struct {
	Index   int    // Position in original slice
	URN     string // Resource URN if available
	Type    string // Resource type if available
	Message string // Error description
	Err     error  // Underlying error
}

// Error implements the error interface.
func (e *MappingError) Error() string {
	if e.URN != "" {
		return "mapping " + e.URN + ": " + e.Message
	}
	return fmt.Sprintf("mapping resource at index %d: %s", e.Index, e.Message)
}

// Unwrap returns the underlying error.
func (e *MappingError) Unwrap() error {
	return e.Err
}

// ErrNilResource is returned when a nil resource is encountered.
var ErrNilResource = errors.New("nil resource")

// MappingResult contains the results of mapping resources with error tracking.
type MappingResult struct {
	Resources []engine.ResourceDescriptor // Successfully mapped resources
	Errors    []MappingError              // Errors encountered during mapping
	Skipped   int                         // Count of skipped resources
}

// MapResource converts a pulumirpc.AnalyzerResource to an engine.ResourceDescriptor.
//
// The mapping extracts cost-relevant fields from the Pulumi resource representation
// and normalizes them to the internal format used by the cost calculation engine.
// This function performs the core mapping logic for individual resources.
// Field mappings:
//   - Type: Direct copy from r.Type
//   - ID: Extracted from URN (last :: segment)
//   - Provider: Extracted from provider resource type or resource type prefix
//   - Properties: Converted from protobuf Struct to Go map
func MapResource(r *pulumirpc.AnalyzerResource) engine.ResourceDescriptor {
	return engine.ResourceDescriptor{
		Type:       r.GetType(),
		ID:         extractResourceID(r.GetUrn()),
		Provider:   extractProvider(r),
		Properties: structToMap(r.GetProperties()),
	}
}

// MapResources converts a slice of AnalyzerResource to ResourceDescriptors.
//
// This is the primary entry point for batch resource mapping. All resources
// are converted, and any individual mapping failures are handled gracefully
// (the resource is included with best-effort field extraction).
//
// Note: This function skips nil resources silently. Use MapResourcesWithErrors
// for explicit error tracking.
func MapResources(resources []*pulumirpc.AnalyzerResource) []engine.ResourceDescriptor {
	if len(resources) == 0 {
		return nil
	}

	result := make([]engine.ResourceDescriptor, 0, len(resources))
	for _, r := range resources {
		if r == nil {
			continue
		}
		result = append(result, MapResource(r))
	}
	return result
}

// MapResourcesWithErrors converts resources with explicit error tracking.
//
// This function provides detailed error information for each resource that
// fails to map. Use this when you need visibility into mapping failures
// for diagnostics or debugging.
//
// Graceful degradation: nil resources are skipped and counted, valid
// resources are always processed regardless of failures on other resources.
func MapResourcesWithErrors(resources []*pulumirpc.AnalyzerResource) MappingResult {
	result := MappingResult{
		Resources: make([]engine.ResourceDescriptor, 0, len(resources)),
		Errors:    make([]MappingError, 0),
	}

	for i, r := range resources {
		if r == nil {
			result.Skipped++
			result.Errors = append(result.Errors, MappingError{
				Index:   i,
				Message: "nil resource pointer",
				Err:     ErrNilResource,
			})
			continue
		}

		result.Resources = append(result.Resources, MapResource(r))
	}

	return result
}

// extractResourceID extracts the resource name from a Pulumi URN.
//
// URN format: urn:pulumi:stack::project::type::name
//
// Examples:
//   - "urn:pulumi:dev::myapp::aws:ec2/instance:Instance::webserver" → "webserver"
//   - "urn:pulumi:prod::api::azure:compute/vm:VM::api-server-01" → "api-server-01"
//   - "" → ""
//
// extractResourceID extracts the resource-name segment from a Pulumi URN.
// If urn is empty, it returns the empty string. If the URN contains "::"
// separators, the last segment is returned as the resource name; otherwise the
// original urn string is returned.
func extractResourceID(urn string) string {
	if urn == "" {
		return ""
	}

	parts := strings.Split(urn, "::")
	if len(parts) >= minURNParts {
		return parts[len(parts)-1] // Last part is the resource name
	}
	return urn
}

// extractProviderFromRequest extracts the provider name from an AnalyzeRequest.
//
// This function is similar to extractProvider but works with AnalyzeRequest
// which is used for single-resource analysis. It uses the same two-tier strategy:
//  1. First, try to extract from the provider resource's type field
//
// extractProviderFromRequest derives the provider name from an AnalyzeRequest.
//
// It first checks the request's Provider type (expected in the form "pulumi:providers:NAME")
// and returns the NAME segment when present. If that field is absent or malformed, it
// falls back to parsing the resource type prefix from r.GetType() and returns the first
// provider-like segment found. It returns "unknown" if no provider name can be determined.
//
// Parameters:
//
//	r - the AnalyzeRequest to inspect for provider information.
func extractProviderFromRequest(r *pulumirpc.AnalyzeRequest) string {
	// Try provider resource first
	if p := r.GetProvider(); p != nil {
		if providerType := p.GetType(); providerType != "" {
			// Format: pulumi:providers:aws
			parts := strings.Split(providerType, ":")
			if len(parts) >= minProviderTypeParts {
				return parts[2]
			}
		}
	}

	// Fall back to resource type prefix
	return extractProviderFromType(r.GetType())
}

// extractProviderFromType extracts the provider name from a resource type string.
//
// Format: "aws:ec2/instance:Instance" → "aws"
// extractProviderFromType extracts the provider name from a resource type string.
// It returns the first colon-separated segment (for example, "aws" from "aws:ec2/instance:Instance"), or "unknown" if the input is empty or does not contain a valid prefix.
func extractProviderFromType(resourceType string) string {
	if resourceType == "" {
		return "unknown"
	}

	parts := strings.Split(resourceType, ":")
	if len(parts) >= 1 && parts[0] != "" {
		return parts[0]
	}

	return "unknown"
}

// extractProvider extracts the provider name from the resource.
//
// The function uses a two-tier strategy:
//  1. First, try to extract from the provider resource's type field
//     (format: "pulumi:providers:aws" → "aws")
//  2. Fall back to extracting from the resource type prefix
//     (format: "aws:ec2/instance:Instance" → "aws")
//
// extractProvider returns the provider name for the given AnalyzerResource.
// It first inspects the resource's Provider.Type (expected format `pulumi:providers:<provider>`) and returns the third segment when present.
// If that is not available, it falls back to parsing the resource's type prefix via extractProviderFromType.
// If neither approach yields a provider, it returns "unknown".
func extractProvider(r *pulumirpc.AnalyzerResource) string {
	// Try provider resource first
	if p := r.GetProvider(); p != nil {
		if providerType := p.GetType(); providerType != "" {
			// Format: pulumi:providers:aws
			parts := strings.Split(providerType, ":")
			if len(parts) >= minProviderTypeParts {
				return parts[2]
			}
		}
	}

	// Fall back to resource type prefix
	return extractProviderFromType(r.GetType())
}

// structToMap converts a protobuf Struct to a Go map.
//
// This function uses the standard protobuf AsMap() conversion which handles
// all protobuf Value types:
//   - NullValue → nil
//   - BoolValue → bool
//   - NumberValue → float64
//   - StringValue → string
//   - ListValue → []interface{}
//   - Struct → map[string]interface{}
//
// Returns an empty map if the input is nil.
func structToMap(s *structpb.Struct) map[string]interface{} {
	if s == nil {
		return make(map[string]interface{})
	}
	return s.AsMap()
}
