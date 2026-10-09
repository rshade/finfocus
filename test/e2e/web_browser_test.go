//go:build e2e_web

package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// webOutput prevents process logging from racing with assertions.
type webOutput struct {
	mu   sync.Mutex
	data bytes.Buffer
}

func (b *webOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.data.Write(p)
}
func (b *webOutput) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.data.String() }

type webFixture struct{ binary, root, plugin string }
type webProcess struct {
	origin, bootstrap, home, project string
	env                              []string
	client                           *http.Client
	logs                             *webOutput
}

func webRequired(t *testing.T, message string) {
	t.Helper()
	if os.Getenv("FINFOCUS_WEB_E2E_REQUIRED") == "1" {
		t.Fatal(message)
	}
	t.Skip(message)
}

func newWebFixture(t *testing.T) webFixture {
	t.Helper()
	binary := os.Getenv("FINFOCUS_BINARY")
	if binary == "" {
		webRequired(t, "FINFOCUS_BINARY is missing; run make test-e2e-web")
	}
	binary, err := filepath.Abs(binary)
	require.NoError(t, err)
	if _, err = os.Stat(binary); err != nil {
		webRequired(t, "FINFOCUS_BINARY is unavailable; run make test-e2e-web")
	}
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	source, err := os.ReadFile("fixtures/web/plugin.go.txt")
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "main.go")
	require.NoError(t, os.WriteFile(path, source, 0o600))
	plugin := filepath.Join(t.TempDir(), "finfocus-plugin-webfixture")
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", plugin, path)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "compile local plugin: %s", output)
	return webFixture{binary: binary, root: root, plugin: plugin}
}

// webEnvironment clears credentials only for child processes. The user's stores
// and environment are untouched; all configuration paths live under t.TempDir.
func webEnvironment(home, project string) []string {
	env := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		blocked := false
		for _, prefix := range []string{"AWS_", "AZURE_", "GOOGLE_", "GCP_", "PULUMI_", "FINFOCUS_", "WEB_"} {
			if strings.HasPrefix(key, prefix) {
				blocked = true
			}
		}
		if !blocked {
			env = append(env, entry)
		}
	}
	return append(
		env,
		"FINFOCUS_HOME="+home,
		"FINFOCUS_PROJECT_DIR="+project,
		"FINFOCUS_HISTORY_ENABLED=false",
		"FINFOCUS_CACHE_ENABLED=false",
		"FINFOCUS_SKIP_MIGRATION_CHECK=1",
		"AWS_EC2_METADATA_DISABLED=true",
	)
}

func (f webFixture) start(t *testing.T, source string, encrypted bool, fixtureEnv ...string) *webProcess {
	t.Helper()
	home, project := t.TempDir(), t.TempDir()
	require.NoError(
		t,
		os.WriteFile(filepath.Join(project, "Pulumi.yaml"), []byte("name: web-fixture\nruntime: yaml\n"), 0o600),
	)
	config := []byte("scoring:\n  enabled: true\n  plugin: webfixture\n  identifier_mode: raw\n")
	require.NoError(t, os.WriteFile(filepath.Join(home, "config.yaml"), config, 0o600))
	plugin, err := os.ReadFile(f.plugin)
	require.NoError(t, err)
	for _, name := range []string{"webfixture", "webalternate"} {
		dir := filepath.Join(home, "plugins", name, "0.1.0")
		require.NoError(t, os.MkdirAll(dir, 0o750))
		binary := "finfocus-plugin-" + name
		require.NoError(t, os.WriteFile(filepath.Join(dir, binary), plugin, 0o755))
		manifest := map[string]any{
			"name":                name,
			"version":             "0.1.0",
			"description":         "local web fixture",
			"author":              "FinFocus tests",
			"supported_providers": []string{"aws", "gcp", "kubernetes"},
			"protocols":           []string{"grpc"},
			"binary":              binary,
		}
		data, marshalErr := json.Marshal(manifest)
		require.NoError(t, marshalErr)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "plugin.manifest.json"), data, 0o600))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(project, ".finfocus"), 0o750))
	dismissed := []byte(
		`{"version":1,"dismissals":{"fixture-dismissed":{"recommendation_id":"fixture-dismissed","status":"dismissed","reason":"BUSINESS_CONSTRAINT","dismissed_at":"2026-10-01T00:00:00Z","expires_at":null,"last_known":{"description":"Archived test-instance recommendation","estimated_savings":1,"currency":"USD","type":"RIGHTSIZE","resource_id":"test-instance-archived"},"history":[]}}}`,
	)
	require.NoError(t, os.WriteFile(filepath.Join(project, ".finfocus", "dismissed.json"), dismissed, 0o600))
	script, readErr := os.ReadFile("fixtures/web/pulumi.sh")
	require.NoError(t, readErr)
	require.NoError(t, os.WriteFile(filepath.Join(project, "pulumi"), script, 0o755))
	emptyState := filepath.Join(project, "empty-state.json")
	require.NoError(t, os.WriteFile(emptyState, []byte(`{"version":3,"deployment":{"resources":[]}}`), 0o600))
	env := append(
		webEnvironment(home, project),
		"PATH="+project+string(os.PathListSeparator)+os.Getenv("PATH"),
		"WEB_STATE="+emptyState,
		"WEB_PLAN="+source,
	)
	env = append(env, fixtureEnv...)
	args := []string{"--web", "--no-browser", "--from", "2026-10-01", "--to", "2026-10-07"}
	if encrypted {
		require.NoError(
			t,
			os.WriteFile(filepath.Join(project, "Pulumi.dev.yaml"), []byte("encryptionsalt: v1:fixture\n"), 0o600),
		)
		env = append(
			env,
			"PATH="+project+string(os.PathListSeparator)+os.Getenv("PATH"),
			"WEB_ENCRYPTED=true",
			"WEB_STATE="+filepath.Join(f.root, "testdata/overview/state-no-changes.json"),
			"WEB_PLAN="+filepath.Join(f.root, "testdata/overview/plan-no-changes.json"),
		)
		args = append(args, "--stack", "dev")
	} else if !slices.Contains(fixtureEnv, "WEB_AUTO_DETECT=true") {
		args = append(args, "--pulumi-json", source)
	}
	cmd := exec.Command(f.binary, args...)
	cmd.Dir = project
	cmd.Env = env
	logs := &webOutput{}
	cmd.Stderr = logs
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	started := time.Now()
	require.NoError(t, cmd.Start())
	urls := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			_, _ = logs.Write([]byte(line + "\n"))
			if strings.HasPrefix(line, "http://127.0.0.1:") {
				select {
				case urls <- line:
				default:
				}
			}
		}
	}()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case processErr := <-done:
			assert.NoErrorf(t, processErr, "server shutdown: %s", logs.String())
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			t.Error("web server did not stop after interrupt")
		}
	})
	var bootstrap string
	select {
	case bootstrap = <-urls:
	case processErr := <-done:
		t.Fatalf("web launch failed: %v\n%s", processErr, logs.String())
	case <-time.After(30 * time.Second):
		t.Fatalf("web launch timed out: %s", logs.String())
	}
	parsed, err := url.Parse(bootstrap)
	require.NoError(t, err)
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar, Timeout: 30 * time.Second}
	resp, err := client.Get(bootstrap)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, 200, resp.StatusCode)
	t.Logf("real server URL available after %s", time.Since(started))
	return &webProcess{
		origin:    parsed.Scheme + "://" + parsed.Host,
		bootstrap: bootstrap,
		home:      home,
		project:   project,
		env:       env,
		client:    client,
		logs:      logs,
	}
}

