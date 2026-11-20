package config

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/tailscale/hujson"
)

const (
	keyPluginRegistries            = "plugin_registries"
	keyPluginRegistryConfirmations = "plugin_registry_confirmations"
	registrySourcesPointer         = "/" + keyPluginRegistries
	registryConfirmationsPointer   = "/" + keyPluginRegistryConfirmations
	httpsScheme                    = "https"

	// PluginRegistryTrustCommunity is the default custom registry trust level.
	PluginRegistryTrustCommunity = "community"
	// PluginRegistryTrustExperimental marks an experimental custom registry.
	PluginRegistryTrustExperimental = "experimental"
)

var (
	// ErrPluginRegistriesGlobalOnly prevents project configuration from choosing executable sources.
	ErrPluginRegistriesGlobalOnly = errors.New(
		"plugin registries and confirmations must be configured in global config",
	)
	// ErrPluginRegistryExists is returned when a source name is already in use.
	ErrPluginRegistryExists = errors.New("plugin registry name already exists")
	// ErrPluginRegistryNotFound is returned when no configured source has the requested name.
	ErrPluginRegistryNotFound = errors.New("plugin registry not found")
	// ErrPluginRegistryChanged is returned when the source differs from the current configuration.
	ErrPluginRegistryChanged = errors.New("plugin registry changed; reload and retry")
)

// PluginRegistrySource is an external index explicitly configured by the user.
type PluginRegistrySource struct {
	Name   string `json:"name"             yaml:"name"`
	URL    string `json:"url"              yaml:"url"`
	Trust  string `json:"trust,omitempty"  yaml:"trust,omitempty"`
	SHA256 string `json:"sha256,omitempty" yaml:"sha256,omitempty"`
}

type pluginRegistryFieldError struct {
	path    string
	message string
}

func (e *pluginRegistryFieldError) Error() string { return e.message }

// ValidatePluginRegistrySource checks the source settings without contacting the index.
func ValidatePluginRegistrySource(source PluginRegistrySource) error {
	if !validPluginRegistryName(source.Name) {
		return &pluginRegistryFieldError{
			path:    "name",
			message: "registry name must use lowercase letters, numbers, and hyphens",
		}
	}
	if !validPluginRegistryURL(source.URL) {
		return &pluginRegistryFieldError{
			path: "url", message: "registry URL must use HTTPS with a host and no credentials, query, or fragment",
		}
	}
	switch source.Trust {
	case "", PluginRegistryTrustCommunity, PluginRegistryTrustExperimental:
	default:
		return &pluginRegistryFieldError{path: "trust", message: "registry trust must be community or experimental"}
	}
	if source.SHA256 != "" {
		decoded, err := hex.DecodeString(source.SHA256)
		if err != nil || len(decoded) != sha256.Size {
			return &pluginRegistryFieldError{
				path:    "sha256",
				message: "registry sha256 must contain 64 hexadecimal digits",
			}
		}
	}
	return nil
}

