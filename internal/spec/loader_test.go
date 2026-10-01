package spec

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewLoader tests the creation of a new spec loader.
func TestNewLoader(t *testing.T) {
	t.Parallel()

	baseDir := "/test/path"
	loader := NewLoader(baseDir)

	require.NotNil(t, loader, "NewLoader should not return nil")
	assert.Equal(t, baseDir, loader.specDir, "specDir should match input")
}

// TestLoadSpec_Success tests successful spec loading scenarios.
func TestLoadSpec_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		provider     string
		service      string
		sku          string
		specContent  string
		wantProvider string
		wantService  string
		wantSKU      string
	}{
		{
			name:     "valid simple spec",
			provider: "aws",
			service:  "ec2",
			sku:      "t3.micro",
			specContent: `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  onDemandHourly: 0.0104
  monthlyEstimate: 7.59
`,
			wantProvider: "aws",
			wantService:  "ec2",
			wantSKU:      "t3.micro",
		},
		{
			name:     "spec with dots in SKU",
			provider: "aws",
			service:  "rds",
			sku:      "db.t3.medium",
			specContent: `provider: aws
service: rds
sku: db.t3.medium
currency: USD
pricing:
  onDemandHourly: 0.068
  monthlyEstimate: 49.64
`,
			wantProvider: "aws",
			wantService:  "rds",
			wantSKU:      "db.t3.medium",
		},
		{
			name:     "spec with dashes in SKU",
			provider: "azure",
			service:  "compute",
			sku:      "standard-d2s-v3",
			specContent: `provider: azure
service: compute
sku: standard-d2s-v3
currency: USD
pricing:
  onDemandHourly: 0.096
`,
			wantProvider: "azure",
			wantService:  "compute",
			wantSKU:      "standard-d2s-v3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()

			// Create spec file
			filename := filepath.Join(tmpDir, tt.provider+"-"+tt.service+"-"+tt.sku+".yaml")
			err := os.WriteFile(filename, []byte(tt.specContent), 0644)
			require.NoError(t, err)

			loader := NewLoader(tmpDir)
			spec, err := loader.LoadSpec(tt.provider, tt.service, tt.sku)

			require.NoError(t, err)
			require.NotNil(t, spec)

			// Verify the spec structure
			pricingSpec, ok := spec.(*PricingSpec)
			require.True(t, ok, "spec should be *PricingSpec")
			assert.Equal(t, tt.wantProvider, pricingSpec.Provider)
			assert.Equal(t, tt.wantService, pricingSpec.Service)
			assert.Equal(t, tt.wantSKU, pricingSpec.SKU)
			assert.NotEmpty(t, pricingSpec.Pricing, "pricing should not be empty")
		})
	}
}

// TestLoadSpec_Errors tests error handling in LoadSpec.
func TestLoadSpec_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		provider    string
		service     string
		sku         string
		setupFunc   func(t *testing.T, dir string)
		wantErrType error
		wantErrMsg  string
	}{
		{
			name:        "missing file returns ErrSpecNotFound",
			provider:    "aws",
			service:     "ec2",
			sku:         "nonexistent",
			setupFunc:   func(_ *testing.T, _ string) {},
			wantErrType: ErrSpecNotFound,
		},
		{
			name:     "invalid YAML returns parse error",
			provider: "invalid",
			service:  "yaml",
			sku:      "test",
			setupFunc: func(t *testing.T, dir string) {
				filename := filepath.Join(dir, "invalid-yaml-test.yaml")
				invalidYAML := `this is not: valid
  yaml: [unclosed
`
				err := os.WriteFile(filename, []byte(invalidYAML), 0644)
				require.NoError(t, err)
			},
			wantErrMsg: "parsing spec YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			tt.setupFunc(t, tmpDir)

			loader := NewLoader(tmpDir)
			spec, err := loader.LoadSpec(tt.provider, tt.service, tt.sku)

			require.Error(t, err)
			assert.Nil(t, spec, "spec should be nil on error")

			if tt.wantErrType != nil {
				require.ErrorIs(t, err, tt.wantErrType)
			}
			if tt.wantErrMsg != "" {
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			}
		})
	}
}

// TestLoadSpec_NonexistentDirectory tests loading from a non-existent directory.
func TestLoadSpec_NonexistentDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	nonexistentDir := filepath.Join(tmpDir, "nonexistent")

	loader := NewLoader(nonexistentDir)
	spec, err := loader.LoadSpec("aws", "ec2", "t3.micro")

	require.Error(t, err)
	assert.Nil(t, spec)
	assert.ErrorIs(t, err, ErrSpecNotFound)
}

