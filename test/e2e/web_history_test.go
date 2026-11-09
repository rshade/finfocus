//go:build e2e_web

package e2e

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const historyURN = "urn:pulumi:dev::web-fixture::aws:ec2/instance:Instance::test-instance"

const historyState = `{
  "version": 3,
  "deployment": {
    "resources": [
      {
        "urn": "urn:pulumi:dev::web-fixture::pulumi:pulumi:Stack::web-fixture-dev",
        "type": "pulumi:pulumi:Stack",
        "custom": false
      },
      {
        "urn": "urn:pulumi:dev::web-fixture::aws:ec2/instance:Instance::test-instance",
        "type": "aws:ec2/instance:Instance",
        "custom": true,
        "id": "i-test",
        "created": "2026-06-15T00:00:00Z",
        "inputs": {"instanceType": "t3.micro", "ami": "ami-12345678"},
        "outputs": {"instanceType": "t3.micro", "ami": "ami-12345678"}
      }
    ]
  }
}
`

func seedCostHistory(t *testing.T, root, home string) {
	t.Helper()
	dir := filepath.Join(home, "history")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		"go", "run", "./internal/history/seedhistory",
		filepath.Join(dir, "web-fixture@dev.history.db"),
		"web-fixture", "dev", historyURN,
	)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "seed cost history: %s", output)
}

func historyString(t *testing.T, doc map[string]any, path ...string) string {
	t.Helper()
	var current any = doc
	for _, key := range path {
		next, ok := current.(map[string]any)
		require.Truef(t, ok, "%s is not an object", strings.Join(path, "."))
		current = next[key]
	}
	text, ok := current.(string)
	require.Truef(t, ok, "%s is not a string (%T)", strings.Join(path, "."), current)
	return text
}

func historyLoaded(t *testing.T, page playwright.Page) {
	t.Helper()
	webWait(t, page, `() => {
		const series = document.querySelector('.history-series tbody tr');
		const totals = document.querySelector('#history-diff-totals');
		const chart = document.querySelector('.history-chart');
		return !!series && !!totals && !!chart && chart.getAttribute('aria-hidden') === 'true';
	}`)
}

func assertHistoryLayout(t *testing.T, page playwright.Page) {
	t.Helper()
	overflow, err := page.Evaluate(`() => document.documentElement.scrollWidth > window.innerWidth + 1`)
	require.NoError(t, err)
	assert.Equal(t, false, overflow, "cost history fits the viewport")
	issues, err := page.Evaluate(webContrastScript)
	require.NoError(t, err)
	assert.Empty(t, issues)
}