func (s *webProcess) query(t *testing.T, path string, body any) map[string]any {
	t.Helper()
	method := http.MethodGet
	var reader io.Reader
	if body != nil {
		method = http.MethodPost
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(context.Background(), method, s.origin+path, reader)
	require.NoError(t, err)
	request.Header.Set("Origin", s.origin)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equalf(t, 200, response.StatusCode, "%s: %s", path, data)
	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))
	return result
}

func webWait(t *testing.T, page playwright.Page, expression string) {
	t.Helper()
	_, err := page.WaitForFunction(
		expression,
		nil,
		playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(30000)},
	)
	if err != nil {
		text, _ := page.Locator("body").TextContent()
		t.Logf("browser DOM at wait failure: %s", text)
	}
	require.NoError(t, err)
}

func webTab(t *testing.T, page playwright.Page, selector string) {
	t.Helper()
	for i := 0; i < 120; i++ {
		focused, err := page.Evaluate(`selector => document.activeElement.matches(selector)`, selector)
		require.NoError(t, err)
		if focused == true {
			visible, evalErr := page.Evaluate(
				`() => {const s=getComputedStyle(document.activeElement);return s.outlineStyle!=='none'&&parseFloat(s.outlineWidth)>0}`,
			)
			require.NoError(t, evalErr)
			assert.Equal(t, true, visible, "visible keyboard focus on %s", selector)
			return
		}
		require.NoError(t, page.Keyboard().Press("Tab"))
	}
	t.Fatalf("keyboard cannot reach %s", selector)
}

func webType(t *testing.T, page playwright.Page, selector, value string) {
	t.Helper()
	webTab(t, page, selector)
	require.NoError(t, page.Keyboard().Press("ControlOrMeta+A"))
	require.NoError(t, page.Keyboard().Type(value))
}

func webRoute(t *testing.T, page playwright.Page, route string) {
	t.Helper()
	webTab(t, page, `a[data-route="`+route+`"]`)
	require.NoError(t, page.Keyboard().Press("Enter"))
	webWait(t, page, `() => location.hash === '#/`+route+`' && document.querySelector('#view h2') !== null`)
}

func webBrowser(t *testing.T) playwright.Browser {
	t.Helper()
	pw, err := playwright.Run()
	if err != nil {
		webRequired(t, "Playwright driver is missing; run make test-e2e-web: "+err.Error())
	}
	t.Cleanup(func() { assert.NoError(t, pw.Stop()) })
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(true)})
	if err != nil {
		if strings.Contains(err.Error(), "Executable doesn't exist") {
			webRequired(t, "Chromium is missing; run make test-e2e-web: "+err.Error())
		}
		require.NoError(t, err)
	}
	t.Cleanup(func() { assert.NoError(t, browser.Close()) })
	return browser
}

func webPage(t *testing.T, browser playwright.Browser, s *webProcess, scheme *playwright.ColorScheme) playwright.Page {
	t.Helper()
	ctx, err := browser.NewContext(playwright.BrowserNewContextOptions{ColorScheme: scheme})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, ctx.Close()) })
	page, err := ctx.NewPage()
	require.NoError(t, err)
	_, err = page.Goto(s.bootstrap)
	require.NoError(t, err)
	return page
}

