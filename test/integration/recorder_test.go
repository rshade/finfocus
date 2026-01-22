// Package integration_test provides integration tests for the recorder plugin.
package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
)

// TestRecorderPlugin_Integration verifies that the recorder plugin can be discovered and used.
func TestRecorderPlugin_Integration(t *testing.T) {
	// Locate the plugin binary
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "../..")
	binPath := filepath.Join(projectRoot, "bin", "finfocus-plugin-recorder")

	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	// Verify binary exists
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skipf("Plugin binary not found at %s. Run 'make build-recorder' first.", binPath)
	}

	// Create a temporary directory for recording
	tempDir, err := os.MkdirTemp("", "recorder-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Configure environment variables for the plugin process
	// Using t.Setenv ensures automatic cleanup and isolation
	t.Setenv("FINFOCUS_RECORDER_OUTPUT_DIR", tempDir)
	t.Setenv("FINFOCUS_RECORDER_MOCK_RESPONSE", "true")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Launch the plugin
	launcher := pluginhost.NewProcessLauncher()
	client, err := pluginhost.NewClient(ctx, launcher, binPath)
	require.NoError(t, err, "Failed to launch recorder plugin")
	defer client.Close()

	// T049: Verify discovery/name
	assert.Equal(t, "recorder", client.Name)

	// T050: Contract test - Validate RPC responses

	// 1. GetProjectedCost
	reqProjected := &proto.GetProjectedCostRequest{
		Resources: []*proto.ResourceDescriptor{
			{
				Type:     "aws:ec2/instance:Instance",
				Provider: "aws",
				Properties: map[string]string{
					"instanceType":     "t3.micro",
					"availabilityZone": "us-east-1a", // For region extraction
				},
			},
		},
	}

	respProjected, err := client.API.GetProjectedCost(ctx, reqProjected)
	require.NoError(t, err)
	require.NotEmpty(t, respProjected.Results)

	result := respProjected.Results[0]
	assert.Equal(t, "USD", result.Currency)
	// Since we enabled mock response, we expect non-zero cost
	assert.Greater(t, result.MonthlyCost, 0.0)

	// 2. GetActualCost
	now := time.Now()
	start := now.Add(-24 * time.Hour).Unix()
	end := now.Unix()

	reqActual := &proto.GetActualCostRequest{
		ResourceIDs: []string{"i-1234567890abcdef0"},
		StartTime:   start,
		EndTime:     end,
	}

	respActual, err := client.API.GetActualCost(ctx, reqActual)
	require.NoError(t, err)
	assert.NotEmpty(t, respActual.Results)

	// Verify that files were recorded
	// The recorder plugin writes files asynchronously; poll with timeout
	var files []os.DirEntry
	require.Eventually(t, func() bool {
		var err error
		files, err = os.ReadDir(tempDir)
		return err == nil && len(files) >= 2
	}, 2*time.Second, 50*time.Millisecond, "Expected recorded JSON files")

	// Check file content
	foundProjected := false
	foundActual := false

	for _, file := range files {
		contentBytes, err := os.ReadFile(filepath.Join(tempDir, file.Name()))
		require.NoError(t, err)
		content := string(contentBytes)

		if strings.Contains(content, "method") {
			if strings.Contains(content, "GetProjectedCost") {
				assert.Contains(
					t, content, "aws:ec2/instance:Instance",
					"GetProjectedCost file should contain resource type",
				)
				foundProjected = true
			} else if strings.Contains(content, "GetActualCost") {
				assert.Contains(
					t, content, "i-1234567890abcdef0",
					"GetActualCost file should contain resource ID",
				)
				foundActual = true
			}
		}
	}
	assert.True(t, foundProjected, "Should have recorded a GetProjectedCost request")
	assert.True(t, foundActual, "Should have recorded a GetActualCost request")
}
