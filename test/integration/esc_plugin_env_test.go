package integration_test

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/esc/esctest"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	pulumidetect "github.com/rshade/finfocus/internal/pulumi"
)

func TestESCPluginEnvOverridesShell(t *testing.T) {
	binPath := recorderBinary(t)
	if _, err := os.Stat(binPath); err != nil {
		t.Skipf("Plugin binary not found at %s. Run 'make build-recorder' first.", binPath)
	}

	const secret = "unrelated-value-7f3a"
	shellDir := t.TempDir()
	escDir := t.TempDir()
	api := esctest.New("tok", map[string]*esctest.Environment{
		"acme/billing/recorder": {
			ETag: "rev-1",
			Variables: map[string]esctest.Variable{
				"FINFOCUS_RECORDER_OUTPUT_DIR": {Value: escDir},
				"UNRELATED_SECRET":             {Value: secret},
			},
		},
	})
	t.Cleanup(api.Close)

	pulumiHome := t.TempDir()
	project := t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(project, "Pulumi.yaml"), []byte("name: demo\nruntime: yaml\n"), 0o600),
	)
	require.NoError(t, os.WriteFile(filepath.Join(project, "Pulumi.dev.yaml"),
		[]byte("environment:\n  - acme/billing/recorder\n"), 0o600))
	writeWorkspace(t, pulumiHome, project, `{"stack":"dev"}`)

	t.Setenv("FINFOCUS_HOME", t.TempDir())
	t.Setenv("PULUMI_HOME", pulumiHome)
	t.Setenv("PULUMI_BACKEND_URL", api.URL)
	t.Setenv("PULUMI_ACCESS_TOKEN", "tok")
	t.Setenv("FINFOCUS_RECORDER_OUTPUT_DIR", shellDir)

	stack, err := pulumidetect.LoadStackContext(project, "")
	require.NoError(t, err)
	src := esc.NewSource(esc.Options{
		Stack:    stack,
		UseStack: true,
		Getenv:   os.Getenv,
		Budget:   5 * time.Second,
	})
	var logs lockedBuffer
	logger := zerolog.New(&logs)
	ctx, cancel := context.WithTimeout(logger.WithContext(context.Background()), 20*time.Second)
	defer cancel()
	ctx = logging.ContextWithPluginLogWriter(ctx, &logs)
	ctx = pluginhost.WithInstallName(esc.WithSource(ctx, src), "recorder")

	client, err := pluginhost.NewClient(ctx, pluginhost.NewProcessLauncher(), binPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })

	_, err = client.API.GetProjectedCost(ctx, &proto.GetProjectedCostRequest{
		Resources: []*proto.ResourceDescriptor{{
			Type: "aws:ec2/instance:Instance", Provider: "aws",
			Properties: map[string]string{
				"instanceType":     "t3.micro",
				"availabilityZone": "us-east-1a",
			},
		}},
	})
	require.NoError(t, err)

	projected := filesNamed(t, escDir, "GetProjectedCost")
	assert.NotEmpty(t, projected, "recordings belong in the ESC directory")
	assert.Empty(t, filesNamed(t, shellDir, "GetProjectedCost"))
	// GetPluginInfo runs once before the descriptor is known, so that one file
	// can land in the shell directory. Nothing after the restart does.
	for _, name := range fileNames(t, shellDir) {
		assert.Contains(t, name, "GetPluginInfo")
	}
	scanned := logs.String() + readTree(t, escDir) + readTree(t, shellDir)
	assert.NotContains(t, scanned, "UNRELATED_SECRET")
	assert.NotContains(t, scanned, secret)
}

func recorderBinary(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	require.True(t, ok)
	binPath := filepath.Join(filepath.Dir(filename), "..", "..", "bin", "finfocus-plugin-recorder")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	return binPath
}

func writeWorkspace(t *testing.T, home, project, body string) {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join(project, "Pulumi.yaml"))
	require.NoError(t, err)
	sum := sha1.Sum([]byte(abs))
	path := filepath.Join(home, "workspaces", fmt.Sprintf("demo-%x-workspace.json", sum))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

func fileNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names
}

func filesNamed(t *testing.T, dir, part string) []string {
	t.Helper()
	var matched []string
	for _, name := range fileNames(t, dir) {
		if strings.Contains(name, part) {
			matched = append(matched, name)
		}
	}
	return matched
}

func readTree(t *testing.T, dir string) string {
	t.Helper()
	var buf strings.Builder
	for _, name := range fileNames(t, dir) {
		body, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		buf.Write(body)
	}
	return buf.String()
}
