package carbon

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func testResource(sku, region string) *pbc.ResourceDescriptor {
	return &pbc.ResourceDescriptor{Provider: "aws", ResourceType: "aws:ec2/instance:Instance", Sku: sku, Region: region}
}

func TestEstimateResource(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/estimates.json")
	require.NoError(t, err)
	var fixtures []struct {
		SKU         string  `json:"SKU"`
		Region      string  `json:"Region"`
		Utilization float64 `json:"Utilization"`
		Grams       float64 `json:"Grams"`
	}
	require.NoError(t, json.Unmarshal(data, &fixtures))
	for _, tt := range fixtures {
		t.Run(tt.SKU+"/"+tt.Region+"/"+strconv.FormatFloat(tt.Utilization, 'g', -1, 64), func(t *testing.T) {
			t.Parallel()
			got, estimateErr := EstimateResource(testResource(tt.SKU, tt.Region), Config{Utilization: tt.Utilization})
			require.NoError(t, estimateErr)
			assert.InDelta(t, tt.Grams, got.Grams, 1e-8)
			assert.Positive(t, got.KWh)
			assert.Contains(t, got.Note, "operational")
			assert.Contains(t, got.Note, DatasetRevision)
		})
	}
}

func TestEstimateDeclines(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name        string
		resource    *pbc.ResourceDescriptor
		errContains string
	}{
		{"nil", nil, "resource"}, {"missing_sku", testResource("", "us-east-1"), "instance"},
		{"unknown_sku", testResource("t3.99xlarge", "us-east-1"), "instance"},
		{"missing_region", testResource("t3.medium", ""), "region"},
		{"unknown_region", testResource("t3.medium", "eu-central-3"), "region"},
		{"wrong_provider", &pbc.ResourceDescriptor{Provider: "gcp"}, "AWS EC2"},
		{"wrong_type", &pbc.ResourceDescriptor{Provider: "aws", ResourceType: "aws:s3/bucket:Bucket"}, "AWS EC2"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := EstimateResource(tt.resource, Config{Utilization: 0.5})
			require.ErrorContains(t, err, tt.errContains)
		})
	}
}

func TestDataCoverage(t *testing.T) {
	t.Parallel()
	d, err := loadData()
	require.NoError(t, err)
	assert.Len(t, d.Instances, 34)
	assert.Len(t, d.Regions, 38)
	assert.Equal(t, DatasetRevision, d.Revision)
	assert.InDelta(t, 0.5, d.Instances["t3.nano"].MemoryGiB, 1e-12)
	for sku := range d.Instances {
		got, estimateErr := EstimateResource(testResource(sku, "us-east-1"), Config{Utilization: 0.5})
		require.NoError(t, estimateErr, sku)
		assert.Positive(t, got.Grams, sku)
	}
	for region := range d.Regions {
		got, estimateErr := EstimateResource(testResource("t3.medium", region), Config{Utilization: 0.5})
		require.NoError(t, estimateErr, region)
		assert.Positive(t, got.Grams, region)
	}
}
