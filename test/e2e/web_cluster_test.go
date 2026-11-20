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

// The Cluster view runs against the real endpoints. The fixture plugin provides
// neither usage_stats nor allocation, so allocation and the policy panel answer
// "unavailable", and an empty label key is the CLI's own --group-by rejection.
//
//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUICluster(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	plan := filepath.Join(f.root, "test/e2e/fixtures/web/estimate-picker-plan.json")
	s := f.start(t, plan, false)
	for _, scheme := range []*playwright.ColorScheme{playwright.ColorSchemeLight, playwright.ColorSchemeDark} {
		page := webPage(t, browser, s, scheme)
		webRoute(t, page, "cluster")
		for _, width := range []int{1280, 390} {
			require.NoError(t, page.SetViewportSize(width, 900))
			name := string(*scheme) + "-" + strconv.Itoa(width)

			_, err := page.Locator("#cluster-group-by").
				SelectOption(playwright.SelectOptionValues{Values: playwright.StringSlice("namespace")})
			require.NoError(t, err)
			require.NoError(t, page.Locator("form button[type=submit]").Click())
			webWait(t, page, `() => ['unavailable', 'invalid', 'failed', 'done'].includes(document.querySelector('#cluster-status')?.dataset.state)`)
			state, err := page.Evaluate(`() => document.querySelector('#cluster-status').dataset.state`)
			require.NoError(t, err)
			require.Equal(t, "unavailable", state)
			text, err := page.Locator(".cluster-unavailable").TextContent()
			require.NoError(t, err)
			assert.Contains(t, text, "usage_stats")
			assert.Contains(t, text, "finfocus plugin install kubernetes")
			hidden, err := page.Evaluate(`() => document.querySelector('#cluster-error').hidden`)
			require.NoError(t, err)
			assert.Equal(t, true, hidden, "unavailable is an explanation, not an error")
			assertClusterLayout(t, page)
			webScreenshot(t, page, "cluster-unavailable-"+name, true)

			_, err = page.Locator("#cluster-group-by").
				SelectOption(playwright.SelectOptionValues{Values: playwright.StringSlice("label")})
			require.NoError(t, err)
			require.NoError(t, page.Locator("#cluster-label-key").Fill(""))
			require.NoError(t, page.Locator("form button[type=submit]").Click())
			webWait(t, page, `() => document.querySelector('#cluster-status')?.dataset.state === 'invalid'`)
			status, err := page.Locator("#cluster-status").TextContent()
			require.NoError(t, err)
			assert.Contains(t, status, `invalid --group-by "label:"`)
			assertClusterLayout(t, page)
			webScreenshot(t, page, "cluster-invalid-"+name, true)

			require.NoError(t, page.Locator(".cluster-policy summary").Click())
			webWait(t, page, `() => document.querySelector('.cluster-policy-status')?.dataset.state === 'unavailable'`)
			policy, err := page.Locator(".cluster-policy-status").TextContent()
			require.NoError(t, err)
			assert.Contains(t, policy, "allocation")
			assertClusterLayout(t, page)
			webScreenshot(t, page, "cluster-policy-"+name, true)
			require.NoError(t, page.Locator(".cluster-policy summary").Click())
		}
	}
}

func assertClusterLayout(t *testing.T, page playwright.Page) {
	t.Helper()
	overflow, err := page.Evaluate(`() => document.documentElement.scrollWidth > window.innerWidth`)
	require.NoError(t, err)
	assert.Equal(t, false, overflow, "cluster view fits the viewport")
	issues, err := page.Evaluate(webContrastScript)
	require.NoError(t, err)
	assert.Empty(t, issues)
}
