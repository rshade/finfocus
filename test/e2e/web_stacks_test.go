//go:build e2e_web

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func copyWebState(t *testing.T, from, dir, stack string) {
	t.Helper()
	data, err := os.ReadFile(from)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, stack+".json"), data, 0o600))
}

// webStackStates reads GET /api/stacks through the page, so it carries the
// page's session cookie and origin.
func webStackStates(t *testing.T, page playwright.Page) map[string]string {
	t.Helper()
	raw, err := page.Evaluate(`() => fetch('/api/stacks', {credentials: 'same-origin'}).then(r => r.json())`)
	require.NoError(t, err)
	body, ok := raw.(map[string]any)
	require.True(t, ok)
	list, ok := body["stacks"].([]any)
	require.True(t, ok)
	states := map[string]string{}
	for _, item := range list {
		entry, entryOK := item.(map[string]any)
		require.True(t, entryOK)
		states[entry["name"].(string)] = entry["state"].(string)
	}
	return states
}

func webOnStack(stack string) string {
	return `() => location.hash === '#/s/` + stack + `/overview' && ` +
		`document.querySelector('[role="tab"][aria-selected="true"]')?.dataset.stack === '` + stack + `' && ` +
		`document.querySelector('#view .view-loading') === null`
}

// webFocusedStack names the stack tab that holds keyboard focus, or "" when
// focus is anywhere else. A tab switch must leave focus on the new tab.
func webFocusedStack(t *testing.T, page playwright.Page) string {
	t.Helper()
	raw, err := page.Evaluate(`() => document.activeElement?.closest('[role="tab"]')?.dataset.stack ?? ''`)
	require.NoError(t, err)
	name, ok := raw.(string)
	require.True(t, ok)
	return name
}

// webTableHas waits for a resource to be in the overview table: an empty table
// already has a row, so a row alone does not mean the stack has loaded.
func webTableHas(text string) string {
	return `document.querySelector('.overview-table')?.textContent.includes('` + text + `')`
}

//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUIMultiStackTabs(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	states := t.TempDir()
	copyWebState(t, filepath.Join(f.root, "test/e2e/fixtures/web/estimate-picker-state.json"), states, "dev")
	copyWebState(t, filepath.Join(f.root, "testdata/overview/state-no-changes.json"), states, "prod")
	plan := filepath.Join(f.root, "testdata/overview/plan-no-changes.json")
	s := f.start(t, plan, false,
		"WEB_STATE_ONLY=true", "WEB_STACKS=dev,prod,bad", "WEB_FAIL_STACK=bad", "WEB_STATE_DIR="+states)
	page := webPage(t, browser, s, playwright.ColorSchemeLight)

	webWait(t, page, webOnStack("dev")+` && `+webTableHas("existing"))
	tabs, err := page.Evaluate(`() => [...document.querySelectorAll('[role="tab"]')].map(t => t.dataset.stack)`)
	require.NoError(t, err)
	assert.Equal(t, []any{"dev", "prod", "bad"}, tabs)
	label, err := page.Locator("#stack-label").TextContent()
	require.NoError(t, err)
	assert.Equal(t, "dev", label)
	devText, err := page.Locator(".overview-table").TextContent()
	require.NoError(t, err)
	assert.Contains(t, devText, "existing")
	assert.NotContains(t, devText, "web-1", "dev shows only its own resources")
	require.Eventually(t, func() bool {
		return assert.ObjectsAreEqual(
			map[string]string{"dev": "ready", "prod": "idle", "bad": "idle"}, webStackStates(t, page))
	}, 30*time.Second, 100*time.Millisecond, "only the selected stack has started")
	webScreenshot(t, page, "stacks-dev", false)

	webTab(t, page, `[role="tab"][aria-selected="true"]`)
	require.NoError(t, page.Keyboard().Press("ArrowRight"))
	webWait(t, page, webOnStack("prod")+` && `+webTableHas("web-1"))
	prodText, err := page.Locator(".overview-table").TextContent()
	require.NoError(t, err)
	assert.Contains(t, prodText, "web-1")
	assert.NotContains(t, prodText, "existing", "prod shows only its own resources")
	label, err = page.Locator("#stack-label").TextContent()
	require.NoError(t, err)
	assert.Equal(t, "prod", label)

	assert.Equal(t, "prod", webFocusedStack(t, page), "focus stays on the selected tab")
	require.NoError(t, page.Keyboard().Press("End"))
	webWait(t, page, webOnStack("bad")+
		` && document.querySelector('[role="tab"][data-stack="bad"]')?.getAttribute('aria-label') === 'bad, failed'`)
	failure, err := page.Locator("#view").TextContent()
	require.NoError(t, err)
	assert.Contains(t, failure, "Overview loading failed")
	assert.NotContains(t, failure, "fixture failure", "the browser sees the safe label only")
	issues, err := page.Evaluate(webContrastScript)
	require.NoError(t, err)
	assert.Empty(t, issues, "failed tab contrast")
	webScreenshot(t, page, "stacks-bad", false)

	assert.Equal(t, "bad", webFocusedStack(t, page), "focus stays on the selected tab")
	require.NoError(t, page.Keyboard().Press("Home"))
	webWait(t, page, webOnStack("dev")+` && `+webTableHas("existing"))
	assert.Equal(t, "dev", webFocusedStack(t, page), "focus stays on the selected tab")
	again, err := page.Locator(".overview-table").TextContent()
	require.NoError(t, err)
	assert.Contains(t, again, "existing")
	assert.NotContains(t, again, "web-1")
	require.Eventually(t, func() bool {
		return assert.ObjectsAreEqual(
			map[string]string{"dev": "ready", "prod": "ready", "bad": "failed"}, webStackStates(t, page))
	}, 30*time.Second, 100*time.Millisecond, "prod must finish loading and bad must stay failed")
	issues, err = page.Evaluate(webContrastScript)
	require.NoError(t, err)
	assert.Empty(t, issues, "unselected failed tab contrast")

	_, err = page.Evaluate(`() => { location.hash = '#/s/bad/cost'; }`)
	require.NoError(t, err)
	webWait(t, page, `() => !document.querySelector('#stack-notice')?.hidden && `+
		`document.querySelector('#stack-notice').textContent.includes('bad failed to load.')`)
	link, err := page.Locator("#stack-notice a").GetAttribute("href")
	require.NoError(t, err)
	assert.Equal(t, "#/s/bad/overview", link, "the notice points to the failed stack's Overview")
	issues, err = page.Evaluate(webContrastScript)
	require.NoError(t, err)
	assert.Empty(t, issues, "stack notice contrast")
	require.NoError(t, page.Locator("#stack-notice a").Click())
	webWait(t, page, `() => location.hash === '#/s/bad/overview' && document.querySelector('#stack-notice').hidden`)
	_, err = page.Evaluate(`() => { location.hash = '#/s/dev/overview'; }`)
	require.NoError(t, err)
	webWait(t, page, webOnStack("dev")+` && `+webTableHas("existing"))

	for _, width := range []int{1280, 390} {
		require.NoError(t, page.SetViewportSize(width, 900))
		offenders, overflowErr := page.Evaluate(`() => [...document.querySelectorAll('body *')]
			.filter(e => e.getBoundingClientRect().right > window.innerWidth + 1 && !e.closest('.table-scroll, [role="region"][aria-label*="Scrollable"]'))
			.map(e => e.tagName.toLowerCase() + '.' + e.className + '#' + e.id).slice(0, 8)`)
		require.NoError(t, overflowErr)
		assert.Empty(t, offenders, "nothing in the header or view spills past a %dpx viewport", width)
	}
}
