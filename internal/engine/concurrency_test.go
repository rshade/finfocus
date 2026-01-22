package engine_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/rshade/finfocus/internal/engine"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// TCPLauncher adapts the mock server address to the Launcher interface.
type TCPLauncher struct {
	Address string
}

func (l *TCPLauncher) Start(
	ctx context.Context,
	path string,
	args ...string,
) (*grpc.ClientConn, func() error, error) {
	conn, err := grpc.NewClient(
		l.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, nil, err
	}
	// Wait for connection to be ready since we use it immediately
	conn.Connect()
	return conn, func() error { return conn.Close() }, nil
}

func TestEngineConcurrency(t *testing.T) {
	// Start a mock plugin server on a TCP port
	mockServer, err := plugin.StartMockServerTCP()
	require.NoError(t, err)
	defer mockServer.Stop()

	// Configure the mock plugin
	resourceType := "aws:ec2/instance:Instance"
	mockServer.Plugin.SetProjectedCostResponse(resourceType, &proto.CostResult{
		Currency:      "USD",
		MonthlyCost:   100.0,
		HourlyCost:    0.137,
		Notes:         "Test cost",
		CostBreakdown: map[string]float64{"compute": 100.0},
	})

	mockServer.Plugin.SetActualCostResponse("i-123", &proto.ActualCostResult{
		TotalCost: 50.0,
		Currency:  "USD",
		CostBreakdown: map[string]float64{
			"compute": 50.0,
		},
	})

	// Create a client connecting to the mock server
	ctx := context.Background()
	launcher := &TCPLauncher{Address: mockServer.Address()}

	client, err := pluginhost.NewClient(ctx, launcher, "mock-binary")
	require.NoError(t, err)
	defer client.Close()

	// Create engine
	eng := engine.New([]*pluginhost.Client{client}, nil)

	// Concurrency parameters
	concurrency := 50
	iterations := 100

	var wg sync.WaitGroup
	start := time.Now()

	// Run concurrent projected cost requests
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				resources := []engine.ResourceDescriptor{
					{
						Type:     resourceType,
						ID:       fmt.Sprintf("i-%d-%d", id, j),
						Provider: "aws",
					},
				}

				results, err := eng.GetProjectedCost(ctx, resources)

				// Assertions inside goroutine might panic test, better to collect errors
				// But for race detection, just running the code is the main goal
				if err != nil {
					t.Errorf("GetProjectedCost error: %v", err)
					return
				}
				if len(results) != 1 {
					t.Errorf("Expected 1 result, got %d", len(results))
					return
				}
				if results[0].Monthly != 100.0 {
					t.Errorf("Expected monthly 100.0, got %f", results[0].Monthly)
				}
			}
		}(i)
	}

	// Run concurrent actual cost requests
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				req := engine.ActualCostRequest{
					Resources: []engine.ResourceDescriptor{
						{
							Type:     resourceType,
							ID:       "i-123", // Mock configured for this ID
							Provider: "aws",
						},
					},
					From: time.Now().Add(-24 * time.Hour),
					To:   time.Now(),
				}

				results, err := eng.GetActualCostWithOptions(ctx, req)

				if err != nil {
					t.Errorf("GetActualCost error: %v", err)
					return
				}
				if len(results) != 1 {
					t.Errorf("Expected 1 result, got %d", len(results))
					return
				}
				if results[0].TotalCost != 50.0 {
					t.Errorf("Expected total cost 50.0, got %f", results[0].TotalCost)
				}
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)
	t.Logf("Processed %d requests in %v (%.2f req/s)",
		concurrency*iterations*2, duration, float64(concurrency*iterations*2)/duration.Seconds())
}

func TestEngineConcurrency_SharedState(t *testing.T) {
	// Test to verify no data races when multiple engines share clients or when clients share connections
	// In our case, Engine owns the clients, but let's simulate shared usage if possible or just heavy load on one engine

	// Start a mock plugin server
	mockServer, err := plugin.StartMockServerTCP()
	require.NoError(t, err)
	defer mockServer.Stop()

	mockServer.Plugin.SetProjectedCostResponse("aws:ec2:Instance", &proto.CostResult{
		MonthlyCost: 10.0,
		Currency:    "USD",
	})

	ctx := context.Background()
	launcher := &TCPLauncher{Address: mockServer.Address()}
	client, err := pluginhost.NewClient(ctx, launcher, "mock-binary")
	require.NoError(t, err)
	defer client.Close()

	eng := engine.New([]*pluginhost.Client{client}, nil)

	// Simulate concurrent read/write to plugin configuration while engine is querying
	// This tests the thread safety of the mock plugin itself as well as the engine's handling
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Reader goroutines (Engine queries)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					_, _ = eng.GetProjectedCost(
						ctx,
						[]engine.ResourceDescriptor{{Type: "aws:ec2:Instance", ID: "i-1"}},
					)
					time.Sleep(1 * time.Millisecond)
				}
			}
		}()
	}

	// Writer goroutines (Plugin config updates)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					// Toggle cost to verify no race conditions in mock
					mockServer.Plugin.SetProjectedCostResponse(
						"aws:ec2:Instance",
						&proto.CostResult{
							MonthlyCost: float64(time.Now().UnixNano() % 100),
							Currency:    "USD",
						},
					)
					time.Sleep(5 * time.Millisecond)
				}
			}
		}()
	}

	time.Sleep(2 * time.Second)
	close(done)
	wg.Wait()
}
