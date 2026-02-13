package pluginhost

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	"github.com/rshade/finfocus/internal/constants"
	"github.com/rshade/finfocus/internal/logging"
)

const (
	defaultTimeout      = 10 * time.Second
	connectionDelay     = 100 * time.Millisecond
	connectionTimeout   = 100 * time.Millisecond
	processWaitDelay    = 100 * time.Millisecond // Time to wait for I/O after killing process
	pluginBindTimeout   = 60 * time.Second       // Time to wait for plugin to bind (large plugins need more time)
	ciPluginBindTimeout = 120 * time.Second      // Increased timeout for CI environments
	bindCheckInterval   = 100 * time.Millisecond // Interval between bind checks
	stdoutPortFallback  = 5 * time.Second        // Time to wait for stdout port before full timeout

	// Retry configuration for port collision handling.
	maxPortRetries    = 5
	ciMaxPortRetries  = 10 // Increased retries for CI environments
	initialBackoff    = 100 * time.Millisecond
	maxBackoff        = 2 * time.Second
	backoffMultiplier = 2
)

// getPluginBindTimeout returns the timeout for plugin binding, with increased timeout in CI environments.
func getPluginBindTimeout() time.Duration {
	// Increase timeout in CI environments where resources may be constrained
	//nolint:goconst // "true" is used in multiple contexts, not worth a constant
	if os.Getenv("CI") == "true" {
		return ciPluginBindTimeout
	}
	return pluginBindTimeout
}

// portListener holds a reference to an open listener for port reservation.
// It is used to prevent race conditions during plugin startup by keeping
// a TCP listener open while a port is being allocated. The listener is
// stored in ProcessLauncher.portListeners and must be explicitly released
// via releasePortListener before the plugin can bind to the port.
type portListener struct {
	listener net.Listener
	port     int
}

// ProcessLauncher launches plugins as separate TCP server processes.
type ProcessLauncher struct {
	timeout       time.Duration
	portListeners map[int]*portListener
	mu            sync.Mutex
	maxRetries    int // Maximum number of launch retries
}

// NewProcessLauncher creates a new ProcessLauncher configured with the package default timeout and an initialized map for tracking reserved port listeners.
func NewProcessLauncher() *ProcessLauncher {
	maxRetries := maxPortRetries
	// Increase retries in CI environments
	if os.Getenv("CI") == "true" {
		maxRetries = ciMaxPortRetries
	}
	return &ProcessLauncher{
		timeout:       defaultTimeout,
		portListeners: make(map[int]*portListener),
		maxRetries:    maxRetries,
	}
}

// NewProcessLauncherWithRetries creates a new ProcessLauncher with configurable retry attempts.
func NewProcessLauncherWithRetries(maxRetries int) *ProcessLauncher {
	return &ProcessLauncher{
		timeout:       defaultTimeout,
		portListeners: make(map[int]*portListener),
		maxRetries:    maxRetries,
	}
}

// Start launches a plugin process with TCP communication and returns the gRPC connection.
// This method uses retry logic with exponential backoff to handle potential port collisions.
func (p *ProcessLauncher) Start(
	ctx context.Context,
	path string,
	args ...string,
) (*grpc.ClientConn, func() error, error) {
	return p.StartWithRetry(ctx, path, args...)
}

// StartWithRetry attempts to launch a plugin with retry logic for port collisions.
func (p *ProcessLauncher) StartWithRetry(
	ctx context.Context,
	path string,
	args ...string,
) (*grpc.ClientConn, func() error, error) {
	log := logging.FromContext(ctx)
	var lastErr error
	backoff := initialBackoff

	for attempt := range p.maxRetries {
		if attempt > 0 {
			log.Debug().
				Ctx(ctx).
				Str("component", "pluginhost").
				Int("attempt", attempt+1).
				Int("max_attempts", p.maxRetries).
				Dur("backoff", backoff).
				Msg("retrying plugin launch after port collision")
			time.Sleep(backoff)
			backoff = min(backoff*backoffMultiplier, maxBackoff)
		}

		conn, closeFn, err := p.startOnce(ctx, path, args...)
		if err == nil {
			return conn, closeFn, nil
		}

		// Check if error is port-related
		if isPortCollisionError(err) {
			lastErr = err
			continue
		}

		// Non-port error, fail immediately
		return nil, nil, err
	}

	return nil, nil, fmt.Errorf("failed after %d attempts: %w", p.maxRetries, lastErr)
}

