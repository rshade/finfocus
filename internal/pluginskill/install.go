// Package pluginskill installs the FinFocus plugin agent skills into a plugin
// project through the skills CLI (npx skills add). The install is
// best-effort: every failure is returned as a warning, never as an error, so
// plugin init and plugin upgrade succeed without Node or network access.
package pluginskill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	// CLIPackage is the pinned skills CLI npm package. Pinning keeps the
	// flags stable and avoids running whatever version was published last.
	CLIPackage = "skills@1.7.0"

	// RepoTreeURL is the prefix of a GitHub tree URL in the finfocus repository.
	RepoTreeURL = "https://github.com/rshade/finfocus/tree/"

	// MainSource is used for builds that have no release tag of their own.
	MainSource = RepoTreeURL + "main/agent-skills"

	// DefaultTimeout bounds one npx run, including the package download.
	DefaultTimeout = 3 * time.Minute

	// lastReleaseWithoutSkill is the newest finfocus tag that predates the
	// finfocus-plugin-dev skill; installing from it would find no skill.
	lastReleaseWithoutSkill = "v0.4.0"

	outputTailLines = 8

	lockFile = "skills-lock.json"

	// Skill files are committed project files, readable by everyone.
	dirPerm  = 0o755
	filePerm = 0o644
)

// Skills are the skills installed into a plugin project.
var Skills = []string{"finfocus-plugin-dev", "finfocus-plugin-upgrade"} //nolint:gochecknoglobals // fixed list

// agentDirs maps each skills CLI agent to the project directory it writes.
var agentDirs = []struct{ agent, dir string }{ //nolint:gochecknoglobals // fixed list
	{"codex", ".agents/skills"},
	{"claude-code", ".claude/skills"},
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// Result describes one install attempt. It is also the "skill" object of
// plugin upgrade's JSON output.
type Result struct {
	Installed bool     `json:"installed"`
	Source    string   `json:"source"`
	Command   string   `json:"command"`
	Paths     []string `json:"paths,omitempty"`
	Warning   string   `json:"warning,omitempty"`
}

// Runner runs a process in dir with env appended to the current environment
// and returns its combined output.
type Runner func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error)

// Installer runs the skills CLI. Run and LookPath are injected so tests never
// start npx.
type Installer struct {
	Run      Runner
	LookPath func(file string) (string, error)
	Timeout  time.Duration
}

// New returns an Installer backed by os/exec.
func New() Installer {
	return Installer{Run: runProcess, LookPath: exec.LookPath, Timeout: DefaultTimeout}
}

// Source returns the skills source for a finfocus version: the release's own
// tag when it is a release newer than the last one without the skill, and
// main otherwise (development builds, git describe output, prereleases).
func Source(version string) string {
	if tag, ok := releaseTag(version); ok {
		return RepoTreeURL + tag + "/agent-skills"
	}
	return MainSource
}

// Args returns the full command line that installs the skills.
func Args(version string) []string {
	args := []string{"npx", "-y", CLIPackage, "add", Source(version)}
	for _, s := range Skills {
		args = append(args, "--skill", s)
	}
	for _, a := range agentDirs {
		args = append(args, "--agent", a.agent)
	}
	return append(args, "--copy", "-y")
}

// Command returns Args as one line a user can copy into a shell.
func Command(version string) string {
	return strings.Join(Args(version), " ")
}

// Skipped returns the result for an install that was deliberately not run.
func Skipped(version, reason string) Result {
	return Result{
		Source:  Source(version),
		Command: Command(version),
		Warning: fmt.Sprintf("skill install skipped (%s)", reason),
	}
}

// Install installs the skills into dir. It never returns an error: problems
// are reported in Result.Warning together with the command to run later.
//
// The skills CLI runs in an empty staging directory, never in dir: npx reads
// .npmrc and node_modules from its working directory, and a plugin checkout
// is not trusted to choose the registry or the package that runs. Only the
// installed skill directories and their lockfile entries are copied into dir.
func (i Installer) Install(ctx context.Context, dir, version string) Result {
	res := Result{Source: Source(version), Command: Command(version)}

	npx, err := i.LookPath("npx")
	if err != nil {
		res.Warning = "npx not found on PATH; install Node.js to add the FinFocus agent skills"
		return res
	}

	stage, err := os.MkdirTemp("", "finfocus-skills-")
	if err != nil {
		res.Warning = fmt.Sprintf("creating a staging directory for the skills CLI: %v", err)
		return res
	}
	defer os.RemoveAll(stage)

	timeout := i.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	env := []string{"DO_NOT_TRACK=1", "DISABLE_TELEMETRY=1"}
	out, err := i.Run(runCtx, stage, env, npx, Args(version)[1:]...)
	if err != nil {
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			res.Warning = fmt.Sprintf("skills CLI timed out after %s", timeout)
		} else {
			res.Warning = fmt.Sprintf("skills CLI failed: %v", err)
		}
		if tail := outputTail(out); tail != "" {
			res.Warning += "\n" + tail
		}
		return res
	}

	paths, missing, warnings := publish(stage, dir)
	res.Paths = paths
	res.Installed = len(missing) == 0 && len(warnings) == 0

	if len(missing) > 0 {
		warnings = append(warnings, fmt.Sprintf("skills CLI did not install %s from %s",
			strings.Join(missing, ", "), res.Source))
	}
	if res.Source == MainSource {
		warnings = append(warnings, fmt.Sprintf(
			"finfocus %q is not a release build; skills come from main and may not match this binary", version))
	}
	res.Warning = strings.Join(warnings, "\n")
	return res
}

