package spec_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/rshade/finfocus/internal/spec"
)

// TestParseYAML_ValidSpec tests parsing a valid YAML pricing spec.
func TestParseYAML_ValidSpec(t *testing.T) {
	yamlContent := `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  onDemandHourly: 0.0104
  monthlyEstimate: 7.59
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

	require.NoError(t, err)
	assert.Equal(t, "aws", pricingSpec.Provider)
	assert.Equal(t, "ec2", pricingSpec.Service)
	assert.Equal(t, "t3.micro", pricingSpec.SKU)
	assert.Equal(t, "USD", pricingSpec.Currency)
	assert.NotNil(t, pricingSpec.Pricing)
	assert.Equal(t, 0.0104, pricingSpec.Pricing["onDemandHourly"])
}

// TestParseYAML_WithMetadata tests parsing a spec with optional metadata.
func TestParseYAML_WithMetadata(t *testing.T) {
	yamlContent := `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  onDemandHourly: 0.0104
metadata:
  region: us-east-1
  availabilityZone: us-east-1a
  description: "Test instance"
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

	require.NoError(t, err)
	require.NotNil(t, pricingSpec.Metadata)
	assert.Equal(t, "us-east-1", pricingSpec.Metadata["region"])
	assert.Equal(t, "us-east-1a", pricingSpec.Metadata["availabilityZone"])
	assert.Equal(t, "Test instance", pricingSpec.Metadata["description"])
}

// TestParseYAML_ComplexPricing tests parsing complex pricing structures.
func TestParseYAML_ComplexPricing(t *testing.T) {
	yamlContent := `provider: aws
service: rds
sku: db.t3.medium
currency: USD
pricing:
  onDemand:
    hourly: 0.068
    monthly: 49.64
  reserved:
    oneYear:
      upfront: 316
      hourly: 0.036
    threeYear:
      upfront: 570
      hourly: 0.022
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

	require.NoError(t, err)
	assert.Equal(t, "aws", pricingSpec.Provider)
	assert.Equal(t, "rds", pricingSpec.Service)
	assert.Equal(t, "db.t3.medium", pricingSpec.SKU)

	// Verify nested pricing structure
	onDemand, ok := pricingSpec.Pricing["onDemand"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 0.068, onDemand["hourly"])
	assert.Equal(t, 49.64, onDemand["monthly"])
}

// TestParseYAML_EmptyPricing tests parsing a spec with empty pricing (invalid).
func TestParseYAML_EmptyPricing(t *testing.T) {
	yamlContent := `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing: {}
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

	require.NoError(t, err)
	assert.Empty(t, pricingSpec.Pricing)
}

// TestParseYAML_MissingFields tests parsing with missing required fields.
func TestParseYAML_MissingFields(t *testing.T) {
	yamlContent := `provider: aws
service: ec2
currency: USD
pricing:
  hourly: 0.01
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

	require.NoError(t, err)
	// YAML parsing succeeds but fields are empty
	assert.Empty(t, pricingSpec.SKU)
}

// TestParseYAML_InvalidYAML tests parsing invalid YAML.
func TestParseYAML_InvalidYAML(t *testing.T) {
	invalidYAML := `provider: aws
service: ec2
sku: t3.micro
pricing: [unclosed
  array
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(invalidYAML), &pricingSpec)

	assert.Error(t, err)
}

