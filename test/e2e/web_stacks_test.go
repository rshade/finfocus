//go:build e2e_web

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
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

// TestWebUISingleStackTab (#2088): one stack shows as a lone selected tab and
// keeps #/<view> addresses, with no stack list request or activation.
//
//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUISingleStackTab(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	states := t.TempDir()
	copyWebState(t, filepath.Join(f.root, "test/e2e/fixtures/web/estimate-picker-state.json"), states, "dev")
	plan := filepath.Join(f.root, "testdata/overview/plan-no-changes.json")
	s := f.start(t, plan, false, "WEB_STATE_ONLY=true", "WEB_STACKS=dev", "WEB_STATE_DIR="+states)
	// Listen before navigation: boot is where a stack request would appear.
	var requestsMu sync.Mutex
	var stackRequests []string
	page := openDashboardPage(t, browser, s, playwright.ColorSchemeLight, func(page playwright.Page) {
		page.OnRequest(func(request playwright.Request) {
			if strings.Contains(request.URL(), "/api/stacks") || strings.Contains(request.URL(), "/s/") {
				requestsMu.Lock()
				defer requestsMu.Unlock()
				stackRequests = append(stackRequests, request.Method()+" "+request.URL())
			}
		})
	})

	webWait(t, page, `() => location.hash === '#/overview' && `+webTableHas("existing"))
	tabs, err := page.Evaluate(`() => [...document.querySelectorAll('[role="tab"]')].map(t => ({
		name: t.querySelector('.stack-tab-name')?.textContent,
		selected: t.getAttribute('aria-selected'),
		href: t.getAttribute('href'),
	}))`)
	require.NoError(t, err)
	assert.Equal(t, []any{map[string]any{"name": "dev", "selected": "true", "href": "#/overview"}}, tabs)
	label, err := page.Locator("#stack-label").TextContent()
	require.NoError(t, err)
	assert.Equal(t, "dev", label)
	issues, err := page.Evaluate(webContrastScript)
	require.NoError(t, err)
	assert.Empty(t, issues, "lone tab contrast")
	webScreenshot(t, page, "stacks-single", false)

	require.NoError(t, page.Locator(`.app-nav a[data-route="cost"]`).Click())
	webWait(t, page, `() => location.hash === '#/cost' && `+
		`document.querySelector('[role="tab"]')?.getAttribute('href') === '#/cost'`)
	requestsMu.Lock()
	defer requestsMu.Unlock()
	assert.Empty(t, stackRequests, "one stack makes no stack list request, activation, or /s/ request")
}

// webTabMarker reads a stack tab's marker: its text, its kind, the glyph CSS
// draws before it, and the tab's accessible name.
func webTabMarker(t *testing.T, page playwright.Page, selector string) map[string]any {
	t.Helper()
	raw, err := page.Evaluate(`selector => {
		const tab = document.querySelector(selector);
		const marker = tab.querySelector('.stack-tab-state');
		return {
			text: marker.textContent,
			kind: marker.dataset.kind ?? '',
			glyph: getComputedStyle(marker, '::before').content,
			label: tab.getAttribute('aria-label') ?? '',
		};
	}`, selector)
	require.NoError(t, err)
	marker, ok := raw.(map[string]any)
	require.True(t, ok)
	return marker
}

// openEnrichmentGate releases every resource the fixture plugin holds back.
func openEnrichmentGate(t *testing.T, gate string) {
	t.Helper()
	for _, name := range []string{"first", "rest"} {
		require.NoError(t, os.WriteFile(filepath.Join(gate, name), nil, 0o600))
	}
}

func webTabLabelIs(selector, label string) string {
	return `(document.querySelector('` + selector + `')?.getAttribute('aria-label') ?? '') === '` + label + `'`
}

// TestWebUIStackTabLoadingMarker (#2098): a stack that is still pricing says
// "loading" on its tab, in text and accessible name and without the warning
// glyph, single or multi, and the marker clears once the stack is ready. The
// enrichment gate holds every stack in loading until the test opens it.
//
//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUIStackTabLoadingMarker(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	plan := filepath.Join(f.root, "testdata/overview/plan-no-changes.json")
	loading := func(name string) map[string]any {
		return map[string]any{"text": "loading", "kind": "progress", "glyph": "none", "label": name + ", loading"}
	}
	none := map[string]any{"text": "", "kind": "", "glyph": "none", "label": ""}
	const dev, prod, lone = `[role="tab"][data-stack="dev"]`, `[role="tab"][data-stack="prod"]`, `#stack-tab-single`

	for _, scheme := range []*playwright.ColorScheme{playwright.ColorSchemeLight, playwright.ColorSchemeDark} {
		states := t.TempDir()
		copyWebState(t, filepath.Join(f.root, "test/e2e/fixtures/web/estimate-picker-state.json"), states, "dev")
		copyWebState(t, filepath.Join(f.root, "testdata/overview/state-no-changes.json"), states, "prod")

		gate := t.TempDir()
		s := f.start(t, plan, false, "WEB_STATE_ONLY=true", "WEB_STACKS=dev,prod",
			"WEB_STATE_DIR="+states, "WEB_ENRICHMENT_GATE="+gate)
		page := webPage(t, browser, s, scheme)
		webWait(t, page, webOnStack("dev")+` && `+webTableHas("existing")+` && `+webTabLabelIs(dev, "dev, loading"))
		assert.Equal(t, loading("dev"), webTabMarker(t, page, dev))
		assert.Equal(t, none, webTabMarker(t, page, prod), "a stack not opened yet has no marker")

		webTab(t, page, `[role="tab"][aria-selected="true"]`)
		require.NoError(t, page.Keyboard().Press("ArrowRight"))
		webWait(t, page, webOnStack("prod")+` && `+webTableHas("web-1")+
			` && `+webTabLabelIs(prod, "prod, loading")+` && `+webTabLabelIs(dev, "dev, loading"))
		assert.Equal(t, loading("dev"), webTabMarker(t, page, dev), "a stack loading in the background")
		assert.Equal(t, loading("prod"), webTabMarker(t, page, prod))
		issues, err := page.Evaluate(webContrastScript)
		require.NoError(t, err)
		assert.Empty(t, issues, "selected and unselected loading tab contrast (%s)", *scheme)
		webScreenshot(t, page, "stacks-loading-"+string(*scheme), false)

		openEnrichmentGate(t, gate)
		webWait(t, page, `() => `+webTabLabelIs(prod, "")+` && `+webTabLabelIs(dev, ""))
		assert.Equal(t, none, webTabMarker(t, page, prod), "a ready stack has no marker")
		assert.Equal(t, none, webTabMarker(t, page, dev), "a stack that finished in the background has no marker")

		gate = t.TempDir()
		s = f.start(t, plan, false, "WEB_STATE_ONLY=true", "WEB_STACKS=dev",
			"WEB_STATE_DIR="+states, "WEB_ENRICHMENT_GATE="+gate)
		page = webPage(t, browser, s, scheme)
		webWait(t, page, `() => location.hash === '#/overview' && `+webTableHas("existing")+
			` && `+webTabLabelIs(lone, "dev, loading"))
		assert.Equal(t, loading("dev"), webTabMarker(t, page, lone), "the lone tab uses the same marker")
		issues, err = page.Evaluate(webContrastScript)
		require.NoError(t, err)
		assert.Empty(t, issues, "lone loading tab contrast (%s)", *scheme)

		openEnrichmentGate(t, gate)
		webWait(t, page, `() => `+webTabLabelIs(lone, ""))
		assert.Equal(t, none, webTabMarker(t, page, lone))
	}
}
