//go:build e2e_web

package e2e

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/stretchr/testify/require"
)

//nolint:paralleltest // Browser journey shares the Playwright driver process.
func TestWebUIPluginCatalogInstallAndSettings(t *testing.T) {
	f := newWebFixture(t)
	binary := buildRecorderPlugin(t, f.root)
	archive, assetName := pluginArchive(t, binary)
	sum := sha256.Sum256(archive)
	checksums := hex.EncodeToString(sum[:]) + "  " + assetName + "\n"

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/rshade/finfocus-plugin-recorder/releases/latest",
			"/repos/rshade/finfocus-plugin-recorder/releases/tags/0.1.0":
			writeRelease(t, w, server.URL, assetName, int64(len(archive)))
		case "/download/" + assetName:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(archive)
		case "/download/checksums.txt":
			_, _ = io.WriteString(w, checksums)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	catalog := filepath.Join(t.TempDir(), "catalog.json")
	require.NoError(t, os.WriteFile(catalog, []byte(`{
		"schema_version": "1.0.0",
		"plugins": {
			"recorder": {
				"name": "recorder",
				"description": "Records requests for tests",
				"repository": "rshade/finfocus-plugin-recorder",
				"security_level": "community",
				"supported_providers": ["test"],
				"capabilities": ["projected"]
			}
		}
	}`), 0o600))
	logPath := filepath.Join(t.TempDir(), "plugins.log")

	browser := webBrowser(t)
	session := f.start(
		t,
		filepath.Join(f.root, "testdata/simple-plan.json"),
		false,
		"FINFOCUS_CATALOG_FILE="+catalog,
		"FINFOCUS_GITHUB_API="+server.URL,
		"FINFOCUS_LOG_FILE="+logPath,
	)
	page := webPage(t, browser, session, playwright.ColorSchemeLight)
	webRoute(t, page, "plugins")
	webWait(t, page, `() => document.querySelector('#plugin-install-recorder') !== null`)

	webTab(t, page, "#plugin-install-recorder")
	require.NoError(t, page.Keyboard().Press("Enter"))
	webWait(t, page, `() => document.querySelector('#plugin-confirm-dialog[open]') !== null`)
	summary := mustText(t, page, "#plugin-confirm-summary")
	require.Contains(t, summary, "install recorder")
	require.Contains(t, summary, "Version 0.1.0")
	require.Contains(t, summary, "Repository rshade/finfocus-plugin-recorder")
	require.Contains(t, summary, "Security level community")

	webTab(t, page, "#plugin-confirm-accept")
	require.NoError(t, page.Keyboard().Press("Enter"))
	webWait(t, page, `() => {
		const progress = document.querySelector('#plugin-progress');
		return progress && progress.textContent.includes('finished') && document.querySelector('#plugin-configure-recorder');
	}`)
	installed := filepath.Join(session.home, "plugins", "recorder", "0.1.0", "finfocus-plugin-recorder")
	if runtime.GOOS == "windows" {
		installed += ".exe"
	}
	_, err := os.Stat(installed)
	require.NoError(t, err)

	webTab(t, page, "#plugin-configure-recorder")
	require.NoError(t, page.Keyboard().Press("Enter"))
	webWait(t, page, `() => document.querySelector('#plugin-setting-mock_response') !== null`)
	_, err = page.Locator("#plugin-setting-mock_response").
		SelectOption(playwright.SelectOptionValues{Values: playwright.StringSlice("true")})
	require.NoError(t, err)
	webTab(t, page, "#plugin-save")
	require.NoError(t, page.Keyboard().Press("Enter"))
	webWait(t, page, `() => {
		const dialog = document.querySelector('#plugin-confirm-dialog[open]');
		return dialog && dialog.textContent.includes('plugins.recorder.mock_response');
	}`)
	webTab(t, page, "#plugin-confirm-accept")
	require.NoError(t, page.Keyboard().Press("Enter"))
	webWait(t, page, `() => document.querySelector('#plugin-message')?.textContent.includes('next time this plugin is launched')`)

	projectConfig := filepath.Join(session.project, ".finfocus", "config.hujson")
	saved, err := os.ReadFile(projectConfig)
	require.NoError(t, err)
	require.Contains(t, string(saved), "mock_response")
	require.Contains(t, string(saved), "true")
	require.NotContains(t, string(saved), "[REDACTED]")

	require.Eventually(t, func() bool {
		data, readErr := os.ReadFile(logPath)
		return readErr == nil && bytes.Contains(data, []byte("mock response mode enabled"))
	}, 20*time.Second, 200*time.Millisecond, "plugin log %q:\n%s\nserver log:\n%s", logPath, fileText(logPath), session.logs.String())

	session.query(t, "/api/cost/actual/query", map[string]any{})
}

func fileText(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	return string(data)
}

func buildRecorderPlugin(t *testing.T, root string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "finfocus-plugin-recorder")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", out, "./plugins/recorder/cmd")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "build recorder: %s", output)
	return out
}

func pluginArchive(t *testing.T, binaryPath string) ([]byte, string) {
	t.Helper()
	binary, err := os.ReadFile(binaryPath)
	require.NoError(t, err)
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	name := fmt.Sprintf("recorder_0.1.0_%s_%s%s", runtime.GOOS, runtime.GOARCH, ext)
	entry := filepath.Base(binaryPath)
	var buf bytes.Buffer
	if runtime.GOOS == "windows" {
		zw := zip.NewWriter(&buf)
		header, headerErr := zw.CreateHeader(&zip.FileHeader{Name: entry, Method: zip.Deflate})
		require.NoError(t, headerErr)
		_, err = header.Write(binary)
		require.NoError(t, err)
		require.NoError(t, zw.Close())
		return buf.Bytes(), name
	}
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     entry,
		Mode:     0o755,
		Size:     int64(len(binary)),
		Typeflag: tar.TypeReg,
	}))
	_, err = tw.Write(binary)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())
	return buf.Bytes(), name
}

func writeRelease(t *testing.T, w http.ResponseWriter, origin, assetName string, size int64) {
	t.Helper()
	release := map[string]any{
		"tag_name": "0.1.0",
		"name":     "0.1.0",
		"assets": []map[string]any{
			{
				"name":                 assetName,
				"size":                 size,
				"browser_download_url": origin + "/download/" + assetName,
			},
			{
				"name":                 "checksums.txt",
				"size":                 80,
				"browser_download_url": origin + "/download/checksums.txt",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(release))
}
