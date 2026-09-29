package plugin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
)

func TestMockServer_Dial(t *testing.T) {
	tests := []struct {
		name      string
		startFunc func() (*MockServer, error)
	}{
		{
			name:      "bufconn",
			startFunc: StartMockServer,
		},
		{
			name:      "tcp",
			startFunc: StartMockServerTCP,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := tt.startFunc()
			require.NoError(t, err)
			t.Cleanup(server.Stop)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			t.Cleanup(cancel)

			conn, err := server.Dial(ctx)
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = conn.Close()
			})

			client := pbc.NewCostSourceServiceClient(conn)
			resp, err := client.Name(ctx, &pbc.NameRequest{})
			require.NoError(t, err)
			assert.Equal(t, "mock-plugin", resp.GetName())
		})
	}
}

// TestMockServer_DialTCPBlocks verifies TCP dialing preserves blocking
// connection behavior: dialing an unreachable address fails only when the
// context expires, not immediately.
func TestMockServer_DialTCPBlocks(t *testing.T) {
	server := &MockServer{address: "127.0.0.1:1"}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	t.Cleanup(cancel)

	start := time.Now()
	conn, err := server.Dial(ctx)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Nil(t, conn)
	assert.GreaterOrEqual(t, elapsed, 400*time.Millisecond)
}

// TestTestHelper_Dial verifies the TestHelper wiring still produces a working
// bufconn connection.
func TestTestHelper_Dial(t *testing.T) {
	helper := NewTestHelper(t)

	conn := helper.Dial()
	require.NotNil(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	client := pbc.NewCostSourceServiceClient(conn)
	resp, err := client.Name(ctx, &pbc.NameRequest{})
	require.NoError(t, err)
	assert.Equal(t, "mock-plugin", resp.GetName())
}