// TestParseYAML_ExtraFields tests that extra unknown fields are ignored.
func TestParseYAML_ExtraFields(t *testing.T) {
	yamlContent := `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  hourly: 0.01
unknownField: "ignored"
anotherExtra: 123
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

	require.NoError(t, err)
	assert.Equal(t, "aws", pricingSpec.Provider)
	assert.Equal(t, "ec2", pricingSpec.Service)
	// Extra fields are ignored
}

// TestParseYAML_DifferentCurrencies tests specs with different currencies.
func TestParseYAML_DifferentCurrencies(t *testing.T) {
	testCases := []struct {
		name     string
		currency string
	}{
		{"USD currency", "USD"},
		{"EUR currency", "EUR"},
		{"GBP currency", "GBP"},
		{"JPY currency", "JPY"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			yamlContent := `provider: aws
service: ec2
sku: t3.micro
currency: ` + tc.currency + `
pricing:
  hourly: 0.01
`
			var pricingSpec spec.PricingSpec
			err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

			require.NoError(t, err)
			assert.Equal(t, tc.currency, pricingSpec.Currency)
		})
	}
}

// TestParseYAML_MultiCloudProviders tests parsing specs from different providers.
func TestParseYAML_MultiCloudProviders(t *testing.T) {
	providers := []struct {
		provider string
		service  string
		sku      string
	}{
		{"aws", "ec2", "t3.micro"},
		{"azure", "compute", "standard-d2s-v3"},
		{"gcp", "compute", "n1-standard-1"},
		{"kubernetes", "pod", "small"},
	}

	for _, p := range providers {
		t.Run(p.provider, func(t *testing.T) {
			yamlContent := `provider: ` + p.provider + `
service: ` + p.service + `
sku: ` + p.sku + `
currency: USD
pricing:
  hourly: 0.01
`
			var pricingSpec spec.PricingSpec
			err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

			require.NoError(t, err)
			assert.Equal(t, p.provider, pricingSpec.Provider)
			assert.Equal(t, p.service, pricingSpec.Service)
			assert.Equal(t, p.sku, pricingSpec.SKU)
		})
	}
}

// TestParseSpecFilename_ValidFormats tests parsing valid spec filenames.
func TestParseSpecFilename_ValidFormats(t *testing.T) {
	testCases := []struct {
		filename string
		provider string
		service  string
		sku      string
	}{
		{
			filename: "aws-ec2-t3.micro.yaml",
			provider: "aws",
			service:  "ec2",
			sku:      "t3.micro",
		},
		{
			filename: "aws-rds-db.t3.medium.yaml",
			provider: "aws",
			service:  "rds",
			sku:      "db.t3.medium",
		},
		{
			filename: "azure-compute-standard-d2s-v3.yaml",
			provider: "azure",
			service:  "compute",
			sku:      "standard-d2s-v3",
		},
		{
			filename: "gcp-compute-n1-standard-1.yaml",
			provider: "gcp",
			service:  "compute",
			sku:      "n1-standard-1",
		},
		{
			filename: "aws-ec2-t3.micro.yml",
			provider: "aws",
			service:  "ec2",
			sku:      "t3.micro",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.filename, func(t *testing.T) {
			provider, service, sku, valid := spec.ParseSpecFilename(tc.filename)

			assert.True(t, valid)
			assert.Equal(t, tc.provider, provider)
			assert.Equal(t, tc.service, service)
			assert.Equal(t, tc.sku, sku)
		})
	}
}

// TestParseSpecFilename_InvalidFormats tests parsing invalid spec filenames.
func TestParseSpecFilename_InvalidFormats(t *testing.T) {
	invalidFilenames := []struct {
		filename string
		reason   string
	}{
		{"aws-ec2.yaml", "only two parts"},
		{"aws.yaml", "only one part"},
		{".yaml", "empty filename"},
		{"nohyphens.yaml", "no hyphens"},
		{"aws-ec2-t3.micro.json", "wrong extension (.json)"},
		{"aws-ec2-t3.micro.txt", "wrong extension (.txt)"},
		{"aws--t3.micro.yaml", "empty service part"},
		{"-ec2-t3.micro.yaml", "empty provider"},
		{"aws-ec2-.yaml", "empty SKU"},
	}

	for _, tc := range invalidFilenames {
		t.Run(tc.reason, func(t *testing.T) {
			provider, service, sku, valid := spec.ParseSpecFilename(tc.filename)

			assert.False(t, valid, "Expected invalid: %s", tc.reason)
			assert.Empty(t, provider)
			assert.Empty(t, service)
			assert.Empty(t, sku)
		})
	}
}

// TestParseSpecFilename_SKUWithMultipleDashes tests SKUs containing multiple dashes.
func TestParseSpecFilename_SKUWithMultipleDashes(t *testing.T) {
	filename := "azure-compute-standard-d2s-v3-spot.yaml"
	provider, service, sku, valid := spec.ParseSpecFilename(filename)

	assert.True(t, valid)
	assert.Equal(t, "azure", provider)
	assert.Equal(t, "compute", service)
	assert.Equal(t, "standard-d2s-v3-spot", sku)
}

// TestParseSpecFilename_BothExtensions tests that both .yaml and .yml are accepted.
func TestParseSpecFilename_BothExtensions(t *testing.T) {
	testCases := []string{
		"aws-ec2-t3.micro.yaml",
		"aws-ec2-t3.micro.yml",
	}

	for _, filename := range testCases {
		t.Run(filename, func(t *testing.T) {
			provider, service, sku, valid := spec.ParseSpecFilename(filename)

			assert.True(t, valid)
			assert.Equal(t, "aws", provider)
			assert.Equal(t, "ec2", service)
			assert.Equal(t, "t3.micro", sku)
		})
	}
}

// TestParseYAML_FromFile tests parsing YAML from an actual file.
func TestParseYAML_FromFile(t *testing.T) {
	tmpDir := t.TempDir()

	yamlContent := `provider: aws
service: s3
sku: standard-storage
currency: USD
pricing:
  perGB: 0.023
  requestCost:
    put: 0.005
    get: 0.0004
metadata:
  storageClass: STANDARD
  redundancy: "99.999999999%"
`
	filename := filepath.Join(tmpDir, "aws-s3-standard-storage.yaml")
	err := os.WriteFile(filename, []byte(yamlContent), 0644)
	require.NoError(t, err)

	data, err := os.ReadFile(filename)
	require.NoError(t, err)

	var pricingSpec spec.PricingSpec
	err = yaml.Unmarshal(data, &pricingSpec)

	require.NoError(t, err)
	assert.Equal(t, "aws", pricingSpec.Provider)
	assert.Equal(t, "s3", pricingSpec.Service)
	assert.Equal(t, "standard-storage", pricingSpec.SKU)
	assert.Equal(t, "USD", pricingSpec.Currency)

	// Verify nested pricing
	assert.Contains(t, pricingSpec.Pricing, "perGB")
	assert.Contains(t, pricingSpec.Pricing, "requestCost")

	// Verify metadata
	assert.Equal(t, "STANDARD", pricingSpec.Metadata["storageClass"])
	assert.Equal(t, "99.999999999%", pricingSpec.Metadata["redundancy"])
}

// TestParseYAML_NumericTypes tests handling different numeric types in pricing.
func TestParseYAML_NumericTypes(t *testing.T) {
	yamlContent := `provider: test
service: compute
sku: instance
currency: USD
pricing:
  floatValue: 0.0104
  intValue: 10
  scientificNotation: 1.5e-3
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

	require.NoError(t, err)
	assert.Equal(t, 0.0104, pricingSpec.Pricing["floatValue"])
	assert.Equal(t, 10, pricingSpec.Pricing["intValue"])
	assert.Equal(t, 1.5e-3, pricingSpec.Pricing["scientificNotation"])
}

