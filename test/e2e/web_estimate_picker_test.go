//go:build e2e_web

package e2e

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUIEstimatePicker(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	state := filepath.Join(f.root, "test/e2e/fixtures/web/estimate-picker-state.json")
	plan := filepath.Join(f.root, "test/e2e/fixtures/web/estimate-picker-plan.json")
	s := f.start(t, plan, false, "WEB_AUTO_DETECT=true", "WEB_STATE="+state)
	page := webPage(t, browser, s, playwright.ColorSchemeLight)
	webRoute(t, page, "estimate")
	webWait(
		t,
		page,
		`() => document.querySelector('.estimate-table')?.getAttribute('aria-busy') === 'false' && document.querySelector('.estimate-input') !== null`,
	)
	for _, width := range []int{1280, 390} {
		require.NoError(t, page.SetViewportSize(width, 900))
		count, countErr := page.Locator("#estimate-resource option").Count()
		require.NoError(t, countErr)
		assert.Equal(t, 2, count, "only existing and planned instances are visible")
		selected, selectErr := page.Locator("#estimate-resource").InputValue()
		require.NoError(t, selectErr)
		assert.NotContains(t, selected, "pulumi:providers")
		assert.NotContains(t, selected, "pulumi:pulumi:Stack")
		webType(t, page, "[aria-label='Current instanceType']", "t3.large")
		require.NoError(t, page.Locator("#estimate-show-all").Check())
		webWait(
			t,
			page,
			`() => document.querySelector('#estimate-resource')?.options.length === 4 && document.querySelector('.estimate-table')?.getAttribute('aria-busy') === 'false'`,
		)
		retained, retainErr := page.Locator("#estimate-resource").InputValue()
		require.NoError(t, retainErr)
		assert.Equal(t, selected, retained)
		edited, editErr := page.Locator("[aria-label='Current instanceType']").InputValue()
		require.NoError(t, editErr)
		assert.Equal(t, "t3.large", edited)
		webScreenshot(t, page, "estimate-picker-all-"+strconv.Itoa(width), true)
		require.NoError(t, page.Locator("#estimate-show-all").Uncheck())
		webWait(
			t,
			page,
			`() => document.querySelector('#estimate-resource')?.options.length === 2 && document.querySelector('.estimate-table')?.getAttribute('aria-busy') === 'false'`,
		)
		webScreenshot(t, page, "estimate-picker-default-"+strconv.Itoa(width), true)
		overflow, overflowErr := page.Evaluate(`() => document.documentElement.scrollWidth > window.innerWidth`)
		require.NoError(t, overflowErr)
		assert.Equal(t, false, overflow, "picker toolbar fits narrow viewport")
	}
}
