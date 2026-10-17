package registry

import (
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// CatalogPlugin is one row of the built-in catalog merged with what is installed.
type CatalogPlugin struct {
	Name              string
	Description       string
	Providers         []string
	Capabilities      []string
	SecurityLevel     string
	Repository        string
	InCatalog         bool
	InstalledVersions []string
	LatestVersion     string
	UpdateAvailable   bool
	Warning           string
}

// MergeCatalog lists every catalog entry plus installed plugins the catalog
// does not contain. latest is not called for plugins outside the catalog.
// A latest error leaves LatestVersion empty and records Warning.
func MergeCatalog(
	entries []RegistryEntry,
	installed []PluginInfo,
	latest func(RegistryEntry) (string, error),
) []CatalogPlugin {
	byName := map[string][]string{}
	for _, plugin := range installed {
		byName[plugin.Name] = append(byName[plugin.Name], plugin.Version)
	}
	seen := map[string]bool{}
	rows := make([]CatalogPlugin, 0, len(entries)+len(byName))
	for _, entry := range entries {
		seen[entry.Name] = true
		row := CatalogPlugin{
			Name:              entry.Name,
			Description:       entry.Description,
			Providers:         copyStrings(entry.SupportedProviders),
			Capabilities:      copyStrings(entry.Capabilities),
			SecurityLevel:     entry.SecurityLevel,
			Repository:        entry.Repository,
			InCatalog:         true,
			InstalledVersions: sortedVersions(byName[entry.Name]),
		}
		if latest != nil {
			version, err := latest(entry)
			if err != nil {
				row.Warning = "latest version is unavailable"
			} else {
				row.LatestVersion = version
				row.UpdateAvailable = versionIsNewer(version, row.InstalledVersions)
			}
		}
		rows = append(rows, row)
	}
	var extras []string
	for name := range byName {
		if !seen[name] {
			extras = append(extras, name)
		}
	}
	slices.Sort(extras)
	for _, name := range extras {
		rows = append(rows, CatalogPlugin{
			Name:              name,
			InCatalog:         false,
			InstalledVersions: sortedVersions(byName[name]),
			Providers:         []string{},
			Capabilities:      []string{},
		})
	}
	slices.SortFunc(rows, func(a, b CatalogPlugin) int {
		return strings.Compare(a.Name, b.Name)
	})
	return rows
}

func copyStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string{}, values...)
}

func sortedVersions(versions []string) []string {
	if len(versions) == 0 {
		return []string{}
	}
	out := append([]string(nil), versions...)
	slices.SortFunc(out, func(a, b string) int {
		left, leftErr := semver.NewVersion(a)
		right, rightErr := semver.NewVersion(b)
		if leftErr != nil || rightErr != nil {
			return strings.Compare(a, b)
		}
		return left.Compare(right)
	})
	return slices.Compact(out)
}

func versionIsNewer(latest string, installed []string) bool {
	if latest == "" || len(installed) == 0 {
		return false
	}
	newest := installed[0]
	for _, version := range installed[1:] {
		cmp, err := CompareVersions(version, newest)
		if err == nil && cmp > 0 {
			newest = version
		}
	}
	cmp, err := CompareVersions(latest, newest)
	return err == nil && cmp > 0
}
