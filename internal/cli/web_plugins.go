package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/webui"
)

var pluginReasonURL = regexp.MustCompile(`https?://\S+`)

const (
	githubRateLimitMessage = "GitHub API rate limit exceeded. Set GITHUB_TOKEN for higher limits"
	pluginReopenFailed     = "plugin installed but the dashboard could not reopen it"
	pluginRemoveReopen     = "plugin removed but the dashboard could not reopen it"
	catalogReleaseScan     = 100
)

// wirePlugins serves the Plugins page. The web UI never imports the registry;
// these callbacks are the only install and settings path the page has.
func wirePlugins(ctx context.Context, options *webui.SessionOptions, cmd *cobra.Command, data *webData) {
	flagValue, _ := cmd.Flags().GetString("project-dir")
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	projectDir := config.ResolveProjectDir(ctx, flagValue, cwd)
	globalPath := filepath.Join(config.ResolveConfigDir(), "config.hujson")
	options.PluginCatalog = pluginCatalog
	options.PluginPlan = pluginPlan
	options.PluginPrepare = func(_ context.Context, name string, _ webui.PluginOperationRequest) error {
		return preparePluginChange(name)
	}
	options.PluginInstall = func(
		installCtx context.Context, name string, request webui.PluginOperationRequest, report func(webui.PluginProgress),
	) error {
		return installConfirmedPlugin(installCtx, name, request, report, data.reloadPlugins)
	}
	options.PluginRemove = func(
		removeCtx context.Context, name string, request webui.PluginOperationRequest, report func(webui.PluginProgress),
	) error {
		return removeConfirmedPlugin(removeCtx, name, request, report, data.reloadPlugins)
	}
	options.PluginValidate = validateInstalledPlugin
	options.PluginSettings = func(settingsCtx context.Context, name string) (webui.PluginSettingsView, error) {
		return pluginSettingsView(settingsCtx, name, globalPath, projectDir)
	}
	options.PluginSettingsPrepare = func(
		prepareCtx context.Context, name string, request config.WriteRequest,
	) (config.WriteRequest, error) {
		return preparePluginSettings(prepareCtx, name, globalPath, projectDir, request)
	}
	options.ReloadPlugins = data.reloadPlugins
}

func pluginCatalog(ctx context.Context) ([]webui.PluginCatalogRow, error) {
	entries, err := registry.GetAllPluginEntries()
	if err != nil {
		return nil, err
	}
	installed, err := registry.NewDefault().ListPlugins()
	if err != nil {
		return nil, err
	}
	merged := registry.MergeCatalog(entries, installed, func(entry registry.RegistryEntry) (string, error) {
		release, _, releaseErr := resolveCatalogRelease(ctx, &entry, "")
		if releaseErr != nil {
			return "", releaseErr
		}
		return registry.CanonicalVersion(release.TagName, entry.TagPrefix), nil
	})
	rows := make([]webui.PluginCatalogRow, 0, len(merged))
	for _, row := range merged {
		rows = append(rows, webui.PluginCatalogRow{
			Name:              row.Name,
			Description:       row.Description,
			Providers:         row.Providers,
			Capabilities:      row.Capabilities,
			SecurityLevel:     row.SecurityLevel,
			Repository:        row.Repository,
			InCatalog:         row.InCatalog,
			InstalledVersions: row.InstalledVersions,
			LatestVersion:     row.LatestVersion,
			UpdateAvailable:   row.UpdateAvailable,
			Warning:           row.Warning,
		})
	}
	return rows, nil
}

func pluginPlan(ctx context.Context, name string, request webui.PluginPlanRequest) (webui.PluginPlan, error) {
	if request.Action == "remove" {
		return planPluginRemoval(name, request.Version)
	}
	entry, err := registry.GetPlugin(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found in registry") {
			return webui.PluginPlan{}, pluginStatus(http.StatusNotFound, "not_found", "plugin is not in the catalog")
		}
		if strings.Contains(err.Error(), "catalog overlay") {
			return webui.PluginPlan{}, err
		}
		return webui.PluginPlan{}, catalogHostError(ctx, name, err)
	}
	release, fallbackFrom, err := resolveCatalogRelease(ctx, entry, request.Version)
	if err != nil {
		return webui.PluginPlan{}, catalogHostError(ctx, name, err)
	}
	checksum := "missing"
	if registry.FindChecksumAsset(&release) != nil {
		checksum = "present"
	}
	return webui.PluginPlan{
		Action:        request.Action,
		Name:          name,
		Version:       registry.CanonicalVersion(release.TagName, entry.TagPrefix),
		Repository:    entry.Repository,
		SecurityLevel: entry.SecurityLevel,
		FallbackFrom:  fallbackFrom,
		ChecksumFile:  checksum,
		InCatalog:     true,
	}, nil
}

