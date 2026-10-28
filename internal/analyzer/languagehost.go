package analyzer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rshade/finfocus/pkg/version"
)

// projectDirEnvVar is the variable config.ResolveProjectDir reads.
const projectDirEnvVar = "FINFOCUS_PROJECT_DIR"

// LanguageHostDefaultWaitDelay is how long RunPlugin waits after cancel
// before the analyzer process is killed.
const LanguageHostDefaultWaitDelay = 3 * time.Second

// LanguageHostOptions configures the language host. Zero values select the
// process executable, the legacy analyzer entry name, and
// LanguageHostDefaultWaitDelay.
type LanguageHostOptions struct {
	// Executable is the binary RunPlugin starts. Empty means os.Executable.
	Executable string
	// AnalyzerName is Args[0] of the analyzer process, so a symlinked
	// language host still dispatches to analyzer mode.
	AnalyzerName string
	// WaitDelay bounds how long the analyzer may run after cancel.
	WaitDelay time.Duration
}

// LanguageHost serves Pulumi's LanguageRuntime service for a finfocus policy pack.
type LanguageHost struct {
	pulumirpc.UnimplementedLanguageRuntimeServer

	opts   LanguageHostOptions
	base   context.Context
	cancel context.CancelFunc
}

// NewLanguageHost returns a language host with defaults filled in for any
// zero option.
func NewLanguageHost(opts LanguageHostOptions) *LanguageHost {
	if opts.Executable == "" {
		if exe, err := os.Executable(); err == nil {
			opts.Executable = exe
		}
	}
	if opts.AnalyzerName == "" {
		opts.AnalyzerName = policyPackEntryName(policyPackBinaryName, runtime.GOOS)
	}
	if opts.WaitDelay <= 0 {
		opts.WaitDelay = LanguageHostDefaultWaitDelay
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &LanguageHost{opts: opts, base: ctx, cancel: cancel}
}

// Shutdown cancels running RunPlugin calls so their analyzer processes stop.
func (h *LanguageHost) Shutdown() {
	h.cancel()
}

// ShutdownCap is how long the process may spend in GracefulStop before Stop.
// It is WaitDelay plus one second, inside Pulumi's five-second force-kill.
func (h *LanguageHost) ShutdownCap() time.Duration {
	return h.opts.WaitDelay + time.Second
}

// GetPluginInfo returns the finfocus version.
func (h *LanguageHost) GetPluginInfo(context.Context, *emptypb.Empty) (*pulumirpc.PluginInfo, error) {
	return &pulumirpc.PluginInfo{Version: version.GetVersion()}, nil
}

// Handshake accepts the engine handshake. A policy pack runtime has no
// protocol options to negotiate.
func (h *LanguageHost) Handshake(
	context.Context,
	*pulumirpc.LanguageHandshakeRequest,
) (*pulumirpc.LanguageHandshakeResponse, error) {
	return &pulumirpc.LanguageHandshakeResponse{}, nil
}

// GetRequiredPlugins reports no plugins. A finfocus policy pack has none.
func (h *LanguageHost) GetRequiredPlugins(
	context.Context,
	*pulumirpc.GetRequiredPluginsRequest,
) (*pulumirpc.GetRequiredPluginsResponse, error) {
	return &pulumirpc.GetRequiredPluginsResponse{}, nil
}

// GetRequiredPackages reports no packages. A finfocus policy pack has none.
func (h *LanguageHost) GetRequiredPackages(
	context.Context,
	*pulumirpc.GetRequiredPackagesRequest,
) (*pulumirpc.GetRequiredPackagesResponse, error) {
	return &pulumirpc.GetRequiredPackagesResponse{}, nil
}

// GetProgramDependencies reports no language dependencies.
func (h *LanguageHost) GetProgramDependencies(
	context.Context,
	*pulumirpc.GetProgramDependenciesRequest,
) (*pulumirpc.GetProgramDependenciesResponse, error) {
	return &pulumirpc.GetProgramDependenciesResponse{}, nil
}

// InstallDependencies succeeds without output. A finfocus policy pack has
// nothing to install.
func (h *LanguageHost) InstallDependencies(
	*pulumirpc.InstallDependenciesRequest,
	grpc.ServerStreamingServer[pulumirpc.InstallDependenciesResponse],
) error {
	return nil
}

// About returns this host's executable path and the finfocus version.
func (h *LanguageHost) About(context.Context, *pulumirpc.AboutRequest) (*pulumirpc.AboutResponse, error) {
	return &pulumirpc.AboutResponse{
		Executable: h.opts.Executable,
		Version:    version.GetVersion(),
	}, nil
}

// RunPlugin starts the analyzer and streams its output unchanged.
func (h *LanguageHost) RunPlugin(
	req *pulumirpc.RunPluginRequest,
	stream grpc.ServerStreamingServer[pulumirpc.RunPluginResponse],
) error {
	if err := validateRunPlugin(req); err != nil {
		return err
	}

	ctx, stop := context.WithCancel(stream.Context())
	defer stop()
	go func() {
		select {
		case <-h.base.Done():
			stop()
		case <-ctx.Done():
		}
	}()

	cmd := h.analyzerCommand(ctx, req)
	send := newPluginSender(stream)
	cmd.Stdout = &pluginOutputWriter{send: send, stdout: true}
	cmd.Stderr = &pluginOutputWriter{send: send, stdout: false}
	if startErr := cmd.Start(); startErr != nil {
		return fmt.Errorf("starting analyzer: %w", startErr)
	}
	return finishRunPlugin(cmd.Wait(), send)
}

func validateRunPlugin(req *pulumirpc.RunPluginRequest) error {
	if len(req.GetArgs()) == 0 {
		return status.Error(codes.InvalidArgument, "args is required")
	}
	info := req.GetInfo()
	if info == nil {
		return status.Error(codes.InvalidArgument, "info is required")
	}
	if !filepath.IsAbs(info.GetProgramDirectory()) {
		return status.Error(codes.InvalidArgument, "info.program_directory must be absolute")
	}
	if req.GetKind() != "analyzer" {
		return status.Error(codes.InvalidArgument, "kind must be analyzer")
	}
	return nil
}

func (h *LanguageHost) analyzerCommand(ctx context.Context, req *pulumirpc.RunPluginRequest) *exec.Cmd {
	// Args[0] is the legacy analyzer name, not the executable path. os.Executable
	// resolves symlinks, so the default Args[0] would dispatch to the CLI.
	cmd := exec.CommandContext( //nolint:gosec // binary is this process; args[0] is the engine address
		ctx, h.opts.Executable, req.GetArgs()[0], ".")
	cmd.Args[0] = h.opts.AnalyzerName
	cmd.Dir = req.GetInfo().GetProgramDirectory()
	cmd.Env = append(os.Environ(), req.GetEnv()...)
	cmd.Env = append(cmd.Env, projectDirEnv(req)...)
	cmd.WaitDelay = h.opts.WaitDelay
	setCmdCancel(cmd)
	return cmd
}

// projectDirEnv hands the analyzer the Pulumi project through the same
// FINFOCUS_PROJECT_DIR the CLI already reads. The analyzer runs in the policy
// pack directory, which holds no Pulumi.yaml, so without it project-tier config
// is never found. An explicit setting wins, and a pwd that is not an absolute
// directory is ignored.
func projectDirEnv(req *pulumirpc.RunPluginRequest) []string {
	pwd := req.GetPwd()
	if !filepath.IsAbs(pwd) {
		return nil
	}
	if info, err := os.Stat(pwd); err != nil || !info.IsDir() {
		return nil
	}
	if os.Getenv(projectDirEnvVar) != "" {
		return nil
	}
	prefix := projectDirEnvVar + "="
	for _, entry := range req.GetEnv() {
		if strings.HasPrefix(entry, prefix) && len(entry) > len(prefix) {
			return nil
		}
	}
	return []string{prefix + pwd}
}

func finishRunPlugin(waitErr error, send func(*pulumirpc.RunPluginResponse) error) error {
	if waitErr == nil || errors.Is(waitErr, context.Canceled) || errors.Is(waitErr, context.DeadlineExceeded) {
		return nil
	}
	exitErr, ok := errors.AsType[*exec.ExitError](waitErr)
	if ok {
		code := exitErr.ExitCode()
		if code == 0 {
			return nil
		}
		// Process exit codes fit in int32.
		exitCode := int32(code) //nolint:gosec // process exit codes fit in int32
		msg := &pulumirpc.RunPluginResponse{
			Output: &pulumirpc.RunPluginResponse_Exitcode{Exitcode: exitCode},
		}
		if sendErr := send(msg); sendErr != nil {
			return sendErr
		}
		return nil
	}
	return waitErr
}

func newPluginSender(
	stream grpc.ServerStreamingServer[pulumirpc.RunPluginResponse],
) func(*pulumirpc.RunPluginResponse) error {
	var mu sync.Mutex
	return func(msg *pulumirpc.RunPluginResponse) error {
		mu.Lock()
		defer mu.Unlock()
		return stream.Send(msg)
	}
}

// pluginOutputWriter copies each write onto the RunPlugin stream. The copy
// is made before Send because the buffer is reused by os/exec.
type pluginOutputWriter struct {
	send   func(*pulumirpc.RunPluginResponse) error
	stdout bool
}

func (w *pluginOutputWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	buf := make([]byte, len(p))
	copy(buf, p)
	msg := &pulumirpc.RunPluginResponse{}
	if w.stdout {
		msg.Output = &pulumirpc.RunPluginResponse_Stdout{Stdout: buf}
	} else {
		msg.Output = &pulumirpc.RunPluginResponse_Stderr{Stderr: buf}
	}
	if err := w.send(msg); err != nil {
		return 0, err
	}
	return len(p), nil
}
