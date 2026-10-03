package plugin_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/cli"
)

// TestPluginInit_GeneratedProjectInstallsAndValidates builds a scaffolded
// plugin with its own toolchain: its tests must pass, and `make install` must
// produce a plugin that `finfocus plugin validate` accepts.
func TestPluginInit_GeneratedProjectInstallsAndValidates(t *testing.T) {
	if testing.Short() {
		t.Skip("downloads plugin dependencies and builds the generated project")
	}
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make is not installed")
	}

	outputDir := t.TempDir()
	home := t.TempDir()

	initCmd := cli.NewPluginInitCmd()
	var initOut bytes.Buffer
	initCmd.SetOut(&initOut)
	initCmd.SetErr(&initOut)
	initCmd.SetArgs([]string{
		"scaffold-check",
		"--author", "Test Author",
		"--providers", "aws",
		"--output-dir", outputDir,
		"--no-skill",
	})
	require.NoError(t, initCmd.Execute(), initOut.String())
	projectDir := filepath.Join(outputDir, "scaffold-check")

	run := func(env []string, name string, args ...string) {
		t.Helper()
		cmd := exec.Command(name, args...)
		cmd.Dir = projectDir
		cmd.Env = append(os.Environ(), env...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%s %v:\n%s", name, args, out)
	}
	run(nil, "go", "mod", "tidy")
	run(nil, "go", "test", "./...")

	// Without GOPATH set, HOME would move Go's read-only module cache into the
	// temp dir, which t.TempDir cleanup then cannot remove.
	goEnv, err := exec.Command("go", "env", "GOMODCACHE", "GOCACHE").Output()
	require.NoError(t, err)
	goPaths := strings.Split(strings.TrimSpace(string(goEnv)), "\n")
	require.Len(t, goPaths, 2)
	run([]string{"HOME=" + home, "GOMODCACHE=" + goPaths[0], "GOCACHE=" + goPaths[1]}, "make", "install")

	t.Setenv("HOME", home)
	t.Setenv("FINFOCUS_HOME", filepath.Join(home, ".finfocus"))
	root := cli.NewRootCmd("test")
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"plugin", "validate"})
	require.NoError(t, root.Execute(), out.String())
	assert.Contains(t, out.String(), "Validating scaffold-check v0.1.0... OK")
}
