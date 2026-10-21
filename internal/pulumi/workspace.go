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

const defaultBackend = "https://api.pulumi.com"

// SelectedStack reads the stack Pulumi selected for projectDir from the
// workspace file. A missing or unreadable file returns an error.
func SelectedStack(projectDir string) (string, error) {
	project, err := readProjectFile(projectDir)
	if err != nil {
		return "", err
	}
	name := project.Name
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
	return pickWorkspaceStack(file, activeBackend(project.Backend.URL))
}

type workspaceFile struct {
	Stack  string            `json:"stack"`
	Stacks map[string]string `json:"stacks"`
}

func pickWorkspaceStack(file workspaceFile, backend string) (string, error) {
	if len(file.Stacks) == 0 {
		if strings.TrimSpace(file.Stack) == "" {
			return "", errors.New("workspace file has no stack")
		}
		return file.Stack, nil
	}
	for _, key := range backendKeys(backend) {
		if name, ok := file.Stacks[key]; ok && name != "" {
			return name, nil
		}
	}
	return "", fmt.Errorf("workspace file has no stack for backend %s", backend)
}

// activeBackend resolves the backend the same way ESC authentication does:
// PULUMI_BACKEND_URL, then the project's backend.url, then the current login
// in credentials.json, then Pulumi Cloud.
func activeBackend(projectURL string) string {
	if backend := os.Getenv("PULUMI_BACKEND_URL"); backend != "" {
		return backend
	}
	if backend := strings.TrimSpace(projectURL); backend != "" {
		return backend
	}
	if backend := currentLogin(); backend != "" {
		return backend
	}
	return defaultBackend
}

func currentLogin() string {
	dir := os.Getenv("PULUMI_CREDENTIALS_PATH")
	if dir == "" {
		dir = pulumiHomeDir()
	}
	//nolint:gosec // G703: Pulumi's own credentials location.
	data, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		return ""
	}
	var creds struct {
		Current string `json:"current"`
	}
	if json.Unmarshal(data, &creds) != nil {
		return ""
	}
	return creds.Current
}

func backendKeys(backend string) []string {
	trimmed := strings.TrimRight(backend, "/")
	return []string{backend, trimmed, trimmed + "/"}
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