// startOnce performs a single attempt to start the plugin.
func (p *ProcessLauncher) startOnce(
	ctx context.Context,
	path string,
	args ...string,
) (*grpc.ClientConn, func() error, error) {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "pluginhost").
		Str("operation", "start_plugin").
		Str("plugin_path", path).
		Msg("starting plugin process")

	// Allocate port and keep listener open to prevent race condition
	port, pl, err := p.allocatePortWithListener(ctx)
	if err != nil {
		log.Error().
			Ctx(ctx).
			Str("component", "pluginhost").
			Err(err).
			Msg("failed to allocate port for plugin")
		return nil, nil, err
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "pluginhost").
		Int("port", port).
		Msg("allocated port for plugin (listener held open)")

	// Release the listener before starting the plugin so plugin can bind
	if releaseErr := p.releasePortListener(port); releaseErr != nil {
		log.Warn().
			Ctx(ctx).
			Str("component", "pluginhost").
			Err(releaseErr).
			Int("port", port).
			Msg("failed to release port listener")
	}
	_ = pl // Silence unused variable warning

	cmd, stdoutBuf, err := p.startPlugin(ctx, path, port, args)
	if err != nil {
		log.Error().
			Ctx(ctx).
			Str("component", "pluginhost").
			Err(err).
			Str("plugin_path", path).
			Int("port", port).
			Msg("failed to start plugin process")
		return nil, nil, err
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "pluginhost").
		Int("pid", cmd.Process.Pid).
		Msg("plugin process started")

	// Wait for plugin to bind to port, with stdout port detection fallback
	connectPort, bindErr := p.waitForPluginBindWithFallback(ctx, port, stdoutBuf, path)
	if bindErr != nil {
		p.killProcess(cmd)
		return nil, nil, fmt.Errorf("plugin failed to bind to port: %w", bindErr)
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "pluginhost").
		Int("port", connectPort).
		Msg("plugin bound to port successfully")

	conn, err := p.connectToPlugin(ctx, fmt.Sprintf("127.0.0.1:%d", connectPort), cmd)
	if err != nil {
		log.Error().
			Ctx(ctx).
			Str("component", "pluginhost").
			Err(err).
			Str("address", fmt.Sprintf("127.0.0.1:%d", port)).
			Msg("failed to connect to plugin")
		return nil, nil, err
	}

	log.Info().
		Ctx(ctx).
		Str("component", "pluginhost").
		Str("plugin_path", path).
		Int("port", connectPort).
		Int("pid", cmd.Process.Pid).
		Msg("plugin connected successfully")

	closeFn := p.createCloseFn(ctx, conn, cmd)
	return conn, closeFn, nil
}

// allocatePort allocates a port (legacy method, still available for backward compatibility).
// Note: This method has a race condition window between port allocation and plugin startup.
// Prefer using allocatePortWithListener for new code.
func (p *ProcessLauncher) allocatePort(ctx context.Context) (int, error) {
	port, _, err := p.allocatePortWithListener(ctx)
	if err != nil {
		return 0, err
	}
	// Immediately release for backward compatibility
	if releaseErr := p.releasePortListener(port); releaseErr != nil {
		return 0, fmt.Errorf("releasing port listener: %w", releaseErr)
	}
	return port, nil
}

