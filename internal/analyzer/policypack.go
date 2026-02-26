package analyzer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"

	"github.com/rshade/finfocus/internal/logging"
)

const (
	// policyPackDirName is the subdirectory name for the policy pack.
	policyPackDirName = "analyzer"

	// policyPackBinaryName is the binary name Pulumi expects in the policy pack directory.
	policyPackBinaryName = "pulumi-analyzer-policy-finfocus"

	// pulumiPolicyFilename is the YAML configuration file for the policy pack.
	pulumiPolicyFilename = "PulumiPolicy.yaml"
)

// PolicyPackConfig represents the PulumiPolicy.yaml configuration.
type PolicyPackConfig struct {
	Name        string `yaml:"name"`
	Runtime     string `yaml:"runtime"`
	Description string `yaml:"description"`
}

// defaultPolicyPackConfig returns the default policy pack configuration.
func defaultPolicyPackConfig() PolicyPackConfig {
	return PolicyPackConfig{
		Name:        "finfocus",
		Runtime:     "finfocus",
		Description: "FinFocus cost estimation analyzer",
	}
}

// ResolvePolicyPackDir returns the policy pack directory path.
// Resolution precedence:
//  1. $FINFOCUS_HOME/analyzer/ if FINFOCUS_HOME is set
//  2. $HOME/.finfocus/analyzer/ (default)
func ResolvePolicyPackDir() (string, error) {
	if ffHome := os.Getenv("FINFOCUS_HOME"); ffHome != "" {
		return filepath.Join(ffHome, policyPackDirName), nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}

	return filepath.Join(homeDir, ".finfocus", policyPackDirName), nil
}

// WritePulumiPolicyYAML writes the PulumiPolicy.yaml configuration file to the given directory.
// The directory must already exist. The file is written with 0644 permissions.
func WritePulumiPolicyYAML(dir string) error {
	cfg := defaultPolicyPackConfig()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling policy pack config: %w", err)
	}

	path := filepath.Join(dir, pulumiPolicyFilename)
	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
		return fmt.Errorf("writing %s: %w", pulumiPolicyFilename, writeErr)
	}

	return nil
}

// SetupPolicyPack creates the policy pack directory structure with PulumiPolicy.yaml
// and a binary reference (symlink on Unix, copy on Windows). It is idempotent: calling
// it when the directory already exists will re-write the YAML and re-create the binary reference.
//
// Parameters:
//   - ctx: context for logging
//   - execPath: path to the finfocus binary to link/copy
//
// Returns the policy pack directory path, the method used ("symlink" or "copy"), and any error.
func SetupPolicyPack(ctx context.Context, execPath string) (string, string, error) {
	log := logging.FromContext(ctx)

	ppDir, err := ResolvePolicyPackDir()
	if err != nil {
		return "", "", fmt.Errorf("resolving policy pack directory: %w", err)
	}

	// Create the directory (idempotent)
	if mkErr := os.MkdirAll(ppDir, 0o750); mkErr != nil {
		return "", "", fmt.Errorf("creating policy pack directory %s: %w", ppDir, mkErr)
	}

	// Write PulumiPolicy.yaml
	if yamlErr := WritePulumiPolicyYAML(ppDir); yamlErr != nil {
		return "", "", fmt.Errorf("writing policy pack YAML: %w", yamlErr)
	}

	// Create binary reference (symlink or copy)
	binaryPath := filepath.Join(ppDir, policyPackBinaryName)

	// Remove existing binary reference if present (idempotent re-setup)
	if _, statErr := os.Lstat(binaryPath); statErr == nil {
		if removeErr := os.Remove(binaryPath); removeErr != nil {
			return "", "", fmt.Errorf("removing existing binary reference: %w", removeErr)
		}
	}

	method, linkErr := linkOrCopy(ctx, execPath, binaryPath)
	if linkErr != nil {
		return "", "", fmt.Errorf("creating binary reference: %w", linkErr)
	}

	log.Info().
		Ctx(ctx).
		Str("component", "analyzer").
		Str("operation", "setup_policy_pack").
		Str("dir", ppDir).
		Str("method", method).
		Msg("policy pack set up")

	return ppDir, method, nil
}

// copyBinary creates a copy of the source binary at the destination path.
// This is used on Windows where symlinks may not be available.
func copyBinary(src, dst string) error {
	return copyFile(src, dst)
}

// policyPackBinaryPath returns the full path to the binary inside a policy pack directory.
func policyPackBinaryPath(dir string) string {
	name := policyPackBinaryName
	if runtime.GOOS == goosWindows {
		name += ".exe"
	}
	return filepath.Join(dir, name)
}
