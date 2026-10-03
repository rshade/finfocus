// Package pluginskill installs the FinFocus plugin agent skills into a plugin
// project through the skills CLI (npx skills add). The install is
// best-effort: every failure is returned as a warning, never as an error, so
// plugin init and plugin upgrade succeed without Node or network access.
package pluginskill

import (
	"context"
	"errors"
	"fmt"
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

// Install runs the skills CLI in dir. It never returns an error: problems are
// reported in Result.Warning together with the command to run later.
func (i Installer) Install(ctx context.Context, dir, version string) Result {
	res := Result{Source: Source(version), Command: Command(version)}

	npx, err := i.LookPath("npx")
	if err != nil {
		res.Warning = "npx not found on PATH; install Node.js to add the FinFocus agent skills"
		return res
	}

	timeout := i.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	env := []string{"DO_NOT_TRACK=1", "DISABLE_TELEMETRY=1"}
	out, err := i.Run(runCtx, dir, env, npx, Args(version)[1:]...)
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

	// The skills CLI exits 0 even when a requested skill is missing from the
	// source, so only files that now exist count as installed.
	var missing []string
	for _, s := range Skills {
		found := false
		for _, a := range agentDirs {
			p := path.Join(a.dir, s)
			if exists(dir, p) {
				res.Paths = append(res.Paths, p)
				found = true
			}
		}
		if !found {
			missing = append(missing, s)
		}
	}
	if exists(dir, "skills-lock.json") {
		res.Paths = append(res.Paths, "skills-lock.json")
	}
	slices.Sort(res.Paths)

	var warnings []string
	if len(missing) > 0 {
		warnings = append(warnings, fmt.Sprintf("skills CLI did not install %s from %s",
			strings.Join(missing, ", "), res.Source))
	}
	if res.Source == MainSource {
		warnings = append(warnings, fmt.Sprintf(
			"finfocus %q is not a release build; skills come from main and may not match this binary", version))
	}
	res.Installed = len(missing) == 0
	res.Warning = strings.Join(warnings, "\n")
	return res
}

func exists(dir, rel string) bool {
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	return err == nil
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
