# SP3b — Monorepo Plugin Releases Implementation Plan

<!-- markdownlint-configure-file { "MD010": { "code_blocks": false } } -->

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `finfocus plugin install|update <name>` install plugins released from the finfocus monorepo under prefixed tags (`kubernetes-v0.1.0`), and give monorepo plugins a release pipeline that never collides with the CLI's.

**Architecture:** A registry entry gains an optional `tag_prefix`. The prefix travels in `AssetNamingHints`; every place that today treats `release.TagName` as the version instead uses the *canonical version* (tag minus prefix). "Latest" for a prefixed entry means the highest semver among stable releases carrying that prefix, not `/releases/latest`. A generic workflow builds and uploads assets for any non-`v*` release tag of the form `<plugin>-v<semver>`, and CLI workflows ignore those tags.

**Tech Stack:** Go 1.27.1, `github.com/Masterminds/semver/v3`, testify, `httptest`, GitHub Actions, bash.

**Spec:** `docs/superpowers/specs/2026-09-24-k8s-cost-allocation-design.md` (§4 "Registry and release (SP3b)")

## Global Constraints

- Entries without `tag_prefix` behave exactly as today (every existing test stays green unchanged).
- Tag format: `<prefix><semver-with-v>`, prefix matching `^[a-uw-z0-9][a-z0-9-]*-$` (e.g. `kubernetes-`).
- Canonical version (install dir name, asset-name version, `config` installed version) is the tag with the prefix removed, e.g. `v0.1.0` — `ListLatestPlugins` requires a semver directory name.
- Asset names: `finfocus-plugin-<name>_<canonical-version>_<os>_<arch>.tar.gz` (`.zip` on Windows) plus `checksums.txt` — matches existing `buildAssetPatterns`.
- Never `git commit`; stage and hand off. Run `make lint` (long) and `make test`.
- Use testify `require`/`assert` in new tests (CLAUDE.md), even where the file's older tests use `t.Fatalf`.

## Review Focus

- **Repo-wide "latest" in a busy monorepo.** Core releases (`v0.3.8`) outnumber plugin releases, so the newest 10 releases may contain none with the prefix. Prefixed lookups and prefixed fallback scan the newest 100 stable releases and filter by prefix (Task 2, Task 3 tests).
- **Semver vs publish order.** A `kubernetes-v0.1.10` published before a `kubernetes-v0.1.9` hotfix must still win (Task 2 test).
- **User passes a bare version.** `plugin install kubernetes@v0.1.0` or `@0.1.0` must resolve to tag `kubernetes-v0.1.0`; `@kubernetes-v0.1.0` must also work (Task 1 `ReleaseTag` tests).
- **Updating a prefixed plugin.** `Update` must compare canonical versions; today a non-semver tag makes `CompareVersions` fail and forces a reinstall every time (Task 3 test).
- **Shell injection through release tags.** Step outputs derived from a tag must reach `run:` only via `env:`, and the tag regex must be bounded semver (fixed during execution; see ledger).
- **CLI workflows firing on plugin tags.** `goreleaser.yml` (release `created`, any tag) would publish CLI archives to a plugin release; `nightly.yml` runs on every published release; `git describe` could stamp the CLI with `kubernetes-v0.1.0` (Task 5).

---

### Task 1: Tag-prefix model (entry field, hints, version/tag helpers)

**Files:**

- Modify: `internal/registry/entry.go` (struct at :13-39, `ValidateRegistryEntry`)
- Modify: `internal/registry/github.go` (`AssetNamingHints` at ~:381)
- Modify: `internal/registry/installer.go:47-56` (`convertToAssetNamingHints`)
- Modify: `internal/registry/version.go` (append helpers)
- Test: `internal/registry/version_test.go`, `internal/registry/entry_test.go`

**Interfaces:**

- Produces:
  - `RegistryEntry.TagPrefix string` (`json:"tag_prefix,omitempty"`)
  - `AssetNamingHints.TagPrefix string`
  - `func HintsForEntry(entry *RegistryEntry) *AssetNamingHints` — nil when the entry has neither `AssetHints` nor `TagPrefix`
  - `func CanonicalVersion(tag, prefix string) string`
  - `func ReleaseTag(version, prefix string) string`
  - `func tagPrefixOf(h *AssetNamingHints) string` (nil-safe, unexported)

- [ ] **Step 1: Write failing tests**

Append to `internal/registry/version_test.go`:

