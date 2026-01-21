//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestContext manages the lifecycle of a single E2E test run.
type TestContext struct {
	ID          string
	StackName   string
	T           *testing.T
	StartTime   time.Time
	Region      string
	ProjectDir  string   // Path to the Pulumi project directory
	WorkDir     string   // Working directory (copy of project)
	PreviewJSON string   // Path to the preview JSON file
	AWSEnv      []string // AWS credentials from SDK
	Cleanup     *CleanupManager
}

// NewTestContext creates a new TestContext with a unique ID.
func NewTestContext(t *testing.T, prefix string) *TestContext {
	stackName := GenerateStackName(prefix)
	region := os.Getenv("E2E_REGION")
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	// Get timeout from environment or use default
	timeout := 60 * time.Minute
	if timeoutStr := os.Getenv("E2E_TIMEOUT_MINS"); timeoutStr != "" {
		if mins, err := time.ParseDuration(timeoutStr + "m"); err == nil {
			timeout = mins
		}
	}

	return &TestContext{
		ID:        stackName,
		StackName: stackName,
		T:         t,
		StartTime: time.Now(),
		Region:    region,
		Cleanup:   NewCleanupManager(t, timeout),
	}
}

// SetupAWSEnv loads AWS credentials using the SDK and populates AWSEnv.
func (tc *TestContext) SetupAWSEnv(ctx context.Context) {
	awsEnv, err := GetAWSCredentialsEnv(ctx)
	if err != nil {
		tc.T.Logf("Warning: failed to load AWS credentials from SDK: %v", err)
	} else {
		tc.AWSEnv = awsEnv
		tc.T.Log("Loaded AWS credentials from SDK")
	}
}

// SetupProject initializes a Pulumi project from the given project directory.
// The steps are:
// 1. Create temp directory
// 2. Run go mod tidy (for Go projects only)
// 3. pulumi stack init
// 4. pulumi config set aws:region
// 5. pulumi preview --json > preview.json
// 6. pulumi up -y
func (tc *TestContext) SetupProject(ctx context.Context, projectPath string) error {
	tc.T.Logf("Setting up project from %s with stack %s (region: %s)", projectPath, tc.StackName, tc.Region)

	// Load AWS credentials using SDK
	tc.SetupAWSEnv(ctx)

	// Get absolute path to project
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	tc.ProjectDir = absProjectPath

	// Clean up any existing work directory
	if tc.WorkDir != "" {
		if err := os.RemoveAll(tc.WorkDir); err != nil {
			tc.T.Logf("Warning: failed to clean up previous work dir: %v", err)
		}
	}

	// Create a temp directory and copy the project there
	workDir, err := os.MkdirTemp("", "pulumi-e2e-")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	tc.WorkDir = workDir
	tc.PreviewJSON = filepath.Join(workDir, "preview.json")

	// Ensure cleanup on failure
	setupSucceeded := false
	defer func() {
		if !setupSucceeded && tc.WorkDir != "" {
			if cleanupErr := os.RemoveAll(tc.WorkDir); cleanupErr != nil {
				tc.T.Logf("Warning: failed to clean up temp dir on setup failure: %v", cleanupErr)
			}
			tc.WorkDir = "" // Prevent double cleanup
		}
	}()

	// Copy project files to work directory
	tc.T.Log("Copying project files...")
	if err := tc.CopyDir(absProjectPath, workDir); err != nil {
		return fmt.Errorf("failed to copy project: %w", err)
	}

	// Check if this is a Go project (has go.mod) - skip go mod tidy for YAML projects
	goModPath := filepath.Join(workDir, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		tc.T.Log("Running go mod tidy (Go project detected)...")
		if err := tc.runCmd(ctx, workDir, "go", "mod", "tidy"); err != nil {
			return fmt.Errorf("go mod tidy failed: %w", err)
		}
	} else {
		tc.T.Log("Skipping go mod tidy (YAML/non-Go project)")
	}

	// Initialize Pulumi stack with local backend
	tc.T.Logf("Initializing Pulumi stack %s...", tc.StackName)
	stateDir := filepath.Join(workDir, ".pulumi-state")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("failed to create state dir: %w", err)
	}

	// pulumi login to local backend
	if err := tc.runCmdWithEnv(ctx, workDir,
		[]string{"PULUMI_BACKEND_URL=file://" + stateDir},
		"pulumi", "login", "--local"); err != nil {
		return fmt.Errorf("pulumi login failed: %w", err)
	}

	// pulumi stack init
	if err := tc.runCmdWithEnv(ctx, workDir,
		[]string{"PULUMI_BACKEND_URL=file://" + stateDir},
		"pulumi", "stack", "init", tc.StackName); err != nil {
		return fmt.Errorf("pulumi stack init failed: %w", err)
	}

	// Set AWS region configuration
	tc.T.Logf("Setting AWS region to %s...", tc.Region)
	if err := tc.runCmdWithEnv(ctx, workDir,
		[]string{"PULUMI_BACKEND_URL=file://" + stateDir},
		"pulumi", "config", "set", "aws:region", tc.Region); err != nil {
		return fmt.Errorf("pulumi config set failed: %w", err)
	}

	// Run pulumi preview --json and save output
	tc.T.Log("Running pulumi preview --json...")
	previewOutput, err := tc.runCmdOutput(ctx, workDir,
		[]string{"PULUMI_BACKEND_URL=file://" + stateDir},
		"pulumi", "preview", "--json")
	if err != nil {
		return fmt.Errorf("pulumi preview --json failed: %w", err)
	}
	if err := os.WriteFile(tc.PreviewJSON, previewOutput, 0644); err != nil {
		return fmt.Errorf("failed to write preview JSON: %w", err)
	}
	tc.T.Logf("Preview JSON saved to %s (%d bytes)", tc.PreviewJSON, len(previewOutput))
	if len(previewOutput) > 1000 {
		tc.T.Logf("Preview JSON head:\n%s", string(previewOutput[:1000]))
	} else {
		tc.T.Logf("Preview JSON content:\n%s", string(previewOutput))
	}

	// Run pulumi up
	tc.T.Log("Running pulumi up...")
	if err := tc.runCmdWithEnv(ctx, workDir,
		[]string{"PULUMI_BACKEND_URL=file://" + stateDir},
		"pulumi", "up", "-y"); err != nil {
		return fmt.Errorf("pulumi up failed: %w", err)
	}

	tc.T.Logf("Stack %s deployed successfully", tc.StackName)
	setupSucceeded = true
	return nil
}

