package pluginhost_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestNewClient_LauncherError(t *testing.T) {
	// Test what happens when launcher fails
	mockLauncher := &mockLauncher{
		startError: errors.New("launcher failed"),
	}

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, mockLauncher, "/path/to/plugin")

	if err == nil {
		t.Error("expected error when launcher fails")
	}

	if client != nil {
		t.Error("expected nil client when launcher fails")
	}

	if err.Error() != "launcher failed" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNewClient_InvalidBinaryPath(t *testing.T) {
	// Test with invalid binary path using real launcher
	launcher := pluginhost.NewProcessLauncher()

	ctx := context.Background()
	client, err := pluginhost.NewClient(ctx, launcher, "/nonexistent/plugin/binary")

	if err == nil {
		t.Error("expected error for nonexistent binary")
	}

	if client != nil {
		t.Error("expected nil client for nonexistent binary")
	}
}

func TestClient_StructureValidation(t *testing.T) {
	// Test that Client struct has the expected fields
	client := &pluginhost.Client{
		Name:  "test",
		Close: func() error { return nil },
	}

	if client.Name != "test" {
		t.Error("Name field not working")
	}

	if client.Close == nil {
		t.Error("Close function should not be nil")
	}

	// Test calling Close function
	if err := client.Close(); err != nil {
		t.Errorf("Close function failed: %v", err)
	}
}

func TestClient_CloseWithError(t *testing.T) {
	// Test Close function that returns an error
	expectedError := errors.New("cleanup failed")
	client := &pluginhost.Client{
		Close: func() error {
			return expectedError
		},
	}

	err := client.Close()
	if err == nil {
		t.Error("expected error from Close function")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("expected specific error, got %v", err)
	}
}

func TestLauncherInterface(_ *testing.T) {
	// Test that our launcher types implement the Launcher interface
	var launcher pluginhost.Launcher

	// ProcessLauncher should implement Launcher
	launcher = pluginhost.NewProcessLauncher()
	_ = launcher // Verify it compiles

	// StdioLauncher should implement Launcher
	launcher = pluginhost.NewStdioLauncher()
	_ = launcher // Verify it compiles
}

func TestNewClient_WithProcessLauncher(t *testing.T) {
	// Test that NewClient works with real ProcessLauncher (will fail but shouldn't panic)
	launcher := pluginhost.NewProcessLauncher()

	ctx, cancel := context.WithTimeout(context.Background(), 100) // Very short timeout
	defer cancel()

	// This should fail gracefully
	client, err := pluginhost.NewClient(
		ctx,
		launcher,
		"/bin/false",
	) // Command that exists but fails

	if err == nil {
		t.Log("Note: /bin/false might not exist on this system")
		if client != nil {
			client.Close()
		}
	} else {
		// Error is expected - just verify it doesn't panic
		t.Logf("Expected error with /bin/false: %v", err)
	}
}

func TestNewClient_WithStdioLauncher(t *testing.T) {
	// Test that NewClient works with real StdioLauncher (will fail but shouldn't panic)
	launcher := pluginhost.NewStdioLauncher()

	ctx, cancel := context.WithTimeout(context.Background(), 100) // Very short timeout
	defer cancel()

	// This should fail gracefully
	client, err := pluginhost.NewClient(
		ctx,
		launcher,
		"/bin/false",
	) // Command that exists but fails

	if err == nil {
		t.Log("Note: /bin/false might not exist on this system")
		if client != nil {
			client.Close()
		}
	} else {
		// Error is expected - just verify it doesn't panic
		t.Logf("Expected error with /bin/false: %v", err)
	}
}

func TestNewClient_ContextCancellation(t *testing.T) {
	// Test behavior with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	launcher := pluginhost.NewProcessLauncher()
	client, err := pluginhost.NewClient(ctx, launcher, "/bin/echo")

	// Should handle cancelled context gracefully
	if client != nil {
		client.Close()
	}

	// Error or success both acceptable - just shouldn't panic
	t.Logf("Context cancellation result: err=%v", err)
}

// Mock launcher for error testing

type mockLauncher struct {
	startError error
}

func (m *mockLauncher) Start(
	_ context.Context,
	_ string,
	_ ...string,
) (*grpc.ClientConn, func() error, error) {
	if m.startError != nil {
		return nil, nil, m.startError
	}

	// This is a simplified mock - in reality we can't return a nil connection
	// but for error testing this is sufficient
	return nil, func() error { return nil }, errors.New("mock launcher always fails after start")
}

func TestConvertCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		input    []pbc.PluginCapability
		expected []string
	}{
		{
			name: "AllCapabilities",
			input: []pbc.PluginCapability{
				pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS,
				pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS,
				pbc.PluginCapability_PLUGIN_CAPABILITY_RECOMMENDATIONS,
				pbc.PluginCapability_PLUGIN_CAPABILITY_DRY_RUN,
				pbc.PluginCapability_PLUGIN_CAPABILITY_BUDGETS,
				pbc.PluginCapability_PLUGIN_CAPABILITY_CARBON,
				pbc.PluginCapability_PLUGIN_CAPABILITY_ENERGY,
				pbc.PluginCapability_PLUGIN_CAPABILITY_WATER,
				pbc.PluginCapability_PLUGIN_CAPABILITY_PRICING_SPEC,
				pbc.PluginCapability_PLUGIN_CAPABILITY_ESTIMATE_COST,
				pbc.PluginCapability_PLUGIN_CAPABILITY_DISMISS_RECOMMENDATIONS,
			},
			expected: []string{
				"projected_costs",
				"actual_costs",
				"recommendations",
				"dry_run",
				"budgets",
				"carbon",
				"energy",
				"water",
				"pricing_spec",
				"estimate_cost",
				"dismiss_recommendations",
			},
		},
		{
			name:     "Empty",
			input:    []pbc.PluginCapability{},
			expected: nil,
		},
		{
			name:     "Nil",
			input:    nil,
			expected: nil,
		},
		{
			name: "SkipsUnspecified",
			input: []pbc.PluginCapability{
				pbc.PluginCapability_PLUGIN_CAPABILITY_UNSPECIFIED,
				pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS,
				pbc.PluginCapability_PLUGIN_CAPABILITY_UNSPECIFIED,
				pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS,
			},
			expected: []string{
				"projected_costs",
				"actual_costs",
			},
		},
		{
			name: "SubsetOfCapabilities",
			input: []pbc.PluginCapability{
				pbc.PluginCapability_PLUGIN_CAPABILITY_PROJECTED_COSTS,
				pbc.PluginCapability_PLUGIN_CAPABILITY_ACTUAL_COSTS,
			},
			expected: []string{
				"projected_costs",
				"actual_costs",
			},
		},
		{
			name: "OnlyUnspecified",
			input: []pbc.PluginCapability{
				pbc.PluginCapability_PLUGIN_CAPABILITY_UNSPECIFIED,
				pbc.PluginCapability_PLUGIN_CAPABILITY_UNSPECIFIED,
			},
			expected: []string{}, // Returns empty slice, not nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := pluginhost.ConvertCapabilities(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