```go
func TestCanonicalVersion(t *testing.T) {
	tests := []struct {
		name, tag, prefix, want string
	}{
		{"no prefix passes through", "v1.2.3", "", "v1.2.3"},
		{"prefix stripped", "kubernetes-v0.1.0", "kubernetes-", "v0.1.0"},
		{"prefix absent leaves tag", "v0.3.7", "kubernetes-", "v0.3.7"},
		{"other plugin prefix untouched", "prometheus-v0.1.0", "kubernetes-", "prometheus-v0.1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CanonicalVersion(tt.tag, tt.prefix))
		})
	}
}

func TestReleaseTag(t *testing.T) {
	tests := []struct {
		name, version, prefix, want string
	}{
		{"no prefix passes through", "v1.0.0", "", "v1.0.0"},
		{"bare v version gets prefix", "v0.1.0", "kubernetes-", "kubernetes-v0.1.0"},
		{"bare semver gets v and prefix", "0.1.0", "kubernetes-", "kubernetes-v0.1.0"},
		{"already prefixed kept", "kubernetes-v0.1.0", "kubernetes-", "kubernetes-v0.1.0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ReleaseTag(tt.version, tt.prefix))
		})
	}
}

func TestHintsForEntry(t *testing.T) {
	assert.Nil(t, HintsForEntry(&RegistryEntry{Name: "x"}))

	h := HintsForEntry(&RegistryEntry{Name: "kubernetes", TagPrefix: "kubernetes-"})
	require.NotNil(t, h)
	assert.Equal(t, "kubernetes-", h.TagPrefix)

	h = HintsForEntry(&RegistryEntry{
		Name:       "aws-public",
		AssetHints: &RegistryAssetHints{AssetPrefix: "finfocus-plugin-aws-public", DefaultRegion: "us-east-1"},
	})
	require.NotNil(t, h)
	assert.Equal(t, "finfocus-plugin-aws-public", h.AssetPrefix)
	assert.Equal(t, "us-east-1", h.Region)
	assert.Empty(t, h.TagPrefix)
}
```

Append to `internal/registry/entry_test.go`:

```go
func TestValidateRegistryEntry_TagPrefix(t *testing.T) {
	base := RegistryEntry{Name: "kubernetes", Repository: "rshade/finfocus"}
	for _, p := range []string{"", "kubernetes-", "k8s-alloc-"} {
		e := base
		e.TagPrefix = p
		assert.NoError(t, ValidateRegistryEntry(e), "prefix %q", p)
	}
	for _, p := range []string{"kubernetes", "Kubernetes-", "-", "kube/", "v"} {
		e := base
		e.TagPrefix = p
		err := ValidateRegistryEntry(e)
		require.Error(t, err, "prefix %q", p)
		assert.Contains(t, err.Error(), "tag_prefix")
	}
}
```

Add `"github.com/stretchr/testify/assert"` / `require` imports if the files lack them.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/registry/ -run 'TestCanonicalVersion|TestReleaseTag|TestHintsForEntry|TestValidateRegistryEntry_TagPrefix'`
Expected: FAIL — `undefined: CanonicalVersion` (and friends).

- [ ] **Step 3: Implement**

`internal/registry/entry.go` — add the field as the last struct member and validation:

```go
	// TagPrefix marks a plugin released from a monorepo under prefixed tags,
	// e.g. "kubernetes-" for tag "kubernetes-v0.1.0". Empty for single-plugin repos.
	TagPrefix string `json:"tag_prefix,omitempty"`
```

```go
var tagPrefixPattern = regexp.MustCompile(`^[a-uw-z0-9][a-z0-9-]*-$`)
```

At the end of `ValidateRegistryEntry`, before the final `return nil`:

```go
	if entry.TagPrefix != "" && !tagPrefixPattern.MatchString(entry.TagPrefix) {
		return fmt.Errorf("invalid tag_prefix %q for %s: must match %s",
			entry.TagPrefix, entry.Name, tagPrefixPattern.String())
	}
```

`internal/registry/github.go` — add to `AssetNamingHints`:

```go
	// TagPrefix is stripped from release tags to obtain the canonical version.
	TagPrefix string