//nolint:gocognit,paralleltest // Ordered browser journey shares a driver; fixture parity keeps comparisons together.
func TestWebUIBrowser(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	s := f.start(t, filepath.Join(f.root, "testdata/simple-plan.json"), false)
	page := webPage(t, browser, s, playwright.ColorSchemeLight)
	webWait(
		t,
		page,
		`() => document.querySelector('.overview-table tbody tr button') && document.querySelector('.phase-checklist').hidden`,
	)
	t.Run("keyboard_all_views", func(t *testing.T) {
		var requestMu sync.Mutex
		requests := map[string]bool{}
		page.OnRequest(func(r playwright.Request) {
			if r.Method() != "POST" {
				return
			}
			body, err := r.PostData()
			if err != nil {
				return
			}
			requestMu.Lock()
			requests[r.URL()+" "+body] = true
			requestMu.Unlock()
		})
		webType(t, page, "#overview-filter", "test-instance")
		webWait(t, page, `() => document.querySelector('.overview-table tbody').textContent.includes('test-instance')`)
		webTab(t, page, ".sort-button")
		require.NoError(t, page.Keyboard().Press("Enter"))
		webTab(t, page, ".overview-table tbody .resource-link")
		require.NoError(t, page.Keyboard().Press("Enter"))
		webWait(t, page, `() => document.querySelector('.detail-panel').hidden === false`)
		webTab(t, page, ".detail-panel button")
		require.NoError(t, page.Keyboard().Press("Enter"))
		focused, err := page.Evaluate(`() => document.activeElement.classList.contains('resource-link')`)
		require.NoError(t, err)
		assert.Equal(t, true, focused)
		webRoute(t, page, "cost")
		webWait(
			t,
			page,
			`() => document.querySelector('#cost-group') && document.querySelector('.overview-table tbody button')`,
		)
		webType(t, page, "#cost-filter", "test-instance")
		require.NoError(t, page.Keyboard().Press("Enter"))
		webTab(t, page, "#cost-sort")
		require.NoError(t, page.Keyboard().Press("ArrowDown"))
		webWait(t, page, `() => document.querySelector(".overview-table").getAttribute("aria-busy") === "false"`)
		webTab(t, page, ".overview-table tbody button")
		require.NoError(t, page.Keyboard().Press("Enter"))
		webWait(t, page, `() => document.querySelector('dialog[open]') !== null`)
		require.NoError(t, page.Keyboard().Press("Escape"))
		webWait(t, page, `() => document.activeElement.classList.contains('detail-button')`)
		for _, group := range []string{"", "resource", "type", "provider", "daily", "monthly"} {
			webTab(t, page, "#cost-group")
			require.NoError(t, page.Keyboard().Press("Home"))
			steps := map[string]int{"": 0, "resource": 1, "type": 2, "provider": 3, "daily": 4, "monthly": 5}[group]
			for i := 0; i < steps; i++ {
				require.NoError(t, page.Keyboard().Press("ArrowDown"))
			}
			require.NoError(t, page.Keyboard().Press("Tab"))
			webWait(
				t,
				page,
				`() => document.querySelector('#cost-group').value === '`+group+`' && document.querySelector('.overview-table').getAttribute('aria-busy') === 'false'`,
			)
			result := s.query(t, "/api/cost/actual/query", map[string]any{"groupBy": group})
			assert.NotEmpty(t, result["rows"], group)
		}
		requestMu.Lock()
		for _, group := range []string{"", "resource", "type", "provider", "daily", "monthly"} {
			found := false
			for request := range requests {
				if strings.Contains(request, "/api/cost/actual/query ") &&
					strings.Contains(request, `"groupBy":"`+group+`"`) {
					found = true
				}
			}
			assert.True(t, found, "native group selector refetched %q", group)
		}
		requestMu.Unlock()

		webRoute(t, page, "recommendations")
		webWait(t, page, `() => document.querySelector('.overview-table tbody button') !== null`)
		webType(t, page, "#recommendations-filter", "test-instance")
		require.NoError(t, page.Keyboard().Press("Enter"))
		webTab(t, page, "#recommendations-sort")
		require.NoError(t, page.Keyboard().Press("ArrowDown"))
		storeBefore, readErr := os.ReadFile(filepath.Join(s.project, ".finfocus", "dismissed.json"))
		require.NoError(t, readErr)
		before := s.query(t, "/api/recommendations/query", map[string]any{})
		webTab(t, page, "#recommendations-dismissed")
		require.NoError(t, page.Keyboard().Press("Space"))
		webWait(
			t,
			page,
			`() => document.querySelector('#recommendations-dismissed').checked && document.querySelector('.overview-table').getAttribute('aria-busy') === 'false'`,
		)
		after := s.query(t, "/api/recommendations/query", map[string]any{"includeDismissed": true})
		assert.Greater(t, len(after["items"].([]any)), len(before["items"].([]any)))
		webWait(
			t,
			page,
			`() => document.querySelector('.overview-table tbody').textContent.includes('Archived test-instance')`,
		)
		store, err := os.ReadFile(filepath.Join(s.project, ".finfocus", "dismissed.json"))
		require.NoError(t, err)
		assert.Equal(t, storeBefore, store, "include-dismissed never mutates the dismissal store")
		assert.NotContains(t, string(store), `"undismissed"`)
		requestMu.Lock()
		refetched := false
		for request := range requests {
			if strings.Contains(request, "/api/recommendations/query ") &&
				strings.Contains(request, `"includeDismissed":true`) {
				refetched = true
			}
		}
		requestMu.Unlock()
		assert.True(t, refetched, "native include-dismissed refetched")

		webTab(t, page, "#recommendations-dismissed")
		require.NoError(t, page.Keyboard().Press("Space"))
		webWait(
			t,
			page,
			`() => !document.querySelector('#recommendations-dismissed').checked && !document.querySelector('.overview-table tbody').textContent.includes('Archived test-instance') && document.querySelector('.overview-table').getAttribute('aria-busy') === 'false'`,
		)

		webTab(t, page, ".overview-table tbody button")
		require.NoError(t, page.Keyboard().Press("Enter"))
		webWait(t, page, `() => document.querySelector('dialog[open]')?.textContent.includes('Risk0.20')`)
		require.NoError(t, page.Keyboard().Press("Escape"))
		webWait(t, page, `() => document.activeElement.classList.contains('detail-button')`)
		webRoute(t, page, "estimate")
		webWait(t, page, `() => document.querySelector('[aria-label="Current instanceType"]') !== null`)
		original, err := page.Locator("[aria-label='Estimate comparison']").TextContent()
		require.NoError(t, err)
		webType(t, page, "[aria-label='Current instanceType']", "t3.large")
		require.NoError(t, page.Keyboard().Press("Enter"))
		webWait(
			t,
			page,
			`() => document.querySelector('[aria-label="Estimate comparison"]').textContent.includes('40.00') || document.querySelector('[aria-label="Estimate comparison"]').textContent.includes('120.00')`,
		)
		modified, err := page.Locator("[aria-label='Estimate comparison']").TextContent()
		require.NoError(t, err)
		assert.NotEqual(t, original, modified)
		webScreenshot(t, page, "estimate-after-edit", true)
		focused, err = page.Evaluate(
			`() => document.activeElement.getAttribute('aria-label') === 'Current instanceType'`,
		)
		require.NoError(t, err)
		assert.Equal(t, true, focused)
		webTab(t, page, "#estimate-mode")
		require.NoError(t, page.Keyboard().Press("End"))
		require.NoError(t, page.Keyboard().Press("Tab"))
		webWait(t, page, `() => document.querySelector('.overview-table').getAttribute('aria-busy') === 'false'`)
		mode, err := page.Locator("#estimate-mode").InputValue()
		require.NoError(t, err)
		assert.NotEmpty(t, mode)
		selected, err := page.Locator("[aria-label='Estimate comparison']").TextContent()
		require.NoError(t, err)
		assert.NotEqual(t, modified, selected, "pricing selection changes real plugin estimate")
	})
	t.Run("contrast_light_dark", func(t *testing.T) {
		for _, scheme := range []*playwright.ColorScheme{playwright.ColorSchemeLight, playwright.ColorSchemeDark} {
			p := webPage(t, browser, s, scheme)
			for _, route := range []string{"overview", "cost", "recommendations", "estimate"} {
				webRoute(t, p, route)
				webWait(
					t,
					p,
					`() => document.querySelector('.overview-table tbody tr') !== null && document.querySelector('.overview-table').getAttribute('aria-busy') !== 'true'`,
				)
				issues, err := p.Evaluate(webContrastScript)
				require.NoError(t, err)
				assert.Empty(t, issues, "%s %s contrast", *scheme, route)
				webScreenshot(t, p, route+"-"+string(*scheme), false)
				if route == "estimate" {
					webScreenshot(t, p, route+"-"+string(*scheme)+"-full", true)
				}
			}
		}
	})
}

