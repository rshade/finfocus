package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"strconv"

	"github.com/tailscale/hujson"
	"gopkg.in/yaml.v3"
)

// PluginConfigValues reads plugins.<name> from a config file without redacting
// secret values. Missing files return an empty map. Nested objects are skipped.
func PluginConfigValues(path, plugin string) (map[string]string, error) {
	doc, err := loadPluginDocument(path)
	if err != nil {
		return nil, err
	}
	return pluginScalars(doc, plugin), nil
}

// loadPluginDocument keeps JSON numbers as [json.Number] so an integer above the
// float64 exact range is not rounded before it is injected into a plugin.
func loadPluginDocument(path string) (map[string]any, error) {
	if path == "" {
		return map[string]any{}, nil
	}
	data, err := os.ReadFile(PreferredConfigPath(path))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("reading configuration: %w", err)
	}
	return decodeNumberDocument(data)
}

func decodeNumberDocument(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	if isYAMLDocument(data) {
		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("parsing configuration: %w", err)
		}
		if doc == nil {
			return map[string]any{}, nil
		}
		return doc, nil
	}
	value, err := hujson.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing configuration: %w", err)
	}
	value.Standardize()
	decoder := json.NewDecoder(bytes.NewReader(value.Pack()))
	decoder.UseNumber()
	var doc map[string]any
	if err = decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("parsing configuration: %w", err)
	}
	if doc == nil {
		return map[string]any{}, nil
	}
	return doc, nil
}

func pluginScalars(doc map[string]any, plugin string) map[string]string {
	out := map[string]string{}
	plugins, _ := doc[keyPlugins].(map[string]any)
	raw, _ := plugins[plugin].(map[string]any)
	for key, value := range raw {
		text, ok := configScalar(value)
		if ok {
			out[key] = text
		}
	}
	return out
}

func configScalar(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case bool:
		return strconv.FormatBool(typed), true
	case int:
		return strconv.Itoa(typed), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case json.Number:
		return typed.String(), true
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return "", false
		}
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	default:
		return "", false
	}
}
