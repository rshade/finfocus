package spec_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/spec"
)

// TestNewLoader_CreatesLoader tests loader creation.
func TestNewLoader_CreatesLoader(t *testing.T) {
	specDir := "/test/specs"
	loader := spec.NewLoader(specDir)

	assert.NotNil(t, loader)
}

// TestLoadSpec_ValidSpec tests loading a valid pricing spec.
func TestLoadSpec_ValidSpec(t *testing.T) {
	tmpDir := t.TempDir()

	specContent := `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  onDemandHourly: 0.0104
  monthlyEstimate: 7.59
`
	filename := filepath.Join(tmpDir, "aws-ec2-t3.micro.yaml")
	err := os.WriteFile(filename, []byte(specContent), 0644)
	require.NoError(t, err)

	loader := spec.NewLoader(tmpDir)
	result, err := loader.LoadSpec("aws", "ec2", "t3.micro")

	require.NoError(t, err)
	require.NotNil(t, result)

	pricingSpec, ok := result.(*spec.PricingSpec)
	require.True(t, ok)
	assert.Equal(t, "aws", pricingSpec.Provider)
	assert.Equal(t, "ec2", pricingSpec.Service)
	assert.Equal(t, "t3.micro", pricingSpec.SKU)
	assert.Equal(t, "USD", pricingSpec.Currency)
}

// TestLoadSpec_NonExistentFile tests error when spec file doesn't exist.
func TestLoadSpec_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()

	loader := spec.NewLoader(tmpDir)
	result, err := loader.LoadSpec("aws", "ec2", "nonexistent")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, spec.ErrSpecNotFound)
}

// TestLoadSpec_NonExistentDirectory tests error when directory doesn't exist.
func TestLoadSpec_NonExistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nonexistentDir := filepath.Join(tmpDir, "nonexistent")

	loader := spec.NewLoader(nonexistentDir)
	result, err := loader.LoadSpec("aws", "ec2", "t3.micro")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, spec.ErrSpecNotFound)
}

// TestLoadSpec_InvalidYAML tests error when YAML is malformed.
func TestLoadSpec_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	invalidContent := `provider: aws
service: ec2
sku: t3.micro
pricing: [unclosed
`
	filename := filepath.Join(tmpDir, "aws-ec2-t3.micro.yaml")
	err := os.WriteFile(filename, []byte(invalidContent), 0644)
	require.NoError(t, err)

	loader := spec.NewLoader(tmpDir)
	result, err := loader.LoadSpec("aws", "ec2", "t3.micro")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "parsing spec YAML")
}

// TestLoadSpec_SKUWithDots tests loading spec with dots in SKU.
func TestLoadSpec_SKUWithDots(t *testing.T) {
	tmpDir := t.TempDir()

	specContent := `provider: aws
service: rds
sku: db.t3.medium
currency: USD
pricing:
  hourly: 0.068
`
	filename := filepath.Join(tmpDir, "aws-rds-db.t3.medium.yaml")
	err := os.WriteFile(filename, []byte(specContent), 0644)
	require.NoError(t, err)

	loader := spec.NewLoader(tmpDir)
	result, err := loader.LoadSpec("aws", "rds", "db.t3.medium")

	require.NoError(t, err)
	pricingSpec, ok := result.(*spec.PricingSpec)
	require.True(t, ok)
	assert.Equal(t, "db.t3.medium", pricingSpec.SKU)
}

// TestLoadSpec_SKUWithDashes tests loading spec with dashes in SKU.
func TestLoadSpec_SKUWithDashes(t *testing.T) {
	tmpDir := t.TempDir()

	specContent := `provider: azure
service: compute
sku: standard-d2s-v3
currency: USD
pricing:
  hourly: 0.096
`
	filename := filepath.Join(tmpDir, "azure-compute-standard-d2s-v3.yaml")
	err := os.WriteFile(filename, []byte(specContent), 0644)
	require.NoError(t, err)

	loader := spec.NewLoader(tmpDir)
	result, err := loader.LoadSpec("azure", "compute", "standard-d2s-v3")

	require.NoError(t, err)
	pricingSpec, ok := result.(*spec.PricingSpec)
	require.True(t, ok)
	assert.Equal(t, "standard-d2s-v3", pricingSpec.SKU)
}