// webScreenshot records optional native browser artifacts without changing interactions.
func webScreenshot(t *testing.T, page playwright.Page, name string, fullPage bool) {
	t.Helper()
	directory := os.Getenv("FINFOCUS_WEB_SCREENSHOT_DIR")
	if directory == "" {
		return
	}
	require.NoError(t, os.MkdirAll(directory, 0o750))
	path := filepath.Join(directory, name+".png")
	_, err := page.Screenshot(playwright.PageScreenshotOptions{
		Path: playwright.String(path), FullPage: playwright.Bool(fullPage),
	})
	require.NoError(t, err)
	t.Logf("native browser screenshot: %s", path)
}

// Native computed colors cover the actual DOM in each view and color scheme.
const webContrastScript = `() => {
 const rgb = s => (s.match(/[\d.]+/g)||[]).map(Number);
 const luminance = c => c.slice(0,3).map(v=>{v/=255;return v<=.04045?v/12.92:((v+.055)/1.055)**2.4}).reduce((a,v,i)=>a+v*[.2126,.7152,.0722][i],0);
 const ratio = (a,b) => {const x=luminance(rgb(a)),y=luminance(rgb(b));return (Math.max(x,y)+.05)/(Math.min(x,y)+.05)};
 const background = e => {for(let n=e;n;n=n.parentElement){const s=getComputedStyle(n).backgroundColor;if(rgb(s).length<4||rgb(s)[3]!==0)return s}return getComputedStyle(document.body).backgroundColor};
 const issues=[];
 for(const e of document.querySelectorAll('body *')){
  if(!e.getClientRects().length||e.closest('[hidden]')||e.matches('option,svg,svg *,script,style'))continue;
  const s=getComputedStyle(e),bg=background(e);
  const text=[...e.childNodes].some(n=>n.nodeType===3&&n.textContent.trim())||e.matches('input,select');
  if(text&&!e.disabled){const large=parseFloat(s.fontSize)>=24||(parseFloat(s.fontSize)>=18.66&&parseInt(s.fontWeight)>=700);const value=ratio(s.color,bg);if(value+.01<(large?3:4.5))issues.push(e.tagName+':text:'+value.toFixed(2)+':'+e.textContent.slice(0,40))}
  if(e.matches('input,select,button')&&!e.disabled&&s.borderStyle!=='none'&&parseFloat(s.borderWidth)>0){const value=ratio(s.borderColor,background(e.parentElement));if(value+.01<3)issues.push(e.tagName+':border:'+value.toFixed(2))}
 }
 const focused=document.activeElement;if(focused!==document.body){const s=getComputedStyle(focused);if(s.outlineStyle!=='none'&&ratio(s.outlineColor,background(focused.parentElement))+.01<3)issues.push('focus contrast')}
 return issues;
}`

//nolint:paralleltest // Browser and performance acceptance run sequentially for deterministic timing.
func TestWebUIEncryptedRetryAndPreview(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	s := f.start(t, "", true)
	page := webPage(t, browser, s, playwright.ColorSchemeLight)
	for _, preview := range []bool{false, true} {
		if preview {
			webTab(t, page, ".preview-button")
			require.NoError(t, page.Keyboard().Press("Enter"))
		}
		webWait(t, page, `() => document.querySelector('#passphrase-input')?.closest('dialog').open`)
		input := page.Locator("#passphrase-input")
		kind, err := input.GetAttribute("type")
		require.NoError(t, err)
		assert.Equal(t, "password", kind)
		webType(t, page, "#passphrase-input", "web-fixture-wrong")
		require.NoError(t, page.Keyboard().Press("Enter"))
		webWait(
			t,
			page,
			`() => document.querySelector('#passphrase-input').closest('dialog').open && !document.querySelector('#passphrase-error').hidden && document.querySelector('#passphrase-error').textContent.length > 0 && document.querySelector('#passphrase-input').value === '' && !document.querySelector('.passphrase-form button[type="submit"]').disabled`,
		)
		text, err := page.Locator("body").TextContent()
		require.NoError(t, err)
		assert.NotContains(t, text, "web-fixture-wrong")
		webType(t, page, "#passphrase-input", "web-fixture-unlock")
		require.NoError(t, page.Keyboard().Press("Enter"))
		webWait(
			t,
			page,
			`() => !document.querySelector('#passphrase-input').closest('dialog').open && document.querySelector('.overview-table tbody tr button') !== null && !document.querySelector('.preview-button').disabled`,
		)
	}
	assert.NotContains(t, s.logs.String(), "web-fixture-wrong")
	assert.NotContains(t, s.logs.String(), "web-fixture-unlock")
}

