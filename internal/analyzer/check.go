package analyzer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/yaml.v3"
)

const (
	checkStatusPass = "pass"
	checkStatusFail = "fail"
	checkStatusSkip = "skip"

	checkReportSize = 4
	checkTimeout    = 5 * time.Second
)

// CheckResult represents the outcome of a single analyzer setup verification.
type CheckResult struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

// CheckReport aggregates all check results for analyzer setup verification.
type CheckReport struct {
	Checks  []CheckResult `json:"checks"`
	AllPass bool          `json:"all_pass"`
}

// RunChecks executes analyzer setup checks in order and returns a report.
// Callers must provide a non-nil context.
func RunChecks(ctx context.Context) (*CheckReport, error) {
	if ctx == nil {
		return nil, errors.New("RunChecks requires a non-nil context")
	}

	results := make([]CheckResult, 0, checkReportSize)

	policyDirResult, policyPackDir := checkPolicyPackDir()
	results = append(results, policyDirResult)
	if policyDirResult.Status != checkStatusPass {
		results = append(results,
			skippedCheckResult("pulumi_policy_yaml", "PulumiPolicy.yaml",
				"skipped because policy pack directory check failed"),
			skippedCheckResult("binary_in_path", "Analyzer binary in PATH",
				"skipped because policy pack directory check failed"),
			skippedCheckResult("grpc_smoke_test", "gRPC smoke test",
				"skipped because policy pack directory check failed"),
		)
		return &CheckReport{Checks: results, AllPass: false}, nil
	}

	policyYAMLResult := checkPulumiPolicyYAML(policyPackDir)
	results = append(results, policyYAMLResult)
	if policyYAMLResult.Status != checkStatusPass {
		results = append(results,
			skippedCheckResult("binary_in_path", "Analyzer binary in PATH",
				"skipped because PulumiPolicy.yaml check failed"),
			skippedCheckResult("grpc_smoke_test", "gRPC smoke test",
				"skipped because PulumiPolicy.yaml check failed"),
		)
		return &CheckReport{Checks: results, AllPass: false}, nil
	}

	binaryResult := checkBinaryInPATH()
	results = append(results, binaryResult)
	if binaryResult.Status != checkStatusPass {
		results = append(results,
			skippedCheckResult("grpc_smoke_test", "gRPC smoke test",
				"skipped because PATH binary check failed"),
		)
		return &CheckReport{Checks: results, AllPass: false}, nil
	}

	grpcResult := checkGRPCSmokeTest(ctx)
	results = append(results, grpcResult)

	return &CheckReport{
		Checks:  results,
		AllPass: grpcResult.Status == checkStatusPass,
	}, nil
}

func checkPolicyPackDir() (CheckResult, string) {
	dir, err := ResolvePolicyPackDir()
	if err != nil {
		return CheckResult{
			Name:        "policy_pack_dir",
			DisplayName: "Policy pack directory",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("failed to resolve policy pack directory: %v", err),
			Remediation: "Set FINFOCUS_HOME to a valid directory and run: finfocus analyzer install",
		}, ""
	}

	info, statErr := os.Stat(dir)
	if statErr != nil {
		return CheckResult{
			Name:        "policy_pack_dir",
			DisplayName: "Policy pack directory",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("policy pack directory not found: %s", dir),
			Remediation: "Run: finfocus analyzer install",
		}, ""
	}
	if !info.IsDir() {
		return CheckResult{
			Name:        "policy_pack_dir",
			DisplayName: "Policy pack directory",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("policy pack path is not a directory: %s", dir),
			Remediation: "Remove the file and run: finfocus analyzer install",
		}, ""
	}

	return CheckResult{
		Name:        "policy_pack_dir",
		DisplayName: "Policy pack directory",
		Status:      checkStatusPass,
		Message:     fmt.Sprintf("directory exists: %s", dir),
	}, dir
}

func checkPulumiPolicyYAML(dir string) CheckResult {
	yamlPath := filepath.Join(dir, pulumiPolicyFilename)

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return CheckResult{
			Name:        "pulumi_policy_yaml",
			DisplayName: "PulumiPolicy.yaml",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("failed to read %s: %v", yamlPath, err),
			Remediation: "Re-run: finfocus analyzer install",
		}
	}

	var cfg PolicyPackConfig
	if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr != nil {
		return CheckResult{
			Name:        "pulumi_policy_yaml",
			DisplayName: "PulumiPolicy.yaml",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("invalid YAML in %s: %v", yamlPath, unmarshalErr),
			Remediation: "Re-run: finfocus analyzer install",
		}
	}

	if cfg.Runtime != "finfocus" {
		return CheckResult{
			Name:        "pulumi_policy_yaml",
			DisplayName: "PulumiPolicy.yaml",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("invalid runtime %q in %s", cfg.Runtime, yamlPath),
			Remediation: "Set runtime: finfocus in PulumiPolicy.yaml or run: finfocus analyzer install",
		}
	}

	return CheckResult{
		Name:        "pulumi_policy_yaml",
		DisplayName: "PulumiPolicy.yaml",
		Status:      checkStatusPass,
		Message:     fmt.Sprintf("%s is valid", pulumiPolicyFilename),
	}
}

