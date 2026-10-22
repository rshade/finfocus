package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestResourceDescriptorValidateMetadataNamespaces(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		key     string
		wantErr bool
	}{
		{"terraform:address", false},
		{"terraform:unknownProperties", false},
		{"terraform:module.path-1", false},
		{"pulumi:urn", false},
		{"ordinary.property", false},
		{"terraform:", true},
		{"terraform:invalid key", true},
		{"terraform:address:part", true},
		{"pulumi:terraform:address", true},
		{"terraform:pulumi:urn", true},
		{"arbitrary:address", true},
		{"pulumi:", true},
	} {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()
			resource := ResourceDescriptor{
				Type:       "aws_instance",
				ID:         "aws_instance.web",
				Properties: map[string]any{tc.key: "value"},
			}
			err := resource.Validate()
			if tc.wantErr {
				require.ErrorIs(t, err, ErrResourceValidation)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestEstimateCostAcceptsTerraformMetadata(t *testing.T) {
	t.Parallel()
	resource := &ResourceDescriptor{
		Type:     "aws:ec2/instance:Instance",
		ID:       "aws_instance.web",
		Provider: "aws",
		Properties: map[string]any{
			"size":                        2,
			"terraform:address":           "aws_instance.web",
			"terraform:provider":          "registry.opentofu.org/hashicorp/aws",
			"terraform:unknownProperties": []string{"arn"},
		},
	}
	plugin := &estimateMockPlugin{
		estimateCostFunc: func(_ context.Context, req *pbc.EstimateCostRequest, _ ...grpc.CallOption) (*pbc.EstimateCostResponse, error) {
			return &pbc.EstimateCostResponse{
				Currency:    "USD",
				CostMonthly: req.GetAttributes().GetFields()["size"].GetNumberValue() * 2,
			}, nil
		},
	}
	eng := New([]*pluginhost.Client{{Name: "aws-public", API: plugin}}, nil)
	baseline, err := eng.EstimateBaseline(context.Background(), resource, "")
	require.NoError(t, err)
	require.NotNil(t, baseline)
	assert.InDelta(t, 4.0, baseline.Baseline.Monthly, 0.001)
	result, err := eng.EstimateCost(
		context.Background(),
		&EstimateRequest{Resource: resource, PropertyOverrides: map[string]string{"size": "4"}},
	)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.InDelta(t, 4.0, result.Baseline.Monthly, 0.001)
	assert.InDelta(t, 8.0, result.Modified.Monthly, 0.001)
}
