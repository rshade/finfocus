package terraform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os/exec"
	"path/filepath"
	"strings"
)

const unscopedStack = "unscoped"

// Identity is the history key for one Terraform directory.
type Identity struct {
	Project string
	Stack   string
	Label   string
}

// ErrUnscopedWorkspace is returned when include-workspace is on and the
// workspace is named unscoped.
var ErrUnscopedWorkspace = errors.New("OpenTofu workspace name unscoped is reserved")

// ResolveIdentity hashes the normalized origin remote and the path of dir
// relative to the git root. With no git root or no origin, it hashes the
// absolute path. includeWorkspace appends the OpenTofu workspace and is the
// only path that runs workspace show.
func ResolveIdentity(ctx context.Context, dir string, includeWorkspace bool) (Identity, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return Identity{}, fmt.Errorf("resolving terraform directory: %w", err)
	}
	abs = filepath.Clean(abs)
	workspace := ""
	if includeWorkspace {
		workspace, err = currentWorkspace(ctx, abs)
		if err != nil {
			return Identity{}, err
		}
		if workspace == unscopedStack {
			return Identity{}, ErrUnscopedWorkspace
		}
	}
	root, remote, ok := originRemote(ctx, abs)
	if !ok {
		return absoluteIdentity(abs, workspace), nil
	}
	rel, relErr := filepath.Rel(resolveSymlinks(root), resolveSymlinks(abs))
	outside := rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
	if relErr != nil || outside {
		// A directory that is not inside the git root uses the absolute-path identity.
		//nolint:nilerr // relErr selects the absolute identity
		return absoluteIdentity(abs, workspace), nil
	}
	rel = filepath.ToSlash(rel)
	hostPath, ok := normalizeRemote(remote)
	if !ok {
		return absoluteIdentity(abs, workspace), nil
	}
	lines := []string{"tf-id-v1", hostPath, rel}
	label := rel
	stack := unscopedStack
	if includeWorkspace {
		lines = append(lines, workspace)
		label = rel + " " + workspace
		stack = workspace
	}
	return Identity{Project: hashLines(lines), Stack: stack, Label: label}, nil
}

func absoluteIdentity(abs, workspace string) Identity {
	cleaned := filepath.ToSlash(abs)
	lines := []string{"tf-id-abspath-v1", cleaned}
	label := "path:" + cleaned
	stack := unscopedStack
	if workspace != "" {
		lines = append(lines, workspace)
		label = label + " " + workspace
		stack = workspace
	}
	return Identity{Project: hashLines(lines), Stack: stack, Label: label}
}

// resolveSymlinks returns path with symbolic links resolved. git reports the
// repository root with links resolved, so a directory reached through a link
// (a symlinked checkout, or /var on macOS) must be resolved the same way
// before it is compared with the root. An unresolvable path is returned as is.
func resolveSymlinks(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

func hashLines(lines []string) string {
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}

func originRemote(ctx context.Context, dir string) (string, string, bool) {
	rootBytes, err := exec.CommandContext(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel").
		Output()
	if err != nil {
		return "", "", false
	}
	root := strings.TrimSpace(string(rootBytes))
	remoteBytes, err := exec.CommandContext(ctx, "git", "-C", root, "remote", "get-url", "origin").
		Output()
	if err != nil {
		return root, "", false
	}
	remote := strings.TrimSpace(string(remoteBytes))
	if remote == "" {
		return root, "", false
	}
	return root, remote, true
}

func normalizeRemote(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "://") {
		parsed, err := url.Parse(raw)
		if err != nil || parsed.Host == "" {
			return "", false
		}
		host := strings.ToLower(parsed.Hostname())
		path := strings.Trim(parsed.Path, "/")
		path = strings.TrimSuffix(path, ".git")
		if host == "" || path == "" {
			return "", false
		}
		return host + "/" + path, true
	}
	userHost, path, found := strings.Cut(raw, ":")
	if !found {
		return "", false
	}
	_, host, found := strings.Cut(userHost, "@")
	if !found || host == "" {
		return "", false
	}
	path = strings.Trim(path, "/")
	path = strings.TrimSuffix(path, ".git")
	if path == "" {
		return "", false
	}
	return strings.ToLower(host) + "/" + path, true
}

func currentWorkspace(ctx context.Context, dir string) (string, error) {
	binary, err := findBinary()
	if err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, binary, "workspace", "show")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("reading OpenTofu workspace: %w", err)
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return "", errors.New("tofu workspace show returned an empty name")
	}
	return name, nil
}