func planPluginRemoval(name, version string) (webui.PluginPlan, error) {
	installed, err := registry.NewDefault().ListPlugins()
	if err != nil {
		return webui.PluginPlan{}, err
	}
	found := false
	for _, plugin := range installed {
		if plugin.Name == name && plugin.Version == version {
			found = true
			break
		}
	}
	if !found {
		return webui.PluginPlan{}, pluginStatus(http.StatusNotFound, "not_found", "plugin version is not installed")
	}
	plan := webui.PluginPlan{
		Action:       "remove",
		Name:         name,
		Version:      version,
		ChecksumFile: "unknown",
	}
	entry, err := registry.GetPlugin(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found in registry") {
			return plan, nil
		}
		if strings.Contains(err.Error(), "catalog overlay") {
			return webui.PluginPlan{}, err
		}
		return webui.PluginPlan{}, err
	}
	plan.InCatalog = true
	plan.Repository = entry.Repository
	plan.SecurityLevel = entry.SecurityLevel
	return plan, nil
}

func resolveCatalogRelease(
	ctx context.Context, entry *registry.RegistryEntry, requested string,
) (registry.GitHubRelease, string, error) {
	owner, repo, err := splitOwnerRepo(entry.Repository)
	if err != nil {
		return registry.GitHubRelease{}, "", err
	}
	client := registry.NewGitHubClient()
	prefix := entry.TagPrefix
	var latest *registry.GitHubRelease
	switch {
	case requested != "":
		latest, err = client.GetReleaseByTag(ctx, owner, repo, registry.ReleaseTag(requested, prefix))
	case prefix != "":
		latest, err = client.GetLatestReleaseWithPrefix(ctx, owner, repo, prefix)
	default:
		latest, err = client.GetLatestRelease(ctx, owner, repo)
	}
	if err != nil {
		return registry.GitHubRelease{}, "", err
	}
	if len(latest.Assets) > 0 {
		return *latest, "", nil
	}
	releases, listErr := client.ListStableReleases(ctx, owner, repo, catalogReleaseScan)
	if listErr != nil {
		return registry.GitHubRelease{}, "", listErr
	}
	earlier, ok := earlierWithAssets(releases, prefix, *latest)
	if !ok {
		return registry.GitHubRelease{}, "", &registry.NoAssetsError{
			Plugin:   entry.Name,
			Version:  registry.CanonicalVersion(latest.TagName, prefix),
			Platform: "catalog",
		}
	}
	fallbackFrom := requested
	if fallbackFrom == "" {
		fallbackFrom = registry.CanonicalVersion(latest.TagName, prefix)
	}
	return earlier, fallbackFrom, nil
}

func earlierWithAssets(
	releases []registry.GitHubRelease, prefix string, latest registry.GitHubRelease,
) (registry.GitHubRelease, bool) {
	var best registry.GitHubRelease
	found := false
	latestCanon := registry.CanonicalVersion(latest.TagName, prefix)
	for _, release := range releases {
		if len(release.Assets) == 0 {
			continue
		}
		if prefix != "" && !strings.HasPrefix(release.TagName, prefix) {
			continue
		}
		canon := registry.CanonicalVersion(release.TagName, prefix)
		cmp, cmpErr := registry.CompareVersions(canon, latestCanon)
		if cmpErr != nil || cmp >= 0 {
			continue
		}
		if !found {
			best = release
			found = true
			continue
		}
		better, betterErr := registry.CompareVersions(canon, registry.CanonicalVersion(best.TagName, prefix))
		if betterErr == nil && better > 0 {
			best = release
		}
	}
	return best, found
}

func splitOwnerRepo(repository string) (string, string, error) {
	owner, repo, ok := strings.Cut(repository, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return "", "", fmt.Errorf("plugin repository %q is not owner/repo", repository)
	}
	return owner, repo, nil
}

func catalogHostError(ctx context.Context, name string, err error) error {
	logging.FromContext(ctx).Warn().
		Str("component", "cli").
		Str("operation", "plugin_catalog").
		Str("plugin", name).
		Msg("catalog host returned an error")
	if err != nil && strings.Contains(err.Error(), githubRateLimitMessage) {
		return pluginStatus(http.StatusBadGateway, "upstream", githubRateLimitMessage)
	}
	if errors.Is(err, registry.ErrReleaseHasNoAssets) {
		return pluginStatus(http.StatusBadGateway, "upstream", "release has no assets yet")
	}
	return pluginStatus(http.StatusBadGateway, "upstream", "catalog host returned an error")
}

func preparePluginChange(name string) error {
	err := registry.NewInstaller(config.New().PluginDir).LockAvailable(name)
	if err == nil {
		return nil
	}
	if errors.Is(err, registry.ErrPluginBusy) {
		return pluginStatus(http.StatusConflict, "busy", "plugin is currently being modified")
	}
	return err
}