//nolint:paralleltest // Browser and performance acceptance run sequentially for deterministic timing.
func TestWebUIPerformance(t *testing.T) {
	f := newWebFixture(t)
	for _, count := range []int{250, 1000} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			steps := make([]map[string]any, count)
			for i := range steps {
				steps[i] = map[string]any{
					"op":     "create",
					"urn":    fmt.Sprintf("urn:pulumi:dev::web-fixture::aws:ec2/instance:Instance::resource-%04d", i),
					"type":   "aws:ec2/instance:Instance",
					"inputs": map[string]any{"instanceType": "t3.micro"},
				}
			}
			data, err := json.Marshal(map[string]any{"steps": steps})
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "plan.json")
			require.NoError(t, os.WriteFile(path, data, 0o600))
			started := time.Now()
			s := f.start(t, path, false)
			response, err := s.client.Get(s.origin + "/api/overview/stream")
			require.NoError(t, err)
			defer response.Body.Close()
			scanner := bufio.NewScanner(response.Body)
			scanner.Buffer(make([]byte, 4096), 8*1024*1024)
			ready := false
			for scanner.Scan() {
				line := scanner.Text()
				if line == "event: ready" ||
					strings.HasPrefix(line, "data:") && strings.Contains(line, `"ready":true`) {
					ready = true
					break
				}
			}
			require.Truef(t, ready, "ready event or snapshot: %s", s.logs.String())
			elapsed := time.Since(started)
			t.Logf("%d resources launch→ready: %s", count, elapsed)
			if count == 250 {
				assert.Less(t, elapsed, 30*time.Second)
			}
			for _, query := range []map[string]any{{"filter": "resource-01"}, {"sort": "cost"}, {"sort": "name"}, {"sort": "type"}, {"sort": "delta"}} {
				started = time.Now()
				result := s.query(t, "/api/overview/query", query)
				elapsed = time.Since(started)
				assert.NotEmpty(t, result["rows"])
				assert.Less(t, elapsed, time.Second)
				t.Logf("%d resources query %v: %s", count, query, elapsed)
			}
		})
	}
}

func (f webFixture) cli(t *testing.T, s *webProcess, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.binary, args...)
	cmd.Dir = s.project
	cmd.Env = s.env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	require.NoErrorf(t, err, "CLI %v: %s", args, stderr.String())
	return output
}

