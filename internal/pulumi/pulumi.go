package pulumi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rshade/finfocus/internal/logging"
)

// Default timeouts for Pulumi CLI commands.
const (
	DefaultPreviewTimeout = 5 * time.Minute
	DefaultExportTimeout  = 60 * time.Second
)

// PreviewOptions configures a Pulumi preview command execution.
type PreviewOptions struct {
	ProjectDir string        // Directory containing Pulumi.yaml.
	Stack      string        // Specific stack name (empty = current).
	Timeout    time.Duration // Max execution time (default: 5 minutes).
	Passphrase *string       // PULUMI_CONFIG_PASSPHRASE injected into subprocess env only (nil = inherit parent env; non-nil = inject, even if empty).
}

// ExportOptions configures a Pulumi stack export command execution.
type ExportOptions struct {
	ProjectDir string        // Directory containing Pulumi.yaml.
	Stack      string        // Specific stack name (empty = current).
	Timeout    time.Duration // Max execution time (default: 60 seconds).
	Passphrase *string       // PULUMI_CONFIG_PASSPHRASE injected into subprocess env only (nil = inherit parent env; non-nil = inject, even if empty).
}

// StackInfo represents a single stack from pulumi stack ls --json.
type StackInfo struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
	URL     string `json:"url"`
}

// CommandRunner executes an external command and returns its stdout, stderr, and error.
// extraEnv entries (e.g. "KEY=value") are appended to the subprocess environment only;
// they do not mutate the parent process environment.
// This interface enables testing without spawning real subprocesses.
type CommandRunner interface {
	Run(
		ctx context.Context,
		dir string,
		name string,
		extraEnv []string,
		args ...string,
	) (stdout []byte, stderr []byte, err error)
}

// execRunner is the default CommandRunner that uses exec.CommandContext.
type execRunner struct{}

func (r *execRunner) Run(
	ctx context.Context, dir string, name string, extraEnv []string, args ...string,
) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// Runner is the package-level CommandRunner. Replace in tests with a mock.
var Runner CommandRunner = &execRunner{} //nolint:gochecknoglobals // Required for test injection
var runnerMu sync.Mutex                  //nolint:gochecknoglobals // Guards package-level Runner access.

func getRunner() CommandRunner {
	runnerMu.Lock()
	defer runnerMu.Unlock()
	return Runner
}

func setRunner(r CommandRunner) {
	runnerMu.Lock()
	defer runnerMu.Unlock()
	Runner = r
}

// SetRunnerForTest replaces Runner with r and returns a function that restores
// the original. Callers should defer (or t.Cleanup) the returned function.
// This avoids cross-package reassignment of the Runner variable.
func SetRunnerForTest(r CommandRunner) func() {
	orig := getRunner()
	setRunner(r)
	return func() { setRunner(orig) }
}

// FindBinary locates the `pulumi` executable using the system PATH and returns its full path.
// If the executable cannot be found, it returns the error produced by NotFoundError.
func FindBinary() (string, error) {
	path, err := exec.LookPath("pulumi")
	if err != nil {
		return "", NotFoundError()
	}
	return path, nil
}

// FindProject searches upward from dir for a Pulumi project file (Pulumi.yaml or Pulumi.yml)
// and returns the directory that contains the first match. If no project file is found when
// walking up to the filesystem root, it returns an empty string and ErrNoProject.
func FindProject(dir string) (string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}

	current := absDir
	for {
		for _, name := range []string{"Pulumi.yaml", "Pulumi.yml"} {
			candidate := filepath.Join(current, name)
			if _, statErr := os.Stat(candidate); statErr == nil {
				return current, nil
			}
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root.
			return "", ErrNoProject
		}
		current = parent
	}
}

// GetCurrentStack returns the name of the currently selected Pulumi stack for the project
// at projectDir by running `pulumi stack ls --json`. Returns NoCurrentStackError if no
// stack is marked current, or an error if the command fails or output cannot be parsed.
func GetCurrentStack(ctx context.Context, projectDir string) (string, error) {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "pulumi").
		Str("operation", "get_current_stack").
		Str("project_dir", projectDir).
		Msg("listing Pulumi stacks")

	stdout, stderr, err := getRunner().Run(ctx, projectDir, "pulumi", nil, "stack", "ls", "--json")
	if err != nil {
		return "", fmt.Errorf("running pulumi stack ls: %w: %s", err, strings.TrimSpace(string(stderr)))
	}

	var stacks []StackInfo
	if unmarshalErr := json.Unmarshal(stdout, &stacks); unmarshalErr != nil {
		return "", fmt.Errorf("parsing pulumi stack ls output: %w", unmarshalErr)
	}

	var available []string
	for _, s := range stacks {
		available = append(available, s.Name)
		if s.Current {
			log.Debug().
				Ctx(ctx).
				Str("component", "pulumi").
				Str("stack", s.Name).
				Msg("found current stack")
			return s.Name, nil
		}
	}

	return "", NoCurrentStackError(available)
}

