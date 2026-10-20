package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"

	"github.com/rshade/finfocus/internal/esc"
)

var envNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ESCConfig is the top-level esc section. A nil Environments pointer means
// the stack imports are used. A pointer to an empty slice disables ESC.
type ESCConfig struct {
	Environments *[]string                  `json:"environments,omitempty" yaml:"environments,omitempty"`
	Plugins      map[string]ESCPluginConfig `json:"plugins,omitempty"      yaml:"plugins,omitempty"`
}

// ESCPluginConfig is the esc.plugins.<name> section.
type ESCPluginConfig struct {
	Environments *[]string `json:"environments,omitempty" yaml:"environments,omitempty"`
	Variables    []string  `json:"variables,omitempty"    yaml:"variables,omitempty"`
}

// escCatalogNames, when set, adds catalog plugin names to the unknown-plugin check.
var escCatalogNames func() []string //nolint:gochecknoglobals // Set by the CLI before validation.

// SetESCCatalogNamesForTest installs the catalog names used by ESC validation
// and returns a restore function.
func SetESCCatalogNamesForTest(names []string) func() {
	previous := escCatalogNames
	copied := append([]string(nil), names...)
	escCatalogNames = func() []string { return copied }
	return func() { escCatalogNames = previous }
}

func (c *Config) ensureESC() *ESCConfig {
	if c.ESC == nil {
		c.ESC = &ESCConfig{}
	}
	return c.ESC
}

func (c *Config) getESCValue(parts []string) (any, error) {
	if len(parts) == 0 {
		return c.ESC, nil
	}
	section := c.ESC
	if section == nil {
		section = &ESCConfig{}
	}
	if len(parts) == 1 && parts[0] == escKeyEnvironments {
		return derefStrings(section.Environments), nil
	}
	if len(parts) == 3 && parts[0] == keyPlugins {
		plugin := section.Plugins[parts[1]]
		switch parts[2] {
		case escKeyEnvironments:
			return derefStrings(plugin.Environments), nil
		case escKeyVariables:
			return append([]string(nil), plugin.Variables...), nil
		}
	}
	return nil, fmt.Errorf("unknown esc setting: %s", strings.Join(parts, "."))
}

func (c *Config) setESCValue(parts []string, value string) error {
	if len(parts) == 1 && parts[0] == escKeyEnvironments {
		c.ensureESC().Environments = stringListPtr(value)
		return nil
	}
	if len(parts) == 3 && parts[0] == keyPlugins {
		plugin := c.ensureESC().Plugins[parts[1]]
		if c.ESC.Plugins == nil {
			c.ESC.Plugins = map[string]ESCPluginConfig{}
		}
		switch parts[2] {
		case escKeyEnvironments:
			plugin.Environments = stringListPtr(value)
		case escKeyVariables:
			plugin.Variables = splitList(value)
		default:
			return fmt.Errorf("unknown esc setting: %s", strings.Join(parts, "."))
		}
		c.ESC.Plugins[parts[1]] = plugin
		return nil
	}
	return fmt.Errorf("unknown esc setting: %s", strings.Join(parts, "."))
}

func derefStrings(list *[]string) []string {
	if list == nil {
		return nil
	}
	return append([]string(nil), (*list)...)
}