// TestLoadSpec_PermissionError tests loading a spec file with restricted permissions.
func TestLoadSpec_PermissionError(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows - POSIX permissions not enforced")
	}
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()

	// Create a spec file with no read permissions
	filename := filepath.Join(tmpDir, "aws-ec2-restricted.yaml")
	err := os.WriteFile(filename, []byte("test: data"), 0000)
	require.NoError(t, err)
	defer os.Chmod(filename, 0644) // Restore for cleanup

	loader := NewLoader(tmpDir)
	spec, err := loader.LoadSpec("aws", "ec2", "restricted")

	require.Error(t, err)
	assert.Nil(t, spec)
	assert.Contains(t, err.Error(), "reading spec file")
}

// TestParseSpecFilename_Valid tests parsing valid spec filenames.
func TestParseSpecFilename_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
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

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			t.Parallel()
			provider, service, sku, valid := ParseSpecFilename(tt.filename)

			assert.True(t, valid, "filename should be valid")
			assert.Equal(t, tt.provider, provider)
			assert.Equal(t, tt.service, service)
			assert.Equal(t, tt.sku, sku)
		})
	}
}

// TestParseSpecFilename_Invalid tests parsing invalid spec filenames.
func TestParseSpecFilename_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		filename string
		reason   string
	}{
		{
			filename: "aws-ec2.yaml",
			reason:   "insufficient parts",
		},
		{
			filename: "aws.yaml",
			reason:   "only one part",
		},
		{
			filename: ".yaml",
			reason:   "empty name",
		},
		{
			filename: "nohyphens.yaml",
			reason:   "no hyphens",
		},
		{
			filename: "aws-ec2-t3.micro.json",
			reason:   "wrong extension",
		},
		{
			filename: "aws--t3.micro.yaml",
			reason:   "empty service part",
		},
	}

	for _, tt := range tests {
		t.Run(tt.reason, func(t *testing.T) {
			t.Parallel()
			provider, service, sku, valid := ParseSpecFilename(tt.filename)

			assert.False(t, valid, "filename should be invalid: %s", tt.reason)
			assert.Empty(t, provider)
			assert.Empty(t, service)
			assert.Empty(t, sku)
		})
	}
}

// TestListSpecs tests listing specs in a directory.
func TestListSpecs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create test files
	testFiles := []struct {
		name       string
		shouldList bool
	}{
		{"aws-ec2-t3.micro.yaml", true},
		{"aws-rds-db.t3.medium.yaml", true},
		{"azure-compute-standard-d2s.yaml", true},
		{"notaspec.txt", false},
		{"readme.md", false},
		{"subdir", false}, // Directory
	}

	for _, tf := range testFiles {
		path := filepath.Join(tmpDir, tf.name)
		if tf.name == "subdir" {
			err := os.Mkdir(path, 0755)
			require.NoError(t, err)
		} else {
			err := os.WriteFile(path, []byte("test: data\n"), 0644)
			require.NoError(t, err)
		}
	}

	loader := NewLoader(tmpDir)
	specs, err := loader.ListSpecs()

	require.NoError(t, err)

	// Count expected specs
	expectedCount := 0
	for _, tf := range testFiles {
		if tf.shouldList {
			expectedCount++
		}
	}

	assert.Len(t, specs, expectedCount, "should list only .yaml files")

	// Verify only .yaml files are included
	for _, spec := range specs {
		assert.Equal(t, ".yaml", filepath.Ext(spec), "all specs should have .yaml extension")
	}
}

// TestListSpecs_EmptyDirectory tests listing specs in an empty directory.
func TestListSpecs_EmptyDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	loader := NewLoader(tmpDir)
	specs, err := loader.ListSpecs()

	require.NoError(t, err)
	assert.Empty(t, specs, "empty directory should return empty list")
}

// TestListSpecs_NonexistentDirectory tests listing specs in a non-existent directory.
func TestListSpecs_NonexistentDirectory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	nonexistentDir := filepath.Join(tmpDir, "nonexistent")

	loader := NewLoader(nonexistentDir)
	specs, err := loader.ListSpecs()

	require.NoError(t, err, "should not error for non-existent directory")
	assert.Nil(t, specs, "should return nil for non-existent directory")
}

