package proto

import (
	"context"
	"strings"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// UnknownPulumiValue is the sentinel Pulumi writes for an output that is not
// known at preview time. It is never a region, SKU, or tag value.
const UnknownPulumiValue = "04da6b54-80e4-46f7-96ec-b56ff0331ba9"

// PrepareProjectedDescriptor builds the plugin request for one resource.
// The Pulumi unknown sentinel is dropped from tags and from SKU and region.
// When the resource's own region is empty and exactly one referenced resource
// (distinct ref.*.urn) has a ref.*.region tag, that region fills Region.
// The referenced resource's SKU is never copied into Sku.
func PrepareProjectedDescriptor(
	ctx context.Context,
	id, provider, resourceType string,
	properties map[string]string,
) *pbc.ResourceDescriptor {
	tags := make(map[string]string, len(properties))
	for key, value := range properties {
		if strings.Contains(value, UnknownPulumiValue) {
			continue
		}
		tags[key] = value
	}

	sku, region := resolveSKUAndRegion(ctx, provider, resourceType, tags)
	if sku == UnknownPulumiValue {
		sku = ""
	}
	if region == UnknownPulumiValue {
		region = ""
	}
	if region == "" {
		region = inheritedRegion(tags)
	}

	return &pbc.ResourceDescriptor{
		Id:           id,
		Provider:     provider,
		ResourceType: resourceType,
		Sku:          sku,
		Region:       region,
		Tags:         tags,
	}
}

// HasRefTags reports whether tags contain a resolved ref.* key.
func HasRefTags(tags map[string]string) bool {
	for key := range tags {
		if strings.HasPrefix(key, "ref.") {
			return true
		}
	}
	return false
}

// ValidateProjectedDescriptor applies strict projected-cost validation.
// A resource with no SKU of its own that already carries ref.* tags uses the
// lenient validator so the plugin can see those tags. The referenced SKU is
// not inserted as a placeholder.
func ValidateProjectedDescriptor(descriptor *pbc.ResourceDescriptor) error {
	req := &pbc.GetProjectedCostRequest{Resource: descriptor}
	if descriptor.GetSku() == "" && HasRefTags(descriptor.GetTags()) {
		return pluginsdk.ValidateProjectedCostRequestLenient(req)
	}
	return pluginsdk.ValidateProjectedCostRequest(req)
}

// inheritedRegion returns a region only when exactly one referenced resource
// has a non-empty ref.<property>.region value.
func inheritedRegion(tags map[string]string) string {
	regionsByURN := make(map[string]string)
	for key, value := range tags {
		if value == "" || !strings.HasPrefix(key, "ref.") || !strings.HasSuffix(key, ".region") {
			continue
		}
		urnKey := strings.TrimSuffix(key, "region") + "urn"
		urn := tags[urnKey]
		if urn == "" {
			urn = key
		}
		if prev, ok := regionsByURN[urn]; ok && prev != value {
			regionsByURN[urn] = ""
			continue
		}
		regionsByURN[urn] = value
	}

	found := ""
	count := 0
	for _, region := range regionsByURN {
		if region == "" {
			continue
		}
		count++
		found = region
	}
	if count == 1 {
		return found
	}
	return ""
}