// Real CLI and web transports use three differently sized, scrubbed stacks.
// The typed parity test additionally checks their shared TUI renderers.
//
//nolint:gocognit,paralleltest // Ordered browser journey shares a driver; fixture parity keeps comparisons together.
func TestWebUIThreeFixtureParityAndCluster(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	for _, name := range []string{"state-no-changes.json", "state-mixed-changes.json", "state-cluster-expansion.json"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(f.root, "testdata/overview", name))
			require.NoError(t, err)
			var state struct {
				Deployment struct {
					Resources []struct {
						URN    string         `json:"urn"`
						Type   string         `json:"type"`
						Custom bool           `json:"custom"`
						Inputs map[string]any `json:"inputs"`
					} `json:"resources"`
				} `json:"deployment"`
			}
			require.NoError(t, json.Unmarshal(data, &state))
			steps := []map[string]any{}
			for _, r := range state.Deployment.Resources {
				if !r.Custom || strings.HasPrefix(r.Type, "pulumi:") {
					continue
				}
				if r.Inputs == nil {
					r.Inputs = map[string]any{}
				}
				r.Inputs["instanceType"] = "t3.micro"
				steps = append(steps, map[string]any{"urn": r.URN, "type": r.Type, "op": "create", "inputs": r.Inputs})
			}
			plan, err := json.Marshal(map[string]any{"steps": steps})
			require.NoError(t, err)
			path := filepath.Join(t.TempDir(), "plan.json")
			require.NoError(t, os.WriteFile(path, plan, 0o600))
			s := f.start(t, path, false)
			p := webPage(t, browser, s, playwright.ColorSchemeLight)
			webWait(
				t,
				p,
				`() => document.querySelector('.phase-checklist').hidden && document.querySelector('.overview-table tbody .resource-link') !== null`,
			)
			var overview map[string]any
			require.NoError(
				t,
				json.Unmarshal(
					f.cli(
						t,
						s,
						"overview",
						"--pulumi-json",
						path,
						"--from",
						"2026-10-01",
						"--to",
						"2026-10-07",
						"--output",
						"json",
					),
					&overview,
				),
			)
			expanded := []string{}
			for _, row := range overview["resources"].([]any) {
				expanded = append(expanded, row.(map[string]any)["urn"].(string))
			}
			page := s.query(t, "/api/overview/query", map[string]any{"sort": "name", "expanded": expanded})
			sources := []any{}
			for _, row := range page["rows"].([]any) {
				sources = append(sources, row.(map[string]any)["source"])
			}
			assert.ElementsMatch(t, overview["resources"], sources)
			summary := overview["summary"].(map[string]any)
			totals := page["totals"].(map[string]any)
			for cliKey, webKey := range map[string]string{"totalActualMTD": "totalActual", "projectedMonthly": "totalProjected", "projectedDelta": "totalDelta", "potentialSavings": "totalSavings"} {
				assert.Equal(t, summary[cliKey], totals[webKey], name+" "+cliKey)
			}
			for _, group := range []string{"", "resource", "type", "provider", "daily", "monthly"} {
				args := []string{
					"cost",
					"actual",
					"--pulumi-json",
					path,
					"--from",
					"2026-10-01",
					"--to",
					"2026-10-07",
					"--output",
					"json",
				}
				if group != "" {
					args = append(args, "--group-by", group)
				}
				var actual any
				require.NoError(t, json.Unmarshal(f.cli(t, s, args...), &actual))
				got := s.query(t, "/api/cost/actual/query", map[string]any{"groupBy": group})
				if group == "daily" || group == "monthly" {
					assert.Equal(t, actual, got["results"])
				} else {
					assert.ElementsMatch(t, actual, got["results"])
				}
			}
			var recommendations map[string]any
			require.NoError(
				t,
				json.Unmarshal(
					f.cli(t, s, "cost", "recommendations", "--pulumi-json", path, "--output", "json"),
					&recommendations,
				),
			)
			gotRecs := s.query(t, "/api/recommendations/query", map[string]any{})
			assert.Equal(t, recommendations["summary"], gotRecs["summary"])
			resources := s.query(t, "/api/estimate/resources", nil)["resources"].([]any)
			urn := resources[0].(map[string]any)["id"].(string)
			resource := resources[0].(map[string]any)
			for _, adapter := range []string{"webfixture", "webalternate"} {
				mode, marshalErr := json.Marshal([]string{adapter, "hourly"})
				require.NoError(t, marshalErr)
				gotEstimate := s.query(
					t,
					"/api/estimate/recalculate",
					map[string]any{
						"urn":         urn,
						"pricingMode": string(mode),
						"overrides":   map[string]string{"instanceType": "t3.large"},
					},
				)
				var estimate map[string]any
				require.NoError(
					t,
					json.Unmarshal(
						f.cli(
							t,
							s,
							"cost",
							"estimate",
							"--provider",
							resource["provider"].(string),
							"--resource-type",
							resource["type"].(string),
							"--adapter",
							adapter,
							"--property",
							"instanceType=t3.large",
							"--output",
							"json",
						),
						&estimate,
					),
				)
				// Single-resource CLI has no original properties. Its default rate
				// equals t3.micro; shared acceptance covers exact property deltas.
				for _, key := range []string{"baseline", "modified"} {
					want := estimate[key].(map[string]any)
					got := gotEstimate[key].(map[string]any)
					for _, field := range []string{"hourly", "monthly", "currency", "resourceType"} {
						assert.Equal(t, want[field], got[field], adapter+" "+key+" "+field)
					}
				}
				assert.Equal(t, estimate["totalChange"], gotEstimate["totalChange"])
				assert.NotZero(t, gotEstimate["totalChange"])
			}
			if name == "state-cluster-expansion.json" {
				webWait(t, p, `() => document.querySelector('.cluster-toggle') !== null`)
				webTab(t, p, ".cluster-toggle")
				require.NoError(t, p.Keyboard().Press("Enter"))
				webWait(
					t,
					p,
					`() => document.querySelector('.cluster-toggle').getAttribute('aria-expanded') === 'true'`,
				)
				focused, focusErr := p.Evaluate(`() => document.activeElement.classList.contains('cluster-toggle')`)
				require.NoError(t, focusErr)
				assert.Equal(t, true, focused)
				require.NoError(t, p.Keyboard().Press("Enter"))
				webWait(
					t,
					p,
					`() => document.querySelector('.cluster-toggle').getAttribute('aria-expanded') === 'false'`,
				)
			}
		})
	}
}

//nolint:paralleltest // Deterministic enrichment gates must be released in order.
func TestWebUIProgressiveLoading(t *testing.T) {
	f := newWebFixture(t)
	gate := t.TempDir()
	steps := []map[string]any{}
	for i := range 12 {
		name := fmt.Sprintf("progress-first-%02d", i)
		if i >= 10 {
			name = fmt.Sprintf("progress-rest-%02d", i)
		}
		steps = append(
			steps,
			map[string]any{
				"urn":    "urn:pulumi:test::web::aws:ec2/instance:Instance::" + name,
				"type":   "aws:ec2/instance:Instance",
				"op":     "create",
				"inputs": map[string]any{"instanceType": "t3.micro"},
			},
		)
	}
	data, err := json.Marshal(map[string]any{"steps": steps})
	require.NoError(t, err)
	source := filepath.Join(t.TempDir(), "progressive-plan.json")
	require.NoError(t, os.WriteFile(source, data, 0o600))
	s := f.start(t, source, false, "WEB_ENRICHMENT_GATE="+gate)
	page := webPage(t, webBrowser(t), s, playwright.ColorSchemeLight)
	_, err = page.WaitForFunction(
		`() => !document.querySelector('.phase-checklist').hidden && document.querySelectorAll('.phase[data-status="done"]').length > 0 && document.querySelector('.phase[data-status="active"]') !== null`,
		nil,
		playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(30000)},
	)
	require.NoError(t, err, "visible completed and active loading phases before releasing enrichment")
	visible, visibleErr := page.Locator(".phase-checklist").IsVisible()
	require.NoError(t, visibleErr)
	assert.True(t, visible, "loading checklist is rendered before enrichment is released")
	require.NoError(t, os.WriteFile(filepath.Join(gate, "first"), nil, 0o600))
	webWait(
		t,
		page,
		`() => !document.querySelector('.phase-checklist').hidden && !document.querySelector('.progress-line').hidden && document.querySelector('.progress-line').textContent === 'Enriched 10 of 12 resources' && [...document.querySelectorAll('.overview-table tbody tr')].filter(r=>r.textContent.includes('progress-first') && r.textContent.includes('$')).length === 10`,
	)
	pending, evalErr := page.Evaluate(
		`() => [...document.querySelectorAll('.overview-table tbody tr')].filter(r=>r.textContent.includes('progress-rest')).length === 2 && [...document.querySelectorAll('.overview-table tbody tr')].filter(r=>r.textContent.includes('progress-rest')).every(r=>!r.textContent.includes('$'))`,
	)
	require.NoError(t, evalErr)
	assert.Equal(t, true, pending, "remaining rows have no cost before their gate opens")
	require.NoError(t, os.WriteFile(filepath.Join(gate, "rest"), nil, 0o600))
	webWait(
		t,
		page,
		`() => document.querySelector('.phase-checklist').hidden && document.querySelector('.progress-line').hidden && [...document.querySelectorAll('.overview-table tbody tr')].filter(r=>r.textContent.includes('progress-') && r.textContent.includes('$')).length === 12`,
	)
}

