package benchmarks_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/rshade/finfocus/internal/ingest"
)

// generateStepJSON builds a step JSON string with realistic steps/newState format.
func generateStepJSON(index int) string {
	urn := fmt.Sprintf("urn:pulumi:dev::test::aws:ec2/instance:Instance::i-%d", index)
	step := `{"op":"create","urn":"%s","type":"aws:ec2/instance:Instance",` +
		`"newState":{"type":"aws:ec2/instance:Instance","urn":"%s",` +
		`"inputs":{"instanceType":"t3.micro"}}}`
	return fmt.Sprintf(step, urn, urn)
}

// BenchmarkParse_PulumiPlan benchmarks parsing of a typical Pulumi plan JSON.
func BenchmarkParse_PulumiPlan(b *testing.B) {
	b.ReportAllocs()
	// Generate a simple plan string using realistic steps/newState format
	resources := make([]string, 100)
	for i := 0; i < 100; i++ {
		resources[i] = generateStepJSON(i)
	}
	jsonStr := fmt.Sprintf(`{"steps": [%s]}`, strings.Join(resources, ","))
	data := []byte(jsonStr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result ingest.PulumiPlan
		if err := json.Unmarshal(data, &result); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParse_LargePlan benchmarks parsing of a large Pulumi plan JSON (10k resources).
func BenchmarkParse_LargePlan(b *testing.B) {
	b.ReportAllocs()
	// Generate a large plan string using realistic steps/newState format
	count := 10000
	resources := make([]string, count)
	for i := 0; i < count; i++ {
		resources[i] = generateStepJSON(i)
	}
	jsonStr := fmt.Sprintf(`{"steps": [%s]}`, strings.Join(resources, ","))
	data := []byte(jsonStr)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result ingest.PulumiPlan
		if err := json.Unmarshal(data, &result); err != nil {
			b.Fatal(err)
		}
	}
}