```

`internal/registry/installer.go` — replace `convertToAssetNamingHints` with:

```go
// HintsForEntry builds asset naming hints from a registry entry.
// It returns nil when the entry carries no hints and no tag prefix.
func HintsForEntry(entry *RegistryEntry) *AssetNamingHints {
	if entry == nil || (entry.AssetHints == nil && entry.TagPrefix == "") {
		return nil
	}
	h := &AssetNamingHints{TagPrefix: entry.TagPrefix}
	if entry.AssetHints != nil {
		h.AssetPrefix = entry.AssetHints.AssetPrefix
		h.Region = entry.AssetHints.DefaultRegion
		h.VersionPrefix = entry.AssetHints.VersionPrefix
	}
	return h
}
```

Update the two callers: `installFromRegistry` (`assetHints := HintsForEntry(entry)`) and `resolvePluginSource` (`hints := HintsForEntry(entry)`). Run `grep -rn convertToAssetNamingHints internal/` and replace any remaining test references with `HintsForEntry(&RegistryEntry{AssetHints: …})`.

`internal/registry/version.go` — append:

```go
// CanonicalVersion strips a monorepo tag prefix, turning "kubernetes-v0.1.0"
// into "v0.1.0". Tags without the prefix are returned unchanged.
func CanonicalVersion(tag, prefix string) string {
	if prefix == "" {
		return tag
	}
	return strings.TrimPrefix(tag, prefix)
}

// ReleaseTag converts a user-supplied version into the release tag to fetch.
// With a prefix, "0.1.0" and "v0.1.0" both become "<prefix>v0.1.0".
func ReleaseTag(version, prefix string) string {
	if prefix == "" || strings.HasPrefix(version, prefix) {
		return version
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return prefix + version
}

func tagPrefixOf(h *AssetNamingHints) string {
	if h == nil {
		return ""
	}
	return h.TagPrefix
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/registry/...`
Expected: PASS (new and existing).

- [ ] **Step 5: Stage and hand off**

```bash
git add internal/registry/entry.go internal/registry/github.go internal/registry/installer.go internal/registry/version.go internal/registry/version_test.go internal/registry/entry_test.go
```

Proposed message: `feat(registry): add tag_prefix for monorepo plugin releases`

---

### Task 2: Latest release by prefix

**Files:**

- Modify: `internal/registry/github.go` (after `ListStableReleases`)
- Test: `internal/registry/github_prefix_test.go` (new)

**Interfaces:**

- Consumes: `CanonicalVersion` (Task 1), `ListStableReleases(ctx, owner, repo, limit)`.
- Produces:
  - `func selectLatestByPrefix(releases []GitHubRelease, prefix string) (*GitHubRelease, error)`
  - `func (c *GitHubClient) GetLatestReleaseWithPrefix(ctx context.Context, owner, repo, prefix string) (*GitHubRelease, error)`
  - `const prefixedReleaseScan = 100`

- [ ] **Step 1: Write failing tests**

Create `internal/registry/github_prefix_test.go`:

```go
package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectLatestByPrefix(t *testing.T) {
	releases := []GitHubRelease{
		{TagName: "v0.3.8"},
		{TagName: "kubernetes-v0.1.9"}, // published after 0.1.10 (hotfix)
		{TagName: "kubernetes-v0.1.10"},
		{TagName: "prometheus-v0.2.0"},
		{TagName: "kubernetes-vbad"},
	}
	got, err := selectLatestByPrefix(releases, "kubernetes-")
	require.NoError(t, err)
	assert.Equal(t, "kubernetes-v0.1.10", got.TagName)

	_, err = selectLatestByPrefix(releases, "datadog-")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `tag prefix "datadog-"`)
}

func TestGetLatestReleaseWithPrefix_ScansPastCoreReleases(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/repos/rshade/finfocus/releases", r.URL.Path)
		assert.Equal(t, "100", r.URL.Query().Get("per_page"))
		var rels []GitHubRelease
		for i := 0; i < 20; i++ { // 20 newer CLI releases first
			rels = append(rels, GitHubRelease{TagName: "v0.4." + string(rune('a'+i))})
		}
		rels = append(rels,
			GitHubRelease{TagName: "kubernetes-v0.2.0-rc.1", Prerelease: true},
			GitHubRelease{TagName: "kubernetes-v0.1.0"},
		)
		require.NoError(t, json.NewEncoder(w).Encode(rels))
	}))
	defer server.Close()

	c := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
	got, err := c.GetLatestReleaseWithPrefix(context.Background(), "rshade", "finfocus", "kubernetes-")
	require.NoError(t, err)
	assert.Equal(t, "kubernetes-v0.1.0", got.TagName, "prerelease must be skipped")
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/registry/ -run 'TestSelectLatestByPrefix|TestGetLatestReleaseWithPrefix'`
Expected: FAIL — `undefined: selectLatestByPrefix`.

- [ ] **Step 3: Implement** (in `github.go`; `semver` is already imported by `version.go` in the same package — add the import here if the compiler asks)

```go
// prefixedReleaseScan bounds how many recent stable releases are searched for
// a monorepo plugin's tags; core releases are interleaved with plugin releases.
const prefixedReleaseScan = 100

