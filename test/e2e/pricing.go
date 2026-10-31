package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// PricingReference holds expected monthly costs for resources.
// Key: Resource type or description. Value: Expected monthly cost in USD.
var PricingReference = map[string]float64{
	"t2.micro": 8.468, // $0.0116/hour * 730 hours
	"t3.micro": 7.59,  // ~$0.0104/hour * 730 hours
	"gp3":      0.64,  // 8GB * $0.08/GB-month
}

// VolumePricePerGBMonth holds us-east-1 EBS prices per GB-month by volume type.
var VolumePricePerGBMonth = map[string]float64{
	"gp2": 0.10,
	"gp3": 0.08,
}

// GetExpectedCost returns the expected monthly cost for a given resource key.
func GetExpectedCost(key string) (float64, bool) {
	val, ok := PricingReference[key]
	return val, ok
}

// FixtureMonthlyCost prices what a fixture's Pulumi.yaml deploys: each
// aws:ec2:Instance by instance type and each aws:ebs:Volume by size and type.
// Deriving the total from the fixture keeps the expectation in step with it;
// a hand-kept constant drifted when fixtures/ec2 moved to t2.micro plus a
// volume (#347) and the nightly E2E job read the wrong figure for months.
func FixtureMonthlyCost(t *testing.T, dir string) float64 {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "Pulumi.yaml"))
	require.NoError(t, err)
	var project struct {
		Resources map[string]struct {
			Type       string         `yaml:"type"`
			Properties map[string]any `yaml:"properties"`
		} `yaml:"resources"`
	}
	require.NoError(t, yaml.Unmarshal(data, &project))

	var total float64
	for name, resource := range project.Resources {
		switch resource.Type {
		case "aws:ec2:Instance":
			instanceType, _ := resource.Properties["instanceType"].(string)
			price, ok := PricingReference[instanceType]
			require.Truef(t, ok, "no pricing reference for %s instance type %q", name, instanceType)
			total += price
		case "aws:ebs:Volume":
			volumeType, _ := resource.Properties["type"].(string)
			size, _ := resource.Properties["size"].(int)
			perGB, ok := VolumePricePerGBMonth[volumeType]
			require.Truef(t, ok, "no per-GB price for %s volume type %q", name, volumeType)
			require.Positivef(t, size, "%s has no size", name)
			total += float64(size) * perGB
		}
	}
	return total
}
