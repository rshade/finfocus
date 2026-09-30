package engine_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	_ context.Context,
	_ string,
	_ ...string,
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
	t.Parallel()

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

	runConcurrentProjectedCosts(ctx, t, eng, resourceType, &wg, concurrency, iterations)
	runConcurrentActualCosts(ctx, t, eng, resourceType, &wg, concurrency, iterations)

	wg.Wait()
	duration := time.Since(start)
	t.Logf("Processed %d requests in %v (%.2f req/s)",
		concurrency*iterations*2, duration, float64(concurrency*iterations*2)/duration.Seconds())
}

// runConcurrentProjectedCosts spawns concurrency goroutines that each issue
// iterations projected cost requests against the engine. Assertions inside
// goroutines must not use require (FailNow is not goroutine-safe).
func runConcurrentProjectedCosts(
	ctx context.Context,
	t *testing.T,
	eng *engine.Engine,
	resourceType string,
	wg *sync.WaitGroup,
	concurrency, iterations int,
) {
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

				results, projectedErr := eng.GetProjectedCost(ctx, resources)
				if !assert.NoError(t, projectedErr, "GetProjectedCost error") {
					return
				}
				if !assert.Len(t, results, 1, "projected cost results") {
					return
				}
				assert.InDelta(t, 100.0, results[0].Monthly, 1e-9, "monthly cost")
			}
		}(i)
	}
}

// runConcurrentActualCosts spawns concurrency goroutines that each issue
// iterations actual cost requests against the engine. Assertions inside
// goroutines must not use require (FailNow is not goroutine-safe).
func runConcurrentActualCosts(
	ctx context.Context,
	t *testing.T,
	eng *engine.Engine,
	resourceType string,
	wg *sync.WaitGroup,
	concurrency, iterations int,
) {
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

				results, actualErr := eng.GetActualCostWithOptions(ctx, req)
				if !assert.NoError(t, actualErr, "GetActualCost error") {
					return
				}
				if !assert.Len(t, results, 1, "actual cost results") {
					return
				}
				assert.InDelta(t, 50.0, results[0].TotalCost, 1e-9, "total cost")
			}
		}()
	}
}

//nolint:paralleltest // sleeps on the real clock for 500ms or more and asserts on timing
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
