package conformance

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

// providerWildcard is the provider a plugin lists when it prices any provider.
const providerWildcard = "*"

// ProbeResource is the resource descriptor the conformance probes send.
type ProbeResource struct {
	// Provider is the cloud provider, such as "aws" or "kubernetes".
	Provider string
	// ResourceType is the resource type the plugin prices.
	ResourceType string
	// SKU is optional; empty when the plugin needs none.
	SKU string
	// Region is optional; empty when the plugin needs none.
	Region string
}

// DefaultProbeResource returns the built-in AWS probe resource.
func DefaultProbeResource() ProbeResource {
	return ProbeResource{
		Provider:     testProviderAWS,
		ResourceType: testResourceTypeEC2Instance,
		SKU:          testSKUT3Micro,
		Region:       testRegionUSEast1,
	}
}

// IsZero reports whether no probe field is set.
func (p ProbeResource) IsZero() bool {
	return p == ProbeResource{}
}

// Validate rejects a partial probe: a provider and a resource type are set
// together or not at all, and a SKU or region needs both.
func (p ProbeResource) Validate() error {
	if p.IsZero() {
		return nil
	}
	if p.Provider == "" || p.ResourceType == "" {
		return errors.New("probe resource needs both a provider and a resource type")
	}
	return nil
}

// Descriptor builds the request descriptor for the probe resource.
func (p ProbeResource) Descriptor() *pbc.ResourceDescriptor {
	return &pbc.ResourceDescriptor{
		Provider:     p.Provider,
		ResourceType: p.ResourceType,
		Sku:          p.SKU,
		Region:       p.Region,
	}
}

// ProbeNeed states what a test case needs from the probe resource.
type ProbeNeed int

const (
	// ProbeNone means the case does not depend on the resource type.
	ProbeNone ProbeNeed = iota
	// ProbeSupportedType means the plugin must price the probe resource type.
	ProbeSupportedType
	// ProbeSentinelType means the case sends a sentinel resource type that only
	// a plugin built to recognize it can answer, so it runs only with the
	// built-in probe.
	ProbeSentinelType
)

type probeSource int

const (
	probeDefault probeSource = iota
	probeCustom
	probeUnavailable
)

// probePlan is the probe resource chosen for one suite run.
type probePlan struct {
	resource ProbeResource
	source   probeSource
	reason   string
}

func defaultProbePlan() probePlan {
	return probePlan{resource: DefaultProbeResource(), source: probeDefault}
}

// resolveProbe picks the probe resource. A configured resource wins. Without
// one, the built-in AWS probe is used unless the plugin lists providers and
// none of them is AWS; then no type-dependent probe can be trusted.
func resolveProbe(configured ProbeResource, providers []string) probePlan {
	if !configured.IsZero() {
		return probePlan{resource: configured, source: probeCustom}
	}
	if len(providers) == 0 || slices.ContainsFunc(providers, coversAWS) {
		return defaultProbePlan()
	}
	return probePlan{
		resource: DefaultProbeResource(),
		source:   probeUnavailable,
		reason: fmt.Sprintf(
			"plugin lists providers [%s] and the built-in probe is AWS; "+
				"pass --provider and --resource-type to probe with a resource it prices",
			strings.Join(providers, ", "),
		),
	}
}

func coversAWS(provider string) bool {
	return provider == providerWildcard || strings.EqualFold(provider, testProviderAWS)
}

// skipReason returns why a case with the given need cannot run, or "".
func (p probePlan) skipReason(need ProbeNeed) string {
	switch need {
	case ProbeSupportedType:
		if p.source == probeUnavailable {
			return p.reason
		}
	case ProbeSentinelType:
		switch p.source {
		case probeUnavailable:
			return p.reason
		case probeCustom:
			return "sentinel resource types are recognized only by a plugin built for them; " +
				"this probe runs with the built-in resource only"
		case probeDefault:
		}
	case ProbeNone:
	}
	return ""
}

// pluginProviders asks the plugin which providers it prices. An unanswered or
// unimplemented GetPluginInfo returns nil, which keeps the built-in probe.
func pluginProviders(ctx context.Context, client pbc.CostSourceServiceClient) []string {
	info, err := client.GetPluginInfo(ctx, &pbc.GetPluginInfoRequest{})
	if err != nil {
		return nil
	}
	return info.GetProviders()
}