// allocatePortWithListener allocates a port and keeps the listener open to prevent race conditions.
// The caller must call releasePortListener when ready for the plugin to bind.
func (p *ProcessLauncher) allocatePortWithListener(
	ctx context.Context,
) (int, *portListener, error) {
	lc := &net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, nil, fmt.Errorf("creating listener: %w", err)
	}

	tcpAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = listener.Close()
		return 0, nil, errors.New("listener is not TCP address")
	}
	port := tcpAddr.Port

	pl := &portListener{
		listener: listener,
		port:     port,
	}

	p.mu.Lock()
	p.portListeners[port] = pl
	p.mu.Unlock()

	return port, pl, nil
}

// releasePortListener closes the listener for a reserved port, allowing the plugin to bind.
func (p *ProcessLauncher) releasePortListener(port int) error {
	p.mu.Lock()
	pl, exists := p.portListeners[port]
	if exists {
		delete(p.portListeners, port)
	}
	p.mu.Unlock()

	if !exists {
		return fmt.Errorf("no listener for port %d", port)
	}

	if err := pl.listener.Close(); err != nil {
		return fmt.Errorf("closing listener: %w", err)
	}

	return nil
}

// waitForPluginBind polls until the plugin binds to the specified port or timeout.
func (p *ProcessLauncher) waitForPluginBind(ctx context.Context, port int) error {
	ticker := time.NewTicker(bindCheckInterval)
	defer ticker.Stop()

	address := fmt.Sprintf("127.0.0.1:%d", port)
	dialer := &net.Dialer{Timeout: connectionTimeout}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for plugin to bind: %w", ctx.Err())
		case <-ticker.C:
			// Try to connect - if plugin is listening, this will succeed
			conn, err := dialer.DialContext(ctx, "tcp", address)
			if err == nil {
				_ = conn.Close()
				return nil // Plugin is listening!
			}
			// Keep trying...
		}
	}
}

// waitForPluginBindWithFallback waits for the plugin to bind to the assigned port.
// If binding fails within stdoutPortFallback, it checks whether the plugin wrote
// a port number to stdout (backward-compatible protocol for older plugins).
// Returns the port to connect on and any error.
func (p *ProcessLauncher) waitForPluginBindWithFallback(
	ctx context.Context,
	assignedPort int,
	stdoutBuf *bytes.Buffer,
	pluginPath string,
) (int, error) {
	log := logging.FromContext(ctx)

	// First, try the assigned --port with a short timeout to detect older plugins quickly
	shortCtx, shortCancel := context.WithTimeout(ctx, stdoutPortFallback)
	defer shortCancel()

	if err := p.waitForPluginBind(shortCtx, assignedPort); err == nil {
		return assignedPort, nil
	}

	// Short timeout elapsed — check if plugin wrote a port to stdout
	if stdoutPort, ok := parsePortFromStdout(stdoutBuf); ok && stdoutPort != assignedPort {
		log.Warn().
			Ctx(ctx).
			Str("component", "pluginhost").
			Int("assigned_port", assignedPort).
			Int("stdout_port", stdoutPort).
			Str("plugin_path", pluginPath).
			Msg("plugin did not bind to --port; falling back to stdout-advertised port (older plugin protocol)")

		// Try binding on the stdout-advertised port with the remaining full timeout
		fullCtx, fullCancel := context.WithTimeout(ctx, getPluginBindTimeout())
		defer fullCancel()

		err := p.waitForPluginBind(fullCtx, stdoutPort)
		if err == nil {
			return stdoutPort, nil
		}
		return 0, fmt.Errorf("plugin advertised port %d on stdout but failed to bind: %w",
			stdoutPort, err)
	}

	// No stdout port detected — continue waiting on assigned port with full timeout
	log.Debug().
		Ctx(ctx).
		Str("component", "pluginhost").
		Int("port", assignedPort).
		Str("plugin_path", pluginPath).
		Msg("no stdout port detected, continuing to wait for --port binding")

	fullCtx, fullCancel := context.WithTimeout(ctx, getPluginBindTimeout())
	defer fullCancel()

	if err := p.waitForPluginBind(fullCtx, assignedPort); err != nil {
		log.Error().
			Ctx(ctx).
			Str("component", "pluginhost").
			Err(err).
			Int("port", assignedPort).
			Str("plugin_path", pluginPath).
			Str("guidance", "ensure the plugin supports --port flag or upgrade to pluginsdk v0.5.5+").
			Msg("plugin failed to bind to port")
		return 0, err
	}

	return assignedPort, nil
}