// selectLatestByPrefix returns the release with the highest semver among tags
// carrying prefix. Tags whose remainder is not semver are ignored.
func selectLatestByPrefix(releases []GitHubRelease, prefix string) (*GitHubRelease, error) {
	var best *GitHubRelease
	var bestVer *semver.Version
	for i := range releases {
		tag := releases[i].TagName
		if !strings.HasPrefix(tag, prefix) {
			continue
		}
		v, err := semver.NewVersion(strings.TrimPrefix(CanonicalVersion(tag, prefix), "v"))
		if err != nil {
			continue
		}
		if bestVer == nil || v.GreaterThan(bestVer) {
			best, bestVer = &releases[i], v
		}
	}
	if best == nil {
		return nil, fmt.Errorf("no stable release with tag prefix %q found", prefix)
	}
	return best, nil
}

// GetLatestReleaseWithPrefix finds the newest (by semver) stable release whose
// tag starts with prefix, for plugins released from a monorepo.
func (c *GitHubClient) GetLatestReleaseWithPrefix(
	ctx context.Context,
	owner, repo, prefix string,
) (*GitHubRelease, error) {
	releases, err := c.ListStableReleases(ctx, owner, repo, prefixedReleaseScan)
	if err != nil {
		return nil, err
	}
	release, err := selectLatestByPrefix(releases, prefix)
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", owner, repo, err)
	}
	return release, nil
}
```

Note: `ListStableReleases` stops at `limit` *stable* releases from one page of up to 100; that is the intended bound.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/registry/...`
Expected: PASS.

- [ ] **Step 5: Stage and hand off**

```bash
git add internal/registry/github.go internal/registry/github_prefix_test.go
```

Proposed message: `feat(registry): select latest monorepo plugin release by tag prefix`

---

### Task 3: Install, fallback, and update use canonical versions

**Files:**

- Modify: `internal/registry/installer.go` (`installFromRegistry` :133-196, `installRelease` :245-263, `Update` :614-670)
- Modify: `internal/registry/github.go` (`FindPlatformAssetWithHints` `version :=` line; `FindReleaseWithFallbackInfo` :208-287)
- Modify: `internal/cli/plugin_install.go:467-473` (fallback hints)
- Test: `internal/registry/installer_prefix_test.go` (new)

**Interfaces:**

- Consumes: `HintsForEntry`, `CanonicalVersion`, `ReleaseTag`, `tagPrefixOf` (Task 1); `GetLatestReleaseWithPrefix`, `prefixedReleaseScan` (Task 2).
- Produces: `func (i *Installer) fetchRelease(ctx context.Context, owner, repo, version string, hints *AssetNamingHints) (*GitHubRelease, error)`. Install dir becomes `<pluginDir>/<name>/<canonical>`; `InstallResult.Version`, `UpdateResult.NewVersion`, and `config` installed version are canonical.

- [ ] **Step 1: Write failing tests**

Create `internal/registry/installer_prefix_test.go`. It reuses `createMockArchive` (`installer_api_test.go:167`), `testAssetName` (`github_api_test.go:19`), and `config.ResetGlobalConfigForTest` / `config.InitGlobalConfig` exactly as `installer_api_test.go:27-119` does.

