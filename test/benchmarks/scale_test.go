package benchmarks_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/ingest"
	"github.com/rshade/finfocus/test/benchmarks/generator"
)

// convertToResourceDescriptors converts SyntheticResources to engine.ResourceDescriptor.
func convertToResourceDescriptors(synthPlan generator.SyntheticPlan) []engine.ResourceDescriptor {
	descriptors := make([]engine.ResourceDescriptor, len(synthPlan.Resources))
	for i, r := range synthPlan.Resources {
		descriptors[i] = engine.ResourceDescriptor{
			ID:         r.Name,
			Type:       r.Type,
			Provider:   extractProvider(r.Type),
			Properties: r.Properties,
		}
	}
	return descriptors
}

// extractProvider extracts the provider from a resource type string (e.g., "aws:ec2:Instance" -> "aws").
func extractProvider(resourceType string) string {
	for i, c := range resourceType {
		if c == ':' {
			return resourceType[:i]
		}
	}
	return resourceType
}

// BenchmarkScale1K benchmarks processing 1,000 resources (Small preset).
// Target: Quick completion for regression testing.
func BenchmarkScale1K(b *testing.B) {
	plan, err := generator.GeneratePlan(generator.PresetSmall)
	if err != nil {
		b.Fatal(err)
	}

	resources := convertToResourceDescriptors(plan)
	eng := engine.New(nil, nil)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_, costErr := eng.GetProjectedCost(ctx, resources)
		if costErr != nil {
			b.Fatal(costErr)
		}
	}

	b.ReportMetric(float64(len(resources)), "resources")
}

// BenchmarkScale10K benchmarks processing 10,000 resources (Medium preset).
// Target: Standard benchmark for typical enterprise scale.
func BenchmarkScale10K(b *testing.B) {
	plan, err := generator.GeneratePlan(generator.PresetMedium)
	if err != nil {
		b.Fatal(err)
	}

	resources := convertToResourceDescriptors(plan)
	eng := engine.New(nil, nil)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_, costErr := eng.GetProjectedCost(ctx, resources)
		if costErr != nil {
			b.Fatal(costErr)
		}
	}

	b.ReportMetric(float64(len(resources)), "resources")
}

// BenchmarkScale100K benchmarks processing 100,000 resources (Large preset).
// Target: < 5 minutes for stress testing.
func BenchmarkScale100K(b *testing.B) {
	plan, err := generator.GeneratePlan(generator.PresetLarge)
	if err != nil {
		b.Fatal(err)
	}

	resources := convertToResourceDescriptors(plan)
	eng := engine.New(nil, nil)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_, costErr := eng.GetProjectedCost(ctx, resources)
		if costErr != nil {
			b.Fatal(costErr)
		}
	}

	b.ReportMetric(float64(len(resources)), "resources")
}

// BenchmarkDeeplyNested benchmarks processing resources with deep nesting.
// Tests depth complexity rather than resource count.
func BenchmarkDeeplyNested(b *testing.B) {
	plan, err := generator.GeneratePlan(generator.PresetDeepNesting)
	if err != nil {
		b.Fatal(err)
	}

	resources := convertToResourceDescriptors(plan)
	eng := engine.New(nil, nil)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for range b.N {
		_, costErr := eng.GetProjectedCost(ctx, resources)
		if costErr != nil {
			b.Fatal(costErr)
		}
	}

	b.ReportMetric(float64(generator.PresetDeepNesting.MaxDepth), "max_depth")
	b.ReportMetric(float64(len(resources)), "resources")
}

// BenchmarkJSONParsing benchmarks JSON parsing performance at scale.
// Tests the ingest package's ability to handle large JSON payloads.
func BenchmarkJSONParsing(b *testing.B) {
	plan, err := generator.GeneratePlan(generator.PresetMedium)
	if err != nil {
		b.Fatal(err)
	}

	// Convert to Pulumi plan format
	pulumiPlan := convertToPulumiPlanFormat(plan)
	jsonData, err := json.Marshal(pulumiPlan)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(int64(len(jsonData)))

	for range b.N {
		var parsed ingest.PulumiPlan
		if unmarshalErr := json.Unmarshal(jsonData, &parsed); unmarshalErr != nil {
			b.Fatal(unmarshalErr)
		}
	}
}

// convertToPulumiPlanFormat converts SyntheticPlan to Pulumi plan format for JSON parsing benchmark.
func convertToPulumiPlanFormat(plan generator.SyntheticPlan) ingest.PulumiPlan {
	steps := make([]ingest.PulumiStep, len(plan.Resources))
	for i, r := range plan.Resources {
		steps[i] = ingest.PulumiStep{
			Op:     "create",
			URN:    "urn:pulumi:test::bench::" + r.Type + "::" + r.Name,
			Type:   r.Type,
			Inputs: r.Properties,
		}
	}
	return ingest.PulumiPlan{Steps: steps}
}

// BenchmarkGeneratorOverhead measures the overhead of generating synthetic data.
// This helps ensure the generator isn't the bottleneck in benchmarks.
func BenchmarkGeneratorOverhead(b *testing.B) {
	b.Run("Small_1K", func(b *testing.B) {
		for range b.N {
			_, err := generator.GeneratePlan(generator.PresetSmall)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Medium_10K", func(b *testing.B) {
		for range b.N {
			_, err := generator.GeneratePlan(generator.PresetMedium)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Large_100K", func(b *testing.B) {
		b.Skip("Skipping 100K generation overhead test - too slow for regular benchmarks")
		for range b.N {
			_, err := generator.GeneratePlan(generator.PresetLarge)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