func installConfirmedPlugin(
	ctx context.Context,
	name string,
	request webui.PluginOperationRequest,
	report func(webui.PluginProgress),
	reload func(context.Context) error,
) error {
	dir := config.New().PluginDir
	spec := name
	if request.Version != "" {
		spec = name + "@" + request.Version
	}
	opts := registry.InstallOptions{
		PluginDir:             dir,
		CatalogOnly:           request.CatalogOnly,
		RequireChecksumFile:   request.RequireChecksumFile,
		AcceptMissingChecksum: request.AcceptMissingChecksum,
		OnEvent:               forwardProgress(report),
	}
	if request.Version != "" {
		if _, statErr := os.Stat(filepath.Join(dir, name, request.Version)); statErr == nil {
			opts.Force = true
		}
	}
	if _, err := registry.NewInstaller(dir).Install(ctx, spec, opts, nil); err != nil {
		return err
	}
	return reopenPlugins(ctx, name, reload, pluginReopenFailed)
}

func removeConfirmedPlugin(
	ctx context.Context,
	name string,
	request webui.PluginOperationRequest,
	report func(webui.PluginProgress),
	reload func(context.Context) error,
) error {
	dir := config.New().PluginDir
	err := registry.NewInstaller(dir).Remove(name, registry.RemoveOptions{
		PluginDir: dir,
		Version:   request.Version,
		OnEvent:   forwardProgress(report),
	}, nil)
	if err != nil {
		return err
	}
	return reopenPlugins(ctx, name, reload, pluginRemoveReopen)
}

func reopenPlugins(ctx context.Context, name string, reload func(context.Context) error, message string) error {
	if reload == nil {
		return nil
	}
	if err := reload(ctx); err != nil {
		logging.FromContext(ctx).Warn().
			Str("component", "cli").
			Str("operation", "reload_plugins").
			Str("plugin", name).
			Err(err).
			Msg("plugin files changed but plugins could not be reopened")
		return errors.New(message)
	}
	return nil
}

func forwardProgress(report func(webui.PluginProgress)) func(registry.ProgressEvent) {
	if report == nil {
		return nil
	}
	return func(event registry.ProgressEvent) {
		report(webui.PluginProgress{
			Phase:   string(event.Phase),
			Message: event.Message,
			Bytes:   event.Bytes,
			Total:   event.Total,
			Warning: event.Warning,
		})
	}
}

func validateInstalledPlugin(ctx context.Context, name string) (webui.PluginValidateResult, error) {
	installed, err := registry.NewDefault().ListPlugins()
	if err != nil {
		return webui.PluginValidateResult{}, err
	}
	info, ok := newestPlugin(installed, name)
	if !ok {
		return webui.PluginValidateResult{}, pluginStatus(http.StatusNotFound, "not_found", "plugin is not installed")
	}
	if validateErr := ValidatePlugin(ctx, info); validateErr != nil {
		return webui.PluginValidateResult{Passed: false, Reason: safePluginReason(validateErr)}, nil
	}
	launchCtx, cancel := context.WithTimeout(ctx, pluginProbeTimeout)
	defer cancel()
	client, launchErr := pluginhost.NewClient(
		pluginhost.WithInstallName(launchCtx, name), pluginhost.NewProcessLauncher(), info.Path,
	)
	if launchErr != nil {
		return webui.PluginValidateResult{Passed: false, Reason: safePluginReason(launchErr)}, nil
	}
	defer func() { _ = client.Close() }()
	infoCtx, infoCancel := context.WithTimeout(ctx, pluginInfoTimeout)
	defer infoCancel()
	if _, infoErr := client.API.GetPluginInfo(infoCtx, &proto.Empty{}); infoErr != nil {
		return webui.PluginValidateResult{Passed: false, Reason: safePluginReason(infoErr)}, nil
	}
	return webui.PluginValidateResult{Passed: true}, nil
}

func newestPlugin(plugins []registry.PluginInfo, name string) (registry.PluginInfo, bool) {
	var best registry.PluginInfo
	found := false
	for _, plugin := range plugins {
		if plugin.Name != name {
			continue
		}
		if !found || pluginVersionNewer(plugin.Version, best.Version) {
			best = plugin
			found = true
		}
	}
	return best, found
}

func pluginVersionNewer(candidate, current string) bool {
	cmp, err := registry.CompareVersions(candidate, current)
	if err != nil {
		return candidate > current
	}
	return cmp > 0
}

func safePluginReason(err error) string {
	if err == nil {
		return ""
	}
	message := pluginReasonURL.ReplaceAllString(err.Error(), "[url]")
	if len(message) > pluginReasonLimit {
		message = message[:pluginReasonLimit]
	}
	return message
}

func pluginStatus(status int, code, text string) error {
	return &webui.StatusError{Status: status, Code: code, Text: text}
}
