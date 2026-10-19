// Package carbon estimates operational emissions for AWS EC2 using a pinned
// Cloud Carbon Footprint compute and memory model. It performs no network I/O.
package carbon

import (
	"errors"
	"fmt"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

const (
	monthlyHours            = 730
	powerUsageEffectiveness = 1.135
	memoryKWhPerGBHour      = 0.000392
	wattsPerKilowatt        = 1000
	gramsPerMetricTon       = 1e6
)

// Estimate is a monthly operational carbon and energy estimate.
type Estimate struct {
	Grams float64
	KWh   float64
	Note  string
}

// EstimateResource validates an EC2 descriptor and calculates operational carbon.
// Missing coefficients or instance specifications are declined, never guessed.
func EstimateResource(resource *pbc.ResourceDescriptor, cfg Config) (Estimate, error) {
	return estimateResource(resource, cfg, loadData)
}

func estimateResource(
	resource *pbc.ResourceDescriptor,
	cfg Config,
	load func() (dataset, error),
) (Estimate, error) {
	if err := cfg.validate(); err != nil {
		return Estimate{}, err
	}
	if resource == nil {
		return Estimate{}, errors.New("resource is required")
	}
	if resource.GetProvider() != "aws" || !isEC2(resource.GetResourceType()) {
		return Estimate{}, errors.New("carbon supports AWS EC2 instances only")
	}
	data, err := load()
	if err != nil {
		return Estimate{}, err
	}
	spec, ok := data.Instances[resource.GetSku()]
	if !ok {
		return Estimate{}, fmt.Errorf(
			"unsupported or missing EC2 instance type %q; use a documented t3, m5, c5, or r5 size",
			resource.GetSku(),
		)
	}
	intensity, ok := data.Regions[resource.GetRegion()]
	if !ok {
		return Estimate{}, fmt.Errorf(
			"unsupported or missing AWS region %q; use a region in the pinned CCF dataset",
			resource.GetRegion(),
		)
	}
	watts := spec.MinWatts + cfg.Utilization*(spec.MaxWatts-spec.MinWatts)
	energy := (spec.VCPUs*watts/wattsPerKilowatt + spec.ExtraMemory*memoryKWhPerGBHour) * monthlyHours * powerUsageEffectiveness
	return Estimate{
		Grams: energy * intensity * gramsPerMetricTon,
		KWh:   energy,
		Note: fmt.Sprintf(
			"operational carbon only; CCF %s; utilization=%.4g; hours=%d; PUE=%.3f; excludes embodied, storage and network emissions",
			DatasetRevision,
			cfg.Utilization,
			monthlyHours,
			powerUsageEffectiveness,
		),
	}, nil
}

func isEC2(resourceType string) bool {
	switch resourceType {
	case "aws:ec2/instance:Instance", "aws-native:ec2:Instance", "aws_ec2_instance", "aws_instance":
		return true
	default:
		return false
	}
}
