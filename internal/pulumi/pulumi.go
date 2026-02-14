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
}

// ExportOptions configures a Pulumi stack export command execution.
type ExportOptions struct {
	ProjectDir string        // Directory containing Pulumi.yaml.
	Stack      string        // Specific stack name (empty = current).
	Timeout    time.Duration // Max execution time (default: 60 seconds).
}

// StackInfo represents a single stack from pulumi stack ls --json.
type StackInfo struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
	URL     string `json:"url"`
}

// CommandRunner executes an external command and returns its stdout, stderr, and error.
// This interface enables testing without spawning real subprocesses.
type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) (stdout []byte, stderr []byte, err error)
}

// execRunner is the default CommandRunner that uses exec.CommandContext.
type execRunner struct{}

func (r *execRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// Runner is the package-level CommandRunner. Replace in tests with a mock.
var Runner CommandRunner = &execRunner{} //nolint:gochecknoglobals // Required for test injection

// FindBinary returns the full path to the pulumi executable by searching the system PATH.
// FindBinary locates the `pulumi` executable using the system PATH and returns its full path.
// If the executable cannot be found, it returns the error produced by NotFoundError, which includes
// installation instructions.
func FindBinary() (string, error) {
	path, err := exec.LookPath("pulumi")
	if err != nil {
		return "", NotFoundError()
	}
	return path, nil
}

// FindProject searches upward from dir for a Pulumi project file (Pulumi.yaml or Pulumi.yml)
// and returns the directory that contains the first match. Relative paths are resolved to
// FindProject returns the nearest ancestor directory (including dir itself) that contains
// a Pulumi.yaml or Pulumi.yml file. The provided dir is first resolved to an absolute
// path; if that resolution fails an error wrapping the underlying failure is returned.
// If no project file is found when walking up to the filesystem root, it returns an empty
// string and ErrNoProject.
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

// GetCurrentStack returns the name of the active Pulumi stack for the project at projectDir
// by running `pulumi stack ls --json` and selecting the stack marked current. If no stack is
// GetCurrentStack returns the name of the currently selected Pulumi stack for the project
// located in projectDir. It runs `pulumi stack ls --json` in projectDir and parses the
// output to determine which stack is marked as current.
//
// The ctx is used to derive logging and to cancel the underlying command.
//
// Returns the current stack name on success. If no stack is marked current it returns
// NoCurrentStackError containing the list of available stack names. If the `pulumi`
// command fails or its output cannot be parsed, an error describing the failure is returned.
func GetCurrentStack(ctx context.Context, projectDir string) (string, error) {
	log := logging.FromContext(ctx)
	log.Debug().
		Ctx(ctx).
		Str("component", "pulumi").
		Str("operation", "get_current_stack").
		Str("project_dir", projectDir).
		Msg("listing Pulumi stacks")

	stdout, stderr, err := Runner.Run(ctx, projectDir, "pulumi", "stack", "ls", "--json")
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
	operation      string
	logMessage     string
	wrapErr        func(string) error
}

// runPulumiCommand executes a Pulumi CLI operation using the provided configuration,
// honoring the configured timeout, emitting structured logs, and mapping CLI stderr to
// a wrapped error when the command fails.
//
// The command is executed in cfg.projectDir with additional args from cfg.args and
// an optional --stack flag when cfg.stack is set. If cfg.timeout is zero the
// cfg.defaultTimeout is used instead. On success the command stdout is returned.
//
// Errors returned include:
//   - context.DeadlineExceeded when the operation exceeds the resolved timeout (wrapped
//     in a formatted error that names the operation and timeout duration),
//   - context.Canceled when the provided context is cancelled,
// runPulumiCommand executes a Pulumi CLI invocation described by cfg and returns the command's stdout.
//
// The provided cfg controls the working directory, stack selection, command arguments, and timeouts:
// - cfg.projectDir sets the command working directory.
// - cfg.args are the Pulumi subcommand and its arguments.
// - cfg.stack, if non-empty, is appended as `--stack <stack>`.
// - cfg.timeout overrides cfg.defaultTimeout when non-zero.
// - cfg.operation and cfg.logMessage are used for logging.
// - cfg.wrapErr converts command stderr into a package-specific error for non-timeout failures.
//
// The function enforces a context deadline equal to the chosen timeout. If the context deadline is exceeded
// or cancelled, an error describing the timeout or cancellation for the named operation is returned.
// For other failures, the error returned is the result of calling cfg.wrapErr with the command's stderr.
//
// On success the command's stdout bytes are returned and the error is nil.
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

	stdout, stderr, err := Runner.Run(ctx, cfg.projectDir, "pulumi", args...)
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

// Preview runs `pulumi preview --json` for the project specified by opts and returns the raw
// JSON output bytes. Errors are wrapped by PreviewError; context cancellation and timeouts
// Preview runs `pulumi preview --json` for the project located at opts.ProjectDir and the
// stack specified by opts.Stack. If opts.Stack is empty the current stack is used.
// opts.Timeout, when non-zero, overrides the default preview timeout.
//
// The function respects the provided context for cancellation and deadline propagation.
//
// Preview returns the raw stdout produced by the Pulumi CLI (the preview JSON) as a byte
// slice. It returns an error if the Pulumi command fails, if the context is canceled, or
// if the operation exceeds its timeout.
func Preview(ctx context.Context, opts PreviewOptions) ([]byte, error) {
	return runPulumiCommand(ctx, pulumiCmdConfig{
		projectDir:     opts.ProjectDir,
		stack:          opts.Stack,
		timeout:        opts.Timeout,
		defaultTimeout: DefaultPreviewTimeout,
		args:           []string{"preview", "--json"},
		operation:      "preview",
		logMessage:     "running pulumi preview --json (this may take a moment)...",
		wrapErr:        PreviewError,
	})
}

// StackExport runs `pulumi stack export` for the project at opts.ProjectDir and returns the
// exported stack state as raw JSON bytes. Errors are wrapped by ExportError; context
// StackExport exports the state of a Pulumi stack to JSON using the local Pulumi CLI.
// 
// The opts parameter controls the project directory, target stack (empty selects the current stack),
// and an optional per-operation timeout. It returns the raw JSON bytes produced by `pulumi stack export`.
// An error is returned if the Pulumi CLI cannot be found or fails, if the operation times out, or if the
// provided context is cancelled.
func StackExport(ctx context.Context, opts ExportOptions) ([]byte, error) {
	return runPulumiCommand(ctx, pulumiCmdConfig{
		projectDir:     opts.ProjectDir,
		stack:          opts.Stack,
		timeout:        opts.Timeout,
		defaultTimeout: DefaultExportTimeout,
		args:           []string{"stack", "export"},
		operation:      "stack export",
		logMessage:     "running pulumi stack export...",
		wrapErr:        ExportError,
	})
}