```go
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

// prefixedServer serves a monorepo whose release list interleaves CLI and plugin tags.
func prefixedServer(t *testing.T, pluginTags ...string) *httptest.Server {
	t.Helper()
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		release := func(tag string) GitHubRelease {
			canonical := CanonicalVersion(tag, "kubernetes-")
			name := testAssetName("finfocus-plugin-kubernetes", canonical)
			return GitHubRelease{TagName: tag, Assets: []ReleaseAsset{{
				Name:               name,
				BrowserDownloadURL: fmt.Sprintf("%s/download/%s", server.URL, name),
			}}}
		}
		switch {
		case r.URL.Path == "/repos/rshade/finfocus/releases":
			rels := []GitHubRelease{{TagName: "v0.3.9"}}
			for _, tag := range pluginTags {
				rels = append(rels, release(tag))
			}
			require.NoError(t, json.NewEncoder(w).Encode(rels))
		case len(r.URL.Path) > len("/repos/rshade/finfocus/releases/tags/") &&
			r.URL.Path[:len("/repos/rshade/finfocus/releases/tags/")] == "/repos/rshade/finfocus/releases/tags/":
			tag := filepath.Base(r.URL.Path)
			for _, pt := range pluginTags {
				if pt == tag {
					require.NoError(t, json.NewEncoder(w).Encode(release(tag)))
					return
				}
			}
			http.NotFound(w, r)
		case filepath.Dir(r.URL.Path) == "/download":
			_, _ = w.Write(createMockArchive(t, "finfocus-plugin-kubernetes"))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	return server
}

func newPrefixedInstaller(t *testing.T, server *httptest.Server) (*Installer, string) {
	t.Helper()
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("FINFOCUS_HOME", filepath.Join(tmpHome, ".finfocus"))
	config.ResetGlobalConfigForTest()
	config.InitGlobalConfig()
	pluginDir := filepath.Join(tmpHome, ".finfocus", "plugins")
	client := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}
	return NewInstallerWithClient(client, pluginDir), pluginDir
}

var kubernetesHints = &AssetNamingHints{
	AssetPrefix: "finfocus-plugin-kubernetes",
	TagPrefix:   "kubernetes-",
}

func TestInstallRelease_PrefixedTagUsesCanonicalDir(t *testing.T) {
	server := prefixedServer(t, "kubernetes-v0.1.0")
	defer server.Close()
	inst, pluginDir := newPrefixedInstaller(t, server)

	rel, err := inst.fetchRelease(context.Background(), "rshade", "finfocus", "", kubernetesHints)
	require.NoError(t, err)
	res, err := inst.installRelease(context.Background(), "kubernetes", rel, "rshade/finfocus",
		InstallOptions{}, nil, kubernetesHints)
	require.NoError(t, err)

	assert.Equal(t, "v0.1.0", res.Version)
	_, statErr := os.Stat(filepath.Join(pluginDir, "kubernetes", "v0.1.0"))
	require.NoError(t, statErr)

	plugins, warnings, err := (&Registry{root: pluginDir}).ListLatestPlugins()
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, plugins, 1, "semver dir must be discoverable")
	assert.Equal(t, "v0.1.0", plugins[0].Version)
}

func TestFetchRelease_BareVersionResolvesPrefixedTag(t *testing.T) {
	server := prefixedServer(t, "kubernetes-v0.1.0", "kubernetes-v0.2.0")
	defer server.Close()
	inst, _ := newPrefixedInstaller(t, server)

	for _, v := range []string{"0.1.0", "v0.1.0", "kubernetes-v0.1.0"} {
		rel, err := inst.fetchRelease(context.Background(), "rshade", "finfocus", v, kubernetesHints)
		require.NoError(t, err, v)
		assert.Equal(t, "kubernetes-v0.1.0", rel.TagName, v)
	}
}

func TestFindReleaseWithFallbackInfo_PrefixFiltersCoreReleases(t *testing.T) {
	server := prefixedServer(t, "kubernetes-v0.1.0")
	defer server.Close()
	c := &GitHubClient{HTTPClient: server.Client(), BaseURL: server.URL}

	info, err := c.FindReleaseWithFallbackInfo(context.Background(), "rshade", "finfocus",
		"v9.9.9", "kubernetes", kubernetesHints)
	require.NoError(t, err)
	assert.Equal(t, "kubernetes-v0.1.0", info.Release.TagName)
	assert.True(t, info.WasFallback)
}
```

Helpers verified to exist: `NewInstallerWithClient` (`installer.go:92`), `createMockArchive(t, binaryName) []byte` (`installer_api_test.go:167`), `(*Registry).ListLatestPlugins() ([]PluginInfo, []string, error)` (`registry.go:101`); `Registry{root, launcher}` has unexported fields, which this in-package test sets directly.

Add an update test to the same file (uses `config.AddInstalledPlugin`, whose signature is at `internal/config/plugins.go` — check with `grep -n "func AddInstalledPlugin" internal/config/plugins.go`):