// parsePortFromStdout scans the captured stdout buffer for a port number.
// It recognizes bare port numbers or PORT=NNNNN lines (case-insensitive).
// parsePortFromStdout scans the provided buffer for a plugin port announcement and returns the parsed
// port and true if a valid port is found, or 0 and false otherwise. It recognizes either a case-
// insensitive "PORT=NNNN" key-value line or a bare numeric port on a line, and validates the port
// is in the range 1–65535. If buf is nil or empty no port is returned.
func parsePortFromStdout(buf *bytes.Buffer) (int, bool) {
	if buf == nil || buf.Len() == 0 {
		return 0, false
	}

	scanner := bufio.NewScanner(bytes.NewReader(buf.Bytes()))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try "PORT=NNNNN" format (case-insensitive)
		if strings.HasPrefix(strings.ToUpper(line), "PORT=") {
			portStr := strings.TrimPrefix(strings.ToUpper(line), "PORT=")
			// Use the original casing for the number part
			if idx := strings.Index(line, "="); idx >= 0 {
				portStr = strings.TrimSpace(line[idx+1:])
			}
			if port, err := strconv.Atoi(portStr); err == nil && port > 0 && port <= 65535 {
				return port, true
			}
		}

		// Try bare port number (e.g., analyzer protocol prints just the number)
		if port, err := strconv.Atoi(line); err == nil && port > 0 && port <= 65535 {
			return port, true
		}
	}

	return 0, false
}

// isPortCollisionError checks if an error is related to port collision.
// It uses string pattern matching to handle port collision errors across
// isPortCollisionError reports whether the provided error indicates a port/address
// collision when attempting to bind a network address.
//
// It returns true if err contains common platform-independent phrases that
// indicate the address or port is already in use; returns false for nil or
// unrelated errors. The check uses string matching to remain portable across
// isPortCollisionError reports whether err indicates a port or address collision.
// It returns true if err is non-nil and its error message contains common
// platform strings used for port-binding conflicts such as
// "address already in use", "bind: address already in use", "port is already allocated",
// or "failed to bind to port". If err is nil it returns false.
func isPortCollisionError(err error) bool {
	if err == nil {
		return false
	}

	// Use string matching which is portable across OS/locales.
	// The error message "address already in use" is consistent across platforms,
	// even though the underlying syscall errors differ (EADDRINUSE vs WSAEADDRINUSE).
	errStr := err.Error()
	return strings.Contains(errStr, "address already in use") ||
		strings.Contains(errStr, "bind: address already in use") ||
		strings.Contains(errStr, "port is already allocated") ||
		strings.Contains(errStr, "failed to bind to port")
}

