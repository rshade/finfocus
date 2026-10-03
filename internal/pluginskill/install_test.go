package pluginskill_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginskill"
)

func TestSource(t *testing.T) {
	t.Parallel()
	tests := []struct {
		version string
		want    string
	}{
		{"v0.4.1", pluginskill.RepoTreeURL + "v0.4.1/agent-skills"},
		{"0.5.0", pluginskill.RepoTreeURL + "v0.5.0/agent-skills"},
		{"v1.0.0", pluginskill.RepoTreeURL + "v1.0.0/agent-skills"},
		{"v0.4.0", pluginskill.MainSource},
		{"0.1.0", pluginskill.MainSource},
		{"v0.4.0-3-gabc1234-dirty", pluginskill.MainSource},
		{"v0.5.0-rc.1", pluginskill.MainSource},
		{"v0.5.0+build", pluginskill.MainSource},
		{"dev", pluginskill.MainSource},
		{"", pluginskill.MainSource},
	}
	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, pluginskill.Source(tt.version))
		})
	}
}

func TestArgs(t *testing.T) {
	t.Parallel()
	args := pluginskill.Args("v0.4.1")
	assert.Equal(t, []string{
		"npx", "-y", pluginskill.CLIPackage, "add", pluginskill.RepoTreeURL + "v0.4.1/agent-skills",
		"--skill", "finfocus-plugin-dev", "--skill", "finfocus-plugin-upgrade",
		"--agent", "codex", "--agent", "claude-code",
		"--copy", "-y",
	}, args)
	assert.Equal(t, strings.Join(args, " "), pluginskill.Command("v0.4.1"))
}

type call struct {
	dir  string
	env  []string
	name string
	args []string
}

// fakeInstaller returns an installer whose npx run records the call and, on
// success, creates the files the skills CLI would write for skills.
func fakeInstaller(out string, err error, calls *[]call, skills ...string) pluginskill.Installer {
	return pluginskill.Installer{
		LookPath: func(string) (string, error) { return "/usr/bin/npx", nil },
		Run: func(_ context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
			*calls = append(*calls, call{dir: dir, env: env, name: name, args: args})
			if err == nil {
				writeSkills(dir, skills...)
			}
			return []byte(out), err
		},
	}
}

// writeSkills writes what the skills CLI writes into its working directory.
func writeSkills(dir string, skills ...string) {
	lock := map[string]any{"version": 1, "skills": map[string]any{}}
	for _, s := range skills {
		for _, agentDir := range []string{".agents/skills", ".claude/skills"} {
			skillDir := filepath.Join(dir, agentDir, s)
			_ = os.MkdirAll(filepath.Join(skillDir, "references"), 0o750)
			_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# "+s+"\n"), 0o600)
			_ = os.WriteFile(filepath.Join(skillDir, "references", "a.md"), []byte("ref\n"), 0o600)
		}
		lock["skills"].(map[string]any)[s] = map[string]any{"source": "rshade/finfocus", "ref": "v0.4.1"}
	}
	if len(skills) > 0 {
		data, _ := json.Marshal(lock)
		_ = os.WriteFile(filepath.Join(dir, "skills-lock.json"), data, 0o600)
	}
}

func readLock(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "skills-lock.json"))
	require.NoError(t, err)
	var lock map[string]any
	require.NoError(t, json.Unmarshal(data, &lock))
	return lock
}

func TestInstallSuccess(t *testing.T) {
	t.Parallel()
	var calls []call
	inst := fakeInstaller("Installed 2 skills", nil, &calls, pluginskill.Skills...)
	dir := t.TempDir()

	res := inst.Install(context.Background(), dir, "v0.4.1")

	require.Len(t, calls, 1)
	assert.Equal(t, "/usr/bin/npx", calls[0].name)
	assert.Equal(t, pluginskill.Args("v0.4.1")[1:], calls[0].args)
	assert.Contains(t, calls[0].env, "DO_NOT_TRACK=1")
	assert.Contains(t, calls[0].env, "DISABLE_TELEMETRY=1")

	assert.True(t, res.Installed)
	assert.Empty(t, res.Warning)
	assert.Equal(t, pluginskill.Source("v0.4.1"), res.Source)
	assert.Equal(t, pluginskill.Command("v0.4.1"), res.Command)
	assert.Equal(t, []string{
		".agents/skills/finfocus-plugin-dev",
		".agents/skills/finfocus-plugin-upgrade",
		".claude/skills/finfocus-plugin-dev",
		".claude/skills/finfocus-plugin-upgrade",
		"skills-lock.json",
	}, res.Paths)
}

