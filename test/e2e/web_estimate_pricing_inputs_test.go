//go:build e2e_web

package e2e

import (
	"path/filepath"
	"testing"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func estimateInputKeys(t *testing.T, page playwright.Page) []string {
	t.Helper()
	labels, err := page.Locator(".estimate-input").EvaluateAll(`els => els.map(e => e.dataset.key)`)
	require.NoError(t, err)
	var keys []string
	for _, label := range labels.([]any) {
		keys = append(keys, label.(string))
	}
	return keys
}

//nolint:paralleltest // Builds and executes local fixture binaries; shares a browser driver.
func TestWebUIEstimatePricingInputs(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	state := filepath.Join(f.root, "test/e2e/fixtures/web/estimate-inputs-state.json")
	plan := filepath.Join(f.root, "test/e2e/fixtures/web/estimate-inputs-plan.json")
	s := f.start(t, plan, false, "WEB_AUTO_DETECT=true", "WEB_STATE="+state)
	page := webPage(t, browser, s, playwright.ColorSchemeLight)
	webRoute(t, page, "estimate")
	webWait(
		t,
		page,
		`() => document.querySelector('.estimate-table')?.getAttribute('aria-busy') === 'false' && document.querySelector('.estimate-input') !== null`,
	)

	assert.Equal(t, []string{"instanceType", "tenancy"}, estimateInputKeys(t, page),
		"pricing inputs first, advanced properties hidden")
	details, err := page.Locator(".estimate-details").TextContent()
	require.NoError(t, err)
	assert.Contains(t, details, "arn:aws:ec2:us-east-1:123456789012:instance/i-inputs")
	count, err := page.Locator(".estimate-details input").Count()
	require.NoError(t, err)
	assert.Zero(t, count, "identity metadata has no input")

	webType(t, page, "[aria-label='Current instanceType']", "t3.large")
	require.NoError(t, page.Keyboard().Press("Enter"))
	webWait(t, page, `() => document.querySelector('.totals-bar')?.textContent.includes('↑')`)

	require.NoError(t, page.Locator("#estimate-show-advanced").Check())
	assert.Equal(t, []string{"instanceType", "tenancy", "ami", "subnetId"}, estimateInputKeys(t, page))
	edited, err := page.Locator("[aria-label='Current instanceType']").InputValue()
	require.NoError(t, err)
	assert.Equal(t, "t3.large", edited, "the toggle keeps the edit")
	require.NoError(t, page.Locator("#estimate-show-advanced").Uncheck())
	assert.Equal(t, []string{"instanceType", "tenancy"}, estimateInputKeys(t, page))

	response, err := page.Evaluate(`async () => {
		const r = await fetch('/api/estimate/recalculate', {
			method: 'POST', headers: {'Content-Type': 'application/json'},
			body: JSON.stringify({urn: 'urn:pulumi:dev::inputs::aws:ec2/instance:Instance::web', overrides: {arn: 'x'}}),
		});
		return {status: r.status, body: await r.json()};
	}`)
	require.NoError(t, err)
	got := response.(map[string]any)
	assert.EqualValues(t, 400, got["status"])
	assert.Equal(t, "identity_override", got["body"].(map[string]any)["code"])
}
