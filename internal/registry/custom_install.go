package registry

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rshade/finfocus/internal/config"
)

func (i *Installer) installCustom(
	ctx context.Context, plugin CustomPlugin, version string, opts InstallOptions, progress func(string),
) (*InstallResult, error) {
	specifier := plugin.Entry.Name
	if version != "" {
		if !IsValidVersion(version) {
			return nil, errors.New("custom registry installs require a semantic release version")
		}
		specifier += "@" + version
	}
	spec, err := ParsePluginSpecifier(specifier)
	if err != nil || spec.IsURL || spec.Name != plugin.Entry.Name {
		return nil, ErrCustomPluginNotFound
	}
	if !pluginNamePattern.MatchString(spec.Name) {
		return nil, ErrCustomPluginNotFound
	}
	names, err := builtinNames()
	if err != nil {
		return nil, err
	}
	if _, reserved := names[spec.Name]; reserved {
		return nil, ErrCustomPluginNotFound
	}

	unlock, err := i.acquireLock(spec.Name)
	if err != nil {
		return nil, err
	}
	defer unlock()
	if ownerErr := checkCustomInstallOwner(plugin); ownerErr != nil {
		return nil, ownerErr
	}

	opts.CatalogOnly = true
	opts.SkipChecksum = false
	opts.NoSave = false
	opts.RequireChecksumFile = true
	opts.AcceptMissingChecksum = false
	opts.registrySource = plugin.Source.Name
	return i.installCatalogEntry(ctx, spec, &plugin.Entry, opts, newProgressReport(progress, opts.OnEvent))
}

// checkCustomInstallOwner prevents a registry from replacing a plugin already
// registered to a different source or repository under the same local name.
func checkCustomInstallOwner(plugin CustomPlugin) error {
	installed, err := config.LoadInstalledPlugins()
	if err != nil {
		return fmt.Errorf("reading installed plugin provenance: %w", err)
	}
	for _, current := range installed {
		if current.Name != plugin.Entry.Name {
			continue
		}
		if current.Registry != plugin.Source.Name ||
			!strings.EqualFold(current.URL, "github.com/"+plugin.Entry.Repository) {
			return errors.New("plugin name is already installed from another source")
		}
	}
	return nil
}
