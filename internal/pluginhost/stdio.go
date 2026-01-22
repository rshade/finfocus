package pluginhost

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/rshade/finfocus/internal/logging"
)

const (
	stdioTimeout   = 10 * time.Second
	stdioWaitDelay = 100 * time.Millisecond // Time to wait for I/O after killing process
)

// StdioLauncher launches plugins using stdin/stdout communication.
type StdioLauncher struct {
	timeout time.Duration
}

// NewStdioLauncher creates a new stdio-based plugin launcher.
func NewStdioLauncher() *StdioLauncher {
	return &StdioLauncher{
		timeout: stdioTimeout,
	}
}

// Start launches a plugin using stdio communication and returns the gRPC connection.
//
//nolint:funlen // Comprehensive logging requires additional lines for observability
func (s *StdioLauncher) Start(
	ctx context.Context,
	path string,
	args ...string,
) (*grpc.ClientConn, func() error, error) {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "pluginhost").
		Str("operation", "start_plugin_stdio").
		Str("plugin_path", path).
		Msg("starting plugin process via stdio")

	//nolint:gosec // Plugin path is validated before execution
	cmd := exec.CommandContext(
		ctx,
		path,
		append(args, "--stdio")...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	cmd.Stderr = os.Stderr
	// Set WaitDelay before Start to avoid race condition with watchCtx goroutine
	cmd.WaitDelay = stdioWaitDelay

	if startErr := cmd.Start(); startErr != nil {
		return nil, nil, fmt.Errorf("starting plugin: %w", startErr)
	}

	lc := &net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		return nil, nil, fmt.Errorf("creating proxy listener: %w", err)
	}

	go s.proxy(listener, stdin, stdout)

	address := listener.Addr().String()

	conn, err := grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(TraceInterceptor()))
	if err != nil {
		log.Error().
			Ctx(ctx).
			Str("component", "pluginhost").
			Err(err).
			Msg("failed to create gRPC client")
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		_ = listener.Close()
		return nil, nil, fmt.Errorf("connecting to plugin: %w", err)
	}

	log.Info().
		Ctx(ctx).
		Str("component", "pluginhost").
		Str("plugin_path", path).
		Int("pid", cmd.Process.Pid).
		Msg("plugin connected successfully via stdio")

	closeFn := func() error {
		log.Debug().
			Ctx(ctx).
			Str("component", "pluginhost").
			Str("operation", "close_plugin_stdio").
			Msg("closing stdio plugin connection")

		if connCloseErr := conn.Close(); connCloseErr != nil {
			log.Warn().
				Ctx(ctx).
				Str("component", "pluginhost").
				Err(connCloseErr).
				Msg("error closing gRPC connection")
			return fmt.Errorf("closing connection: %w", connCloseErr)
		}
		if listenerCloseErr := listener.Close(); listenerCloseErr != nil {
			return fmt.Errorf("closing listener: %w", listenerCloseErr)
		}
		if cmd.Process != nil {
			pid := cmd.Process.Pid
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			log.Debug().
				Ctx(ctx).
				Str("component", "pluginhost").
				Int("pid", pid).
				Msg("stdio plugin process terminated")
		}
		return nil
	}

	return conn, closeFn, nil
}

func (s *StdioLauncher) proxy(listener net.Listener, stdin io.WriteCloser, stdout io.ReadCloser) {
	conn, err := listener.Accept()
	if err != nil {
		return
	}
	defer conn.Close()

	go func() {
		_, _ = io.Copy(stdin, conn)
	}()
	_, _ = io.Copy(conn, stdout)
}
