// Package generator provides synthetic infrastructure plan generation for benchmarking.
package generator

import (
	"errors"
	"fmt"
	"math/rand/v2"
)

// Common resource types for synthetic generation.
//
//nolint:gochecknoglobals // Package-level resource types used for generation
var resourceTypes = []string{
	"aws:ec2:Instance",
	"aws:s3:Bucket",
	"aws:s3:BucketObject",
	"aws:iam:Role",
	"aws:iam:Policy",
	"aws:vpc:Vpc",
	"aws:vpc:Subnet",
	"aws:vpc:SecurityGroup",
	"aws:rds:Instance",
	"aws:lambda:Function",
	"azure:compute:VirtualMachine",
	"azure:storage:Account",
	"gcp:compute:Instance",
	"gcp:storage:Bucket",
}

// Validation errors.
var (
	ErrInvalidResourceCount   = errors.New("ResourceCount must be greater than 0")
	ErrInvalidMaxDepth        = errors.New("MaxDepth must be greater than or equal to 0")
	ErrInvalidDependencyRatio = errors.New("DependencyRatio must be between 0.0 and 1.0")
)

// BenchmarkConfig configures the synthetic data generator.
type BenchmarkConfig struct {
	ResourceCount   int     // Total number of resources to generate
	MaxDepth        int     // Maximum nesting level for child resources/properties
	DependencyRatio float64 // Probability (0.0-1.0) of a resource having a dependency
	Seed            int64   // Random seed for deterministic generation
}

// SyntheticResource represents a generic infrastructure resource for testing.
type SyntheticResource struct {
	Type       string                 `json:"type"`
	Name       string                 `json:"name"`
	Properties map[string]interface{} `json:"properties"`
	DependsOn  []string               `json:"dependsOn"`
}

// SyntheticPlan is the top-level container for generated datasets.
type SyntheticPlan struct {
	Resources []SyntheticResource    `json:"resources"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

// Preset configurations for common benchmark scenarios.
//
//nolint:gochecknoglobals // Package-level preset configurations for benchmarks
var (
	PresetSmall = BenchmarkConfig{
		ResourceCount:   1000,
		MaxDepth:        3,
		DependencyRatio: 0.2,
		Seed:            42,
	}

	PresetMedium = BenchmarkConfig{
		ResourceCount:   10000,
		MaxDepth:        5,
		DependencyRatio: 0.3,
		Seed:            42,
	}

	PresetLarge = BenchmarkConfig{
		ResourceCount:   100000,
		MaxDepth:        5,
		DependencyRatio: 0.3,
		Seed:            42,
	}

	PresetDeepNesting = BenchmarkConfig{
		ResourceCount:   1000,
		MaxDepth:        10,
		DependencyRatio: 0.5,
		Seed:            42,
	}
)

// Validate checks the BenchmarkConfig for valid values.
func (c BenchmarkConfig) Validate() error {
	if c.ResourceCount <= 0 {
		return ErrInvalidResourceCount
	}
	if c.MaxDepth < 0 {
		return ErrInvalidMaxDepth
	}
	if c.DependencyRatio < 0.0 || c.DependencyRatio > 1.0 {
		return ErrInvalidDependencyRatio
	}
	return nil
}

// GeneratePlan creates a synthetic infrastructure plan based on the config.
func GeneratePlan(config BenchmarkConfig) (SyntheticPlan, error) {
	if err := config.Validate(); err != nil {
		return SyntheticPlan{}, fmt.Errorf("invalid config: %w", err)
	}

	//nolint:gosec // G404: deterministic benchmark fixture data is not security-sensitive.
	rng := rand.New(rand.NewPCG(uint64(config.Seed), uint64(config.Seed)))
	plan := SyntheticPlan{
		Resources: make([]SyntheticResource, 0, config.ResourceCount),
		Variables: make(map[string]interface{}),
	}

	// Generate resource names first for dependency references
	resourceNames := make([]string, config.ResourceCount)
	for i := range resourceNames {
		resourceNames[i] = fmt.Sprintf("resource-%d", i)
	}

	// Generate resources
	for i := 0; i < config.ResourceCount; i++ {
		resource := SyntheticResource{
			Type:       resourceTypes[rng.IntN(len(resourceTypes))],
			Name:       resourceNames[i],
			Properties: generateProperties(rng, config.MaxDepth, 0),
			DependsOn:  []string{},
		}

		// Add dependencies based on ratio (only to earlier resources)
		if i > 0 && rng.Float64() < config.DependencyRatio {
			// Pick 1-3 dependencies from earlier resources
			numDeps := rng.IntN(3) + 1
			if numDeps > i {
				numDeps = i
			}
			deps := make(map[string]bool)
			for j := 0; j < numDeps; j++ {
				depIdx := rng.IntN(i)
				deps[resourceNames[depIdx]] = true
			}
			for dep := range deps {
				resource.DependsOn = append(resource.DependsOn, dep)
			}
		}

		plan.Resources = append(plan.Resources, resource)
	}

	// Add some global variables
	plan.Variables["environment"] = "benchmark"
	plan.Variables["generated_count"] = config.ResourceCount

	return plan, nil
}

// generateProperties creates nested properties up to maxDepth.
func generateProperties(rng *rand.Rand, maxDepth, currentDepth int) map[string]interface{} {
	props := make(map[string]interface{})

	// Add 2-5 properties
	numProps := rng.IntN(4) + 2
	for i := 0; i < numProps; i++ {
		key := fmt.Sprintf("prop_%d", i)

		if currentDepth < maxDepth && rng.Float64() < 0.3 {
			// 30% chance of nested object
			props[key] = generateProperties(rng, maxDepth, currentDepth+1)
		} else {
			// Generate simple value
			switch rng.IntN(4) {
			case 0:
				props[key] = fmt.Sprintf("value-%d", rng.IntN(1000))
			case 1:
				props[key] = rng.IntN(1000)
			case 2:
				props[key] = rng.Float64() < 0.5
			case 3:
				// Array of strings
				arr := make([]string, rng.IntN(3)+1)
				for j := range arr {
					arr[j] = fmt.Sprintf("item-%d", rng.IntN(100))
				}
				props[key] = arr
			}
		}
	}

	return props
}
