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

// FindBinary locates the pulumi CLI binary in PATH.
// Returns the full path to the binary or ErrPulumiNotFound if not found.
func FindBinary() (string, error) {
	path, err := exec.LookPath("pulumi")
	if err != nil {
		return "", ErrPulumiNotFound
	}
	return path, nil
}

// FindProject walks up the directory tree from dir looking for Pulumi.yaml
// or Pulumi.yml. Returns the directory containing the project file, or
// ErrNoProject if no project file is found before reaching the filesystem root.
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

// GetCurrentStack runs pulumi stack ls --json in projectDir and returns
// the name of the currently active stack. If no stack is marked as current,
// returns ErrNoCurrentStack with the list of available stacks.
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

// runPulumiCommand executes a Pulumi CLI command with timeout, logging, and error handling.
func runPulumiCommand(ctx context.Context, cfg pulumiCmdConfig) ([]byte, error) {
	log := logging.FromContext(ctx)

	timeout := cfg.timeout
	if timeout == 0 {
		timeout = cfg.defaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := cfg.args
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
				"pulumi %s timed out after %s", cfg.operation, timeout,
			)
		}
		if ctx.Err() == context.Canceled {
			return nil, ctx.Err()
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

// Preview runs pulumi preview --json in the project directory and returns
// the raw JSON output bytes.
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

// StackExport runs pulumi stack export in the project directory and returns
// the raw JSON output bytes.
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