func validPluginRegistryName(name string) bool {
	return regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`).MatchString(name)
}

func validPluginRegistryURL(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.Scheme == httpsScheme && parsed.Hostname() != "" && parsed.User == nil &&
		parsed.RawQuery == "" && !parsed.ForceQuery && !strings.Contains(raw, "#")
}

func normalizePluginRegistrySource(source PluginRegistrySource) PluginRegistrySource {
	if source.Trust == "" {
		source.Trust = PluginRegistryTrustCommunity
	}
	source.SHA256 = strings.ToLower(source.SHA256)
	return source
}

func validatePluginRegistrySources(sources []PluginRegistrySource) error {
	names := make(map[string]bool, len(sources))
	for i, source := range sources {
		path := fmt.Sprintf("%s[%d]", keyPluginRegistries, i)
		if err := ValidatePluginRegistrySource(source); err != nil {
			if field, ok := errors.AsType[*pluginRegistryFieldError](err); ok {
				return &pluginRegistryFieldError{path: path + "." + field.path, message: field.message}
			}
			return err
		}
		if names[source.Name] {
			return &pluginRegistryFieldError{path: path + ".name", message: ErrPluginRegistryExists.Error()}
		}
		names[source.Name] = true
	}
	return nil
}

func pluginRegistryKey(name string) string {
	switch {
	case strings.EqualFold(name, keyPluginRegistries):
		return keyPluginRegistries
	case strings.EqualFold(name, keyPluginRegistryConfirmations):
		return keyPluginRegistryConfirmations
	default:
		return ""
	}
}

func validatePluginRegistryKeys(object *hujson.Object) error {
	seen := make(map[string]bool)
	for _, member := range object.Members {
		name, _ := literalString(member.Name)
		key := pluginRegistryKey(name)
		if key == "" {
			continue
		}
		if key != name || seen[key] {
			return &pluginRegistryFieldError{
				path:    name,
				message: "registry config keys must use their exact name and appear once",
			}
		}
		seen[key] = true
	}
	return nil
}

type pluginRegistryDocument struct {
	value         hujson.Value
	Sources       []PluginRegistrySource `json:"plugin_registries"`
	Confirmations map[string]string      `json:"plugin_registry_confirmations"`
}

func readPluginRegistryDocument(path string) (pluginRegistryDocument, error) {
	data, _, err := readTier(path)
	if err != nil {
		return pluginRegistryDocument{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		data = []byte("{}")
	}
	if isYAMLDocument(data) {
		data, err = yamlToJSON(data)
		if err != nil {
			return pluginRegistryDocument{}, fmt.Errorf("converting legacy plugin registry configuration: %w", err)
		}
	}
	value, err := hujson.Parse(data)
	if err != nil {
		return pluginRegistryDocument{}, fmt.Errorf("parsing plugin registry configuration: %w", err)
	}
	object, ok := value.Value.(*hujson.Object)
	if !ok {
		return pluginRegistryDocument{}, errors.New("plugin registry configuration must be an object")
	}
	if err = validatePluginRegistryKeys(object); err != nil {
		return pluginRegistryDocument{}, err
	}
	standard := value.Clone()
	standard.Standardize()
	var document pluginRegistryDocument
	if err = json.Unmarshal(standard.Pack(), &document); err != nil {
		return pluginRegistryDocument{}, fmt.Errorf("reading plugin registry configuration: %w", err)
	}
	if err = validatePluginRegistrySources(document.Sources); err != nil {
		return pluginRegistryDocument{}, err
	}
	document.value = value
	for i := range document.Sources {
		document.Sources[i] = normalizePluginRegistrySource(document.Sources[i])
	}
	return document, nil
}

// GlobalPluginRegistries reads the global sources in configuration order. A
// working-directory fallback may be a project file and supplies no sources.
func GlobalPluginRegistries(ctx context.Context) ([]PluginRegistrySource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir, fallback := resolveConfigDir()
	if fallback {
		return nil, nil
	}
	document, err := readPluginRegistryDocument(filepath.Join(dir, "config.hujson"))
	return document.Sources, err
}

func updatePluginRegistryDocument(
	ctx context.Context, update func(*pluginRegistryDocument) error,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir, fallback := resolveConfigDir()
	if fallback {
		return ErrPluginRegistriesGlobalOnly
	}
	target := writeDestination(filepath.Join(dir, "config.hujson"), TierGlobal)
	unlock, err := acquirePathLock(target)
	if err != nil {
		return err
	}
	defer unlock()

	document, err := readPluginRegistryDocument(target)
	if err != nil {
		return err
	}
	if updateErr := update(&document); updateErr != nil {
		return updateErr
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	_, err = writeAtomic(target, document.value.Pack())
	return err
}

func pluginRegistryIndex(sources []PluginRegistrySource, name string) int {
	return slices.IndexFunc(sources, func(item PluginRegistrySource) bool { return item.Name == name })
}

// AddPluginRegistry appends a new source to the global config.
func AddPluginRegistry(ctx context.Context, source PluginRegistrySource) error {
	if err := ValidatePluginRegistrySource(source); err != nil {
		return err
	}
	source = normalizePluginRegistrySource(source)

	return updatePluginRegistryDocument(ctx, func(document *pluginRegistryDocument) error {
		if pluginRegistryIndex(document.Sources, source.Name) >= 0 {
			return ErrPluginRegistryExists
		}
		literal, err := json.Marshal(source)
		if err != nil {
			return fmt.Errorf("encoding plugin registry source: %w", err)
		}
		if document.Sources == nil {
			if patchErr := setAtPointer(&document.value, registrySourcesPointer, []byte("[]")); patchErr != nil {
				return patchErr
			}
		}
		if clearErr := document.clearConfirmation(source.Name); clearErr != nil {
			return clearErr
		}
		return setAtPointer(&document.value, registrySourcesPointer+"/-", literal)
	})
}

// RemovePluginRegistry removes the named global source and its confirmation.
func RemovePluginRegistry(ctx context.Context, name string) error {
	return updatePluginRegistryDocument(ctx, func(document *pluginRegistryDocument) error {
		i := pluginRegistryIndex(document.Sources, name)
		if i < 0 {
			return ErrPluginRegistryNotFound
		}
		if err := removeAtPointer(&document.value, registrySourcesPointer+"/"+strconv.Itoa(i)); err != nil {
			return err
		}
		return document.clearConfirmation(name)
	})
}

func (d *pluginRegistryDocument) clearConfirmation(name string) error {
	if _, exists := d.Confirmations[name]; !exists {
		return nil
	}
	return removeAtPointer(&d.value, registryConfirmationsPointer+"/"+name)
}

func (d *pluginRegistryDocument) currentSource(source PluginRegistrySource) error {
	i := pluginRegistryIndex(d.Sources, source.Name)
	if i < 0 {
		return ErrPluginRegistryNotFound
	}
	if d.Sources[i] != source {
		return ErrPluginRegistryChanged
	}
	return nil
}

// ConfirmPluginRegistry records explicit approval for the current source identity.
func ConfirmPluginRegistry(ctx context.Context, source PluginRegistrySource) error {
	source = normalizePluginRegistrySource(source)

	return updatePluginRegistryDocument(ctx, func(document *pluginRegistryDocument) error {
		if err := document.currentSource(source); err != nil {
			return err
		}
		if document.Confirmations == nil {
			if err := setAtPointer(&document.value, registryConfirmationsPointer, []byte("{}")); err != nil {
				return err
			}
		}
		fingerprint := strconv.Quote(pluginRegistryFingerprint(source))
		return setAtPointer(&document.value, registryConfirmationsPointer+"/"+source.Name, []byte(fingerprint))
	})
}

// PluginRegistryConfirmed reports whether the currently configured source was approved.
func PluginRegistryConfirmed(ctx context.Context, source PluginRegistrySource) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	source = normalizePluginRegistrySource(source)
	dir, fallback := resolveConfigDir()
	if fallback {
		return false, ErrPluginRegistriesGlobalOnly
	}
	document, err := readPluginRegistryDocument(filepath.Join(dir, "config.hujson"))
	if err != nil {
		return false, err
	}
	if sourceErr := document.currentSource(source); sourceErr != nil {
		return false, sourceErr
	}
	return document.Confirmations[source.Name] == pluginRegistryFingerprint(source), nil
}

func pluginRegistryFingerprint(source PluginRegistrySource) string {
	return hashBytes([]byte(strings.Join([]string{source.Name, source.URL, source.Trust, source.SHA256}, "\x00")))
}
