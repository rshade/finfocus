package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// pluginSecretFile holds dotted plugin setting keys that a plugin advertised
// as secret. It stores names, never values, so a later Config read can mask
// them without launching the plugin again.
const pluginSecretFileName = "plugin-secret-keys.json"

var (
	pluginSecretMu   sync.Mutex          //nolint:gochecknoglobals // remembered descriptor secret key names
	pluginSecretKeys = map[string]bool{} //nolint:gochecknoglobals // remembered descriptor secret key names
	pluginSecretLoad sync.Once           //nolint:gochecknoglobals // loads the key-name file once
)

// NotePluginSecretKeys remembers dotted keys such as plugins.recorder.license.
// Credential-like names are already masked without this record.
func NotePluginSecretKeys(keys []string) {
	if len(keys) == 0 {
		return
	}
	loadPluginSecretKeys()
	pluginSecretMu.Lock()
	defer pluginSecretMu.Unlock()
	for _, key := range keys {
		if key != "" {
			pluginSecretKeys[key] = true
		}
	}
}

// PersistPluginSecretKeys writes the remembered key names beside the global
// config. A missing directory or a failed write leaves the in-memory set.
func PersistPluginSecretKeys() error {
	loadPluginSecretKeys()
	pluginSecretMu.Lock()
	names := make([]string, 0, len(pluginSecretKeys))
	for key := range pluginSecretKeys {
		names = append(names, key)
	}
	pluginSecretMu.Unlock()
	if len(names) == 0 {
		return nil
	}
	slices.Sort(names)
	data, err := json.Marshal(names)
	if err != nil {
		return err
	}
	_, err = writeAtomic(filepath.Join(ResolveConfigDir(), pluginSecretFileName), data)
	return err
}

func pluginSecretNoted(key string) bool {
	loadPluginSecretKeys()
	pluginSecretMu.Lock()
	defer pluginSecretMu.Unlock()
	return pluginSecretKeys[key]
}

func loadPluginSecretKeys() {
	pluginSecretLoad.Do(func() {
		data, err := os.ReadFile(filepath.Join(ResolveConfigDir(), pluginSecretFileName))
		if err != nil {
			return
		}
		var names []string
		if json.Unmarshal(data, &names) != nil {
			return
		}
		pluginSecretMu.Lock()
		defer pluginSecretMu.Unlock()
		for _, key := range names {
			if key != "" {
				pluginSecretKeys[key] = true
			}
		}
	})
}

// ResetPluginSecretsForTest clears the remembered secret keys.
func ResetPluginSecretsForTest() {
	pluginSecretMu.Lock()
	defer pluginSecretMu.Unlock()
	pluginSecretKeys = map[string]bool{}
	pluginSecretLoad = sync.Once{}
}
