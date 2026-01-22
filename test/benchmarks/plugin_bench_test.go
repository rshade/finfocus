package benchmarks_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/test/mocks/plugin"
)

// BenchmarkPlugin_GRPC_Call benchmarks the overhead of a single gRPC call.
func BenchmarkPlugin_GRPC_Call(b *testing.B) {
	b.ReportAllocs()
	// Start mock plugin server
	server, err := plugin.StartMockServerTCP()
	require.NoError(b, err)
	defer server.Stop()

	conn, err := grpc.NewClient(server.Address(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(b, err)
	defer conn.Close()

	client := pb.NewCostSourceServiceClient(conn)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := client.Name(ctx, &pb.NameRequest{})
		if err != nil {
			b.Fatal(err)
		}
	}
}
