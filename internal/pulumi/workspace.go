package pulumi

import (
	"crypto/sha1" //nolint:gosec // G505: Pulumi workspace filenames use SHA-1; this is not a security hash.
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SelectedStack reads the stack Pulumi selected for projectDir from the
// workspace file. A missing or unreadable file returns an error.
func SelectedStack(projectDir string) (string, error) {
	name, err := GetProjectName(projectDir)
	if err != nil {
		return "", err
	}
	filePath, err := projectYAMLPath(projectDir)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("resolving Pulumi.yaml: %w", err)
	}
	home := pulumiHomeDir()
	if home == "" {
		return "", errors.New("pulumi home is not available")
	}
	// Pulumi names the workspace file with SHA-1 of the absolute project path.
	sum := sha1.Sum([]byte(abs)) //nolint:gosec // G401: Pulumi workspace filenames use SHA-1.
	workspace := filepath.Join(home, "workspaces", fmt.Sprintf("%s-%x-workspace.json", name, sum))
	data, readErr := os.ReadFile(workspace)
	if readErr != nil {
		return "", fmt.Errorf("reading workspace file: %w", readErr)
	}
	var file workspaceFile
	if parseErr := json.Unmarshal(data, &file); parseErr != nil {
		return "", fmt.Errorf("parsing workspace file: %w", parseErr)
	}
	return pickWorkspaceStack(file)
}

type workspaceFile struct {
	Stack  string            `json:"stack"`
	Stacks map[string]string `json:"stacks"`
}

func pickWorkspaceStack(file workspaceFile) (string, error) {
	if len(file.Stacks) == 0 {
		if strings.TrimSpace(file.Stack) == "" {
			return "", errors.New("workspace file has no stack")
		}
		return file.Stack, nil
	}
	if len(file.Stacks) == 1 {
		for _, name := range file.Stacks {
			return name, nil
		}
	}
	for _, key := range preferredBackendKeys() {
		if name, ok := file.Stacks[key]; ok && name != "" {
			return name, nil
		}
	}
	return "", errors.New("workspace file has more than one stack")
}

func preferredBackendKeys() []string {
	keys := []string{}
	if backend := os.Getenv("PULUMI_BACKEND_URL"); backend != "" {
		keys = append(keys, backend)
	}
	return append(keys, "https://api.pulumi.com")
}

func projectYAMLPath(projectDir string) (string, error) {
	for _, name := range []string{pulumiProjectYAML, pulumiProjectYML} {
		candidate := filepath.Join(projectDir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no Pulumi.yaml or Pulumi.yml found in %s", projectDir)
}

func pulumiHomeDir() string {
	if home := os.Getenv("PULUMI_HOME"); home != "" {
		return home
	}
	user, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(user, ".pulumi")
}