// TestLoadSpec_WithMetadata tests loading spec with metadata section.
func TestLoadSpec_WithMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	specContent := `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  hourly: 0.0104
metadata:
  region: us-east-1
  vcpus: 2
  memory: "1 GiB"
`
	filename := filepath.Join(tmpDir, "aws-ec2-t3.micro.yaml")
	err := os.WriteFile(filename, []byte(specContent), 0644)
	require.NoError(t, err)

	loader := spec.NewLoader(tmpDir)
	result, err := loader.LoadSpec("aws", "ec2", "t3.micro")

	require.NoError(t, err)
	pricingSpec, ok := result.(*spec.PricingSpec)
	require.True(t, ok)
	require.NotNil(t, pricingSpec.Metadata)
	assert.Equal(t, "us-east-1", pricingSpec.Metadata["region"])
	assert.Equal(t, 2, pricingSpec.Metadata["vcpus"])
}

// TestLoadSpec_PermissionDenied tests error when file permissions deny read.
func TestLoadSpec_PermissionDenied(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("Skipping permission test on Windows or when running as root")
	}

	tmpDir := t.TempDir()

	filename := filepath.Join(tmpDir, "aws-ec2-restricted.yaml")
	err := os.WriteFile(filename, []byte("test: data"), 0000)
	require.NoError(t, err)
	defer os.Chmod(filename, 0644)

	loader := spec.NewLoader(tmpDir)
	result, err := loader.LoadSpec("aws", "ec2", "restricted")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "reading spec file")
}

// TestListSpecs_EmptyDirectory tests listing specs in empty directory.
func TestListSpecs_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	loader := spec.NewLoader(tmpDir)
	specs, err := loader.ListSpecs()

	require.NoError(t, err)
	assert.Empty(t, specs)
}

// TestListSpecs_NonExistentDirectory tests listing specs in non-existent directory.
func TestListSpecs_NonExistentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nonexistentDir := filepath.Join(tmpDir, "nonexistent")

	loader := spec.NewLoader(nonexistentDir)
	specs, err := loader.ListSpecs()

	require.NoError(t, err)
	assert.Nil(t, specs)
}

// TestListSpecs_MultipleSpecs tests listing multiple spec files.
func TestListSpecs_MultipleSpecs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple spec files
	specFiles := []string{
		"aws-ec2-t3.micro.yaml",
		"aws-rds-db.t3.medium.yaml",
		"azure-compute-standard-d2s.yaml",
		"gcp-compute-n1-standard-1.yaml",
	}

	for _, filename := range specFiles {
		path := filepath.Join(tmpDir, filename)
		err := os.WriteFile(
			path,
			[]byte("provider: test\nservice: test\nsku: test\ncurrency: USD\npricing:\n  hourly: 0.01\n"),
			0644,
		)
		require.NoError(t, err)
	}

	loader := spec.NewLoader(tmpDir)
	specs, err := loader.ListSpecs()

	require.NoError(t, err)
	assert.Len(t, specs, len(specFiles))

	// Verify all files are listed
	for _, filename := range specFiles {
		assert.Contains(t, specs, filename)
	}
}

// TestListSpecs_FiltersNonYAMLFiles tests that non-YAML files are excluded.
func TestListSpecs_FiltersNonYAMLFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mixed files
	files := []struct {
		name       string
		shouldList bool
	}{
		{"aws-ec2-t3.micro.yaml", true},
		{"aws-rds-db.t3.medium.yaml", true},
		{"readme.txt", false},
		{"config.json", false},
		{"notes.md", false},
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f.name)
		err := os.WriteFile(path, []byte("test content"), 0644)
		require.NoError(t, err)
	}

	loader := spec.NewLoader(tmpDir)
	specs, err := loader.ListSpecs()

	require.NoError(t, err)

	expectedCount := 0
	for _, f := range files {
		if f.shouldList {
			expectedCount++
			assert.Contains(t, specs, f.name)
		} else {
			assert.NotContains(t, specs, f.name)
		}
	}

	assert.Len(t, specs, expectedCount)
}

