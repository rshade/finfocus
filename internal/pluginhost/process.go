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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/rshade/ax-go"

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
	bindWaitPrecision   = 100 * time.Millisecond // Rounding of the bind wait shown in a timeout error

	// Retry configuration for port collision handling.
	maxPortRetries    = 5
	ciMaxPortRetries  = 10 // Increased retries for CI environments
	initialBackoff    = 100 * time.Millisecond
	maxBackoff        = 2 * time.Second
	backoffMultiplier = 2

	// envValueTrue is the environment variable value that enables a boolean toggle.
	envValueTrue = "true"
)

// getPluginBindTimeout returns the timeout for plugin binding, with increased timeout in CI environments.
func getPluginBindTimeout() time.Duration {
	// Increase timeout in CI environments where resources may be constrained
	if os.Getenv("CI") == envValueTrue {
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

// lockedBuffer wraps [bytes.Buffer] with a mutex for concurrent read/write safety.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (lb *lockedBuffer) Write(p []byte) (int, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Write(p)
}

// Snapshot returns a copy of the current buffer contents.
func (lb *lockedBuffer) Snapshot() []byte {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return append([]byte{}, lb.buf.Bytes()...)
}

// ProcessLauncher launches plugins as separate TCP server processes.
// It is safe for concurrent use.
type ProcessLauncher struct {
	timeout       time.Duration
	portListeners map[int]*portListener
	// pendingPorts holds ports given to a plugin that has not bound yet, so a
	// concurrent launch never receives the same port.
	pendingPorts map[int]struct{}
	// listen opens the reservation listener; nil listens on 127.0.0.1:0. Set by tests.
	listen         func(ctx context.Context) (net.Listener, error)
	mu             sync.Mutex
	maxRetries     int           // Maximum number of launch retries
	stdoutFallback time.Duration // configurable for tests; 0 means use default (stdoutPortFallback)
}

// NewProcessLauncher creates a new ProcessLauncher configured with the package default timeout and an initialized map for tracking reserved port listeners.
func NewProcessLauncher() *ProcessLauncher {
	maxRetries := maxPortRetries
	// Increase retries in CI environments
	if os.Getenv("CI") == envValueTrue {
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
			if !sleepContext(ctx, backoff) {
				return nil, nil, retryStopped(attempt, lastErr)
			}
			backoff = min(backoff*backoffMultiplier, maxBackoff)
		}

		conn, closeFn, err := p.startOnce(ctx, path, args...)
		if err == nil {
			return conn, closeFn, nil
		}

		// A port collision is retried with a new port. A plugin that exits on
		// its own is not, unless its stderr says its port was taken: a config
		// or startup error fails the same way on every attempt.
		if !isPortCollisionError(err) {
			return nil, nil, err
		}
		lastErr = err
		// Once the caller's context has ended, another attempt can only fail to
		// start, and its error would hide why this one failed.
		if ctx.Err() != nil {
			return nil, nil, retryStopped(attempt+1, lastErr)
		}
	}

	return nil, nil, fmt.Errorf("failed after %d attempts: %w", p.maxRetries, lastErr)
}

// retryStopped reports the last launch failure when the retries end early.
func retryStopped(attempts int, lastErr error) error {
	if attempts <= 1 {
		return lastErr
	}
	return fmt.Errorf("failed after %d attempts: %w", attempts, lastErr)
}

// sleepContext waits for d and reports false when ctx ends first.
func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
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
	defer p.releasePendingPort(port)

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

	cmd, out, err := p.startPlugin(ctx, path, port, args)
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

	proc := watchProcess(cmd)

	// Wait for plugin to bind to port, with stdout port detection fallback
	bindStart := time.Now()
	connectPort, bindErr := p.waitForPluginBindWithFallback(ctx, port, &out.stdout, path, proc.exited)
	if bindErr != nil {
		proc.stop()
		return nil, nil, bindFailure(ctx, path, proc, out, bindErr, time.Since(bindStart))
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "pluginhost").
		Int("port", connectPort).
		Msg("plugin bound to port successfully")

	conn, err := p.connectToPlugin(ctx, fmt.Sprintf("127.0.0.1:%d", connectPort), proc)
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

	closeFn := p.createCloseFn(ctx, conn, proc)
	return conn, closeFn, nil
}