```go
func TestUpdate_PrefixedPluginComparesCanonicalVersions(t *testing.T) {
	server := prefixedServer(t, "kubernetes-v0.1.0")
	defer server.Close()
	inst, _ := newPrefixedInstaller(t, server)

	rel, err := inst.fetchRelease(context.Background(), "rshade", "finfocus", "", kubernetesHints)
	require.NoError(t, err)
	_, err = inst.installRelease(context.Background(), "kubernetes", rel, "rshade/finfocus",
		InstallOptions{}, nil, kubernetesHints)
	require.NoError(t, err)

	newVersion := CanonicalVersion(rel.TagName, kubernetesHints.TagPrefix)
	cmp, err := CompareVersions(newVersion, "v0.1.0")
	require.NoError(t, err, "canonical versions must be comparable")
	assert.Equal(t, 0, cmp, "same version must not trigger reinstall")
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/registry/ -run 'Prefix'`
Expected: FAIL — `inst.fetchRelease undefined`.

- [ ] **Step 3: Implement**

`installer.go` — add:

```go
// fetchRelease resolves the release to install: an explicit version (prefixed
// when the plugin ships from a monorepo), the newest prefixed release, or the
// repository's latest release.
func (i *Installer) fetchRelease(
	ctx context.Context,
	owner, repo, version string,
	hints *AssetNamingHints,
) (*GitHubRelease, error) {
	prefix := tagPrefixOf(hints)
	switch {
	case version != "":
		return i.client.GetReleaseByTag(ctx, owner, repo, ReleaseTag(version, prefix))
	case prefix != "":
		return i.client.GetLatestReleaseWithPrefix(ctx, owner, repo, prefix)
	default:
		return i.client.GetLatestRelease(ctx, owner, repo)
	}
}
```

In `installFromRegistry`, compute `assetHints := HintsForEntry(entry)` **before** the release fetch and replace the `if spec.Version != "" { … } else { … }` block's two client calls with `release, err = i.fetchRelease(ctx, owner, repo, spec.Version, assetHints)` (keep the progress messages). In `Update`, replace its equivalent block with `release, err = i.fetchRelease(ctx, owner, repo, opts.Version, assetHints)` and change `newVersion := release.TagName` to:

```go
	newVersion := CanonicalVersion(release.TagName, tagPrefixOf(assetHints))
```

In `installRelease`, change `version := release.TagName` to:

```go
	version := CanonicalVersion(release.TagName, tagPrefixOf(hints))
```

`github.go` — in `FindPlatformAssetWithHints` change `version := release.TagName` to `version := CanonicalVersion(release.TagName, tagPrefixOf(hints))`. In `FindReleaseWithFallbackInfo`:

```go
	prefix := tagPrefixOf(hints)
	scan := maxFallbackReleases
	if prefix != "" {
		scan = prefixedReleaseScan
	}
```

