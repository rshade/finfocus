//go:build e2e_web

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cliAnomalies struct {
	Considered int `json:"considered"`
	Anomalies  []struct {
		Resource string `json:"resource"`
		Type     string `json:"type"`
		Currency string `json:"currency"`
	} `json:"anomalies"`
}

// driftFixture returns a deployed state holding the fixture instance and a
// preview with no steps, so the instance stays an active resource with both
// actual and projected cost.
func driftFixture(t *testing.T, root string) (state, plan string) {
	t.Helper()
	state = deployedState(t, filepath.Join(root, "testdata/simple-plan.json"))
	plan = filepath.Join(t.TempDir(), "empty-plan.json")
	require.NoError(t, os.WriteFile(plan, []byte(`{"steps":[]}`), 0o600))
	return state, plan
}

func anomaliesLoaded(t *testing.T, page playwright.Page) {
	t.Helper()
	webWait(t, page, `() => document.querySelector('.anomalies-table')?.getAttribute('aria-busy') === 'false'`)
}

func assertAnomaliesLayout(t *testing.T, page playwright.Page) {
	t.Helper()
	overflow, err := page.Evaluate(`() => document.documentElement.scrollWidth > window.innerWidth`)
	require.NoError(t, err)
	assert.Equal(t, false, overflow, "anomalies view fits the viewport")
	issues, err := page.Evaluate(webContrastScript)
	require.NoError(t, err)
	assert.Empty(t, issues)
}

// The Anomalies page lists the same resources as `finfocus cost anomalies`
// for the same state and window, reached from the header by keyboard. Drift
// compares actual spend with deployed resources, so both run state-only.
//
//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUIAnomalies(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	state, plan := driftFixture(t, f.root)
	s := f.start(t, plan, false, "WEB_STATE_ONLY=true", "WEB_STATE="+state)

	var want cliAnomalies
	out := f.cli(t, s, "cost", "anomalies", "--pulumi-state", state,
		"--from", "2026-10-01", "--to", "2026-10-07", "--output", "json")
	require.NoError(t, json.Unmarshal(out, &want))
	require.NotEmpty(t, want.Anomalies, "the fixture prices half a month of spend over six days, which drifts")

	for _, scheme := range []*playwright.ColorScheme{playwright.ColorSchemeLight, playwright.ColorSchemeDark} {
		page := webPage(t, browser, s, scheme)
		webWait(t, page, `() => document.querySelector('.overview-table tbody tr') !== null`)
		webRoute(t, page, "anomalies")
		for _, width := range []int{1280, 390} {
			require.NoError(t, page.SetViewportSize(width, 900))
			name := string(*scheme) + "-" + strconv.Itoa(width)
			anomaliesLoaded(t, page)

			heading, err := page.Locator("#view h2").TextContent()
			require.NoError(t, err)
			assert.Equal(t, "Anomalies", heading)

			headers, err := page.Locator(".anomalies-table thead th").AllTextContents()
			require.NoError(t, err)
			assert.Equal(t,
				[]string{"Resource", "Type", "Period", "Baseline", "Observed", "Deviation", "Drift %", "Currency"},
				headers)

			resources, err := page.Locator(".anomalies-table tbody tr td:first-child").AllTextContents()
			require.NoError(t, err)
			wantResources := make([]string, 0, len(want.Anomalies))
			for _, a := range want.Anomalies {
				wantResources = append(wantResources, a.Resource)
			}
			assert.Equal(t, wantResources, resources, "the page lists the CLI's anomalies in the CLI's order")

			status, err := page.Locator("#view [role=status]").First().TextContent()
			require.NoError(t, err)
			assert.Equal(t, strconv.Itoa(len(want.Anomalies))+" of "+strconv.Itoa(want.Considered)+" resources",
				strings.TrimSpace(status))

			amounts, err := page.Locator(".anomalies-table tbody tr:first-child td.col-num").AllTextContents()
			require.NoError(t, err)
			require.Len(t, amounts, 4)
			assert.Regexp(t, `^\$[\d,]+\.\d{2}$`, amounts[0])
			assert.Regexp(t, `^[+-]\$[\d,]+\.\d{2}$`, amounts[2])
			assert.Regexp(t, `^[+-]\d+\.\d%$`, amounts[3])

			assertAnomaliesLayout(t, page)
			webScreenshot(t, page, "anomalies-"+name, true)
		}
	}
}

// The scrollable table region takes keyboard focus, so a narrow viewport can
// still be scrolled without a pointer.
//
//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUIAnomaliesKeyboardScroll(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	state, plan := driftFixture(t, f.root)
	s := f.start(t, plan, false, "WEB_STATE_ONLY=true", "WEB_STATE="+state)
	page := webPage(t, browser, s, playwright.ColorSchemeLight)
	webWait(t, page, `() => document.querySelector('.overview-table tbody tr') !== null`)
	webRoute(t, page, "anomalies")
	anomaliesLoaded(t, page)

	webTab(t, page, "#view .table-wrap")
}

// With nothing drifting, the page says so and still reports how many
// resources were considered.
//
//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUIAnomaliesEmptyState(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	plan := filepath.Join(f.root, "testdata/simple-plan.json")
	s := f.start(t, plan, false)
	page := webPage(t, browser, s, playwright.ColorSchemeLight)
	webWait(t, page, `() => document.querySelector('.overview-table tbody tr') !== null`)
	webRoute(t, page, "anomalies")
	anomaliesLoaded(t, page)

	status, err := page.Locator("#view [role=status]").First().TextContent()
	require.NoError(t, err)
	assert.Equal(t, "No anomalies found (0 of 1 resources).", strings.TrimSpace(status))
	hidden, err := page.Evaluate(`() => document.querySelector('.anomalies-table').closest('.table-wrap').hidden`)
	require.NoError(t, err)
	assert.Equal(t, true, hidden)
	assertAnomaliesLayout(t, page)
	webScreenshot(t, page, "anomalies-empty", true)
}
