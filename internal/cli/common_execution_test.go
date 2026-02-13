package cli

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/engine"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

// mockRecommendationFetcher implements recommendationFetcher for testing.
type mockRecommendationFetcher struct {
	result *engine.RecommendationsResult
	err    error
}

func (m *mockRecommendationFetcher) GetRecommendationsForResources(
	_ context.Context,
	_ []engine.ResourceDescriptor,
) (*engine.RecommendationsResult, error) {
	return m.result, m.err
}

func TestFetchAndMergeRecommendations(t *testing.T) {
	tests := []struct {
		name        string
		resources   []engine.ResourceDescriptor
		results     []engine.CostResult
		fetcher     *mockRecommendationFetcher
		wantRecs    map[int][]string // result index -> expected rec types
		wantNilRecs []int            // result indices expected to have nil Recommendations
	}{
		{
			name: "successful merge by ResourceID",
			resources: []engine.ResourceDescriptor{
				{ID: "res-1", Type: "aws:ec2:Instance"},
				{ID: "res-2", Type: "aws:rds:Instance"},
			},
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
				{ResourceID: "res-2", Monthly: 50.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: []engine.Recommendation{
						{
							ResourceID:       "res-1",
							Type:             "RIGHTSIZE",
							Description:      "Switch to t3.small",
							EstimatedSavings: 5.0,
							Currency:         "USD",
						},
						{
							ResourceID:  "res-2",
							Type:        "TERMINATE",
							Description: "Idle resource",
						},
					},
				},
			},
			wantRecs: map[int][]string{
				0: {"RIGHTSIZE"},
				1: {"TERMINATE"},
			},
		},
		{
			name: "empty recommendations no-op",
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: nil,
				},
			},
			wantNilRecs: []int{0},
		},
		{
			name: "nil result from fetcher no-op",
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: nil,
			},
			wantNilRecs: []int{0},
		},
		{
			name: "fetch error logs warning and returns gracefully",
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
			},
			fetcher: &mockRecommendationFetcher{
				err: errors.New("plugin unavailable"),
			},
			wantNilRecs: []int{0},
		},
		{
			name: "recommendation with empty ResourceID is skipped",
			results: []engine.CostResult{
				{ResourceID: "", Monthly: 10.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: []engine.Recommendation{
						{
							ResourceID:  "",
							Type:        "RIGHTSIZE",
							Description: "Should be skipped",
						},
					},
				},
			},
			wantNilRecs: []int{0},
		},
		{
			name: "mixed empty and valid ResourceID recommendations only merge valid",
			resources: []engine.ResourceDescriptor{
				{ID: "res-1", Type: "aws:ec2:Instance"},
			},
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
				{ResourceID: "", Monthly: 1.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: []engine.Recommendation{
						{
							ResourceID:  "",
							Type:        "TERMINATE",
							Description: "Skipped",
						},
						{
							ResourceID:  "res-1",
							Type:        "RIGHTSIZE",
							Description: "Resize",
						},
					},
				},
			},
			wantRecs:    map[int][]string{0: {"RIGHTSIZE"}},
			wantNilRecs: []int{1},
		},
		{
			name: "multiple resources with partial recommendation coverage",
			resources: []engine.ResourceDescriptor{
				{ID: "res-1", Type: "aws:ec2:Instance"},
				{ID: "res-2", Type: "aws:rds:Instance"},
				{ID: "res-3", Type: "aws:s3:Bucket"},
			},
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 10.0},
				{ResourceID: "res-2", Monthly: 50.0},
				{ResourceID: "res-3", Monthly: 5.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: []engine.Recommendation{
						{
							ResourceID:  "res-1",
							Type:        "RIGHTSIZE",
							Description: "Resize",
						},
						{
							ResourceID:       "res-1",
							Type:             "MIGRATE",
							Description:      "Use Graviton",
							EstimatedSavings: 8.0,
							Currency:         "USD",
						},
					},
				},
			},
			wantRecs:    map[int][]string{0: {"RIGHTSIZE", "MIGRATE"}},
			wantNilRecs: []int{1, 2},
		},
		{
			name: "match by pulumi:cloudId when plugin returns cloud-native IDs",
			resources: []engine.ResourceDescriptor{
				{
					ID:   "urn:pulumi:dev::proj::aws:eks/cluster:Cluster::my-cluster",
					Type: "aws:eks/cluster:Cluster",
					Properties: map[string]interface{}{
						"pulumi:cloudId": "my-cluster-eksCluster-90b4099",
						"pulumi:urn":     "urn:pulumi:dev::proj::aws:eks/cluster:Cluster::my-cluster",
					},
				},
				{
					ID:   "urn:pulumi:dev::proj::aws:ec2/securityGroup:SecurityGroup::my-sg",
					Type: "aws:ec2/securityGroup:SecurityGroup",
					Properties: map[string]interface{}{
						"pulumi:cloudId": "sg-01c95cf41a7b7565c",
						"pulumi:urn":     "urn:pulumi:dev::proj::aws:ec2/securityGroup:SecurityGroup::my-sg",
					},
				},
			},
			results: []engine.CostResult{
				{ResourceID: "urn:pulumi:dev::proj::aws:eks/cluster:Cluster::my-cluster", Monthly: 100.0},
				{ResourceID: "urn:pulumi:dev::proj::aws:ec2/securityGroup:SecurityGroup::my-sg", Monthly: 0.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: []engine.Recommendation{
						{
							ResourceID:       "my-cluster-eksCluster-90b4099",
							Type:             "RIGHTSIZE",
							Description:      "Consider smaller cluster",
							EstimatedSavings: 20.0,
						},
						{
							ResourceID:  "sg-01c95cf41a7b7565c",
							Type:        "DELETE_UNUSED",
							Description: "Unused security group",
						},
					},
				},
			},
			wantRecs: map[int][]string{
				0: {"RIGHTSIZE"},
				1: {"DELETE_UNUSED"},
			},
		},
		{
			name: "match by pulumi:arn when plugin returns ARN-based IDs",
			resources: []engine.ResourceDescriptor{
				{
					ID:   "urn:pulumi:dev::proj::aws:ec2:Instance::web-server",
					Type: "aws:ec2:Instance",
					Properties: map[string]interface{}{
						"pulumi:cloudId": "i-0abc123",
						"pulumi:arn":     "arn:aws:ec2:us-west-2:123456789:instance/i-0abc123",
					},
				},
			},
			results: []engine.CostResult{
				{ResourceID: "urn:pulumi:dev::proj::aws:ec2:Instance::web-server", Monthly: 50.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: []engine.Recommendation{
						{
							ResourceID:  "arn:aws:ec2:us-west-2:123456789:instance/i-0abc123",
							Type:        "RIGHTSIZE",
							Description: "Switch to t3.small",
						},
					},
				},
			},
			wantRecs: map[int][]string{0: {"RIGHTSIZE"}},
		},
		{
			name: "primary URN match takes precedence over alt ID",
			resources: []engine.ResourceDescriptor{
				{
					ID:   "res-1",
					Type: "aws:ec2:Instance",
					Properties: map[string]interface{}{
						"pulumi:cloudId": "i-0abc123",
					},
				},
			},
			results: []engine.CostResult{
				{ResourceID: "res-1", Monthly: 50.0},
			},
			fetcher: &mockRecommendationFetcher{
				result: &engine.RecommendationsResult{
					Recommendations: []engine.Recommendation{
						{
							ResourceID:  "res-1",
							Type:        "RIGHTSIZE",
							Description: "From URN match",
						},
					},
				},
			},
			wantRecs: map[int][]string{0: {"RIGHTSIZE"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fetchAndMergeRecommendations(context.Background(), tt.fetcher, tt.resources, tt.results)

			for idx, wantTypes := range tt.wantRecs {
				require.Len(t, tt.results[idx].Recommendations, len(wantTypes),
					"result[%d] recommendation count mismatch", idx)
				for j, wantType := range wantTypes {
					assert.Equal(t, wantType, tt.results[idx].Recommendations[j].Type,
						"result[%d].Recommendations[%d].Type", idx, j)
				}
			}
			for _, idx := range tt.wantNilRecs {
				assert.Nil(t, tt.results[idx].Recommendations,
					"result[%d].Recommendations should be nil", idx)
			}
		})
	}
}