func (p *ProcessLauncher) startPlugin(
	ctx context.Context,
	path string,
	port int,
	args []string,
) (*exec.Cmd, *bytes.Buffer, error) {
	log := logging.FromContext(ctx)

	// FR-008: Log DEBUG message when PORT is detected in user's environment
	// This helps users understand that their PORT env var is being ignored
	if portEnv := os.Getenv("PORT"); portEnv != "" {
		log.Debug().
			Ctx(ctx).
			Str("component", "pluginhost").
			Str("inherited_port", portEnv).
			Msg("PORT environment variable detected in parent environment (will be ignored, plugin uses --port flag)")
	}

	//nolint:gosec // Plugin path is validated before execution
	cmd := exec.CommandContext(
		ctx,
		path,
		append(args, fmt.Sprintf("--port=%d", port))...)
	// Set FINFOCUS_PLUGIN_PORT environment variable for plugin port communication.
	// The --port flag is authoritative; FINFOCUS_PLUGIN_PORT is for debugging/tooling.
	// Note: PORT is intentionally NOT set (issue #232) - plugins should use --port flag
	// or pluginsdk.GetPort() which reads FINFOCUS_PLUGIN_PORT.
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("%s=%d", pluginsdk.EnvPort, port),
	)

	// Capture stdout in a buffer for backward-compatible port detection.
	// Older plugins may print their port to stdout instead of binding to --port.
	// We tee to stderr so existing log forwarding still works.
	var stdoutBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stderr, &stdoutBuf)

	// In analyzer mode, suppress plugin stderr to prevent verbose logs from cluttering Pulumi preview output
	// This addresses issue #401 where plugin JSON messages appear in user-facing output
	if os.Getenv(constants.EnvAnalyzerMode) == "true" {
		log.Debug().
			Ctx(ctx).
			Str("component", "pluginhost").
			Str("plugin_path", path).
			Msg("suppressing plugin stderr output in analyzer mode")
		cmd.Stderr = io.Discard
	} else {
		cmd.Stderr = os.Stderr
	}
	// Set WaitDelay before Start to avoid race condition with watchCtx goroutine
	cmd.WaitDelay = processWaitDelay

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("starting plugin: %w", err)
	}
	return cmd, &stdoutBuf, nil
}

func (p *ProcessLauncher) connectToPlugin(
	ctx context.Context,
	address string,
	cmd *exec.Cmd,
) (*grpc.ClientConn, error) {
	connCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	for {
		if connCtx.Err() != nil {
			p.killProcess(cmd)
			return nil, fmt.Errorf("timeout connecting to plugin: %w", connCtx.Err())
		}

		conn, err := p.tryConnect(address)
		if err != nil {
			time.Sleep(connectionDelay)
			continue
		}

		if p.isConnectionReady(connCtx, conn) {
			return conn, nil
		}

		_ = conn.Close()
		time.Sleep(connectionDelay)
	}
}

func (p *ProcessLauncher) tryConnect(address string) (*grpc.ClientConn, error) {
	return grpc.NewClient(address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(TraceInterceptor()))
}

func (p *ProcessLauncher) isConnectionReady(ctx context.Context, conn *grpc.ClientConn) bool {
	testCtx, testCancel := context.WithTimeout(ctx, connectionTimeout)
	defer testCancel()

	state := conn.GetState()
	if state == connectivity.Ready || state == connectivity.Idle {
		return true
	}

	conn.WaitForStateChange(testCtx, state)
	newState := conn.GetState()
	return newState == connectivity.Ready
}

func (p *ProcessLauncher) killProcess(cmd *exec.Cmd) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

func (p *ProcessLauncher) createCloseFn(
	ctx context.Context,
	conn *grpc.ClientConn,
	cmd *exec.Cmd,
) func() error {
	return func() error {
		log := logging.FromContext(ctx)
		log.Debug().
			Ctx(ctx).
			Str("component", "pluginhost").
			Str("operation", "close_plugin").
			Msg("closing plugin connection")

		if err := conn.Close(); err != nil {
			log.Warn().
				Ctx(ctx).
				Str("component", "pluginhost").
				Err(err).
				Msg("error closing gRPC connection")
			return fmt.Errorf("closing connection: %w", err)
		}
		if cmd.Process != nil {
			pid := cmd.Process.Pid
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			log.Debug().
				Ctx(ctx).
				Str("component", "pluginhost").
				Int("pid", pid).
				Msg("plugin process terminated")
		}
		return nil
	}
}