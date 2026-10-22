package terraform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	planFileMode    = 0o600
	planDirMode     = 0o700
	minTofu         = "1.6.0"
	openTofuCommand = "tofu"
	versionParts    = 3
)

// RunPlan runs OpenTofu plan into a private temp file and returns
// the JSON from show. The temp directory is removed before return.
func RunPlan(ctx context.Context, dir string) ([]byte, error) {
	binary, err := findBinary()
	if err != nil {
		return nil, err
	}
	if versionErr := checkVersion(ctx, binary, dir); versionErr != nil {
		return nil, versionErr
	}
	tempDir, err := os.MkdirTemp("", "finfocus-tf-plan-")
	if err != nil {
		return nil, fmt.Errorf("creating plan directory: %w", err)
	}
	defer os.RemoveAll(tempDir)
	if chmodErr := os.Chmod(tempDir, planDirMode); chmodErr != nil {
		return nil, PlanFailure(fmt.Errorf("setting plan directory mode: %w", chmodErr))
	}
	planPath := filepath.Join(tempDir, "plan.bin")
	planArgs := []string{"plan", "-input=false", "-lock=false", "-no-color", "-out=" + planPath}
	if _, planErr := runCommand(ctx, binary, dir, planArgs...); planErr != nil {
		return nil, planErr
	}
	if chmodErr := os.Chmod(planPath, planFileMode); chmodErr != nil {
		return nil, PlanFailure(fmt.Errorf("setting plan file mode: %w", chmodErr))
	}
	stdout, err := runCommand(ctx, binary, dir, "show", "-json", "-no-color", planPath)
	if err != nil {
		return nil, err
	}
	return stdout, nil
}

func findBinary() (string, error) {
	if path, err := exec.LookPath(openTofuCommand); err == nil {
		return path, nil
	}
	return "", binaryMissing()
}

func checkVersion(ctx context.Context, binary, dir string) error {
	stdout, err := runCommand(ctx, binary, dir, "version", "-json")
	if err != nil {
		return err
	}
	var info struct {
		TerraformVersion string `json:"terraform_version"`
	}
	if jsonErr := json.Unmarshal(stdout, &info); jsonErr != nil {
		return PlanFailure(fmt.Errorf("reading OpenTofu version: %w", jsonErr))
	}
	older, ok := versionOlder(info.TerraformVersion, minTofu)
	if !ok {
		return PlanFailure(fmt.Errorf("invalid OpenTofu version %q", info.TerraformVersion))
	}
	if older {
		return oldBinaryHint()
	}
	return nil
}

func runCommand(ctx context.Context, binary, dir string, args ...string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	cmd := newCommand(ctx, binary, dir, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, classifyStderr(stderr.String())
	}
	return stdout.Bytes(), nil
}

func versionOlder(version, minimum string) (bool, bool) {
	left, ok := parseVersion(version)
	if !ok {
		return false, false
	}
	right, ok := parseVersion(minimum)
	if !ok {
		return false, false
	}
	for i := range versionParts {
		if left[i] != right[i] {
			return left[i] < right[i], true
		}
	}
	return false, true
}

func parseVersion(version string) ([3]int, bool) {
	version, _, _ = strings.Cut(version, "-")
	version, _, _ = strings.Cut(version, "+")
	parts := strings.Split(version, ".")
	var parsed [3]int
	const minVersionSegments = 2
	if len(parts) < minVersionSegments {
		return parsed, false
	}
	for i := range min(versionParts, len(parts)) {
		number, err := strconv.Atoi(parts[i])
		if err != nil {
			return parsed, false
		}
		parsed[i] = number
	}
	return parsed, true
}