// RunFinFocus executes the finfocus CLI and returns the calculated cost.
func (tc *TestContext) RunFinFocus(ctx context.Context) (float64, error) {
	// Find the finfocus binary
	binaryPath := findFinFocusBinary()
	if binaryPath == "" {
		return 0, fmt.Errorf("finfocus binary not found")
	}

	tc.T.Logf("Running finfocus at %s with preview JSON %s", binaryPath, tc.PreviewJSON)

	// Run finfocus cost projected --pulumi-json <preview.json> --output json
	cmd := exec.CommandContext(ctx, binaryPath, "cost", "projected", "--pulumi-json", tc.PreviewJSON, "--output", "json")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	tc.T.Logf("finfocus stderr:\n%s", stderr.String())
	if err != nil {
		return 0, fmt.Errorf("finfocus failed: %w (stderr: %s)", err, stderr.String())
	}

	tc.T.Logf("finfocus output: %s", stdout.String())

	// Parse the JSON output to extract total cost
	var result struct {
		Summary struct {
			TotalMonthly float64 `json:"totalMonthly"`
			TotalHourly  float64 `json:"totalHourly"`
		} `json:"summary"`
		Resources []struct {
			ResourceType string  `json:"resourceType"`
			Monthly      float64 `json:"monthly"`
		} `json:"resources"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		// Try parsing as NDJSON (one JSON object per line)
		lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
		var totalCost float64
		for _, line := range lines {
			var resource struct {
				MonthlyCost string `json:"monthly_cost"`
			}
			if json.Unmarshal([]byte(line), &resource) == nil {
				if cost, err := strconv.ParseFloat(strings.TrimPrefix(resource.MonthlyCost, "$"), 64); err == nil {
					totalCost += cost
				}
			}
		}
		if totalCost > 0 {
			return totalCost, nil
		}
		return 0, fmt.Errorf("failed to parse finfocus output: %w", err)
	}

	return result.Summary.TotalMonthly, nil
}

// Teardown ensures resources are cleaned up.
func (tc *TestContext) Teardown(ctx context.Context) {
	// Capture interrupt signals to ensure cleanup happens
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	cleanupDone := make(chan struct{})

	go func() {
		select {
		case <-c:
			tc.T.Log("Interrupt received, cleaning up...")
			tc.performCleanup(ctx)
			os.Exit(1)
		case <-cleanupDone:
			return
		}
	}()

	// Ensure cleanup happens using defer in case of panic
	defer func() {
		close(cleanupDone)
		signal.Stop(c)
	}()

	tc.performCleanup(ctx)
}

// performCleanup runs pulumi destroy and cleans up resources.
func (tc *TestContext) performCleanup(ctx context.Context) {
	if tc.WorkDir == "" {
		return
	}

	tc.T.Logf("Starting cleanup for stack %s (timeout: %v)", tc.StackName, tc.Cleanup.Timeout)

	stateDir := filepath.Join(tc.WorkDir, ".pulumi-state")

	// Run pulumi destroy
	tc.T.Log("Running pulumi destroy...")
	if err := tc.runCmdWithEnv(ctx, tc.WorkDir,
		[]string{"PULUMI_BACKEND_URL=file://" + stateDir},
		"pulumi", "destroy", "-y"); err != nil {
		tc.T.Logf("Warning: pulumi destroy failed: %v", err)
	}

	// Remove the stack
	tc.T.Log("Removing Pulumi stack...")
	if err := tc.runCmdWithEnv(ctx, tc.WorkDir,
		[]string{"PULUMI_BACKEND_URL=file://" + stateDir},
		"pulumi", "stack", "rm", "-y", "--force"); err != nil {
		tc.T.Logf("Warning: pulumi stack rm failed: %v", err)
	}

	// Clean up temp directory
	if err := os.RemoveAll(tc.WorkDir); err != nil {
		tc.T.Logf("Warning: failed to remove work dir: %v", err)
	}

	tc.T.Log("Cleanup successful")
}

// Helper methods for running commands

func (tc *TestContext) runCmd(ctx context.Context, dir string, name string, args ...string) error {
	return tc.runCmdWithEnv(ctx, dir, nil, name, args...)
}

func (tc *TestContext) runCmdWithEnv(ctx context.Context, dir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ() // Start with full current environment
	for _, e := range tc.AWSEnv {
		cmd.Env = append(cmd.Env, e)
	}
	for _, e := range env {
		cmd.Env = append(cmd.Env, e)
	}

	// Ensure passphrase from current environment is preserved
	if passphrase := os.Getenv("PULUMI_CONFIG_PASSPHRASE"); passphrase != "" {
		cmd.Env = append(cmd.Env, "PULUMI_CONFIG_PASSPHRASE="+passphrase)
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = os.Stdout // Show progress

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v failed: %w\nstderr: %s", name, args, err, stderr.String())
	}
	return nil
}

func (tc *TestContext) runCmdOutput(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	cmd.Env = os.Environ() // Start with full current environment
	for _, e := range tc.AWSEnv {
		cmd.Env = append(cmd.Env, e)
	}
	for _, e := range env {
		cmd.Env = append(cmd.Env, e)
	}

	// Ensure passphrase from env if not already in env slice
	passphrase := os.Getenv("PULUMI_CONFIG_PASSPHRASE")
	if passphrase != "" {
		cmd.Env = append(cmd.Env, "PULUMI_CONFIG_PASSPHRASE="+passphrase)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.Bytes(), fmt.Errorf("%s %v failed: %w\nstdout: %s\nstderr: %s", name, args, err, stdout.String(), stderr.String())
	}

	if stderr.Len() > 0 {
		tc.T.Logf("%s %v stderr:\n%s", name, args, stderr.String())
	}

	return stdout.Bytes(), nil
}

func (tc *TestContext) CopyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			srcInfo, err := entry.Info()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dstPath, srcInfo.Mode()); err != nil {
				return err
			}
			if err := tc.CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			srcInfo, err := entry.Info()
			if err != nil {
				return err
			}
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, srcInfo.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}
