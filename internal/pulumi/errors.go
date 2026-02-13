// Package pulumi provides functions for detecting and executing the Pulumi CLI
// binary to support automatic cost analysis from Pulumi projects.
package pulumi

import (
	"errors"
	"fmt"
	"strings"
)

// pulumiInstallURL is the URL to install the Pulumi CLI.
const pulumiInstallURL = "https://www.pulumi.com/docs/install/"

// Sentinel errors for structured error handling across the Pulumi integration.
var (
	// ErrPulumiNotFound indicates the pulumi CLI binary is not in PATH.
	ErrPulumiNotFound = fmt.Errorf(
		"pulumi CLI not found in PATH; install from %s or provide --pulumi-json", pulumiInstallURL)

	// ErrNoProject indicates no Pulumi.yaml or Pulumi.yml was found.
	ErrNoProject = errors.New(
		"no Pulumi project found in current or parent directories; use --pulumi-json to provide input directly")

	// ErrNoCurrentStack indicates no stack is marked as current.
	ErrNoCurrentStack = errors.New("no active Pulumi stack")

	// ErrPreviewFailed indicates pulumi preview returned a non-zero exit code.
	ErrPreviewFailed = errors.New("pulumi preview failed")

	// ErrExportFailed indicates pulumi stack export returned a non-zero exit code.
	ErrExportFailed = errors.New("pulumi stack export failed")
)

// NoCurrentStackError creates an error with available stack names listed.
func NoCurrentStackError(available []string) error {
	if len(available) == 0 {
		return fmt.Errorf("%w; no stacks found in this project", ErrNoCurrentStack)
	}
	return fmt.Errorf("%w; use --stack to specify one (available: %s)",
		ErrNoCurrentStack, strings.Join(available, ", "))
}

// PreviewError wraps ErrPreviewFailed with the stderr output from the Pulumi CLI.
func PreviewError(stderr string) error {
	return fmt.Errorf("%w: %s", ErrPreviewFailed, strings.TrimSpace(stderr))
}

// ExportError wraps ErrExportFailed with the stderr output from the Pulumi CLI.
func ExportError(stderr string) error {
	return fmt.Errorf("%w: %s", ErrExportFailed, strings.TrimSpace(stderr))
}
