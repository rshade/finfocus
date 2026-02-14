package recorder

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func testLogger() zerolog.Logger {
	return zerolog.New(os.Stderr).With().Timestamp().Logger().Level(zerolog.Disabled)
}

func TestNewRecorderPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		OutputDir:    tmpDir,
		MockResponse: false,
	}

	plugin := NewRecorderPlugin(cfg, testLogger())

	require.NotNil(t, plugin)
	assert.Equal(t, "recorder", plugin.Name())
	assert.NotNil(t, plugin.recorder)
	assert.Nil(t, plugin.mocker) // Mock mode disabled
}

func TestNewRecorderPlugin_WithMockMode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		OutputDir:    tmpDir,
		MockResponse: true,
	}

	plugin := NewRecorderPlugin(cfg, testLogger())

	require.NotNil(t, plugin)
	assert.NotNil(t, plugin.mocker) // Mock mode enabled
}

func TestRecorderPlugin_Name(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{OutputDir: tmpDir}
	plugin := NewRecorderPlugin(cfg, testLogger())

	assert.Equal(t, "recorder", plugin.Name())
}

func TestRecorderPlugin_GetProjectedCost_MockDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		OutputDir:    tmpDir,
		MockResponse: false,
	}
	plugin := NewRecorderPlugin(cfg, testLogger())

	req := &pbc.GetProjectedCostRequest{
		Resource: &pbc.ResourceDescriptor{
			ResourceType: "aws:ec2:Instance",
			Provider:     "aws",
			Sku:          "t3.medium",
			Region:       "us-east-1",
		},
	}

	resp, err := plugin.GetProjectedCost(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, float64(0), resp.GetCostPerMonth())
	assert.Equal(t, "USD", resp.GetCurrency())
	assert.Contains(t, resp.GetBillingDetail(), "mock responses disabled")
}

func TestRecorderPlugin_GetProjectedCost_MockEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		OutputDir:    tmpDir,
		MockResponse: true,
	}
	plugin := NewRecorderPlugin(cfg, testLogger())

	req := &pbc.GetProjectedCostRequest{
		Resource: &pbc.ResourceDescriptor{
			ResourceType: "aws:ec2:Instance",
			Provider:     "aws",
			Sku:          "t3.medium",
			Region:       "us-east-1",
		},
	}

	resp, err := plugin.GetProjectedCost(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Greater(t, resp.GetCostPerMonth(), float64(0))
	assert.Equal(t, "USD", resp.GetCurrency())
	assert.Contains(t, resp.GetBillingDetail(), "Mock cost")
}

func TestRecorderPlugin_GetActualCost_MockDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		OutputDir:    tmpDir,
		MockResponse: false,
	}
	plugin := NewRecorderPlugin(cfg, testLogger())

	now := time.Now()
	req := &pbc.GetActualCostRequest{
		ResourceId: "arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890abcdef0",
		Start:      timestamppb.New(now.Add(-24 * time.Hour)),
		End:        timestamppb.New(now),
	}

	resp, err := plugin.GetActualCost(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Empty(t, resp.GetResults())
}

func TestRecorderPlugin_GetActualCost_MockEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		OutputDir:    tmpDir,
		MockResponse: true,
	}
	plugin := NewRecorderPlugin(cfg, testLogger())

	now := time.Now()
	req := &pbc.GetActualCostRequest{
		ResourceId: "arn:aws:ec2:us-east-1:123456789012:instance/i-1234567890abcdef0",
		Start:      timestamppb.New(now.Add(-24 * time.Hour)),
		End:        timestamppb.New(now),
	}

	resp, err := plugin.GetActualCost(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.GetResults(), 1)
	assert.Equal(t, "recorder-mock", resp.GetResults()[0].GetSource())
	assert.Greater(t, resp.GetResults()[0].GetCost(), float64(0))
}

func TestRecorderPlugin_GetPricingSpec(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{OutputDir: tmpDir}
	plugin := NewRecorderPlugin(cfg, testLogger())

	req := &pbc.GetPricingSpecRequest{
		Resource: &pbc.ResourceDescriptor{
			ResourceType: "aws:ec2:Instance",
			Provider:     "aws",
		},
	}

	resp, err := plugin.GetPricingSpec(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
}

func TestRecorderPlugin_EstimateCost_MockDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		OutputDir:    tmpDir,
		MockResponse: false,
	}
	plugin := NewRecorderPlugin(cfg, testLogger())

	req := &pbc.EstimateCostRequest{
		ResourceType: "aws:ec2:Instance",
	}

	resp, err := plugin.EstimateCost(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, float64(0), resp.GetCostMonthly())
	assert.Equal(t, "USD", resp.GetCurrency())
}

func TestRecorderPlugin_EstimateCost_MockEnabled(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		OutputDir:    tmpDir,
		MockResponse: true,
	}
	plugin := NewRecorderPlugin(cfg, testLogger())

	req := &pbc.EstimateCostRequest{
		ResourceType: "aws:ec2:Instance",
	}

	resp, err := plugin.EstimateCost(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Greater(t, resp.GetCostMonthly(), float64(0))
	assert.Equal(t, "USD", resp.GetCurrency())
}

func TestRecorderPlugin_Shutdown(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{OutputDir: tmpDir}
	plugin := NewRecorderPlugin(cfg, testLogger())

	// Should not panic
	plugin.Shutdown()
}