func TestWebUISharedPresentation(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	s := f.start(
		t,
		filepath.Join(f.root, "testdata/simple-plan.json"),
		false,
		"WEB_RICH_PRESENTATION=true",
	)
	page := webPage(t, browser, s, playwright.ColorSchemeLight)
	webWait(
		t,
		page,
		`() => document.querySelector('.overview-table tbody tr button') && document.querySelector('.phase-checklist').hidden`,
	)
	budget, err := page.Locator(".budget-footer").TextContent()
	require.NoError(t, err)
	require.Contains(t, budget, "$75.68 / $100.12")
	require.Contains(t, budget, "75.6%")
	budgetPayload := s.query(t, "/api/overview/budget", nil)
	budgetDisplay := budgetPayload["display"].(map[string]any)
	budgetDetails := budgetDisplay["details"].([]any)
	require.NotEmpty(t, budgetDetails)
	forecast := budgetDetails[0].(map[string]any)["forecastedDisplay"].(string)
	require.NotEmpty(t, forecast)
	require.Contains(t, budget, "Forecasted: "+forecast)
	require.Contains(t, budget, "75% threshold triggered")
	webRoute(t, page, "cost")
	webWait(
		t,
		page,
		`() => document.querySelector('#cost-group') && document.querySelector('.overview-table').getAttribute('aria-busy') === 'false'`,
	)
	summary, err := page.Locator("[aria-label='Actual cost summary']").TextContent()
	require.NoError(t, err)
	cost := s.query(t, "/api/cost/actual/query", map[string]any{})
	display := cost["summaryDisplay"].(map[string]any)
	assert.Contains(t, summary, fmt.Sprintf("Resources: %.0f", display["resourceCount"]))
	assert.Contains(
		t,
		summary,
		fmt.Sprintf("Recommendations: %.0f", display["recommendationCount"]),
	)
	require.NotEmpty(t, display["carbonEquivalency"])
	assert.Contains(t, summary, display["carbonEquivalency"])
	for _, value := range display["providers"].([]any) {
		provider := value.(map[string]any)
		assert.Contains(
			t,
			summary,
			fmt.Sprintf(
				"%s: %s (%s)",
				provider["name"],
				provider["costDisplay"],
				provider["shareDisplay"],
			),
		)
	}
	require.NoError(t, page.Locator(".detail-button").First().Click())
	webWait(
		t,
		page,
		`() => document.querySelector('dialog[open]')?.textContent.includes('Sustainability')`,
	)
	detail, err := page.Locator("dialog[open]").TextContent()
	require.NoError(t, err)
	for _, text := range []string{"Provider: aws", "aws:", "2026-10-01 - 2026-10-07", "$12.3456", "12.23 kgCO2e", "Verify memory before resizing", "RIGHTSIZE"} {
		assert.Contains(t, detail, text)
	}
	require.NoError(t, page.Keyboard().Press("Escape"))
	_, err = page.Locator("#cost-group").
		SelectOption(playwright.SelectOptionValues{Values: playwright.StringSlice("daily")})
	require.NoError(t, err)
	webWait(
		t,
		page,
		`() => document.querySelector('.overview-table').getAttribute('aria-busy') === 'false' && document.querySelector('.overview-table tbody').textContent.includes('aws:$')`,
	)
	webRoute(t, page, "recommendations")
	webWait(
		t,
		page,
		`() => document.querySelector('.overview-table').getAttribute('aria-busy') === 'false' && document.querySelector('.detail-button')`,
	)
	text, err := page.Locator(".overview-table").TextContent()
	require.NoError(t, err)
	assert.Contains(t, text, "Rightsize")
	assert.NotContains(t, text, "RECOMMENDATION_ACTION_TYPE_RIGHTSIZE")
	visible, err := page.Evaluate(
		`() => { const cell=document.querySelector('.recommendations-table tbody td:nth-child(4)');const r=cell.getBoundingClientRect();return r.left>=0 && r.right<=innerWidth; }`,
	)
	require.NoError(t, err)
	assert.Equal(t, true, visible, "savings column fits the native 1280px viewport")
	require.NoError(t, page.Locator(".detail-button").First().Click())
	webWait(
		t,
		page,
		`() => document.querySelector('dialog[open]')?.textContent.includes('Scorer signals')`,
	)
	text, err = page.Locator("dialog[open]").TextContent()
	require.NoError(t, err)
	assert.Contains(t, text, "Risk0.20")
	assert.Contains(t, text, "False positive-")
	assert.NotContains(t, text, `"risk": 0.2`)
}

func TestWebUIFirstDeployEstimates(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	s := f.start(
		t,
		filepath.Join(f.root, "testdata/simple-plan.json"),
		false,
		"WEB_AUTO_DETECT=true",
	)
	page := webPage(t, browser, s, playwright.ColorSchemeLight)
	webWait(
		t,
		page,
		`() => document.querySelector('.overview-table tbody tr button') && document.querySelector('.phase-checklist').hidden`,
	)
	resources := s.query(t, "/api/estimate/resources", nil)
	require.NotEmpty(t, resources["resources"], "first deploy planned resources are selectable")
	actual := s.query(t, "/api/cost/actual/query", map[string]any{})
	assert.Empty(t, actual["rows"], "empty deployed state stays empty for actual costs")
	webRoute(t, page, "estimate")
	webWait(t, page, `() => document.querySelector('[aria-label="Current instanceType"]') !== null`)
	before, err := page.Locator("[aria-label='Estimate comparison']").TextContent()
	require.NoError(t, err)
	webType(t, page, "[aria-label='Current instanceType']", "t3.large")
	require.NoError(t, page.Keyboard().Press("Enter"))
	urn := resources["resources"].([]any)[0].(map[string]any)["id"].(string)
	expected := s.query(
		t,
		"/api/estimate/recalculate",
		map[string]any{"urn": urn, "overrides": map[string]string{"instanceType": "t3.large"}},
	)
	modified := expected["display"].(map[string]any)["modified"].(string)
	encodedModified, err := json.Marshal(modified)
	require.NoError(t, err)
	webWait(
		t,
		page,
		`() => document.querySelector('[aria-label="Estimate comparison"]').textContent.includes(`+string(
			encodedModified,
		)+`)`,
	)
	after, err := page.Locator("[aria-label='Estimate comparison']").TextContent()
	require.NoError(t, err)
	assert.NotEqual(t, before, after)
}

