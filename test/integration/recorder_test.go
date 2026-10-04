// Package integration_test provides integration tests for the recorder plugin.
package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/engine"
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
	tempDir := t.TempDir()

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
		var readErr error
		files, readErr = os.ReadDir(tempDir)
		return readErr == nil && len(files) >= 2
	}, 2*time.Second, 50*time.Millisecond, "Expected recorded JSON files")

	// Check file content
	foundProjected := false
	foundActual := false

	for _, file := range files {
		contentBytes, readErr := os.ReadFile(filepath.Join(tempDir, file.Name()))
		require.NoError(t, readErr)
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

// TestRecorderPlugin_RecordsRedactedAttributes checks that core puts a
// resource's nested inputs on the wire as attributes, without credentials or
// Pulumi secrets. The recorder declines Supports and has no batch capability,
// so through the engine it receives only Supports; a direct GetProjectedCost
// covers the projected descriptor.
func TestRecorderPlugin_RecordsRedactedAttributes(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	binPath := filepath.Join(filepath.Dir(filename), "../..", "bin", "finfocus-plugin-recorder")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skipf("Plugin binary not found at %s. Run 'make build-recorder' first.", binPath)
	}

	tempDir := t.TempDir()
	t.Setenv("FINFOCUS_RECORDER_OUTPUT_DIR", tempDir)
	t.Setenv("FINFOCUS_RECORDER_MOCK_RESPONSE", "true")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := pluginhost.NewClient(ctx, pluginhost.NewProcessLauncher(), binPath)
	require.NoError(t, err)
	defer client.Close()

	props := map[string]any{
		"spec": map[string]any{"template": map[string]any{"spec": map[string]any{
			"containers": []any{map[string]any{"resources": map[string]any{
				"requests": map[string]any{"cpu": "500m"},
			}}},
		}}},
		"password": "hunter2-plaintext",
		"userData": map[string]any{
			"4dabf18193072939515e22adb298388d": "1",
			"ciphertext":                       "SECRET-CIPHERTEXT",
		},
	}
	resource := engine.ResourceDescriptor{
		ID: "urn:deploy", Type: "kubernetes:apps/v1:Deployment", Provider: "kubernetes", Properties: props,
	}

	_, err = engine.New([]*pluginhost.Client{client}, nil).GetProjectedCost(ctx, []engine.ResourceDescriptor{resource})
	require.NoError(t, err)
	_, err = client.API.GetProjectedCost(ctx, &proto.GetProjectedCostRequest{
		Resources: []*proto.ResourceDescriptor{{
			// The recorder requires a SKU and region, which core resolves for AWS types only.
			ID: "urn:web", Type: "aws:ec2/instance:Instance", Provider: "aws",
			Properties: map[string]string{"instanceType": "t3.micro", "region": "us-east-1"},
			Attributes: engine.BuildAttributes(ctx, props),
		}},
	})
	require.NoError(t, err)

	recorded := map[string]*pbc.ResourceDescriptor{}
	require.Eventually(t, func() bool {
		recorded = recordedDescriptors(t, tempDir)
		return recorded["Supports"] != nil && recorded["GetProjectedCost"] != nil
	}, 2*time.Second, 50*time.Millisecond, "expected recorded Supports and GetProjectedCost requests")

	for method, descriptor := range recorded {
		attrs := descriptor.GetAttributes()
		value, ok := pluginsdk.AttributeValue(attrs, "spec.template.spec.containers.0.resources.requests.cpu")
		require.True(t, ok, "%s attributes lack the nested request", method)
		assert.Equal(t, "500m", value.GetStringValue())
		assert.NotContains(t, attrs.GetFields(), "password", method)
		assert.NotContains(t, attrs.GetFields(), "userData", method)
		encoded, marshalErr := protojson.Marshal(attrs)
		require.NoError(t, marshalErr)
		assert.NotContains(t, string(encoded), "hunter2-plaintext", method)
		assert.NotContains(t, string(encoded), "SECRET-CIPHERTEXT", method)
	}
}

// recordedDescriptors reads the recorder's output and returns the resource of
// the last request recorded for each method that carries one.
func recordedDescriptors(t *testing.T, dir string) map[string]*pbc.ResourceDescriptor {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	found := map[string]*pbc.ResourceDescriptor{}
	for _, entry := range entries {
		raw, readErr := os.ReadFile(filepath.Join(dir, entry.Name()))
		require.NoError(t, readErr)
		var file struct {
			Method  string          `json:"method"`
			Request json.RawMessage `json:"request"`
		}
		require.NoError(t, json.Unmarshal(raw, &file))
		var req interface {
			protoreflect.ProtoMessage
			GetResource() *pbc.ResourceDescriptor
		}
		switch file.Method {
		case "Supports":
			req = &pbc.SupportsRequest{}
		case "GetProjectedCost":
			req = &pbc.GetProjectedCostRequest{}
		default:
			continue
		}
		require.NoError(t, protojson.Unmarshal(file.Request, req))
		found[file.Method] = req.GetResource()
	}
	return found
}
