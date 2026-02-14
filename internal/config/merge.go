package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Top-level YAML config key names used for shallow merge.
const (
	keyOutput     = "output"
	keyPlugins    = "plugins"
	keyLogging    = "logging"
	keyAnalyzer   = "analyzer"
	keyPluginHost = "plugin_host"
	keyCost       = "cost"
	keyRouting    = "routing"
)

// knownTopLevelKeys lists the YAML keys that correspond to exported Config fields.
// Keys not in this list are silently ignored during merge.
//
//nolint:gochecknoglobals // Compile-time constant lookup table.
var knownTopLevelKeys = map[string]bool{
	keyOutput:     true,
	keyPlugins:    true,
	keyLogging:    true,
	keyAnalyzer:   true,
	keyPluginHost: true,
	keyCost:       true,
	keyRouting:    true,
}

// ShallowMergeYAML loads a YAML file and merges its top-level keys onto
// the target Config. Keys present in the overlay replace entire sections
// in the target. Keys absent in the overlay are left unchanged.
func ShallowMergeYAML(target *Config, overlayPath string) error {
	if target == nil {
		return errors.New("nil target *Config in ShallowMergeYAML")
	}

	data, err := os.ReadFile(overlayPath)
	if err != nil {
		return fmt.Errorf("reading overlay file %s: %w", overlayPath, err)
	}

	// Discover which top-level keys are present in the overlay.
	var overlay map[string]interface{}
	if err = yaml.Unmarshal(data, &overlay); err != nil {
		return fmt.Errorf("parsing overlay YAML from %s: %w", overlayPath, err)
	}

	// Empty or comment-only file: nothing to merge.
	if len(overlay) == 0 {
		return nil
	}

	for key, value := range overlay {
		if !knownTopLevelKeys[key] {
			continue
		}

		// Re-marshal the single section so we can unmarshal it onto the
		// strongly-typed target field.
		sectionBytes, marshalErr := yaml.Marshal(value)
		if marshalErr != nil {
			return fmt.Errorf("re-marshalling overlay section %q: %w", key, marshalErr)
		}

		if err = unmarshalSection(target, key, sectionBytes); err != nil {
			return fmt.Errorf("applying overlay section %q: %w", key, err)
		}
	}

	return nil
}

// unmarshalSection unmarshals raw YAML bytes into the correct field of target
// based on the given key name. Each section is unmarshalled into a fresh
// zero-value to ensure complete replacement (yaml.Unmarshal merges into
// existing maps, which would violate shallow-merge semantics).
func unmarshalSection(target *Config, key string, data []byte) error {
	switch key {
	case keyOutput:
		var v OutputConfig
		if err := yaml.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Output = v
		return nil
	case keyPlugins:
		var v map[string]PluginConfig
		if err := yaml.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Plugins = v
		return nil
	case keyLogging:
		var v LoggingConfig
		if err := yaml.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Logging = v
		return nil
	case keyAnalyzer:
		var v AnalyzerConfig
		if err := yaml.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Analyzer = v
		return nil
	case keyPluginHost:
		var v PluginHostConfig
		if err := yaml.Unmarshal(data, &v); err != nil {
			return err
		}
		target.PluginHostConfig = v
		return nil
	case keyCost:
		var v CostConfig
		if err := yaml.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Cost = v
		return nil
	case keyRouting:
		var v RoutingConfig
		if err := yaml.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Routing = &v
		return nil
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}
}