func TestWebUIMixedCurrencyRows(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	steps := []map[string]any{}
	for _, name := range []string{"currency-usd", "currency-eur", "currency-after"} {
		steps = append(
			steps,
			map[string]any{
				"op":     "create",
				"urn":    "urn:pulumi:dev::test::aws:ec2/instance:Instance::" + name,
				"type":   "aws:ec2/instance:Instance",
				"inputs": map[string]any{"instanceType": "t3.micro", "region": "us-east-1"},
			},
		)
	}
	data, err := json.Marshal(map[string]any{"steps": steps})
	require.NoError(t, err)
	plan := filepath.Join(t.TempDir(), "mixed.json")
	require.NoError(t, os.WriteFile(plan, data, 0o600))
	s := f.start(t, plan, false, "WEB_MIXED_CURRENCY=true")
	page := webPage(t, browser, s, playwright.ColorSchemeLight)
	webWait(
		t,
		page,
		`() => document.querySelector('.phase-checklist').hidden && document.querySelectorAll('.resource-link').length === 3`,
	)
	totals, err := page.Locator(".totals-bar").TextContent()
	require.NoError(t, err)
	assert.Contains(t, totals, "Totals unavailable: resources use different currencies.")
	assert.NotContains(t, totals, "$")
	_, err = page.Reload()
	require.NoError(t, err)
	webWait(t, page, `() => document.querySelectorAll('.resource-link').length === 3`)
	totals, err = page.Locator(".totals-bar").TextContent()
	require.NoError(t, err)
	assert.Contains(t, totals, "Totals unavailable")
	assert.NotContains(t, totals, "$")
}

func TestWebUIOverviewPresentation(t *testing.T) {
	f := newWebFixture(t)
	browser := webBrowser(t)
	directory := t.TempDir()
	resources := []map[string]any{}
	steps := []map[string]any{}
	for _, name := range []string{"updated", "active"} {
		urn := "urn:pulumi:dev::test::aws:ec2/instance:Instance::" + name
		inputs := map[string]any{"instanceType": "t3.micro", "region": "us-east-1"}
		resources = append(resources, map[string]any{"urn": urn, "type": "aws:ec2/instance:Instance", "custom": true, "id": "i-" + name, "inputs": inputs})
		old := map[string]any{"urn": urn, "type": "aws:ec2/instance:Instance", "inputs": inputs}
		op := "same"
		after := old
		if name == "updated" {
			op = "update"
			after = map[string]any{"urn": urn, "type": "aws:ec2/instance:Instance", "inputs": map[string]any{"instanceType": "t3.large", "region": "us-east-1"}}
		}
		steps = append(steps, map[string]any{"urn": urn, "type": "aws:ec2/instance:Instance", "op": op, "oldState": old, "newState": after})
	}
	state, err := json.Marshal(map[string]any{"version": 3, "deployment": map[string]any{"resources": resources}})
	require.NoError(t, err)
	plan, err := json.Marshal(map[string]any{"steps": steps})
	require.NoError(t, err)
	statePath, planPath := filepath.Join(directory, "state.json"), filepath.Join(directory, "plan.json")
	require.NoError(t, os.WriteFile(statePath, state, 0o600))
	require.NoError(t, os.WriteFile(planPath, plan, 0o600))
	s := f.start(t, planPath, false, "WEB_AUTO_DETECT=true", "WEB_RICH_PRESENTATION=true", "WEB_STATE="+statePath)
	page := webPage(t, browser, s, playwright.ColorSchemeLight)
	webWait(t, page, `() => document.querySelector('.phase-checklist').hidden && document.querySelectorAll('.resource-link').length === 2`)
	for _, name := range []string{"updated", "active"} {
		urn := "urn:pulumi:dev::test::aws:ec2/instance:Instance::" + name
		resource := s.query(t, "/api/overview/resource?urn="+url.QueryEscape(urn), nil)
		display := resource["display"].(map[string]any)
		require.NoError(t, page.Locator(".resource-link").Filter(playwright.LocatorFilterOptions{HasText: name}).Click())
		webWait(t, page, `() => document.querySelector('.detail-panel .detail-header') !== null`)
		detail, readErr := page.Locator(".detail-panel").TextContent()
		require.NoError(t, readErr)
		key, title := "impact", "Cost impact"
		if name == "active" {
			key, title = "drift", "Cost drift"
		}
		require.Contains(t, detail, title)
		fields := display[key].([]any)
		require.NotEmpty(t, fields)
		for _, entry := range fields {
			field := entry.(map[string]any)
			assert.Contains(t, detail, field["name"])
			assert.Contains(t, detail, field["value"])
		}
		if name == "updated" {
			assert.Contains(t, detail, "Current (est. monthly)")
			assert.Contains(t, detail, "After Change")
		}
		for _, key := range []string{"actualBreakdown", "projectedBreakdown"} {
			for _, entry := range display[key].([]any) {
				assert.Contains(t, detail, entry.(map[string]any)["costDisplay"])
			}
		}
		assert.Contains(t, detail, "Forecasted:")
		assert.Contains(t, detail, "75% threshold triggered")
		require.NoError(t, page.Locator(".detail-close").Click())
	}
}