func checkBinaryInPATH() CheckResult {
	path, err := exec.LookPath(policyPackBinaryName)
	if err != nil {
		policyPackDir, resolveErr := ResolvePolicyPackDir()
		if resolveErr != nil {
			policyPackDir = "~/.finfocus/analyzer"
		}
		var remediation string
		if runtime.GOOS == goosWindows {
			remediation = fmt.Sprintf(
				"Add the policy pack directory to PATH:\n"+
					"  PowerShell:  $env:PATH = \"%s;$env:PATH\"\n"+
					"  Permanent:   [Environment]::SetEnvironmentVariable('PATH', '%s;' + "+
					"[Environment]::GetEnvironmentVariable('PATH', 'User'), 'User')",
				policyPackDir, policyPackDir)
		} else {
			remediation = fmt.Sprintf("Add the policy pack directory to PATH:\nexport PATH=\"%s:$PATH\"",
				policyPackDir)
		}
		return CheckResult{
			Name:        "binary_in_path",
			DisplayName: "Analyzer binary in PATH",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("binary %q is not in PATH", policyPackBinaryName),
			Remediation: remediation,
		}
	}

	return CheckResult{
		Name:        "binary_in_path",
		DisplayName: "Analyzer binary in PATH",
		Status:      checkStatusPass,
		Message:     fmt.Sprintf("binary found: %s", path),
	}
}

func checkGRPCSmokeTest(ctx context.Context) CheckResult {
	execPath, err := os.Executable()
	if err != nil {
		return CheckResult{
			Name:        "grpc_smoke_test",
			DisplayName: "gRPC smoke test",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("failed to resolve executable path: %v", err),
			Remediation: "Rebuild finfocus and retry",
		}
	}

	if resolvedPath, resolveErr := filepath.EvalSymlinks(execPath); resolveErr == nil {
		execPath = resolvedPath
	}

	smokeCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	cmd := exec.CommandContext(smokeCtx, execPath, "analyzer", "serve")
	stdout, pipeErr := cmd.StdoutPipe()
	if pipeErr != nil {
		return CheckResult{
			Name:        "grpc_smoke_test",
			DisplayName: "gRPC smoke test",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("failed to capture analyzer serve stdout: %v", pipeErr),
			Remediation: "Retry the command and verify local process execution permissions",
		}
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if startErr := cmd.Start(); startErr != nil {
		return CheckResult{
			Name:        "grpc_smoke_test",
			DisplayName: "gRPC smoke test",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("failed to start analyzer serve: %v", startErr),
			Remediation: "Run `finfocus analyzer serve` manually to inspect startup errors",
		}
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()
	defer stopCommand(cancel, cmd, waitDone)

	port, portErr := readServePort(smokeCtx, stdout)
	if portErr != nil {
		// Stop the subprocess and wait for cmd.Wait() to complete before reading
		// stderr. The exec goroutine writes to the stderr buffer concurrently
		// until Wait returns; reading it earlier triggers a data race.
		stopCommand(cancel, cmd, waitDone)

		msg := fmt.Sprintf("failed to read analyzer serve port: %v", portErr)
		if stderrStr := strings.TrimSpace(stderr.String()); stderrStr != "" {
			msg = fmt.Sprintf("%s (stderr: %s)", msg, firstLine(stderrStr))
		}
		return CheckResult{
			Name:        "grpc_smoke_test",
			DisplayName: "gRPC smoke test",
			Status:      checkStatusFail,
			Message:     msg,
			Remediation: "Run `finfocus analyzer serve` manually to verify startup output",
		}
	}

	address := fmt.Sprintf("127.0.0.1:%d", port)
	conn, connErr := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if connErr != nil {
		return CheckResult{
			Name:        "grpc_smoke_test",
			DisplayName: "gRPC smoke test",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("failed to create gRPC client: %v", connErr),
			Remediation: "Verify local loopback networking and retry",
		}
	}
	defer func() {
		_ = conn.Close()
	}()

	client := pulumirpc.NewAnalyzerClient(conn)

	if _, rpcErr := client.GetAnalyzerInfo(smokeCtx, &emptypb.Empty{}); rpcErr != nil {
		return CheckResult{
			Name:        "grpc_smoke_test",
			DisplayName: "gRPC smoke test",
			Status:      checkStatusFail,
			Message:     fmt.Sprintf("GetAnalyzerInfo call failed: %v", rpcErr),
			Remediation: "Ensure `finfocus analyzer serve` starts cleanly and is reachable on localhost",
		}
	}

	return CheckResult{
		Name:        "grpc_smoke_test",
		DisplayName: "gRPC smoke test",
		Status:      checkStatusPass,
		Message:     "analyzer serve responded to GetAnalyzerInfo",
	}
}

func skippedCheckResult(name, displayName, reason string) CheckResult {
	return CheckResult{
		Name:        name,
		DisplayName: displayName,
		Status:      checkStatusSkip,
		Message:     reason,
	}
}

func readServePort(ctx context.Context, reader io.Reader) (int, error) {
	lineCh := make(chan string, 1)
	errCh := make(chan error, 1)

	go func() {
		bufReader := bufio.NewReader(reader)
		line, err := bufReader.ReadString('\n')
		if err != nil && len(line) == 0 {
			errCh <- err
			return
		}
		lineCh <- strings.TrimSpace(line)
	}()

	select {
	case <-ctx.Done():
		return 0, fmt.Errorf("timed out waiting for analyzer port: %w", ctx.Err())
	case err := <-errCh:
		return 0, err
	case line := <-lineCh:
		port, err := strconv.Atoi(line)
		if err != nil {
			return 0, fmt.Errorf("unexpected port output %q: %w", line, err)
		}
		if port <= 0 {
			return 0, fmt.Errorf("invalid port: %d", port)
		}
		return port, nil
	}
}

// stopCommand cancels the context and kills the subprocess. The 1-second
// time.After provides a brief grace period for cmd.Wait() to return after
// SIGKILL; child processes are expected to exit promptly, so a longer timeout
// is unnecessary. The select prevents indefinite blocking if Wait never returns.
func stopCommand(cancel context.CancelFunc, cmd *exec.Cmd, waitDone <-chan error) {
	cancel()
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}

	select {
	case <-waitDone:
	case <-time.After(time.Second):
	}
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}
