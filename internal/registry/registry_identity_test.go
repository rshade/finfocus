package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installAs writes a fake plugin binary named binary under
// <root>/<installName>/v1.0.0. The fake launcher serves it under the binary's
// name, so the plugin reports binary as its name.
func installAs(t *testing.T, root, installName, binary string) {
	t.Helper()
	dir := filepath.Join(root, installName, "v1.0.0")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	require.NoError(
		t,
		os.WriteFile(filepath.Join(dir, binary), []byte("#!/bin/sh\n"), 0o755),
	)
}

// identityRegistry installs aws-public reporting finfocus-plugin-aws-public,
// aws-ce reporting aws-ce, and acme (no catalog entry) reporting acme.
func identityRegistry(t *testing.T) (*Registry, *fakeLauncher) {
	t.Helper()
	root := t.TempDir()
	installAs(t, root, "aws-public", "finfocus-plugin-aws-public")
	installAs(t, root, "aws-ce", "finfocus-plugin-aws-ce")
	installAs(t, root, "acme", "finfocus-plugin-acme")
	launcher := newFakeLauncher(t,
		[]string{"finfocus-plugin-aws-public", "finfocus-plugin-aws-ce", "finfocus-plugin-acme"}, nil)
	return &Registry{root: root, launcher: launcher}, launcher
}

func installNames(t *testing.T, reg *Registry, filter string) []string {
	t.Helper()
	clients, cleanup, err := reg.Open(context.Background(), filter)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	names := make([]string, 0, len(clients))
	for _, c := range clients {
		names = append(names, c.InstallName)
	}
	return names
}

func TestRegistry_Open_AdapterFilter(t *testing.T) {
	t.Setenv("FINFOCUS_CATALOG_FILE", "")

	tests := []struct {
		name        string
		filter      string
		want        []string
		wantStarted int
		wantClosed  int32
	}{
		{"install name starts only that plugin", "aws-public", []string{"aws-public"}, 1, 0},
		{"catalog alias starts only that plugin", "finfocus-plugin-aws-public", []string{"aws-public"}, 1, 0},
		{"reported name starts every plugin and keeps its own", "finfocus-plugin-aws-ce", []string{"aws-ce"}, 3, 2},
		{"unknown name starts every plugin and keeps none", "some-typo", []string{}, 3, 3},
		{"no filter starts every plugin", "", []string{"acme", "aws-ce", "aws-public"}, 3, 0},
	}
	for _, tc := range tests { //nolint:paralleltest // the parent sets FINFOCUS_CATALOG_FILE with t.Setenv
		t.Run(tc.name, func(t *testing.T) {
			reg, launcher := identityRegistry(t)
			assert.Equal(t, tc.want, installNames(t, reg, tc.filter))
			assert.Len(t, launcher.startedNames(), tc.wantStarted)
			assert.Equal(
				t,
				tc.wantClosed,
				launcher.closed.Load(),
				"plugins that do not answer to the filter are closed",
			)
		})
	}
}

func TestRegistry_Open_SetsInstallNameAndAliases(t *testing.T) {
	t.Setenv("FINFOCUS_CATALOG_FILE", "")
	reg, _ := identityRegistry(t)

	clients, cleanup, err := reg.Open(context.Background(), "")
	require.NoError(t, err)
	t.Cleanup(cleanup)

	byInstall := map[string][]string{}
	reported := map[string]string{}
	for _, c := range clients {
		byInstall[c.InstallName] = c.Aliases
		reported[c.InstallName] = c.Name
	}
	assert.Equal(t, []string{"finfocus-plugin-aws-public"}, byInstall["aws-public"])
	assert.Empty(t, byInstall["acme"], "a plugin with no catalog entry has no aliases")
	assert.Equal(t, "finfocus-plugin-aws-public", reported["aws-public"])
}

// Not parallel: t.Setenv, and the shared warning set reset by ResetNameMismatchWarningsForTest.
func TestRegistry_Open_WarnsOnceWhenReportedNameDiffers(t *testing.T) {
	t.Setenv("FINFOCUS_CATALOG_FILE", "")
	ResetNameMismatchWarningsForTest()
	t.Cleanup(ResetNameMismatchWarningsForTest)

	var logs bytes.Buffer
	ctx := zerolog.New(zerolog.SyncWriter(&logs)).Level(zerolog.DebugLevel).WithContext(context.Background())
	for range 2 {
		reg, _ := identityRegistry(t)
		clients, cleanup, err := reg.Open(ctx, "")
		require.NoError(t, err)
		require.Len(t, clients, 3)
		cleanup()
	}

	warned := map[string]string{}
	counts := map[string]int{}
	for line := range strings.SplitSeq(strings.TrimSpace(logs.String()), "\n") {
		var entry struct {
			Level    string `json:"level"`
			Install  string `json:"install_name"`
			Reported string `json:"reported_name"`
		}
		require.NoError(t, json.Unmarshal([]byte(line), &entry))
		if entry.Level == "warn" && entry.Install != "" {
			counts[entry.Install]++
			warned[entry.Install] = entry.Reported
		}
	}
	assert.Equal(t, map[string]string{
		"aws-public": "finfocus-plugin-aws-public",
		"aws-ce":     "finfocus-plugin-aws-ce",
		"acme":       "finfocus-plugin-acme",
	}, warned)
	assert.Equal(t, map[string]int{"aws-public": 1, "aws-ce": 1, "acme": 1}, counts,
		"each mismatched plugin warns once across two opens")
}