func TestDetectPulumiProject_NoBinary(t *testing.T) {
	// When the Pulumi binary is not on PATH, detectPulumiProject should fail
	// with a "find pulumi binary" error.
	t.Setenv("PATH", t.TempDir()) // empty PATH — no pulumi binary

	_, _, err := detectPulumiProject(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find pulumi binary")
}

func TestDetectPulumiProject_NoProject(t *testing.T) {
	// Skip if no pulumi binary on real PATH (CI without pulumi installed).
	if _, err := pulumidetect.FindBinary(); err != nil {
		t.Skip("pulumi binary not available")
	}

	// Run from a temp dir with no Pulumi.yaml so project detection fails.
	origDir, dirErr := os.Getwd()
	require.NoError(t, dirErr)
	tmpDir := t.TempDir()
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	_, _, err := detectPulumiProject(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "find pulumi project")
}

func TestExtractCurrencyFromResults(t *testing.T) {
	tests := []struct {
		name         string
		results      []engine.CostResult
		wantCurrency string
		wantMixed    bool
	}{
		{
			name:         "no results defaults to USD",
			results:      nil,
			wantCurrency: "USD",
		},
		{
			name: "single currency",
			results: []engine.CostResult{
				{Currency: "EUR"},
				{Currency: "EUR"},
			},
			wantCurrency: "EUR",
		},
		{
			name: "mixed currencies",
			results: []engine.CostResult{
				{Currency: "USD"},
				{Currency: "EUR"},
			},
			wantCurrency: "USD",
			wantMixed:    true,
		},
		{
			name: "empty currency defaults to USD",
			results: []engine.CostResult{
				{Currency: ""},
			},
			wantCurrency: "USD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currency, mixed := extractCurrencyFromResults(tt.results)
			assert.Equal(t, tt.wantCurrency, currency)
			assert.Equal(t, tt.wantMixed, mixed)
		})
	}
}

