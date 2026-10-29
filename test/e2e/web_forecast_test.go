//go:build e2e_web

package e2e

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forecastSpendCSV is 180 days of spend ending yesterday, with a weekend dip.
func forecastSpendCSV() []byte {
	var b strings.Builder
	b.WriteString("date,value\n")
	last := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -1)
	first := last.AddDate(0, 0, -179)
	for i := range 180 {
		d := first.AddDate(0, 0, i)
		v := 50 + 0.1*float64(i) + float64((i*37)%11)/10
		if d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
			v -= 15
		}
		fmt.Fprintf(&b, "%s,%.2f\n", d.Format("2006-01-02"), v)
	}
	return []byte(b.String())
}

// The Forecast view runs against the real endpoint: growth from the fixture
// plugin's prices, then the statistical model from an uploaded CSV, then the
// server's own validation error.
//
//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUIForecast(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	plan := filepath.Join(f.root, "test/e2e/fixtures/web/estimate-picker-plan.json")
	s := f.start(t, plan, false)
	for _, scheme := range []*playwright.ColorScheme{playwright.ColorSchemeLight, playwright.ColorSchemeDark} {
		page := webPage(t, browser, s, scheme)
		webRoute(t, page, "forecast")
		for _, width := range []int{1280, 390} {
			require.NoError(t, page.SetViewportSize(width, 900))
			name := string(*scheme) + "-" + strconv.Itoa(width)

			_, err := page.Locator("#forecast-model").
				SelectOption(playwright.SelectOptionValues{Values: playwright.StringSlice("growth")})
			require.NoError(t, err)
			webType(t, page, "#forecast-months", "3")
			require.NoError(t, page.Locator("form button[type=submit]").Click())
			webWait(t, page, `() => document.querySelector('.forecast-table')?.getAttribute('aria-busy') === 'false' && `+
				`(document.querySelectorAll('.forecast-table tbody tr').length === 4 || document.querySelector('#forecast-error')?.hidden === false)`)
			text, err := page.Locator("#view").TextContent()
			require.NoError(t, err)
			require.NotContains(t, text, "Unable to run the forecast", "growth forecast failed: %s", text)
			assert.Contains(t, text, "Plugin growth")
			assert.Contains(t, text, "4 months projected.")
			assertForecastLayout(t, page)
			webScreenshot(t, page, "forecast-growth-"+name, true)

			_, err = page.Locator("#forecast-model").
				SelectOption(playwright.SelectOptionValues{Values: playwright.StringSlice("arima")})
			require.NoError(t, err)
			require.NoError(t, page.Locator("#forecast-spend").SetInputFiles([]playwright.InputFile{
				{Name: "spend.csv", MimeType: "text/csv", Buffer: forecastSpendCSV()},
			}))
			require.NoError(t, page.Locator("form button[type=submit]").Click())
			webWait(t, page, `() => document.querySelector('.forecast-table')?.getAttribute('aria-busy') === 'false' && `+
				`(document.querySelector('.forecast-inputs')?.hidden === false || document.querySelector('#forecast-error')?.hidden === false)`)
			text, err = page.Locator("#view").TextContent()
			require.NoError(t, err)
			require.NotContains(t, text, "Unable to run the forecast", "statistical forecast failed: %s", text)
			assert.Contains(t, text, "bounds at 95% confidence")
			assert.Contains(t, text, "Spend history: 180 points")
			headers, err := page.Locator(".forecast-table thead th").AllTextContents()
			require.NoError(t, err)
			assert.Equal(t, []string{"Month", "Forecast", "Lower", "Upper", "Baseline"}, headers)
			assertForecastLayout(t, page)
			webScreenshot(t, page, "forecast-arima-"+name, true)

			require.NoError(t, page.Locator("#forecast-spend").SetInputFiles([]playwright.InputFile{}))
			require.NoError(t, page.Locator("form button[type=submit]").Click())
			webWait(t, page, `() => document.querySelector('#forecast-error')?.hidden === false`)
			text, err = page.Locator("#forecast-error").TextContent()
			require.NoError(t, err)
			assert.Equal(t, "--model arima requires --spend-history", text)
			hidden, err := page.Evaluate(`() => document.querySelector('.forecast-table').closest('.table-wrap').hidden`)
			require.NoError(t, err)
			assert.Equal(t, true, hidden, "a failed query clears the earlier table")
			assertForecastLayout(t, page)
			webScreenshot(t, page, "forecast-error-"+name, true)
		}
	}
}

func assertForecastLayout(t *testing.T, page playwright.Page) {
	t.Helper()
	overflow, err := page.Evaluate(`() => document.documentElement.scrollWidth > window.innerWidth`)
	require.NoError(t, err)
	assert.Equal(t, false, overflow, "forecast view fits the viewport")
	issues, err := page.Evaluate(webContrastScript)
	require.NoError(t, err)
	assert.Empty(t, issues)
}