// TestListSpecs_ExcludesDirectories tests that subdirectories are excluded.
func TestListSpecs_ExcludesDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file and a directory
	err := os.WriteFile(filepath.Join(tmpDir, "aws-ec2-t3.micro.yaml"), []byte("test"), 0644)
	require.NoError(t, err)

	err = os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	require.NoError(t, err)

	loader := spec.NewLoader(tmpDir)
	specs, err := loader.ListSpecs()

	require.NoError(t, err)
	assert.Len(t, specs, 1)
	assert.Contains(t, specs, "aws-ec2-t3.micro.yaml")
}

// TestListSpecs_PermissionError tests error when directory is not readable.
func TestListSpecs_PermissionError(t *testing.T) {
	if runtime.GOOS == "windows" || os.Getuid() == 0 {
		t.Skip("Skipping permission test on Windows or when running as root")
	}

	tmpDir := t.TempDir()

	err := os.Chmod(tmpDir, 0000)
	require.NoError(t, err)
	defer os.Chmod(tmpDir, 0755)

	loader := spec.NewLoader(tmpDir)
	specs, err := loader.ListSpecs()

	assert.Error(t, err)
	assert.Nil(t, specs)
	assert.Contains(t, err.Error(), "reading spec directory")
}

// TestValidateSpec_ValidSpec tests validation of a valid spec.
func TestValidateSpec_ValidSpec(t *testing.T) {
	pricingSpec := &spec.PricingSpec{
		Provider: "aws",
		Service:  "ec2",
		SKU:      "t3.micro",
		Currency: "USD",
		Pricing: map[string]interface{}{
			"hourly": 0.0104,
		},
	}

	err := spec.ValidateSpec(pricingSpec)
	assert.NoError(t, err)
}

// TestValidateSpec_MissingProvider tests validation fails for missing provider.
func TestValidateSpec_MissingProvider(t *testing.T) {
	pricingSpec := &spec.PricingSpec{
		Service:  "ec2",
		SKU:      "t3.micro",
		Currency: "USD",
		Pricing: map[string]interface{}{
			"hourly": 0.0104,
		},
	}

	err := spec.ValidateSpec(pricingSpec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "provider is required")
}