// bindFailure builds the launch error for a plugin that did not bind. Call it
// after proc.stop, so an exited plugin's status and stderr are complete.
func bindFailure(
	ctx context.Context,
	path string,
	proc *pluginProcess,
	out *pluginOutput,
	bindErr error,
	waited time.Duration,
) error {
	if !errors.Is(bindErr, errPluginExited) {
		return fmt.Errorf("plugin failed to bind to port within %s: %w",
			waited.Round(bindWaitPrecision), bindErr)
	}
	exitErr := newPluginExitError(proc, &out.stderr, out.secrets)
	logging.FromContext(ctx).Error().
		Ctx(ctx).
		Str("component", "pluginhost").
		Str("plugin_path", path).
		Str("exit_status", exitErr.status).
		Str("stderr_excerpt", exitErr.stderr).
		Msg("plugin exited before binding to port")
	return exitErr
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
	p.releasePendingPort(port)
	if releaseErr := p.releasePortListener(port); releaseErr != nil {
		return 0, fmt.Errorf("releasing port listener: %w", releaseErr)
	}
	return port, nil
}

// maxPortAllocationAttempts bounds how many pending ports allocatePortWithListener
// skips before giving up.
const maxPortAllocationAttempts = 32

// allocatePortWithListener allocates a port and keeps the listener open to prevent race conditions.
// The port is also marked pending until releasePendingPort, and ports that are
// already pending are skipped. The caller must call releasePortListener when
// ready for the plugin to bind.
func (p *ProcessLauncher) allocatePortWithListener(
	ctx context.Context,
) (int, *portListener, error) {
	for range maxPortAllocationAttempts {
		listener, err := p.listenLocal(ctx)
		if err != nil {
			return 0, nil, fmt.Errorf("creating listener: %w", err)
		}

		tcpAddr, ok := listener.Addr().(*net.TCPAddr)
		if !ok {
			_ = listener.Close()
			return 0, nil, errors.New("listener is not TCP address")
		}
		port := tcpAddr.Port

		p.mu.Lock()
		if _, pending := p.pendingPorts[port]; pending {
			p.mu.Unlock()
			// Close at once: a listener held on another launch's port would let
			// that launch's TCP readiness probe connect to it instead of the plugin.
			_ = listener.Close()
			continue
		}
		if p.pendingPorts == nil {
			p.pendingPorts = make(map[int]struct{})
		}
		p.pendingPorts[port] = struct{}{}
		pl := &portListener{
			listener: listener,
			port:     port,
		}
		p.portListeners[port] = pl
		p.mu.Unlock()

		return port, pl, nil
	}
	return 0, nil, fmt.Errorf(
		"no free port after %d attempts: every port offered was pending",
		maxPortAllocationAttempts,
	)
}

func (p *ProcessLauncher) listenLocal(ctx context.Context) (net.Listener, error) {
	if p.listen != nil {
		return p.listen(ctx)
	}
	lc := &net.ListenConfig{}
	return lc.Listen(ctx, "tcp", "127.0.0.1:0")
}

