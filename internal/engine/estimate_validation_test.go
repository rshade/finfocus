package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestEstimateValidatesSendableProperties(t *testing.T) {
	t.Parallel()
	for _, secret := range []bool{true, false} {
		name := "plain oversized value rejected"
		if secret {
			name = "opaque oversized secret omitted"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var userData any = strings.Repeat("x", maxPropertyValLen+1)
			if secret {
				userData = map[string]any{
					"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
					"ciphertext":                       userData,
				}
			}
			resource := &ResourceDescriptor{
				Type: "aws:ec2/instance:Instance", Provider: "aws",
				Properties: map[string]any{"instanceType": "t3.micro", "region": "us-west-2", "userData": userData},
			}
			calls := 0
			plugin := &estimateMockPlugin{estimateCostFunc: func(
				_ context.Context, request *pbc.EstimateCostRequest, _ ...grpc.CallOption,
			) (*pbc.EstimateCostResponse, error) {
				calls++
				assert.NotContains(t, request.GetAttributes().GetFields(), "userData")
				return &pbc.EstimateCostResponse{Currency: "USD", CostMonthly: 8.39}, nil
			}}
			eng := New([]*pluginhost.Client{{Name: "aws-public", API: plugin}}, nil)
			result, err := eng.EstimateBaseline(context.Background(), resource, "")
			if !secret {
				require.ErrorIs(t, err, ErrResourceValidation)
				assert.Zero(t, calls)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.InDelta(t, 8.39, result.Baseline.Monthly, 0.001)
			assert.Positive(t, calls)
			assert.Equal(t, userData, resource.Properties["userData"], "validation must preserve original state")
		})
	}
}

func TestEstimateFallbackOmitsOversizedSecret(t *testing.T) {
	t.Parallel()
	secret := map[string]any{
		"4dabf18193072939515e22adb298388d": "1b47061264138c4ac30d75fd1eb44270",
		"ciphertext":                       strings.Repeat("x", maxPropertyValLen+1),
	}
	resource := &ResourceDescriptor{
		Type: "aws:ec2/instance:Instance", Provider: "aws",
		Properties: map[string]any{"instanceType": "t3.micro", "region": "us-west-2", "userData": secret},
	}
	plugin := &estimateMockPlugin{estimateCostFunc: func(
		_ context.Context, _ *pbc.EstimateCostRequest, _ ...grpc.CallOption,
	) (*pbc.EstimateCostResponse, error) {
		return nil, status.Error(codes.Unimplemented, "EstimateCost not implemented")
	}}
	eng := New([]*pluginhost.Client{{Name: "unimpl-plugin", API: plugin}}, &mockSpecLoader{})

	baseline, err := eng.EstimateBaseline(context.Background(), resource, "")
	require.NoError(t, err)
	assert.True(t, baseline.UsedFallback)

	result, err := eng.EstimateCost(context.Background(), &EstimateRequest{
		Resource:          resource,
		PropertyOverrides: map[string]string{"instanceType": "m5.large"},
	})
	require.NoError(t, err)
	assert.True(t, result.UsedFallback)
	assert.Equal(t, secret, resource.Properties["userData"], "fallback must preserve original state")
}