// The Cost view and both detail panels draw collected cost history from the
// display strings. The fixture seeds the timeline the dashboard reads.
//
//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUICostHistory(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	plan := filepath.Join(t.TempDir(), "empty-plan.json")
	require.NoError(t, os.WriteFile(plan, []byte(`{"steps":[]}`), 0o600))
	state := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(state, []byte(historyState), 0o600))
	s := f.start(t, plan, false, "WEB_STATE_ONLY=true", "WEB_STATE="+state, "WEB_BUDGET=120")
	seedCostHistory(t, f.root, s.home)

	view := s.query(t, "/api/history/view", nil)
	require.Equal(t, "ok", view["status"])
	diff := s.query(t, "/api/history/diff?from=v35&to=v36", nil)
	require.Equal(t, "ok", diff["status"])
	resource := s.query(t, "/api/history/resource?id="+url.QueryEscape(historyURN), nil)
	require.Equal(t, "ok", resource["status"])
	budget := historyString(t, view, "display", "budget")
	display, ok := view["display"].(map[string]any)
	require.True(t, ok)
	snapshots, ok := display["snapshots"].([]any)
	require.True(t, ok)
	require.Len(t, snapshots, 2)
	totals := make([]string, 0, len(snapshots))
	dates := make([]string, 0, len(snapshots))
	for _, item := range snapshots {
		row := item.(map[string]any)
		totals = append(totals, row["total"].(string))
		dates = append(dates, row["date"].(string))
	}
	points, ok := resource["display"].(map[string]any)["points"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, points)
	pointAmount := points[0].(map[string]any)["amount"].(string)
	pointDate := points[0].(map[string]any)["date"].(string)
	delta := historyString(t, diff, "display", "delta")
	require.Truef(t, strings.HasPrefix(delta, "+"), "seeded diff increases: %s", delta)
	fromTotal := historyString(t, diff, "display", "from")
	toTotal := historyString(t, diff, "display", "to")

	for _, scheme := range []*playwright.ColorScheme{playwright.ColorSchemeLight, playwright.ColorSchemeDark} {
		page := webPage(t, browser, s, scheme)
		webWait(t, page, `() => document.querySelector('.overview-table tbody .resource-link') !== null`)
		require.NoError(t, page.Locator(".resource-link").First().Click())
		webWait(t, page, `() => document.querySelector('.detail-panel .history-points tbody tr') !== null`)
		overviewHistory, err := page.Locator(".detail-panel .history-points").TextContent()
		require.NoError(t, err)
		assert.Contains(t, overviewHistory, pointDate)
		assert.Contains(t, overviewHistory, pointAmount)
		assert.NotContains(t, overviewHistory, "finfocus cost history collect")
		headings, err := page.Locator(".detail-panel h4").AllTextContents()
		require.NoError(t, err)
		require.NotEmpty(t, headings)
		assert.Equal(t, "Cost history", headings[len(headings)-1])

		webRoute(t, page, "cost")
		historyLoaded(t, page)
		series, err := page.Locator(".history-series").TextContent()
		require.NoError(t, err)
		for _, total := range totals {
			assert.Contains(t, series, total)
		}
		for _, date := range dates {
			assert.Contains(t, series, date)
		}
		budgetText, err := page.Locator(".history-budget").TextContent()
		require.NoError(t, err)
		assert.Contains(t, budgetText, "Budget line: "+budget)
		annotations, err := page.Locator(".history-annotations").TextContent()
		require.NoError(t, err)
		assert.Contains(t, annotations, "baseline")
		assert.Contains(t, annotations, "resized")
		diffText, err := page.Locator("#history-diff-body").TextContent()
		require.NoError(t, err)
		assert.Contains(t, diffText, delta)
		assert.Contains(t, diffText, "increase")
		assert.Contains(t, diffText, fromTotal)
		assert.Contains(t, diffText, toTotal)
		assert.Contains(t, diffText, historyURN)
		hidden, err := page.Locator(".history-chart").GetAttribute("aria-hidden")
		require.NoError(t, err)
		assert.Equal(t, "true", hidden)
		ids := []string{"history-from", "history-to", "history-currency", "history-diff-from", "history-diff-to"}
		for _, id := range ids {
			label, labelErr := page.Locator("label[for=" + id + "]").TextContent()
			require.NoError(t, labelErr)
			assert.NotEmpty(t, strings.TrimSpace(label))
		}
		order, err := page.Evaluate(`() => {
			const table = document.querySelector('#view .overview-table');
			const from = document.querySelector('#history-from');
			const series = document.querySelector('.history-series');
			const diff = document.querySelector('#history-diff-from');
			const following = Node.DOCUMENT_POSITION_FOLLOWING;
			return Boolean(table && from && series && diff)
				&& (table.compareDocumentPosition(from) & following) !== 0
				&& (from.compareDocumentPosition(series) & following) !== 0
				&& (series.compareDocumentPosition(diff) & following) !== 0;
		}`)
		require.NoError(t, err)
		assert.Equal(t, true, order)

		require.NoError(t, page.Locator(".detail-button").First().Click())
		webWait(t, page, `() => document.querySelector('dialog .history-points tbody tr') !== null`)
		costHistory, err := page.Locator("dialog .history-points").TextContent()
		require.NoError(t, err)
		assert.Contains(t, costHistory, pointAmount)
		closeDetails := page.Locator("dialog button").Filter(playwright.LocatorFilterOptions{HasText: "Close details"})
		require.NoError(t, closeDetails.Click())
		webWait(t, page, `() => document.querySelector('dialog')?.open !== true`)

		for _, width := range []int{1280, 390} {
			require.NoError(t, page.SetViewportSize(width, 900))
			historyLoaded(t, page)
			assertHistoryLayout(t, page)
			webScreenshot(t, page, "history-"+string(*scheme)+"-"+strconv.Itoa(width), true)
		}
		if scheme == playwright.ColorSchemeLight {
			require.NoError(t, page.SetViewportSize(1280, 900))
			webTab(t, page, "#history-from")
		}
	}
}
