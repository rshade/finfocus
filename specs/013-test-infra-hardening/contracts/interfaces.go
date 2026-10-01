// Contracts for Test Infrastructure
// Note: As this is an internal infrastructure feature, there are no external API contracts.
// The "contracts" here refer to the Go interfaces defined for the test harness.

package benchmarks

// Generator defines the interface for creating synthetic test data.
type Generator interface {
	// GeneratePlan creates a synthetic plan with the specified configuration.
	GeneratePlan(config BenchmarkConfig) (*SyntheticPlan, error)
}

// BenchmarkConfig controls the generation parameters.
type BenchmarkConfig struct {
	ResourceCount   int
	MaxDepth        int
	DependencyRatio float64
	Seed            int64
}

// SyntheticPlan represents the generated infrastructure.
type SyntheticPlan struct {
	Resources []SyntheticResource
}

// SyntheticResource represents a single node in the plan.
type SyntheticResource struct {
	Type       string
	Name       string
	Properties map[string]any
	DependsOn  []string
}