func TestBuildAltIDIndex(t *testing.T) {
	resources := []engine.ResourceDescriptor{
		{
			ID:   "urn:pulumi:dev::proj::aws:eks/cluster:Cluster::my-cluster",
			Type: "aws:eks/cluster:Cluster",
			Properties: map[string]interface{}{
				"pulumi:cloudId": "my-cluster-90b4099",
				"pulumi:arn":     "arn:aws:eks:us-west-2:123:cluster/my-cluster-90b4099",
				"id":             "my-cluster-90b4099",
			},
		},
		{
			ID:         "urn:pulumi:dev::proj::aws:iam/role:Role::role",
			Type:       "aws:iam/role:Role",
			Properties: map[string]interface{}{},
		},
		{
			ID:   "urn:pulumi:dev::proj::aws:ec2/instance:Instance::web",
			Type: "aws:ec2/instance:Instance",
			Properties: map[string]interface{}{
				"pulumi:cloudId": "i-0abc123",
			},
		},
	}
	results := []engine.CostResult{
		{ResourceID: "urn:pulumi:dev::proj::aws:eks/cluster:Cluster::my-cluster"},
		{ResourceID: "urn:pulumi:dev::proj::aws:iam/role:Role::role"},
		{ResourceID: "urn:pulumi:dev::proj::aws:ec2/instance:Instance::web"},
	}

	altMap := buildAltIDIndex(resources, results)

	// Cloud ID maps to correct result index
	assert.Equal(t, 0, altMap["my-cluster-90b4099"])
	assert.Equal(t, 0, altMap["arn:aws:eks:us-west-2:123:cluster/my-cluster-90b4099"])
	assert.Equal(t, 2, altMap["i-0abc123"])

	// Resource with no properties has no alt IDs
	_, hasRole := altMap["urn:pulumi:dev::proj::aws:iam/role:Role::role"]
	assert.False(t, hasRole, "URN should not appear as alt ID")
}
