package ingest

import (
	"fmt"
	"strings"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/resourcetype"
)

// Property keys for Terraform metadata injected into ResourceDescriptor.Properties.
const (
	// PropertyTerraformID is the Terraform address (e.g. "module.net.aws_instance.web[0]").
	PropertyTerraformID = "terraform:id"
	// PropertyTerraformARN is the cloud ARN from the instance attributes, when present.
	PropertyTerraformARN = "terraform:arn"
	// PropertyTerraformCloudID is the cloud-provider resource ID from the
	// instance attributes (e.g. "i-0abc123"), when present.
	PropertyTerraformCloudID = "terraform:cloudId"
	// PropertyTerraformModule is the module address the resource belongs to, when present.
	PropertyTerraformModule = "terraform:module"
	// PropertyTerraformProvider is the raw provider reference string from state.
	PropertyTerraformProvider = "terraform:provider"
)

// MapTerraformResources expands state resources x instances into ResourceDescriptors.
// A resource with N instances produces N descriptors.
func MapTerraformResources(resources []TerraformStateResource) ([]engine.ResourceDescriptor, error) {
	var out []engine.ResourceDescriptor
	for _, r := range resources {
		for _, inst := range r.Instances {
			desc, err := MapTerraformResource(r, inst)
			if err != nil {
				return nil, fmt.Errorf("mapping %s: %w", terraformAddress(r, inst), err)
			}
			out = append(out, desc)
		}
	}
	return out, nil
}

// MapTerraformResource converts one Terraform state resource instance into an
// engine.ResourceDescriptor. The descriptor Type is the raw TF type (e.g.
// "aws_instance"); type resolution to Pulumi tokens happens later in the CLI
// layer. Property keys are converted snake_case -> camelCase recursively.
// The Provider field is the cloud for the type prefix ("azurerm" -> "azure"),
// which the adapter's SKU/region extraction relies on.
func MapTerraformResource(
	resource TerraformStateResource,
	instance TerraformStateInstance,
) (engine.ResourceDescriptor, error) {
	provider := extractTerraformProvider(resource.Type)
	if provider == "" {
		return engine.ResourceDescriptor{}, fmt.Errorf("cannot derive provider from terraform type %q", resource.Type)
	}

	props := convertKeysRecursive(instance.Attributes)
	address := terraformAddress(resource, instance)
	props[PropertyTerraformID] = address
	props[PropertyTerraformModule] = resource.Module
	props[PropertyTerraformProvider] = resource.Provider
	if arn, ok := instance.Attributes["arn"].(string); ok && arn != "" {
		props[PropertyTerraformARN] = arn
	}
	if id, ok := instance.Attributes["id"].(string); ok && id != "" {
		props[PropertyTerraformCloudID] = id
	}

	return engine.ResourceDescriptor{
		Type:       resource.Type,
		ID:         address,
		Provider:   provider,
		Properties: props,
	}, nil
}

// extractTerraformProvider returns the cloud for a TF type ("aws_instance" ->
// "aws", "azurerm_linux_virtual_machine" -> "azure"), or "" when the type has
// no underscore prefix.
func extractTerraformProvider(resourceType string) string {
	if strings.Index(resourceType, "_") <= 0 {
		return ""
	}
	return resourcetype.ExtractProvider(resourceType)
}

// terraformAddress builds the TF address for a resource instance:
// "aws_instance.web", "aws_instance.web[0]", `aws_instance.web["us-east-1"]`,
// "module.networking.aws_instance.web[0]".
func terraformAddress(r TerraformStateResource, inst TerraformStateInstance) string {
	var sb strings.Builder
	if r.Module != "" {
		sb.WriteString(r.Module)
		sb.WriteString(".")
	}
	sb.WriteString(r.Type)
	sb.WriteString(".")
	sb.WriteString(r.Name)
	switch key := inst.IndexKey.(type) {
	case nil:
		// no index
	case float64: // JSON numbers decode as float64
		fmt.Fprintf(&sb, "[%d]", int(key))
	case string:
		fmt.Fprintf(&sb, "[%q]", key)
	}
	return sb.String()
}

// SnakeToCamel converts a snake_case key to camelCase ("instance_type" ->
// "instanceType"). Keys without underscores are returned unchanged.
func SnakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

// convertKeysRecursive converts all map keys in a property tree to camelCase,
// preserving nested structure (objects and arrays of objects).
func convertKeysRecursive(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[SnakeToCamel(k)] = convertValueRecursive(v)
	}
	return out
}

func convertValueRecursive(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return convertKeysRecursive(val)
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = convertValueRecursive(item)
		}
		return out
	default:
		return val
	}
}
