package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	orphanTestVersion = "v0.0.0-orphantest"
	orphanWait        = 20 * time.Second
)

type orphanFixture struct {
	binary  string
	project string
	pidFile string
	// pulumiPIDFile holds the fixture pulumi CLI's pid and its child's.
	pulumiPIDFile string
	// launchLog lists every launch's plugin and child pid, one pair per line.
	launchLog string
	served    string
	env       []string
}

func newOrphanFixture(t *testing.T) orphanFixture {
	t.Helper()
	work := t.TempDir()
	home := filepath.Join(work, "home")
	project := filepath.Join(work, "project")
	pluginDir := filepath.Join(home, "plugins", "orphanfixture", "0.1.0")
	require.NoError(t, os.MkdirAll(pluginDir, 0o750))
	require.NoError(t, os.MkdirAll(project, 0o750))

	binary := filepath.Join(work, "finfocus")
	out, err := exec.Command("go", "build",
		"-ldflags", "-X github.com/rshade/finfocus/pkg/version.version="+orphanTestVersion,
		"-o", binary, "../../cmd/finfocus").CombinedOutput()
	require.NoError(t, err, "build finfocus: %s", out)

	pluginBinary := "finfocus-plugin-orphanfixture"
	pluginPath := filepath.Join(pluginDir, pluginBinary)
	out, err = exec.Command("go", "build", "-o", pluginPath, "./testdata/orphanplugin").CombinedOutput()
	require.NoError(t, err, "build fixture plugin: %s", out)
	manifest, err := json.Marshal(map[string]any{
		"name":                "orphanfixture",
		"version":             "0.1.0",
		"description":         "router-like fixture",
		"author":              "FinFocus tests",
		"supported_providers": []string{"aws"},
		"protocols":           []string{"grpc"},
		"binary":              pluginBinary,
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(pluginDir, "plugin.manifest.json"), manifest, 0o600))

	pidFile := filepath.Join(work, "pids")
	served := filepath.Join(work, "served")
	env := []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "FINFOCUS_") && !strings.HasPrefix(entry, "PULUMI_") {
			env = append(env, entry)
		}
	}
	env = append(env,
		"FINFOCUS_HOME="+home,
		"FINFOCUS_PROJECT_DIR="+project,
		"FINFOCUS_HISTORY_ENABLED=false",
		"FINFOCUS_CACHE_ENABLED=false",
		"FINFOCUS_SKIP_MIGRATION_CHECK=1",
		"ORPHAN_PID_FILE="+pidFile,
		"ORPHAN_LAUNCH_LOG="+filepath.Join(work, "launches"),
		"ORPHAN_SERVED_FILE="+served,
	)
	return orphanFixture{
		binary: binary, project: project, pidFile: pidFile, launchLog: filepath.Join(work, "launches"),
		served: served, env: env,
	}
}

// recordedPIDs returns every plugin and child pid the fixture recorded: each
// launch in the launch log and the last pair in the pid file. Cleanup kills
// them all, so a failed assertion with several launches leaks no child.
func (f orphanFixture) recordedPIDs() []int {
	var pids []int
	for _, path := range []string{f.launchLog, f.pidFile, f.pulumiPIDFile} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, field := range strings.Fields(string(data)) {
			if pid, convErr := strconv.Atoi(field); convErr == nil && pid > 1 {
				pids = append(pids, pid)
			}
		}
	}
	return pids
}

// withFakePulumi returns the fixture with a fake pulumi CLI built into a
// directory that leads the PATH of its environment. The CLI serves state from
// the repository's overview testdata and records the preview's pids.
func (f orphanFixture) withFakePulumi(t *testing.T, ignoreTerm bool) orphanFixture {
	t.Helper()
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	bin := filepath.Join(f.project, "bin")
	require.NoError(t, os.MkdirAll(bin, 0o750))
	name := "pulumi"
	if exe := filepath.Ext(f.binary); exe != "" {
		name += exe
	}
	out, err := exec.Command("go", "build", "-o", filepath.Join(bin, name), "./testdata/fakepulumi").CombinedOutput()
	require.NoError(t, err, "build fixture pulumi: %s", out)
	require.NoError(t, os.WriteFile(
		filepath.Join(f.project, "Pulumi.yaml"), []byte("name: myapp\nruntime: yaml\n"), 0o600,
	))

	pidFile := filepath.Join(filepath.Dir(f.project), "pulumi-pids")
	f.pulumiPIDFile = pidFile
	f.env = append(f.env,
		"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKEPULUMI_STATE="+filepath.Join(root, "testdata/overview/state-no-changes.json"),
		"FAKEPULUMI_PID_FILE="+pidFile,
	)
	if ignoreTerm {
		f.env = append(f.env, "FAKEPULUMI_IGNORE_TERM=1")
	}
	return f
}

// readPIDPair returns the two pids a fixture process recorded in path.
func readPIDPair(path string) ([2]int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return [2]int{}, false
	}
	fields := strings.Fields(string(data))
	if len(fields) != 2 {
		return [2]int{}, false
	}
	first, errFirst := strconv.Atoi(fields[0])
	second, errSecond := strconv.Atoi(fields[1])
	return [2]int{first, second}, errFirst == nil && errSecond == nil
}