// releasePendingPort ends the reservation taken by allocatePortWithListener.
func (p *ProcessLauncher) releasePendingPort(port int) {
	p.mu.Lock()
	delete(p.pendingPorts, port)
	p.mu.Unlock()
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

// errPluginExited reports that the plugin process ended while core waited for it to bind.
var errPluginExited = errors.New("plugin process exited")

// waitForPluginBind polls until the plugin binds to the specified port, the
// context ends, or exited is closed. A nil exited channel never fires.
func (p *ProcessLauncher) waitForPluginBind(ctx context.Context, port int, exited <-chan struct{}) error {
	ticker := time.NewTicker(bindCheckInterval)
	defer ticker.Stop()

	address := fmt.Sprintf("127.0.0.1:%d", port)
	dialer := &net.Dialer{Timeout: connectionTimeout}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for plugin to bind: %w", ctx.Err())
		case <-exited:
			return errPluginExited
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
// A closed exited channel ends every wait with errPluginExited, so the --port
// guidance is logged only for a process that is still running.
// Returns the port to connect on and any error.
func (p *ProcessLauncher) waitForPluginBindWithFallback(
	ctx context.Context,
	assignedPort int,
	stdoutBuf *lockedBuffer,
	pluginPath string,
	exited <-chan struct{},
) (int, error) {
	log := logging.FromContext(ctx)

	// First, try the assigned --port with a short timeout to detect older plugins quickly
	fallback := p.stdoutFallback
	if fallback == 0 {
		fallback = stdoutPortFallback
	}
	shortCtx, shortCancel := context.WithTimeout(ctx, fallback)
	defer shortCancel()

	shortErr := p.waitForPluginBind(shortCtx, assignedPort, exited)
	if shortErr == nil {
		return assignedPort, nil
	}
	if errors.Is(shortErr, errPluginExited) {
		return 0, shortErr
	}

	// Short timeout elapsed — check if plugin wrote a port to stdout
	if stdoutPort, ok := parsePortFromStdout(stdoutBuf.Snapshot()); ok && stdoutPort != assignedPort {
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

		err := p.waitForPluginBind(fullCtx, stdoutPort, exited)
		if err == nil {
			return stdoutPort, nil
		}
		if errors.Is(err, errPluginExited) {
			return 0, err
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

	if err := p.waitForPluginBind(fullCtx, assignedPort, exited); err != nil {
		if errors.Is(err, errPluginExited) {
			return 0, err
		}
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

// parsePortFromStdout scans the captured stdout data for a port number.
// It recognizes bare port numbers or PORT=NNNNN lines (case-insensitive).
// Returns the parsed port and true if found, or 0 and false otherwise.
func parsePortFromStdout(data []byte) (int, bool) {
	if len(data) == 0 {
		return 0, false
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Try "PORT=NNNNN" format (case-insensitive)
		if strings.HasPrefix(strings.ToUpper(line), "PORT=") {
			var portStr string
			if _, after, ok := strings.Cut(line, "="); ok {
				portStr = strings.TrimSpace(after)
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

// isPortCollisionError reports whether err indicates a port or address collision.
// A plugin that exited before binding counts only when its stderr reported the
// collision; its excerpt is never matched as text.
func isPortCollisionError(err error) bool {
	if err == nil {
		return false
	}
	if exitErr, ok := errors.AsType[*pluginExitError](err); ok {
		return exitErr.portCollision
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

// appendDeliveredEnv adds settings to the plugin process environment.
// Overrides replace an inherited value. The stdio launcher in stdio.go does
// not apply a delivered environment and has no production caller.
func appendDeliveredEnv(ctx context.Context, environ []string) []string {
	delivered, ok := DeliveredEnvFromContext(ctx)
	if !ok {
		return environ
	}
	environ = dropOverrides(environ, delivered.Overrides)
	existing := make(map[string]bool, len(environ))
	for _, entry := range environ {
		key, _, found := strings.Cut(entry, "=")
		if found {
			existing[key] = true
		}
	}
	reserved := map[string]bool{
		pluginsdk.EnvPort:    true,
		pluginsdk.EnvLogFile: true,
		pluginsdk.EnvTraceID: true,
	}
	for _, pair := range delivered.Pairs {
		key, _, found := strings.Cut(pair, "=")
		if !found || key == "" || existing[key] || reserved[key] {
			continue
		}
		environ = append(environ, pair)
		existing[key] = true
	}
	return environ
}

func dropOverrides(environ []string, overrides map[string]bool) []string {
	if len(overrides) == 0 {
		return environ
	}
	filtered := make([]string, 0, len(environ))
	for _, entry := range environ {
		key, _, found := strings.Cut(entry, "=")
		if found && overrides[key] && !reservedEnv(key) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

// pluginOutput holds what core keeps of a plugin's output: stdout for the
// legacy port protocol, and the end of stderr to explain an early exit.
type pluginOutput struct {
	stdout lockedBuffer
	stderr stderrTail
	// secrets are values in the plugin environment that a stderr excerpt masks.
	secrets []string
}

func (p *ProcessLauncher) startPlugin(
	ctx context.Context,
	path string,
	port int,
	args []string,
) (*exec.Cmd, *pluginOutput, error) {
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

	// Propagate logging configuration to plugin processes so well-behaved plugins
	// (using pluginsdk) can write structured logs directly to the same file.
	tap := logging.PluginLogTapFromContext(ctx)
	if tap != nil {
		cmd.Env = dropEnvKey(cmd.Env, pluginsdk.EnvLogFile)
	}
	if logPath := logging.PluginLogPathFromContext(ctx); logPath != "" && tap == nil {
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("%s=%s", pluginsdk.EnvLogFile, logPath),
		)
	}
	if traceID := logging.TraceIDFromContext(ctx); traceID != "" {
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("%s=%s", pluginsdk.EnvTraceID, traceID),
		)
	}
	cmd.Env = appendDeliveredEnv(ctx, cmd.Env)
	delivered, _ := DeliveredEnvFromContext(ctx)
	out := &pluginOutput{secrets: credentialValues(cmd.Env, delivered.Origins)}

	// Determine where plugin stdout/stderr should go.
	// Priority: file logging (always capture) > analyzer mode (discard) >
	// default (stderr passthrough when interactive, discard otherwise)
	pluginWriter := logging.PluginLogWriterFromContext(ctx)

	switch {
	case pluginWriter != nil:
		// Redirect plugin output to the core log file for a clean terminal experience.
		cmd.Stderr = pluginWriter
		cmd.Stdout = io.MultiWriter(pluginWriter, &out.stdout)
	case os.Getenv(constants.EnvAnalyzerMode) == envValueTrue:
		// In analyzer mode, suppress plugin output to prevent cluttering Pulumi preview output.
		// This addresses issue #401 where plugin JSON messages appear in user-facing output.
		log.Debug().
			Ctx(ctx).
			Str("component", "pluginhost").
			Str("plugin_path", path).
			Msg("suppressing plugin stderr output in analyzer mode")
		cmd.Stderr = io.Discard
		cmd.Stdout = io.MultiWriter(io.Discard, &out.stdout)
	default:
		// Fallback (debug mode or no file logging): pass plugin output through to
		// Core's stderr only when it is an interactive terminal. When stderr is a
		// pipe (tests, CI, process supervisors), an orphaned plugin holding the
		// inherited FD would block the parent's Wait() pipe-drain phase
		// indefinitely (issue #1231).
		stderrW := resolveStderrPassthrough(os.Stderr)
		cmd.Stderr = stderrW
		cmd.Stdout = io.MultiWriter(stderrW, &out.stdout)
	}
	cmd.Stderr = io.MultiWriter(cmd.Stderr, &out.stderr)
	if tap != nil {
		cmd.Stderr = newTappedStderr(cmd.Stderr, tap(pluginNameFromPath(path)))
	}
	// Set WaitDelay before Start to avoid race condition with watchCtx goroutine
	cmd.WaitDelay = processWaitDelay

	// Place the plugin in its own process group (with Pdeathsig on Linux) so
	// plugins cannot outlive Core, and kill the whole group when the launch
	// context is canceled (issue #1231).
	configureProcessGroup(cmd)
	cmd.Cancel = func() error {
		if err := killProcessGroup(cmd); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		return nil
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("starting plugin: %w", err)
	}
	return cmd, out, nil
}

func (p *ProcessLauncher) connectToPlugin(
	ctx context.Context,
	address string,
	proc *pluginProcess,
) (*grpc.ClientConn, error) {
	connCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	for {
		if connCtx.Err() != nil {
			proc.stop()
			return nil, fmt.Errorf("timeout connecting to plugin: %w", connCtx.Err())
		}

		conn, err := p.tryConnect(connCtx, address)
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

func (p *ProcessLauncher) tryConnect(ctx context.Context, address string) (*grpc.ClientConn, error) {
	return ax.GRPCDial(ctx, address,
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

// resolveStderrPassthrough returns the writer for plugin stderr/stdout in
// fallback mode (no plugin log writer configured, analyzer mode off). Plugin
// output is passed through to Core's stderr only when stderr is an interactive
// terminal; otherwise it is discarded so an orphaned plugin cannot hold an
// inherited pipe open and block a parent process in Wait()'s pipe-drain phase
// (issue #1231).
func resolveStderrPassthrough(stderr *os.File) io.Writer {
	if stderr != nil && term.IsTerminal(int(stderr.Fd())) {
		return stderr
	}
	return io.Discard
}

func (p *ProcessLauncher) createCloseFn(
	ctx context.Context,
	conn *grpc.ClientConn,
	proc *pluginProcess,
) func() error {
	return func() error {
		log := logging.FromContext(ctx)
		log.Debug().
			Ctx(ctx).
			Str("component", "pluginhost").
			Str("operation", "close_plugin").
			Msg("closing plugin connection")

		var closeErr error
		if err := conn.Close(); err != nil {
			log.Warn().
				Ctx(ctx).
				Str("component", "pluginhost").
				Err(err).
				Msg("error closing gRPC connection")
			closeErr = fmt.Errorf("closing connection: %w", err)
		}
		if proc != nil && proc.cmd.Process != nil {
			pid := proc.cmd.Process.Pid
			proc.stop()
			log.Debug().
				Ctx(ctx).
				Str("component", "pluginhost").
				Int("pid", pid).
				Msg("plugin process terminated")
		}
		return closeErr
	}
}

const pluginBinaryPrefix = "finfocus-plugin-"

// pluginNameFromPath derives the plugin name from its binary path by dropping
// the directory, a ".exe" suffix, and the "finfocus-plugin-" prefix.
func pluginNameFromPath(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), ".exe")
	return strings.TrimPrefix(base, pluginBinaryPrefix)
}

func dropEnvKey(environ []string, key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(environ))
	for _, entry := range environ {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// tappedStderr copies plugin stderr to its normal destination and to a log tap.
// A tap failure never reaches the plugin: a failed write would stop the stderr
// copy and could block the plugin on a full pipe.
type tappedStderr struct {
	io.Writer

	tap io.WriteCloser
	one sync.Once
}

type bestEffortWriter struct{ w io.Writer }

func (b bestEffortWriter) Write(p []byte) (int, error) {
	_, _ = b.w.Write(p)
	return len(p), nil
}

func newTappedStderr(dest io.Writer, tap io.WriteCloser) *tappedStderr {
	return &tappedStderr{
		Writer: io.MultiWriter(dest, bestEffortWriter{w: tap}),
		tap:    tap,
	}
}

func (t *tappedStderr) closeTap() {
	t.one.Do(func() { _ = t.tap.Close() })
}

// closeLogTap flushes and closes the log tap once the process has been waited on.
func closeLogTap(cmd *exec.Cmd) {
	if ts, ok := cmd.Stderr.(*tappedStderr); ok {
		ts.closeTap()
	}
}
