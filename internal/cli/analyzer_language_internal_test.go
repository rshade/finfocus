package cli

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// RunAnalyzerLanguageHost installs process-wide signal handlers.
//
//nolint:paralleltest // signal.NotifyContext is process-wide
func TestRunAnalyzerLanguageHost_BindAndServeFailures(t *testing.T) {
	t.Run("listen error", func(t *testing.T) {
		withLanguageHostListener(t, func() (net.Listener, error) {
			return nil, errors.New("bind failed")
		})
		err := RunAnalyzerLanguageHost(&cobra.Command{})
		require.ErrorContains(t, err, "binding language host")
	})

	t.Run("not a tcp listener", func(t *testing.T) {
		withLanguageHostListener(t, func() (net.Listener, error) {
			return &scriptedListener{
				addr:     stringAddr{network: "unix", value: "pack.sock"},
				closeErr: errors.New("close failed"),
			}, nil
		})
		err := RunAnalyzerLanguageHost(languageHostCmd(t))
		require.ErrorContains(t, err, "getting language host TCP address")
	})

	t.Run("serve error", func(t *testing.T) {
		withLanguageHostListener(t, func() (net.Listener, error) {
			return &scriptedListener{
				addr:      &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1},
				acceptErr: errors.New("accept failed"),
				closeErr:  errors.New("close failed"),
			}, nil
		})
		cmd := &cobra.Command{}
		cmd.SetOut(&lockedDiscard{})
		err := RunAnalyzerLanguageHost(cmd)
		require.ErrorContains(t, err, "serving language host")
	})

	t.Run("port write error", func(t *testing.T) {
		cmd := languageHostCmd(t)
		cmd.SetOut(failWriter{})
		err := RunAnalyzerLanguageHost(cmd)
		require.ErrorContains(t, err, "writing language host port")
	})
}

func languageHostCmd(t *testing.T) *cobra.Command {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	cmd.SetOut(&lockedDiscard{})
	return cmd
}

func withLanguageHostListener(t *testing.T, fn func() (net.Listener, error)) {
	t.Helper()
	previous := listenLanguageHost
	listenLanguageHost = fn
	t.Cleanup(func() { listenLanguageHost = previous })
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

type lockedDiscard struct{}

func (lockedDiscard) Write(p []byte) (int, error) {
	return len(p), nil
}

type stringAddr struct {
	network string
	value   string
}

func (a stringAddr) Network() string { return a.network }
func (a stringAddr) String() string  { return a.value }

type scriptedListener struct {
	addr      net.Addr
	acceptErr error
	closeErr  error
}

func (l *scriptedListener) Accept() (net.Conn, error) {
	if l.acceptErr != nil {
		return nil, l.acceptErr
	}
	return nil, errors.New("no connection")
}

func (l *scriptedListener) Close() error { return l.closeErr }

func (l *scriptedListener) Addr() net.Addr { return l.addr }

type blockingLanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	entered chan struct{}
}

func (h *blockingLanguageHost) GetPluginInfo(
	ctx context.Context,
	_ *emptypb.Empty,
) (*pulumirpc.PluginInfo, error) {
	close(h.entered)
	<-ctx.Done()
	return nil, ctx.Err()
}

func TestStopLanguageHost_StopsStuckRPC(t *testing.T) {
	t.Parallel()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	host := &blockingLanguageHost{entered: make(chan struct{})}
	pulumirpc.RegisterLanguageRuntimeServer(srv, host)
	go func() {
		_ = srv.Serve(lis)
	}()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	done := make(chan struct{})
	go func() {
		_, _ = pulumirpc.NewLanguageRuntimeClient(conn).GetPluginInfo(context.Background(), &emptypb.Empty{})
		close(done)
	}()
	select {
	case <-host.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("rpc did not start")
	}

	started := time.Now()
	stopLanguageHost(srv, 50*time.Millisecond)
	elapsed := time.Since(started)
	assert.GreaterOrEqual(t, elapsed, 40*time.Millisecond)
	assert.Less(t, elapsed, time.Second)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("rpc did not return after stop")
	}
}
