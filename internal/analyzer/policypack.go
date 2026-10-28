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

	// policyPackBinaryName is the legacy analyzer entry Pulumi starts before 3.270.0.
	policyPackBinaryName = "pulumi-analyzer-policy-finfocus"

	// pulumiPolicyFilename is the YAML configuration file for the policy pack.
	pulumiPolicyFilename = "PulumiPolicy.yaml"
)

// LanguagePluginBinaryName is the policy pack entry Pulumi 3.270.0 and later
// starts as the language host for a finfocus policy pack.
const LanguagePluginBinaryName = "pulumi-language-finfocus"

// policyPackEntryNames lists pack entries, legacy analyzer first and the
// language plugin second. SetupPolicyPack creates every name.
func policyPackEntryNames() []string {
	return []string{policyPackBinaryName, LanguagePluginBinaryName}
}

// policyPackEntryName appends the Windows executable suffix. Other operating
// systems keep the name unchanged.
func policyPackEntryName(name, goos string) string {
	if goos == goosWindows {
		return name + ".exe"
	}
	return name
}

// PolicyPackEntryPath returns the path of a policy pack entry in dir,
// including the platform executable suffix.
func PolicyPackEntryPath(dir, name string) string {
	return filepath.Join(dir, policyPackEntryName(name, runtime.GOOS))
}

// PolicyPackConfig represents the PulumiPolicy.yaml configuration.
type PolicyPackConfig struct {
	Name        string `yaml:"name"`
	Runtime     string `yaml:"runtime"`
	Description string `yaml:"description"`
}

// defaultPolicyPackConfig returns the default policy pack configuration.
func defaultPolicyPackConfig() PolicyPackConfig {
	return PolicyPackConfig{
		Name:        policyPackName,
		Runtime:     policyPackName,
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
// The directory must already exist. The file is written with 0600 permissions.
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
// and both binary entries (symlink on Unix, copy on Windows). It is idempotent:
// calling it when the directory already exists rewrites the YAML and recreates
// every entry. The returned method is the language plugin entry's.
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

	method, linkErr := createPolicyPackEntries(ctx, execPath, ppDir)
	if linkErr != nil {
		return "", "", linkErr
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

// createPolicyPackEntries recreates every pack entry. A stale path is removed
// with Lstat so a dangling symlink is replaced and never followed. The method
// is the language plugin entry's; both entries are created the same way.
func createPolicyPackEntries(ctx context.Context, execPath, ppDir string) (string, error) {
	var method string
	for _, name := range policyPackEntryNames() {
		binaryPath := PolicyPackEntryPath(ppDir, name)
		if err := removeExistingEntry(binaryPath); err != nil {
			return "", err
		}
		linked, linkErr := linkOrCopy(ctx, execPath, binaryPath)
		if linkErr != nil {
			return "", fmt.Errorf("creating binary reference: %w", linkErr)
		}
		method = linked
	}
	return method, nil
}

// removeExistingEntry deletes a pack entry when one is present. A missing
// entry is not an error.
func removeExistingEntry(path string) error {
	_, statErr := os.Lstat(path)
	if statErr == nil {
		if removeErr := os.Remove(path); removeErr != nil {
			return fmt.Errorf("removing existing binary reference: %w", removeErr)
		}
		return nil
	}
	if os.IsNotExist(statErr) {
		return nil
	}
	return fmt.Errorf("checking existing binary reference: %w", statErr)
}