// TestListSpecs_PermissionError tests listing specs with permission error.
func TestListSpecs_PermissionError(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows - POSIX permissions not enforced")
	}
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	tmpDir := t.TempDir()

	// Remove read permission
	err := os.Chmod(tmpDir, 0000)
	require.NoError(t, err)
	defer os.Chmod(tmpDir, 0755) // Restore for cleanup

	loader := NewLoader(tmpDir)
	specs, err := loader.ListSpecs()

	require.Error(t, err, "should error for permission denied")
	assert.Nil(t, specs)
	assert.Contains(t, err.Error(), "reading spec directory")
}

// TestValidateSpec tests spec validation.
func TestValidateSpec(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		spec    *PricingSpec
		wantErr string
	}{
		{
			name: "valid spec",
			spec: &PricingSpec{
				Provider: "aws",
				Service:  "ec2",
				SKU:      "t3.micro",
				Currency: "USD",
				Pricing: map[string]interface{}{
					"hourly_cost": 0.0104,
				},
			},
			wantErr: "",
		},
		{
			name: "missing provider",
			spec: &PricingSpec{
				Service:  "ec2",
				SKU:      "t3.micro",
				Currency: "USD",
				Pricing: map[string]interface{}{
					"hourly_cost": 0.0104,
				},
			},
			wantErr: "provider is required",
		},
		{
			name: "missing service",
			spec: &PricingSpec{
				Provider: "aws",
				SKU:      "t3.micro",
				Currency: "USD",
				Pricing: map[string]interface{}{
					"hourly_cost": 0.0104,
				},
			},
			wantErr: "service is required",
		},
		{
			name: "missing SKU",
			spec: &PricingSpec{
				Provider: "aws",
				Service:  "ec2",
				Currency: "USD",
				Pricing: map[string]interface{}{
					"hourly_cost": 0.0104,
				},
			},
			wantErr: "SKU is required",
		},
		{
			name: "missing currency",
			spec: &PricingSpec{
				Provider: "aws",
				Service:  "ec2",
				SKU:      "t3.micro",
				Pricing: map[string]interface{}{
					"hourly_cost": 0.0104,
				},
			},
			wantErr: "currency is required",
		},
		{
			name: "empty pricing",
			spec: &PricingSpec{
				Provider: "aws",
				Service:  "ec2",
				SKU:      "t3.micro",
				Currency: "USD",
				Pricing:  map[string]interface{}{},
			},
			wantErr: "pricing information is required",
		},
		{
			name: "nil pricing",
			spec: &PricingSpec{
				Provider: "aws",
				Service:  "ec2",
				SKU:      "t3.micro",
				Currency: "USD",
				Pricing:  nil,
			},
			wantErr: "pricing information is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateSpec(tt.spec)

			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

// TestLoadSpec_ContentValidation tests that loaded specs have correct content.
func TestLoadSpec_ContentValidation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	specContent := `provider: aws
service: ec2
sku: t3.micro
currency: USD
pricing:
  onDemandHourly: 0.0104
  monthlyEstimate: 7.59
metadata:
  region: us-east-1
  description: "Test spec"
`

	filename := filepath.Join(tmpDir, "aws-ec2-t3.micro.yaml")
	err := os.WriteFile(filename, []byte(specContent), 0644)
	require.NoError(t, err)

	loader := NewLoader(tmpDir)
	spec, err := loader.LoadSpec("aws", "ec2", "t3.micro")

	require.NoError(t, err)
	require.NotNil(t, spec)

	pricingSpec, ok := spec.(*PricingSpec)
	require.True(t, ok, "spec should be *PricingSpec")

	// Verify all fields
	assert.Equal(t, "aws", pricingSpec.Provider)
	assert.Equal(t, "ec2", pricingSpec.Service)
	assert.Equal(t, "t3.micro", pricingSpec.SKU)
	assert.Equal(t, "USD", pricingSpec.Currency)

	// Verify pricing structure
	require.NotNil(t, pricingSpec.Pricing)
	assert.Contains(t, pricingSpec.Pricing, "onDemandHourly")
	assert.Contains(t, pricingSpec.Pricing, "monthlyEstimate")

	// Verify metadata
	require.NotNil(t, pricingSpec.Metadata)
	assert.Contains(t, pricingSpec.Metadata, "region")
	assert.Contains(t, pricingSpec.Metadata, "description")
}