// TestValidateSpec_MissingService tests validation fails for missing service.
func TestValidateSpec_MissingService(t *testing.T) {
	pricingSpec := &spec.PricingSpec{
		Provider: "aws",
		SKU:      "t3.micro",
		Currency: "USD",
		Pricing: map[string]interface{}{
			"hourly": 0.0104,
		},
	}

	err := spec.ValidateSpec(pricingSpec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service is required")
}

// TestValidateSpec_MissingSKU tests validation fails for missing SKU.
func TestValidateSpec_MissingSKU(t *testing.T) {
	pricingSpec := &spec.PricingSpec{
		Provider: "aws",
		Service:  "ec2",
		Currency: "USD",
		Pricing: map[string]interface{}{
			"hourly": 0.0104,
		},
	}

	err := spec.ValidateSpec(pricingSpec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SKU is required")
}

// TestValidateSpec_MissingCurrency tests validation fails for missing currency.
func TestValidateSpec_MissingCurrency(t *testing.T) {
	pricingSpec := &spec.PricingSpec{
		Provider: "aws",
		Service:  "ec2",
		SKU:      "t3.micro",
		Pricing: map[string]interface{}{
			"hourly": 0.0104,
		},
	}

	err := spec.ValidateSpec(pricingSpec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "currency is required")
}

// TestValidateSpec_EmptyPricing tests validation fails for empty pricing.
func TestValidateSpec_EmptyPricing(t *testing.T) {
	pricingSpec := &spec.PricingSpec{
		Provider: "aws",
		Service:  "ec2",
		SKU:      "t3.micro",
		Currency: "USD",
		Pricing:  map[string]interface{}{},
	}

	err := spec.ValidateSpec(pricingSpec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pricing information is required")
}

// TestValidateSpec_NilPricing tests validation fails for nil pricing.
func TestValidateSpec_NilPricing(t *testing.T) {
	pricingSpec := &spec.PricingSpec{
		Provider: "aws",
		Service:  "ec2",
		SKU:      "t3.micro",
		Currency: "USD",
		Pricing:  nil,
	}

	err := spec.ValidateSpec(pricingSpec)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pricing information is required")
}

// TestValidateSpec_OptionalMetadata tests that metadata is optional.
func TestValidateSpec_OptionalMetadata(t *testing.T) {
	pricingSpec := &spec.PricingSpec{
		Provider: "aws",
		Service:  "ec2",
		SKU:      "t3.micro",
		Currency: "USD",
		Pricing: map[string]interface{}{
			"hourly": 0.0104,
		},
		Metadata: nil,
	}

	err := spec.ValidateSpec(pricingSpec)
	assert.NoError(t, err)
}

// TestLoadSpec_MultipleProviders tests loading specs from different providers.
func TestLoadSpec_MultipleProviders(t *testing.T) {
	tmpDir := t.TempDir()

	providers := []struct {
		provider string
		service  string
		sku      string
	}{
		{"aws", "ec2", "t3.micro"},
		{"azure", "compute", "standard-d2s"},
		{"gcp", "compute", "n1-standard-1"},
	}

	for _, p := range providers {
		content := `provider: ` + p.provider + `
service: ` + p.service + `
sku: ` + p.sku + `
currency: USD
pricing:
  hourly: 0.01
`
		filename := filepath.Join(tmpDir, p.provider+"-"+p.service+"-"+p.sku+".yaml")
		err := os.WriteFile(filename, []byte(content), 0644)
		require.NoError(t, err)
	}

	loader := spec.NewLoader(tmpDir)

	for _, p := range providers {
		t.Run(p.provider, func(t *testing.T) {
			result, err := loader.LoadSpec(p.provider, p.service, p.sku)

			require.NoError(t, err)
			pricingSpec, ok := result.(*spec.PricingSpec)
			require.True(t, ok)
			assert.Equal(t, p.provider, pricingSpec.Provider)
			assert.Equal(t, p.service, pricingSpec.Service)
			assert.Equal(t, p.sku, pricingSpec.SKU)
		})
	}
}

// TestLoadSpec_ComplexNestedPricing tests loading spec with complex pricing structure.
func TestLoadSpec_ComplexNestedPricing(t *testing.T) {
	tmpDir := t.TempDir()

	specContent := `provider: aws
service: rds
sku: db.m5.large
currency: USD
pricing:
  onDemand:
    hourly: 0.192
    monthly: 140.16
  reserved:
    oneYear:
      upfront: 1148
      hourly: 0.131
    threeYear:
      upfront: 2068
      hourly: 0.079
`
	filename := filepath.Join(tmpDir, "aws-rds-db.m5.large.yaml")
	err := os.WriteFile(filename, []byte(specContent), 0644)
	require.NoError(t, err)

	loader := spec.NewLoader(tmpDir)
	result, err := loader.LoadSpec("aws", "rds", "db.m5.large")

	require.NoError(t, err)
	pricingSpec, ok := result.(*spec.PricingSpec)
	require.True(t, ok)

	// Verify nested structure exists
	onDemand, ok := pricingSpec.Pricing["onDemand"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 0.192, onDemand["hourly"])
	assert.Equal(t, 140.16, onDemand["monthly"])
}
