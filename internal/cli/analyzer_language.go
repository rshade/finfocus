package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"

	"github.com/rshade/finfocus/internal/analyzer"
	"github.com/rshade/finfocus/internal/logging"
)

// RunAnalyzerLanguageHost serves Pulumi's language host until the command
// context ends or the process receives SIGINT or SIGTERM. Standard output
// receives only the listening port and a newline. Shutdown waits at most
// the host's ShutdownCap (four seconds by default) so the process exits
// before Pulumi force-kills it.
func RunAnalyzerLanguageHost(cmd *cobra.Command) error {
	parent := cmd.Context()
	if parent == nil {
		parent = context.Background()
	}
	stderrLogger := logging.FromContext(parent).
		With().
		Str("component", "analyzer").
		Str("operation", "language_host").
		Logger()

	ctx, stopSignals := signal.NotifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	listener, err := listenLanguageHost()
	if err != nil {
		return fmt.Errorf("binding language host: %w", err)
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil && !errors.Is(closeErr, net.ErrClosed) {
			stderrLogger.Debug().Err(closeErr).Msg("listener close error")
		}
	}()

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return errors.New("getting language host TCP address")
	}

	host := analyzer.NewLanguageHost(analyzer.LanguageHostOptions{})
	defer host.Shutdown()
	grpcServer := grpc.NewServer()
	pulumirpc.RegisterLanguageRuntimeServer(grpcServer, host)

	errCh := make(chan error, 1)
	go func() {
		errCh <- grpcServer.Serve(listener)
	}()

	// Pulumi's plugin handshake is the port alone. Logs stay on stderr.
	if _, writeErr := fmt.Fprintf(cmd.OutOrStdout(), "%d\n", tcpAddr.Port); writeErr != nil {
		grpcServer.Stop()
		return fmt.Errorf("writing language host port: %w", writeErr)
	}
	stderrLogger.Info().Int("port", tcpAddr.Port).Msg("language host listening")

	select {
	case <-ctx.Done():
		stderrLogger.Info().Msg("language host shutting down")
		// Cancel RunPlugin before GracefulStop. GracefulStop waits for the
		// handler, and the handler is what stops the analyzer.
		host.Shutdown()
		stopLanguageHost(grpcServer, host.ShutdownCap())
		return nil
	case serveErr := <-errCh:
		if serveErr != nil {
			return fmt.Errorf("serving language host: %w", serveErr)
		}
		return nil
	}
}

// listenLanguageHost binds the address Pulumi dials. Tests replace it to
// exercise bind and accept failures.
//
//nolint:gochecknoglobals // test seam for bind and accept failures
var listenLanguageHost = defaultListenLanguageHost

func defaultListenLanguageHost() (net.Listener, error) {
	//nolint:noctx // net.Listen does not accept a context
	return net.Listen("tcp", "127.0.0.1:0")
}

// stopLanguageHost cancels no RPCs by itself. The caller shuts the host down
// first so RunPlugin stops the analyzer, then GracefulStop waits for that
// RPC. Stop unblocks the process if the analyzer outlives the cap.
func stopLanguageHost(grpcServer *grpc.Server, shutdownCap time.Duration) {
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()
	timer := time.NewTimer(shutdownCap)
	defer timer.Stop()
	select {
	case <-stopped:
	case <-timer.C:
		grpcServer.Stop()
		<-stopped
	}
}