func TestRecorderPlugin_GetRecommendations(t *testing.T) {
	tests := []struct {
		name         string
		mockResponse bool
	}{
		{
			name:         "mock disabled",
			mockResponse: false,
		},
		{
			name:         "mock enabled",
			mockResponse: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cfg := &Config{
				OutputDir:    tmpDir,
				MockResponse: tt.mockResponse,
			}
			plugin := NewRecorderPlugin(cfg, testLogger())

			req := &pbc.GetRecommendationsRequest{}
			resp, err := plugin.GetRecommendations(context.Background(), req)

			require.NoError(t, err)
			require.NotNil(t, resp)

			if tt.mockResponse {
				// When mock is enabled, expect non-empty recommendations
				require.NotEmpty(t, resp.GetRecommendations())
				for _, rec := range resp.GetRecommendations() {
					assert.NotEmpty(t, rec.GetId())
					assert.NotEmpty(t, rec.GetDescription())
					assert.NotNil(t, rec.GetImpact())
					assert.Greater(t, rec.GetImpact().GetEstimatedSavings(), float64(0))
					assert.Equal(t, "USD", rec.GetImpact().GetCurrency())
				}
			} else {
				// When mock is disabled, expect empty recommendations
				assert.Empty(t, resp.GetRecommendations())
			}
		})
	}
}

func TestRecorderPlugin_GetPluginInfo(t *testing.T) {
	t.Run("successful metadata retrieval", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := &Config{OutputDir: tmpDir}
		plugin := NewRecorderPlugin(cfg, testLogger())

		req := &pbc.GetPluginInfoRequest{}

		resp, err := plugin.GetPluginInfo(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "recorder", resp.GetName())
		assert.Equal(t, "0.1.0", resp.GetVersion())
		assert.NotEmpty(t, resp.GetSpecVersion())
		assert.Len(t, resp.GetProviders(), 1)
		assert.Contains(t, resp.GetProviders(), "test")
		assert.Len(t, resp.GetCapabilities(), 2)
		assert.Contains(t, resp.GetCapabilities(), pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS)
		assert.Contains(t, resp.GetCapabilities(), pbc.PluginCapability_PLUGIN_CAPABILITY_RECOMMENDATIONS)
		assert.NotContains(t, resp.GetCapabilities(), pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS)
	})

	t.Run("recording failure does not affect metadata", func(t *testing.T) {
		// Use a read-only directory to trigger recording failure
		tmpDir := t.TempDir()
		readOnlyDir := filepath.Join(tmpDir, "readonly")
		err := os.MkdirAll(readOnlyDir, 0444) // Read-only permissions
		require.NoError(t, err)

		cfg := &Config{OutputDir: readOnlyDir}
		plugin := NewRecorderPlugin(cfg, testLogger())

		req := &pbc.GetPluginInfoRequest{}

		// GetPluginInfo should succeed even if recording fails
		resp, err := plugin.GetPluginInfo(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "recorder", resp.GetName())
		assert.Equal(t, "0.1.0", resp.GetVersion())
		assert.NotEmpty(t, resp.GetSpecVersion())
		assert.Len(t, resp.GetCapabilities(), 2)
		assert.Contains(t, resp.GetCapabilities(), pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS)
		assert.Contains(t, resp.GetCapabilities(), pbc.PluginCapability_PLUGIN_CAPABILITY_RECOMMENDATIONS)
		assert.NotContains(t, resp.GetCapabilities(), pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS)
	})
}

func TestRecorderPlugin_Supports(t *testing.T) {
	t.Run("returns false for any resource", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := &Config{OutputDir: tmpDir}
		plugin := NewRecorderPlugin(cfg, testLogger())

		req := &pbc.SupportsRequest{
			Resource: &pbc.ResourceDescriptor{
				ResourceType: "aws:ec2:Instance",
				Provider:     "aws",
				Sku:          "t3.medium",
				Region:       "us-east-1",
			},
		}

		resp, err := plugin.Supports(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.False(t, resp.GetSupported())
		assert.Contains(t, resp.GetReason(), "synthetic/demo only")
	})

	t.Run("nil request returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := &Config{OutputDir: tmpDir}
		plugin := NewRecorderPlugin(cfg, testLogger())

		resp, err := plugin.Supports(context.Background(), nil)

		require.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "request is required")
	})

	t.Run("records request to disk", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := &Config{OutputDir: tmpDir}
		plugin := NewRecorderPlugin(cfg, testLogger())

		req := &pbc.SupportsRequest{
			Resource: &pbc.ResourceDescriptor{
				ResourceType: "aws:s3:Bucket",
				Provider:     "aws",
			},
		}

		resp, err := plugin.Supports(context.Background(), req)

		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.False(t, resp.GetSupported())

		// Verify a file was recorded
		files, err := filepath.Glob(filepath.Join(tmpDir, "*.json"))
		require.NoError(t, err)
		assert.Len(t, files, 1)
	})
}

func TestRecorderPlugin_ThreadSafety(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		OutputDir:    tmpDir,
		MockResponse: true,
	}
	plugin := NewRecorderPlugin(cfg, testLogger())

	// Run multiple concurrent requests
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := &pbc.GetProjectedCostRequest{
				Resource: &pbc.ResourceDescriptor{
					ResourceType: "aws:ec2:Instance",
					Provider:     "aws",
				},
			}
			_, _ = plugin.GetProjectedCost(context.Background(), req)
		}()
	}

	wg.Wait()

	// No panic = thread-safe
}
