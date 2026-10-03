// Package detect finds external CLIs that finfocus shells out to.
package detect

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// ErrPulumiMissing means the pulumi binary is not on PATH.
var ErrPulumiMissing = errors.New("pulumi CLI required. Install: https://www.pulumi.com/docs/install/")

const minPulumiVersion = "3.0.0"

// LookFunc resolves a binary name to a path.
type LookFunc func(name string) (string, error)

// RunFunc executes a binary and returns stdout.
type RunFunc func(ctx context.Context, name string, args ...string) ([]byte, error)

// Detect checks that pulumi is on PATH and is at least v3.0.0.
// It returns the binary path and the parsed version without a leading v.
func Detect(ctx context.Context, look LookFunc, run RunFunc) (string, string, error) {
	if look == nil {
		look = exec.LookPath
	}
	if run == nil {
		run = defaultRun
	}
	bin, err := look("pulumi")
	if err != nil {
		return "", "", ErrPulumiMissing
	}
	out, err := run(ctx, bin, "version")
	if err != nil {
		return "", "", fmt.Errorf("running pulumi version: %w", err)
	}
	version, err := ParsePulumiVersion(string(out))
	if err != nil {
		return "", "", err
	}
	return bin, version, nil
}

// ParsePulumiVersion reads the first line of `pulumi version` and enforces the minimum.
func ParsePulumiVersion(output string) (string, error) {
	line := strings.TrimSpace(output)
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	line = strings.TrimPrefix(line, "v")
	parsed, err := semver.NewVersion(line)
	if err != nil {
		return "", fmt.Errorf("parsing pulumi version %q: %w", line, err)
	}
	minimum := semver.MustParse(minPulumiVersion)
	if parsed.LessThan(minimum) {
		return "", fmt.Errorf("pulumi CLI v%s or newer required (found %s)", minPulumiVersion, parsed.String())
	}
	return parsed.String(), nil
}

func defaultRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		if ok {
			return nil, fmt.Errorf("%s: %w: %s", name, err, bytesTrim(exitErr.Stderr))
		}
		return nil, err
	}
	return out, nil
}

func bytesTrim(raw []byte) string {
	return strings.TrimSpace(string(raw))
}
