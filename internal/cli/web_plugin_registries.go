package cli

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/webui"
)

func wireCustomRegistries(options *webui.SessionOptions, service *registry.CustomRegistryService) {
	options.PluginRegistries = func(ctx context.Context) ([]webui.PluginRegistryRow, error) {
		sources, err := service.List(ctx)
		if err != nil {
			return nil, err
		}
		rows := make([]webui.PluginRegistryRow, 0, len(sources))
		for _, source := range sources {
			rows = append(rows, webui.PluginRegistryRow{
				Name: source.Name, URL: source.URL, Trust: source.Trust, SHA256: source.SHA256,
			})
		}
		return rows, nil
	}
	options.PluginRegistryAdd = func(ctx context.Context, request webui.PluginRegistryRequest) error {
		source := config.PluginRegistrySource{
			Name: request.Name, URL: request.URL, Trust: request.Trust, SHA256: request.SHA256,
		}
		if err := config.ValidatePluginRegistrySource(source); err != nil {
			return pluginStatus(http.StatusBadRequest, "invalid_registry", err.Error())
		}
		if err := service.Add(ctx, source); err != nil {
			return pluginStatus(http.StatusBadRequest, "registry_add_failed", safePluginReason(err))
		}
		return nil
	}
	options.PluginRegistryRemove = func(ctx context.Context, name string) error {
		if err := service.Remove(ctx, name); err != nil {
			return pluginStatus(http.StatusBadRequest, "registry_remove_failed", safePluginReason(err))
		}
		return nil
	}
}

func appendCustomCatalog(
	ctx context.Context, service *registry.CustomRegistryService, entries *[]registry.RegistryEntry,
) map[string]string {
	plugins, err := service.Catalog(ctx)
	if err != nil {
		logging.FromContext(ctx).Warn().Str("component", "cli").
			Str("operation", "plugin_catalog").Msg("a custom plugin registry is unavailable")
	}
	sources := make(map[string]string, len(plugins))
	*entries = slices.Grow(*entries, len(plugins))
	for _, plugin := range plugins {
		*entries = append(*entries, plugin.Entry)
		sources[plugin.Entry.Name] = plugin.Source.Name
	}
	return sources
}

func planCustomPlugin(
	ctx context.Context, service *registry.CustomRegistryService, name string, request webui.PluginPlanRequest,
) (webui.PluginPlan, error) {
	plugin, err := service.Resolve(ctx, request.Registry, name)
	if err != nil {
		return webui.PluginPlan{}, customRegistryStatus(err)
	}
	release, fallbackFrom, err := resolveCatalogRelease(ctx, &plugin.Entry, request.Version)
	if err != nil {
		return webui.PluginPlan{}, catalogHostError(ctx, name, err)
	}
	if registry.FindChecksumAsset(&release) == nil {
		return webui.PluginPlan{}, pluginStatus(
			http.StatusConflict, "checksum_required", "custom registry releases require checksums.txt",
		)
	}
	confirmed, err := config.PluginRegistryConfirmed(ctx, plugin.Source)
	if err != nil {
		return webui.PluginPlan{}, customRegistryStatus(err)
	}
	return webui.PluginPlan{
		Action:                       request.Action,
		Name:                         name,
		Version:                      registry.CanonicalVersion(release.TagName, plugin.Entry.TagPrefix),
		Repository:                   plugin.Entry.Repository,
		SecurityLevel:                plugin.Entry.SecurityLevel,
		FallbackFrom:                 fallbackFrom,
		ChecksumFile:                 "present",
		InCatalog:                    true,
		Deprecated:                   plugin.Entry.Deprecated,
		Replacement:                  plugin.Entry.Replacement,
		Registry:                     plugin.Source.Name,
		RegistryRevision:             plugin.Revision(),
		RegistryConfirmationRequired: !confirmed,
	}, nil
}

func installConfirmedCustomPlugin(
	ctx context.Context, service *registry.CustomRegistryService, name string, request webui.PluginOperationRequest,
	report func(webui.PluginProgress), reload func(context.Context) error,
) error {
	plugin, err := service.Resolve(ctx, request.Registry, name)
	if err != nil {
		return customRegistryStatus(err)
	}
	if plugin.Revision() != request.RegistryRevision || plugin.Entry.Repository != request.Repository {
		return pluginStatus(http.StatusConflict, "registry_changed", "registry entry changed; review a new plan")
	}
	opts := registry.InstallOptions{OnEvent: forwardProgress(report)}
	if _, installErr := service.Install(
		ctx, registry.NewInstaller(config.New().PluginDir), plugin, request.Version,
		request.RegistryConfirmation, opts, nil,
	); installErr != nil {
		return installErr
	}
	return reopenPlugins(ctx, name, reload, pluginReopenFailed)
}

func customRegistryStatus(err error) error {
	if errors.Is(err, registry.ErrCustomPluginNotFound) {
		return pluginStatus(http.StatusNotFound, "not_found", err.Error())
	}
	return pluginStatus(http.StatusBadGateway, "custom_registry", safePluginReason(err))
}
