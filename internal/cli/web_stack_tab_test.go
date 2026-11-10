package cli

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const singleStackMeta = `<meta name="finfocus-stack"`

// webShell fetches the dashboard's shell page with the session cookie.
func webShell(t *testing.T, origin string, cookie *http.Cookie) string {
	t.Helper()
	resp := webAPI(t, origin, cookie, "/", "")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}

// TestWebShellNamesTheOneStack (#2088): the single-stack dashboard names the
// Pulumi stack it prices in the shell, so the page shows a one-tab bar at boot.
// With no --stack that is the workspace's selected stack, which the scripted
// `pulumi stack ls` marks as dev; a list that collapses to one name is that
// name.
//
//nolint:paralleltest // startWebPassphraseProject changes cwd, env, global config, the Pulumi runner, and the web hook.
func TestWebShellNamesTheOneStack(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
	}{
		{"workspace stack", nil},
		{"explicit stack", []string{"--stack", "dev"}},
		{"a list that collapses", []string{"--stack", "dev,dev"}},
		{"a repeated flag collapses", []string{"--stack", "dev", "--stack", "dev"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			origin, cookie, _, cancel, wait := startWebPassphraseProject(t, false, &startupPreviewRunner{}, tt.args...)

			shell := webShell(t, origin, cookie)
			assert.Contains(t, shell, `<meta name="finfocus-stack" content="dev">`)
			assert.NotContains(t, shell, `<meta name="finfocus-stacks"`)

			cancel()
			require.NoError(t, wait())
		})
	}
}

// TestWebLoadsTheStackTheShellNames (#2088): the tab names the workspace stack
// resolved at startup, so loading must use that same stack. Left to resolve
// the workspace selection again, a `pulumi stack select` during the passphrase
// wait would show another stack's costs under the first stack's name.
//
//nolint:paralleltest // startWebPassphraseProject changes cwd, env, global config, the Pulumi runner, and the web hook.
func TestWebLoadsTheStackTheShellNames(t *testing.T) {
	runner := &startupPreviewRunner{reselectAfterStartup: true}
	origin, cookie, _, cancel, wait := startWebPassphraseProject(t, false, runner)

	assert.Contains(t, webShell(t, origin, cookie), `<meta name="finfocus-stack" content="dev">`)
	events, ready := readWebEventsUntil(openWebStream(t, origin, cookie), func(event webStreamEvent) bool {
		return isReady(event) || (event.name == "snapshot" && event.data["ready"] == true)
	})
	require.True(t, ready, "the stack must load; events: %v", eventNames(events))
	for _, op := range []string{"stack export", "preview"} {
		runner.mu.Lock()
		commands := runner.ran(op)
		runner.mu.Unlock()
		require.NotEmpty(t, commands, "the pipeline must run pulumi %s", op)
		for _, command := range commands {
			assert.Contains(t, command, "--stack dev", "pulumi %s must name the stack the shell shows", op)
		}
	}

	cancel()
	require.NoError(t, wait())
}

// TestWebShellNamesNoStackForAStateFile (#2088): a state or plan file is not a
// Pulumi stack the dashboard selected, so the page shows no tab bar even when
// --stack is given.
//
//nolint:paralleltest // startWebPassphraseProject changes cwd, env, global config, the Pulumi runner, and the web hook.
func TestWebShellNamesNoStackForAStateFile(t *testing.T) {
	origin, cookie, _, cancel, wait := startWebPassphraseProject(t, false, &startupPreviewRunner{},
		"--pulumi-state", overviewFixture(t, "state-no-changes.json"), "--stack", "dev")

	assert.NotContains(t, webShell(t, origin, cookie), singleStackMeta)

	cancel()
	require.NoError(t, wait())
}

// TestWebProjectSourceNamesNoStackForTerraform (#2088): Terraform has no
// Pulumi stack to name. Not parallel: t.Chdir changes the process directory.
func TestWebProjectSourceNamesNoStackForTerraform(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.tf"), []byte("terraform {}\n"), 0o600))
	t.Chdir(dir)

	stack, err := checkWebProjectSource(context.Background(), overviewParams{stack: "dev"})
	require.NoError(t, err)
	assert.Empty(t, stack)
}