// TestParseYAML_BooleanInMetadata tests boolean values in metadata.
func TestParseYAML_BooleanInMetadata(t *testing.T) {
	yamlContent := `provider: test
service: compute
sku: instance
currency: USD
pricing:
  hourly: 0.01
metadata:
  spotInstance: true
  burstable: false
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

	require.NoError(t, err)
	assert.Equal(t, true, pricingSpec.Metadata["spotInstance"])
	assert.Equal(t, false, pricingSpec.Metadata["burstable"])
}

// TestParseYAML_ListInPricing tests array/list values in pricing.
func TestParseYAML_ListInPricing(t *testing.T) {
	yamlContent := `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  tiers:
    - threshold: 0
      price: 0.0104
    - threshold: 1000
      price: 0.0095
    - threshold: 10000
      price: 0.0089
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

	require.NoError(t, err)
	tiers, ok := pricingSpec.Pricing["tiers"].([]interface{})
	require.True(t, ok)
	assert.Len(t, tiers, 3)
}

// TestParseYAML_UnicodeContent tests parsing YAML with Unicode characters.
func TestParseYAML_UnicodeContent(t *testing.T) {
	yamlContent := `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  hourly: 0.01
metadata:
  description: "インスタンス 🚀"
  region: "日本"
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

	require.NoError(t, err)
	assert.Contains(t, pricingSpec.Metadata["description"], "インスタンス")
	assert.Contains(t, pricingSpec.Metadata["description"], "🚀")
	assert.Equal(t, "日本", pricingSpec.Metadata["region"])
}

// TestParseYAML_EscapedCharacters tests handling escaped characters in YAML.
func TestParseYAML_EscapedCharacters(t *testing.T) {
	yamlContent := `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  hourly: 0.01
metadata:
  description: "Instance with \"quotes\" and \\ backslashes"
`
	var pricingSpec spec.PricingSpec
	err := yaml.Unmarshal([]byte(yamlContent), &pricingSpec)

	require.NoError(t, err)
	assert.Contains(t, pricingSpec.Metadata["description"], "\"quotes\"")
	assert.Contains(t, pricingSpec.Metadata["description"], "\\")
}
