//go:build e2e_web

package e2e

import (
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const carbonRequest = `{"start":"2026-01-01T00:00:00Z","end":"2026-04-01T00:00:00Z",` +
	`"method":"CARBON_ACCOUNTING_METHOD_MARKET_BASED","scope":"CARBON_EMISSIONS_SCOPE_TOTAL","regions":["us-east-1"]}`

const carbonReport = `{"request":` + carbonRequest + `,"records":[` +
	`{"id":"r1","provider":"aws","source":"aws-customer-carbon-footprint-tool","accountId":"111122223333",` +
	`"region":"us-east-1","service":"EC2","start":"2026-01-01T00:00:00Z","end":"2026-02-01T00:00:00Z",` +
	`"method":"CARBON_ACCOUNTING_METHOD_MARKET_BASED","scope":"CARBON_EMISSIONS_SCOPE_TOTAL",` +
	`"modelVersion":"v3.0.0","value":12.5,"unit":"gCO2e","retrievedAt":"2026-03-02T10:00:00Z"},` +
	`{"id":"r2","provider":"aws","source":"aws-customer-carbon-footprint-tool","accountId":"111122223333",` +
	`"region":"us-east-1","service":"S3","start":"2026-02-01T00:00:00Z","end":"2026-03-01T00:00:00Z",` +
	`"method":"CARBON_ACCOUNTING_METHOD_MARKET_BASED","scope":"CARBON_EMISSIONS_SCOPE_TOTAL",` +
	`"modelVersion":"v3.0.0","value":0,"unit":"gCO2e","retrievedAt":"2026-03-02T10:00:00Z"},` +
	`{"id":"r3","provider":"aws","source":"aws-customer-carbon-footprint-tool","accountId":"111122223333",` +
	`"region":"us-east-1","service":"RDS","start":"2026-03-01T00:00:00Z","end":"2026-04-01T00:00:00Z",` +
	`"method":"CARBON_ACCOUNTING_METHOD_MARKET_BASED","scope":"CARBON_EMISSIONS_SCOPE_TOTAL",` +
	`"modelVersion":"v3.0.0","unit":"gCO2e","retrievedAt":"2026-03-02T10:00:00Z"}],"totalGrams":12.5}`

const carbonEmpty = `{"request":` + carbonRequest + `,"records":[],"totalGrams":null}`

const carbonDisabled = `{"error":"Historical carbon reporting is disabled or unsupported. Install aws-ce and enable ` +
	`FINFOCUS_AWS_CARBON_ENABLED=true or plugins.aws-ce.carbon_enabled=true, then restart.","code":"carbon_disabled"}`

//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUICarbonReport(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	plan := filepath.Join(f.root, "test/e2e/fixtures/web/estimate-picker-plan.json")
	s := f.start(t, plan, false)
	for _, scheme := range []*playwright.ColorScheme{playwright.ColorSchemeLight, playwright.ColorSchemeDark} {
		page := webPage(t, browser, s, scheme)
		var scenario atomic.Int32
		require.NoError(t, page.Route("**/api/carbon/actual/query", func(route playwright.Route) {
			status, body := 200, carbonReport
			switch scenario.Load() {
			case 1:
				body = carbonEmpty
			case 2:
				status, body = 503, carbonDisabled
			}
			assert.NoError(t, route.Fulfill(playwright.RouteFulfillOptions{
				Status: playwright.Int(status), ContentType: playwright.String("application/json"), Body: body,
			}))
		}))
		webRoute(t, page, "carbon")
		for _, width := range []int{1280, 390} {
			require.NoError(t, page.SetViewportSize(width, 900))
			name := string(*scheme) + "-" + strconv.Itoa(width)
			for _, field := range []struct{ selector, value string }{
				{"#carbon-start", "2026-01-01"}, {"#carbon-end", "2026-04-01"}, {"#carbon-regions", "us-east-1"},
			} {
				webType(t, page, field.selector, field.value)
			}
			scenario.Store(0)
			require.NoError(t, page.Keyboard().Press("Enter"))
			webWait(t, page, `() => document.querySelector('.carbon-table')?.getAttribute('aria-busy') === 'false' && document.querySelectorAll('.carbon-table tbody tr').length === 3`)
			text, err := page.Locator("#view").TextContent()
			require.NoError(t, err)
			assert.Contains(t, text, "Sum of returned available records:12.5 gCO2e")
			assert.Contains(t, text, "Missing")
			assert.Contains(t, text, "0 gCO2e")
			assert.Contains(t, text, "aws-customer-carbon-footprint-tool")
			assert.Contains(t, text, "v3.0.0")
			assertCarbonLayout(t, page)
			webScreenshot(t, page, "carbon-report-"+name, true)

			scenario.Store(1)
			require.NoError(t, page.Locator("form button[type=submit]").Click())
			webWait(t, page, `() => document.querySelector('.carbon-table')?.getAttribute('aria-busy') === 'false' && document.querySelector('.table-wrap')?.hidden === true`)
			text, err = page.Locator("#view").TextContent()
			require.NoError(t, err)
			assert.Contains(t, text, "us-east-1")
			assert.Contains(t, text, "The total is unavailable, not zero")
			webScreenshot(t, page, "carbon-empty-"+name, true)

			scenario.Store(2)
			require.NoError(t, page.Locator("form button[type=submit]").Click())
			webWait(t, page, `() => document.querySelector('#carbon-error')?.hidden === false`)
			text, err = page.Locator("#view").TextContent()
			require.NoError(t, err)
			assert.Contains(t, text, "FINFOCUS_AWS_CARBON_ENABLED=true")
			assert.NotContains(t, text, "aws-customer-carbon-footprint-tool")
			assertCarbonLayout(t, page)
			webScreenshot(t, page, "carbon-disabled-"+name, true)
		}
	}
}

func assertCarbonLayout(t *testing.T, page playwright.Page) {
	t.Helper()
	overflow, err := page.Evaluate(`() => document.documentElement.scrollWidth > window.innerWidth`)
	require.NoError(t, err)
	assert.Equal(t, false, overflow, "carbon view fits the viewport")
	issues, err := page.Evaluate(webContrastScript)
	require.NoError(t, err)
	assert.Empty(t, issues)
}
