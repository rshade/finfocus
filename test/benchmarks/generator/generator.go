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

// Benchmark tuning values for the preset configurations.
const (
	// benchmarkSeed is the fixed random seed shared by all presets for deterministic generation.
	benchmarkSeed = 42

	// smallResourceCount is the resource count for the small and deep-nesting presets.
	smallResourceCount = 1000
	// mediumResourceCount is the resource count for the medium preset.
	mediumResourceCount = 10000
	// largeResourceCount is the resource count for the large preset.
	largeResourceCount = 100000

	// smallMaxDepth is the nesting depth for the small preset.
	smallMaxDepth = 3
	// mediumMaxDepth is the nesting depth for the medium and large presets.
	mediumMaxDepth = 5
	// deepNestingMaxDepth is the nesting depth for the deep-nesting preset.
	deepNestingMaxDepth = 10

	// smallDependencyRatio is the dependency probability for the small preset.
	smallDependencyRatio = 0.2
	// mediumDependencyRatio is the dependency probability for the medium and large presets.
	mediumDependencyRatio = 0.3
	// deepDependencyRatio is the dependency probability for the deep-nesting preset.
	deepDependencyRatio = 0.5
)

// Random generation bounds for synthetic plan data.
const (
	// maxDependencies bounds the random dependency count per resource (1-3).
	maxDependencies = 3
	// minProperties is the minimum property count per nesting level.
	minProperties = 2
	// propertyCountSpread bounds the random extra properties per nesting level (2-5 total).
	propertyCountSpread = 4
	// nestedObjectChance is the probability of generating a nested object property.
	nestedObjectChance = 0.3
	// boolPropertyChance is the probability of a true boolean property value.
	boolPropertyChance = 0.5
	// maxPropertyValue bounds random numeric property values and string suffixes.
	maxPropertyValue = 1000
	// maxArrayItems bounds the random string-array property length (1-3).
	maxArrayItems = 3
	// maxItemSuffix bounds random array item name suffixes.
	maxItemSuffix = 100
	// propertyValueKinds is the number of simple property value kinds; it must
	// match the count of valueKind* constants below.
	propertyValueKinds = 4
)

// Property value kinds for the random value switch in generateProperties.
const (
	valueKindString = iota
	valueKindInt
	valueKindBool
	valueKindArray
)

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
	Type       string         `json:"type"`
	Name       string         `json:"name"`
	Properties map[string]any `json:"properties"`
	DependsOn  []string       `json:"dependsOn"`
}

// SyntheticPlan is the top-level container for generated datasets.
type SyntheticPlan struct {
	Resources []SyntheticResource `json:"resources"`
	Variables map[string]any      `json:"variables,omitempty"`
}

// Preset configurations for common benchmark scenarios.
//
//nolint:gochecknoglobals // Package-level preset configurations for benchmarks
var (
	PresetSmall = BenchmarkConfig{
		ResourceCount:   smallResourceCount,
		MaxDepth:        smallMaxDepth,
		DependencyRatio: smallDependencyRatio,
		Seed:            benchmarkSeed,
	}

	PresetMedium = BenchmarkConfig{
		ResourceCount:   mediumResourceCount,
		MaxDepth:        mediumMaxDepth,
		DependencyRatio: mediumDependencyRatio,
		Seed:            benchmarkSeed,
	}

	PresetLarge = BenchmarkConfig{
		ResourceCount:   largeResourceCount,
		MaxDepth:        mediumMaxDepth,
		DependencyRatio: mediumDependencyRatio,
		Seed:            benchmarkSeed,
	}

	PresetDeepNesting = BenchmarkConfig{
		ResourceCount:   smallResourceCount,
		MaxDepth:        deepNestingMaxDepth,
		DependencyRatio: deepDependencyRatio,
		Seed:            benchmarkSeed,
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
		Variables: make(map[string]any),
	}

	// Generate resource names first for dependency references
	resourceNames := make([]string, config.ResourceCount)
	for i := range resourceNames {
		resourceNames[i] = fmt.Sprintf("resource-%d", i)
	}

	// Generate resources
	for i := range config.ResourceCount {
		resource := SyntheticResource{
			Type:       resourceTypes[rng.IntN(len(resourceTypes))],
			Name:       resourceNames[i],
			Properties: generateProperties(rng, config.MaxDepth, 0),
			DependsOn:  []string{},
		}

		// Add dependencies based on ratio (only to earlier resources)
		if i > 0 && rng.Float64() < config.DependencyRatio {
			// Pick 1-3 dependencies from earlier resources
			numDeps := min(rng.IntN(maxDependencies)+1, i)
			deps := make(map[string]bool)
			for range numDeps {
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
func generateProperties(rng *rand.Rand, maxDepth, currentDepth int) map[string]any {
	props := make(map[string]any)

	// Add 2-5 properties
	numProps := rng.IntN(propertyCountSpread) + minProperties
	for i := range numProps {
		key := fmt.Sprintf("prop_%d", i)

		if currentDepth < maxDepth && rng.Float64() < nestedObjectChance {
			// 30% chance of nested object
			props[key] = generateProperties(rng, maxDepth, currentDepth+1)
		} else {
			// Generate simple value
			switch rng.IntN(propertyValueKinds) {
			case valueKindString:
				props[key] = fmt.Sprintf("value-%d", rng.IntN(maxPropertyValue))
			case valueKindInt:
				props[key] = rng.IntN(maxPropertyValue)
			case valueKindBool:
				props[key] = rng.Float64() < boolPropertyChance
			case valueKindArray:
				// Array of strings
				arr := make([]string, rng.IntN(maxArrayItems)+1)
				for j := range arr {
					arr[j] = fmt.Sprintf("item-%d", rng.IntN(maxItemSuffix))
				}
				props[key] = arr
			}
		}
	}

	return props
}