// pulumiCmdConfig holds the configuration for running a Pulumi CLI command.
type pulumiCmdConfig struct {
	projectDir     string
	stack          string
	timeout        time.Duration
	defaultTimeout time.Duration
	args           []string
	extraEnv       []string // injected into subprocess env only, never os.Setenv
	operation      string
	logMessage     string
	wrapErr        func(string) error
}

// runPulumiCommand executes a Pulumi CLI invocation described by cfg and returns the
// command's stdout. It enforces a context deadline, emits structured logs, and maps
// stderr to a wrapped error via cfg.wrapErr when the command fails.
func runPulumiCommand(ctx context.Context, cfg pulumiCmdConfig) ([]byte, error) {
	log := logging.FromContext(ctx)

	timeout := cfg.timeout
	if timeout == 0 {
		timeout = cfg.defaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := append([]string{}, cfg.args...)
	if cfg.stack != "" {
		args = append(args, "--stack", cfg.stack)
	}

	log.Info().
		Ctx(ctx).
		Str("component", "pulumi").
		Str("operation", cfg.operation).
		Str("project_dir", cfg.projectDir).
		Str("stack", cfg.stack).
		Msg(cfg.logMessage)

	stdout, stderr, err := getRunner().Run(ctx, cfg.projectDir, "pulumi", cfg.extraEnv, args...)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf(
				"pulumi %s timed out after %s: %w", cfg.operation, timeout, ctx.Err(),
			)
		}
		if ctx.Err() == context.Canceled {
			return nil, fmt.Errorf("pulumi %s canceled: %w", cfg.operation, ctx.Err())
		}
		return nil, cfg.wrapErr(string(stderr))
	}

	log.Debug().
		Ctx(ctx).
		Str("component", "pulumi").
		Int("output_bytes", len(stdout)).
		Msgf("pulumi %s completed", cfg.operation)

	return stdout, nil
}

// Preview runs `pulumi preview --json` for the project at opts.ProjectDir and returns the
// raw JSON output bytes. Errors are wrapped by PreviewError; context cancellation and
// timeout produce descriptive errors.
func Preview(ctx context.Context, opts PreviewOptions) ([]byte, error) {
	var extraEnv []string
	if opts.Passphrase != nil {
		extraEnv = []string{"PULUMI_CONFIG_PASSPHRASE=" + *opts.Passphrase}
	}
	return runPulumiCommand(ctx, pulumiCmdConfig{
		projectDir:     opts.ProjectDir,
		stack:          opts.Stack,
		timeout:        opts.Timeout,
		defaultTimeout: DefaultPreviewTimeout,
		args:           []string{"preview", "--json"},
		extraEnv:       extraEnv,
		operation:      "preview",
		logMessage:     "running pulumi preview --json (this may take a moment)...",
		wrapErr:        PreviewError,
	})
}

// StackExport runs `pulumi stack export` for the project at opts.ProjectDir and returns the
// exported stack state as raw JSON bytes. Errors are wrapped by ExportError; context
// cancellation and timeout produce descriptive errors.
func StackExport(ctx context.Context, opts ExportOptions) ([]byte, error) {
	var extraEnv []string
	if opts.Passphrase != nil {
		extraEnv = []string{"PULUMI_CONFIG_PASSPHRASE=" + *opts.Passphrase}
	}
	return runPulumiCommand(ctx, pulumiCmdConfig{
		projectDir:     opts.ProjectDir,
		stack:          opts.Stack,
		timeout:        opts.Timeout,
		defaultTimeout: DefaultExportTimeout,
		args:           []string{"stack", "export"},
		extraEnv:       extraEnv,
		operation:      "stack export",
		logMessage:     "running pulumi stack export...",
		wrapErr:        ExportError,
	})
}