use `c.GetReleaseByTag(ctx, owner, repo, ReleaseTag(version, prefix))` for the requested version, call `c.ListStableReleases(ctx, owner, repo, scan)`, and at the top of the fallback loop body add `if prefix != "" && !strings.HasPrefix(stableReleases[i].TagName, prefix) { continue }` (match the loop's variable name). Keep `WasFallback` semantics but compare canonical versions: `info.WasFallback = version != "" && CanonicalVersion(release.TagName, prefix) != CanonicalVersion(ReleaseTag(version, prefix), prefix)`.

`internal/cli/plugin_install.go:467-473` — replace the manual hints literal with:

```go
		assetHints = registry.HintsForEntry(entry)
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/registry/... ./internal/cli/ -run 'Plugin|Install|Update|Fallback|Prefix|Release'`
Expected: PASS. Then `go test ./internal/registry/... ./internal/cli/...` — PASS.

- [ ] **Step 5: Stage and hand off**

```bash
git add internal/registry/installer.go internal/registry/github.go internal/registry/installer_prefix_test.go internal/cli/plugin_install.go
```

Proposed message: `feat(registry): install and update prefixed monorepo plugin releases`

---

### Task 4: Registry schema accepts usage/allocation plugins

**Files:**

- Modify: `internal/registry/registry_json_test.go:36-104` (allowed capability set)
- Test: same file

**Interfaces:**

- Produces: `registry.json` entries may declare capabilities `"usage_stats"` and `"allocation"` and a `tag_prefix`. (The `kubernetes` entry itself is added in SP2's final task.)

- [ ] **Step 1: Write the failing test**

Add a table case (or a new test) that validates an in-memory entry:

```go
func TestRegistryCapabilities_AllowUsageAndAllocation(t *testing.T) {
	for _, c := range []string{"usage_stats", "allocation"} {
		assert.True(t, isAllowedRegistryCapability(c), c)
	}
	assert.False(t, isAllowedRegistryCapability("teleport"))
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/registry/ -run TestRegistryCapabilities_AllowUsageAndAllocation`
Expected: FAIL — `undefined: isAllowedRegistryCapability`.

- [ ] **Step 3: Implement**

In `registry_json_test.go`, lift the inline allowed-capabilities set into a helper used by the existing validation test:

```go
var allowedRegistryCapabilities = map[string]bool{
	"projected": true, "actual": true, "cost_retrieval": true,
	"cost_projection": true, "pricing_specs": true, "recommendations": true,
	"usage_stats": true, "allocation": true,
}

func isAllowedRegistryCapability(c string) bool { return allowedRegistryCapabilities[c] }
```

and make the existing loop call `isAllowedRegistryCapability`. Also, in the same existing test, run `ValidateRegistryEntry(entry)` for every embedded entry (if not already) so a malformed `tag_prefix` in `registry.json` fails CI.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/registry/...`
Expected: PASS.

- [ ] **Step 5: Stage and hand off**

```bash
git add internal/registry/registry_json_test.go
```

Proposed message: `test(registry): allow usage_stats and allocation capabilities`

---

### Task 5: Release pipeline for monorepo plugins

**Files:**

- Create: `scripts/release-plugin-assets.sh`
- Create: `.github/workflows/release-monorepo-plugin.yml`
- Modify: `.github/workflows/goreleaser.yml` (job `if`)
- Modify: `.github/workflows/nightly.yml` (release-triggered jobs `if`)
- Modify: `Makefile` (`VERSION?=` line)
- Modify: `.github/workflows/ci.yml` (any `git describe --tags` in the build job)

**Interfaces:**

- Produces: `scripts/release-plugin-assets.sh <module-dir> <package> <binary-name> <canonical-version> <out-dir>` writing `<binary>_<version>_<os>_<arch>.tar.gz|.zip` for linux/darwin amd64+arm64 and windows amd64, plus `checksums.txt`. The workflow runs for published releases whose tag is `<plugin>-v<semver>`, building `plugins/<plugin>/cmd` from module dir `plugins/<plugin>`.

- [ ] **Step 1: Write the script**

```bash
#!/usr/bin/env bash
# Build release archives for a plugin shipped from the finfocus monorepo.
# Usage: release-plugin-assets.sh <module-dir> <package> <binary-name> <version> <out-dir>
set -euo pipefail

if [[ $# -ne 5 ]]; then
  echo "usage: $0 <module-dir> <package> <binary-name> <version> <out-dir>" >&2
  exit 2
fi
module_dir=$1 pkg=$2 binary=$3 version=$4 out_dir=$5

if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  echo "version must be canonical (vX.Y.Z), got: $version" >&2
  exit 2
fi

mkdir -p "$out_dir"
out_dir=$(cd "$out_dir" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

targets=(linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64)
for target in "${targets[@]}"; do
  goos=${target%/*} goarch=${target#*/}
  name="${binary}_${version}_${goos}_${goarch}"
  exe=$binary
  [[ $goos == windows ]] && exe="$binary.exe"
  mkdir -p "$work/$name"
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch \
    go -C "$module_dir" build -trimpath -ldflags "-s -w" -o "$work/$name/$exe" "$pkg"
  if [[ $goos == windows ]]; then
    (cd "$work/$name" && zip -q "$out_dir/$name.zip" "$exe")
  else
    tar -C "$work/$name" -czf "$out_dir/$name.tar.gz" "$exe"
  fi
done

(cd "$out_dir" && sha256sum -- *.tar.gz *.zip > checksums.txt)
echo "assets written to $out_dir"
```

`chmod +x scripts/release-plugin-assets.sh`.

- [ ] **Step 2: Verify the script against the recorder (exists today)**

```bash
out=$(mktemp -d)
./scripts/release-plugin-assets.sh . ./plugins/recorder/cmd finfocus-plugin-recorder v0.1.0 "$out"
ls "$out"
./scripts/release-plugin-assets.sh . ./plugins/recorder/cmd finfocus-plugin-recorder 0.1.0 "$out"; echo "exit=$?"
```

Expected: five archives + `checksums.txt` named `finfocus-plugin-recorder_v0.1.0_linux_amd64.tar.gz` etc.; the second call prints "version must be canonical" and `exit=2`. Also run `shellcheck scripts/release-plugin-assets.sh` — no findings.

Confirm the name matches the installer: `go test ./internal/registry/ -run TestInstallRelease_PrefixedTagUsesCanonicalDir` already exercises `finfocus-plugin-kubernetes_v0.1.0_<os>_<arch>.tar.gz`.

- [ ] **Step 3: Write the workflow**

`.github/workflows/release-monorepo-plugin.yml`:

```yaml
name: Release monorepo plugin

on:
  release:
    types: [published]
  workflow_dispatch:
    inputs:
      tag:
        description: "Plugin release tag, e.g. kubernetes-v0.1.0"
        required: true

permissions:
  contents: write

jobs:
  assets:
    runs-on: ubuntu-latest
    env:
      TAG: ${{ inputs.tag || github.event.release.tag_name }}
    if: ${{ !startsWith(inputs.tag || github.event.release.tag_name, 'v') }}
    steps:
      - name: Parse tag
        id: parse
        run: |
          if [[ ! $TAG =~ ^([a-z0-9][a-z0-9-]*)-(v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?(\+[0-9A-Za-z.]+)?)$ ]]; then
            echo "not a monorepo plugin tag: $TAG" >&2
            exit 1
          fi
          echo "plugin=${BASH_REMATCH[1]}" >> "$GITHUB_OUTPUT"
          echo "version=${BASH_REMATCH[2]}" >> "$GITHUB_OUTPUT"
      - uses: actions/checkout@v7
        with:
          ref: ${{ env.TAG }}
      - uses: actions/setup-go@v7
        with:
          go-version: '1.27.1'
      - name: Build assets
        env:
          PLUGIN: ${{ steps.parse.outputs.plugin }}
          VERSION: ${{ steps.parse.outputs.version }}
        run: |
          test -d "plugins/$PLUGIN/cmd" || { echo "plugins/$PLUGIN/cmd not found" >&2; exit 1; }
          ./scripts/release-plugin-assets.sh "plugins/$PLUGIN" ./cmd \
            "finfocus-plugin-$PLUGIN" "$VERSION" dist
      - name: Upload assets
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: gh release upload "$TAG" dist/* --clobber
```

- [ ] **Step 4: Keep CLI workflows off plugin tags**

`.github/workflows/goreleaser.yml` — on the `goreleaser` job add:

```yaml
    if: ${{ startsWith(inputs.tag || github.event.release.tag_name, 'v') }}
```

`.github/workflows/nightly.yml` — on every job that can run from the `release` trigger, add (combine with any existing `if` using `&&`):

```yaml
    if: ${{ github.event_name != 'release' || startsWith(github.event.release.tag_name, 'v') }}
```

`Makefile` — change the version line to only consider CLI tags:

```make
VERSION?=$(shell git describe --tags --match 'v*' --always --dirty)
```

`ci.yml` — apply the same `--match 'v*'` to any `git describe --tags` invocation (`grep -n "git describe" .github/workflows/*.yml`).

- [ ] **Step 5: Validate workflows and full suite**

Run: `make lint` (includes actionlint and shellcheck; allow >5 min) and `make test`.
Expected: PASS. If `lint-actions` fails only because the snap shellcheck cannot start (known local issue, see memory "Local shellcheck/snap breakage"), run `shellcheck` from a static binary on PATH and report that explicitly.

- [ ] **Step 6: Document**

Add to `CONTRIBUTING.md` (release section) a short "Monorepo plugin releases" subsection: tag format `<plugin>-vX.Y.Z`, release-please component per plugin, assets built by `release-monorepo-plugin.yml`, registry entry needs `tag_prefix`. Run `markdownlint -c .markdownlint.json CONTRIBUTING.md`.

- [ ] **Step 7: Stage and hand off**

```bash
git add scripts/release-plugin-assets.sh .github/workflows/release-monorepo-plugin.yml .github/workflows/goreleaser.yml .github/workflows/nightly.yml .github/workflows/ci.yml Makefile CONTRIBUTING.md
```

Proposed message: `ci: release pipeline for monorepo plugins with prefixed tags`