func TestInstallRunsOutsideTheProject(t *testing.T) {
	t.Parallel()
	var calls []call
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".npmrc"), []byte("registry=https://evil.example\n"), 0o600))

	res := fakeInstaller("", nil, &calls, pluginskill.Skills...).Install(context.Background(), dir, "v0.4.1")

	require.Len(t, calls, 1)
	assert.NotEqual(t, dir, calls[0].dir, "npx must not read the project's .npmrc or node_modules")
	_, err := os.Stat(calls[0].dir)
	assert.True(t, os.IsNotExist(err), "the staging directory is removed")

	assert.True(t, res.Installed)
	for _, p := range []string{
		".agents/skills/finfocus-plugin-dev/SKILL.md",
		".claude/skills/finfocus-plugin-upgrade/references/a.md",
	} {
		assert.FileExists(t, filepath.Join(dir, p))
	}
	skills := readLock(t, dir)["skills"].(map[string]any)
	assert.Contains(t, skills, "finfocus-plugin-dev")
	assert.Contains(t, skills, "finfocus-plugin-upgrade")
}

func TestInstallMergesExistingLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	existing := `{"version": 1, "skills": {"other-skill": {"source": "someone/else"}, ` +
		`"finfocus-plugin-dev": {"source": "rshade/finfocus", "ref": "v0.4.0"}}}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills-lock.json"), []byte(existing), 0o600))
	var calls []call

	res := fakeInstaller("", nil, &calls, pluginskill.Skills...).Install(context.Background(), dir, "v0.4.1")

	assert.True(t, res.Installed)
	assert.Empty(t, res.Warning)
	skills := readLock(t, dir)["skills"].(map[string]any)
	assert.Contains(t, skills, "other-skill")
	assert.Equal(t, "v0.4.1", skills["finfocus-plugin-dev"].(map[string]any)["ref"])
	assert.Contains(t, skills, "finfocus-plugin-upgrade")
}

func TestInstallReplacesOldSkillFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stale := filepath.Join(dir, ".agents/skills/finfocus-plugin-dev/references/removed.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(stale), 0o750))
	require.NoError(t, os.WriteFile(stale, []byte("old\n"), 0o600))
	other := filepath.Join(dir, ".agents/skills/other-skill/SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(other), 0o750))
	require.NoError(t, os.WriteFile(other, []byte("keep\n"), 0o600))
	var calls []call

	fakeInstaller("", nil, &calls, pluginskill.Skills...).Install(context.Background(), dir, "v0.4.1")

	assert.NoFileExists(t, stale)
	assert.FileExists(t, filepath.Join(dir, ".agents/skills/finfocus-plugin-dev/SKILL.md"))
	assert.FileExists(t, other, "skills finfocus did not install are left alone")
}

func TestInstallKeepsUnreadableLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "skills-lock.json"), []byte("not json"), 0o600))
	var calls []call

	res := fakeInstaller("", nil, &calls, pluginskill.Skills...).Install(context.Background(), dir, "v0.4.1")

	data, err := os.ReadFile(filepath.Join(dir, "skills-lock.json"))
	require.NoError(t, err)
	assert.Equal(t, "not json", string(data))
	assert.Contains(t, res.Warning, "skills-lock.json")
	assert.False(t, res.Installed)
	assert.NotContains(t, res.Paths, "skills-lock.json")
	assert.FileExists(t, filepath.Join(dir, ".agents/skills/finfocus-plugin-dev/SKILL.md"))
}

func TestInstallFromMainWarns(t *testing.T) {
	t.Parallel()
	var calls []call
	res := fakeInstaller("", nil, &calls, pluginskill.Skills...).
		Install(context.Background(), t.TempDir(), "v0.4.0-3-gabc")

	assert.True(t, res.Installed)
	assert.Equal(t, pluginskill.MainSource, res.Source)
	assert.Contains(t, res.Warning, "not a release build")
	assert.Contains(t, res.Warning, "main")
}

func TestInstallReportsSkillsTheCLIDidNotWrite(t *testing.T) {
	t.Parallel()
	var calls []call
	inst := fakeInstaller("Installed 1 skill", nil, &calls, "finfocus-plugin-upgrade")

	res := inst.Install(context.Background(), t.TempDir(), "v0.4.1")

	assert.False(t, res.Installed)
	assert.Equal(t, []string{
		".agents/skills/finfocus-plugin-upgrade",
		".claude/skills/finfocus-plugin-upgrade",
		"skills-lock.json",
	}, res.Paths)
	assert.Contains(t, res.Warning, "did not install finfocus-plugin-dev from "+pluginskill.Source("v0.4.1"))
}

func TestInstallNothingWritten(t *testing.T) {
	t.Parallel()
	var calls []call
	res := fakeInstaller("", nil, &calls).Install(context.Background(), t.TempDir(), "v0.4.1")

	assert.False(t, res.Installed)
	assert.Empty(t, res.Paths)
	assert.Contains(t, res.Warning, "did not install finfocus-plugin-dev, finfocus-plugin-upgrade")
}

func TestInstallNpxMissing(t *testing.T) {
	t.Parallel()
	ran := false
	inst := pluginskill.Installer{
		LookPath: func(string) (string, error) { return "", errors.New("not found") },
		Run: func(context.Context, string, []string, string, ...string) ([]byte, error) {
			ran = true
			return nil, nil
		},
	}

	res := inst.Install(context.Background(), "/plugin", "v0.4.1")

	assert.False(t, ran)
	assert.False(t, res.Installed)
	assert.Contains(t, res.Warning, "npx not found")
	assert.Equal(t, pluginskill.Command("v0.4.1"), res.Command)
	assert.Empty(t, res.Paths)
}

func TestInstallFailureKeepsOutputTail(t *testing.T) {
	t.Parallel()
	lines := make([]string, 0, 20)
	for i := range 20 {
		lines = append(lines, "line "+string(rune('a'+i)))
	}
	lines = append(lines, "Error: repository not found")
	var calls []call
	inst := fakeInstaller(strings.Join(lines, "\n")+"\n", errors.New("exit status 1"), &calls)

	res := inst.Install(context.Background(), "/plugin", "v0.4.1")

	assert.False(t, res.Installed)
	assert.Contains(t, res.Warning, "exit status 1")
	assert.Contains(t, res.Warning, "Error: repository not found")
	assert.NotContains(t, res.Warning, "line a")
	assert.Empty(t, res.Paths)
}

func TestInstallFailureStripsANSI(t *testing.T) {
	t.Parallel()
	var calls []call
	inst := fakeInstaller("\x1b[1mError:\x1b[0m boom", errors.New("exit status 1"), &calls)

	res := inst.Install(context.Background(), "/plugin", "v0.4.1")

	assert.Contains(t, res.Warning, "Error: boom")
	assert.NotContains(t, res.Warning, "\x1b")
}

func TestInstallTimeout(t *testing.T) {
	t.Parallel()
	inst := pluginskill.Installer{
		Timeout:  10 * time.Millisecond,
		LookPath: func(string) (string, error) { return "/usr/bin/npx", nil },
		Run: func(ctx context.Context, _ string, _ []string, _ string, _ ...string) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	res := inst.Install(context.Background(), "/plugin", "v0.4.1")

	assert.False(t, res.Installed)
	assert.Contains(t, res.Warning, "timed out")
}

func TestSkipped(t *testing.T) {
	t.Parallel()
	res := pluginskill.Skipped("v0.4.1", "--no-skill")

	assert.False(t, res.Installed)
	assert.Equal(t, pluginskill.Source("v0.4.1"), res.Source)
	assert.Equal(t, pluginskill.Command("v0.4.1"), res.Command)
	assert.Equal(t, "skill install skipped (--no-skill)", res.Warning)
}

func TestNewUsesRealProcess(t *testing.T) {
	t.Parallel()
	inst := pluginskill.New()
	require.NotNil(t, inst.Run)
	require.NotNil(t, inst.LookPath)
	assert.Equal(t, pluginskill.DefaultTimeout, inst.Timeout)
}