func stringListPtr(value string) *[]string {
	list := splitList(value)
	return &list
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	var out []string
	for part := range strings.SplitSeq(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}

type escFinding struct {
	path    string
	message string
}

// FirstESCError returns the first malformed ESC reference or variable name.
func (c *Config) FirstESCError() error {
	if c == nil {
		return nil
	}
	errs, _ := c.escFindings()
	if len(errs) == 0 {
		return nil
	}
	return errors.New(errs[0].message)
}

const (
	escKeyEnvironments = "environments"
	escKeyVariables    = "variables"
)

func (c *Config) escFindings() ([]escFinding, []escFinding) {
	var errs, warns []escFinding
	if c == nil || c.ESC == nil {
		return nil, nil
	}
	errs = append(errs, refFindings("esc.environments", c.ESC.Environments)...)
	names := make([]string, 0, len(c.ESC.Plugins))
	for name := range c.ESC.Plugins {
		names = append(names, name)
	}
	slices.Sort(names)
	known := c.knownESCPluginNames()
	for _, name := range names {
		plugin := c.ESC.Plugins[name]
		base := "esc.plugins." + name
		errs = append(errs, refFindings(base+".environments", plugin.Environments)...)
		errs = append(errs, variableFindings(base, plugin.Variables)...)
		if _, ok := known[name]; !ok {
			warns = append(warns, escFinding{
				path:    base,
				message: fmt.Sprintf("%s: no plugin named %q is installed", base, name),
			})
		}
	}
	return errs, warns
}

func refFindings(path string, list *[]string) []escFinding {
	if list == nil {
		return nil
	}
	var out []escFinding
	for i, ref := range *list {
		if _, err := esc.ParseRef(ref); err != nil {
			out = append(out, escFinding{
				path:    fmt.Sprintf("%s[%d]", path, i),
				message: fmt.Sprintf("%s[%d]: %s", path, i, err.Error()),
			})
		}
	}
	return out
}

func variableFindings(path string, names []string) []escFinding {
	var out []escFinding
	for i, name := range names {
		item := fmt.Sprintf("%s.variables[%d]", path, i)
		if !envNamePattern.MatchString(name) {
			out = append(out, escFinding{
				path:    item,
				message: fmt.Sprintf("%s: invalid environment variable name %q", item, name),
			})
			continue
		}
		if reservedEnvName(name) {
			out = append(out, escFinding{
				path:    item,
				message: fmt.Sprintf("%s: %s is reserved for plugin startup", item, name),
			})
		}
	}
	return out
}

func reservedEnvName(name string) bool {
	switch name {
	case pluginsdk.EnvPort, pluginsdk.EnvLogFile, pluginsdk.EnvTraceID:
		return true
	default:
		return false
	}
}

func (c *Config) knownESCPluginNames() map[string]struct{} {
	names := map[string]struct{}{}
	if escCatalogNames != nil {
		for _, name := range escCatalogNames() {
			if name != "" {
				names[name] = struct{}{}
			}
		}
	}
	dir := os.Getenv("FINFOCUS_PLUGIN_DIR")
	if dir == "" && c != nil && c.PluginDirOverride != "" {
		dir = c.PluginDirOverride
	}
	if dir == "" {
		dir = filepath.Join(ResolveConfigDir(), "plugins")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return names
	}
	for _, entry := range entries {
		if entry.IsDir() {
			names[entry.Name()] = struct{}{}
		}
	}
	return names
}

// PluginOptionsForESC copies the configured per-plugin ESC settings.
func PluginOptionsForESC(cfg *ESCConfig) map[string]esc.PluginOptions {
	if cfg == nil || len(cfg.Plugins) == 0 {
		return nil
	}
	out := make(map[string]esc.PluginOptions, len(cfg.Plugins))
	for name, plugin := range cfg.Plugins {
		item := esc.PluginOptions{Variables: append([]string(nil), plugin.Variables...)}
		if plugin.Environments != nil {
			list := append([]string(nil), (*plugin.Environments)...)
			item.Environments = &list
		}
		out[name] = item
	}
	return out
}

// CloneESC returns a deep copy so a masked display cannot alias the stored section.
func CloneESC(cfg *ESCConfig) *ESCConfig {
	if cfg == nil {
		return nil
	}
	cloned := *cfg
	cloned.Environments = cloneStringPtr(cfg.Environments)
	if cfg.Plugins != nil {
		cloned.Plugins = maps.Clone(cfg.Plugins)
		for name, plugin := range cloned.Plugins {
			plugin.Environments = cloneStringPtr(plugin.Environments)
			plugin.Variables = append([]string(nil), plugin.Variables...)
			cloned.Plugins[name] = plugin
		}
	}
	return &cloned
}

func cloneStringPtr(list *[]string) *[]string {
	if list == nil {
		return nil
	}
	copied := append([]string(nil), (*list)...)
	return &copied
}