// publish copies the skill directories the skills CLI wrote in stage into dir
// and merges their lockfile entries. Every write goes through an [os.Root] on
// dir, so a symlink in the checkout cannot redirect a write outside it. The
// skills CLI exits 0 even when a requested skill is missing from the source,
// so only skills it actually wrote are reported as installed; the rest are
// returned as missing.
func publish(stage, dir string) ([]string, []string, []string) {
	var paths, missing, warnings []string
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, Skills, []string{fmt.Sprintf("opening %s: %v", dir, err)}
	}
	defer root.Close()

	for _, s := range Skills {
		found := false
		for _, a := range agentDirs {
			rel := path.Join(a.dir, s)
			src := filepath.Join(stage, filepath.FromSlash(rel))
			if info, statErr := os.Stat(src); statErr != nil || !info.IsDir() {
				continue
			}
			if copyErr := replaceDir(root, src, rel); copyErr != nil {
				warnings = append(warnings, fmt.Sprintf("copying %s: %v", rel, copyErr))
				continue
			}
			paths = append(paths, rel)
			found = true
		}
		if !found {
			missing = append(missing, s)
		}
	}
	if len(paths) > 0 {
		if lockErr := mergeLock(filepath.Join(stage, lockFile), root, lockFile); lockErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s not updated: %v", lockFile, lockErr))
		} else {
			paths = append(paths, lockFile)
		}
	}
	slices.Sort(paths)
	return paths, missing, warnings
}

// replaceDir replaces rel inside root with a copy of src, so files an older
// install had and the new one does not are removed. Only directories and
// regular files are copied.
func replaceDir(root *os.Root, src, rel string) error {
	if err := root.RemoveAll(filepath.FromSlash(rel)); err != nil {
		return err
	}
	return fs.WalkDir(os.DirFS(src), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.FromSlash(path.Join(rel, p))
		switch {
		case d.IsDir():
			return root.MkdirAll(target, dirPerm)
		case d.Type().IsRegular():
			data, readErr := os.ReadFile(filepath.Join(src, filepath.FromSlash(p)))
			if readErr != nil {
				return readErr
			}
			return root.WriteFile(target, data, filePerm)
		default:
			return fmt.Errorf("%s: not a regular file", p)
		}
	})
}

// mergeLock sets the entries of the installed skills in the project's
// skills-lock.json (name, inside root) from the staged one and keeps every
// other entry. A project lockfile that cannot be parsed is left unchanged.
func mergeLock(stagedPath string, root *os.Root, name string) error {
	stagedData, err := os.ReadFile(stagedPath)
	if err != nil {
		return fmt.Errorf("reading the skills CLI lockfile: %w", err)
	}
	staged, err := parseLock(stagedData)
	if err != nil {
		return fmt.Errorf("reading the skills CLI lockfile: %w", err)
	}
	project := map[string]json.RawMessage{}
	projectData, err := root.ReadFile(name)
	switch {
	case err == nil:
		if project, err = parseLock(projectData); err != nil {
			return err
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}

	stagedSkills := map[string]json.RawMessage{}
	if raw, ok := staged["skills"]; ok {
		if err = json.Unmarshal(raw, &stagedSkills); err != nil {
			return fmt.Errorf("reading the skills CLI lockfile: %w", err)
		}
	}
	projectSkills := map[string]json.RawMessage{}
	if raw, ok := project["skills"]; ok {
		if err = json.Unmarshal(raw, &projectSkills); err != nil {
			return err
		}
	}
	for _, s := range Skills {
		if entry, ok := stagedSkills[s]; ok {
			projectSkills[s] = entry
		}
	}
	if project["skills"], err = json.Marshal(projectSkills); err != nil {
		return err
	}
	if _, ok := project["version"]; !ok {
		project["version"] = staged["version"]
	}

	data, err := json.MarshalIndent(project, "", "  ")
	if err != nil {
		return err
	}
	return root.WriteFile(name, append(data, '\n'), filePerm)
}

func parseLock(data []byte) (map[string]json.RawMessage, error) {
	lock := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}
	return lock, nil
}

func releaseTag(version string) (string, bool) {
	v := version
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) || semver.Prerelease(v) != "" || semver.Build(v) != "" {
		return "", false
	}
	if semver.Compare(v, lastReleaseWithoutSkill) <= 0 {
		return "", false
	}
	return semver.Canonical(v), true
}

// outputTail returns the last non-empty lines of the skills CLI output with
// terminal escapes removed.
func outputTail(out []byte) string {
	clean := ansiEscape.ReplaceAllString(string(out), "")
	var lines []string
	for line := range strings.SplitSeq(strings.ReplaceAll(clean, "\r", "\n"), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > outputTailLines {
		lines = lines[len(lines)-outputTailLines:]
	}
	return strings.Join(lines, "\n")
}

func runProcess(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = nil
	return cmd.CombinedOutput()
}
