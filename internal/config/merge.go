package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/rshade/ax-go"
	"gopkg.in/yaml.v3"
)

// Top-level config key names used for shallow merge.
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

// ShallowMergeYAML loads a Hujson/JSON file (or legacy YAML for backward compatibility)
// and merges its top-level keys onto the target Config. Keys present in the overlay
// replace entire sections in the target. Keys absent in the overlay are left unchanged.
func ShallowMergeYAML(target *Config, overlayPath string) error {
	if target == nil {
		return errors.New("nil target *Config in ShallowMergeYAML")
	}

	// Read the overlay file
	data, err := os.ReadFile(overlayPath)
	if err != nil {
		return fmt.Errorf("reading overlay file %s: %w", overlayPath, err)
	}

	// Try to parse as Hujson/JSON first (ax.ParseConfig strips comments and
	// trailing commas before delegating to encoding/json, so a genuine Hujson
	// overlay - like the skeleton SaveProjectSkeleton writes - parses here;
	// plain json.Unmarshal would reject its "//" comments outright).
	var overlay map[string]json.RawMessage
	if err := ax.ParseConfig(context.Background(), bytes.NewReader(data), &overlay); err != nil {
		// Hujson/JSON parsing failed, try YAML as fallback for legacy files
		var yamlOverlay map[string]interface{}
		if yamlErr := yaml.Unmarshal(data, &yamlOverlay); yamlErr != nil {
			return fmt.Errorf("parsing overlay file from %s (tried Hujson and YAML): %w", overlayPath, yamlErr)
		}
		// Convert YAML map to JSON RawMessage map
		overlay = make(map[string]json.RawMessage)
		for key, value := range yamlOverlay {
			if !knownTopLevelKeys[key] {
				continue
			}
			jsonBytes, marshalErr := json.Marshal(value)
			if marshalErr != nil {
				return fmt.Errorf("converting overlay section %q to JSON: %w", key, marshalErr)
			}
			overlay[key] = jsonBytes
		}
	}

	// Empty or comment-only file: nothing to merge.
	if len(overlay) == 0 {
		return nil
	}

	for key, rawValue := range overlay {
		if !knownTopLevelKeys[key] {
			continue
		}

		if err = unmarshalSection(target, key, rawValue); err != nil {
			return fmt.Errorf("applying overlay section %q: %w", key, err)
		}
	}

	return nil
}

// unmarshalSection unmarshals raw JSON bytes into the correct field of target
// based on the given key name. Each section is unmarshalled into a fresh
// zero-value to ensure complete replacement (json.Unmarshal merges into
// existing maps, which would violate shallow-merge semantics).
func unmarshalSection(target *Config, key string, data []byte) error {
	switch key {
	case keyOutput:
		var v OutputConfig
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Output = v
		return nil
	case keyPlugins:
		var v map[string]PluginConfig
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Plugins = v
		return nil
	case keyLogging:
		var v LoggingConfig
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Logging = v
		return nil
	case keyAnalyzer:
		var v AnalyzerConfig
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Analyzer = v
		return nil
	case keyPluginHost:
		var v PluginHostConfig
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		target.PluginHostConfig = v
		return nil
	case keyCost:
		var v CostConfig
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Cost = v
		return nil
	case keyRouting:
		var v RoutingConfig
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		target.Routing = &v
		return nil
	default:
		// Defensive safety net: callers filter keys against knownTopLevelKeys before
		// calling unmarshalSection, so this branch is effectively unreachable. It exists
		// to catch future mismatches if a key is added to knownTopLevelKeys without a
		// corresponding case here.
		return fmt.Errorf("unknown config key: %s", key)
	}
}
