package pluginhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/logging"
)

// SettingDesc is one advertised plugin setting, or a config key with no descriptor.
type SettingDesc struct {
	Key        string
	Env        string
	Default    string
	HasDefault bool
	Secret     bool
	Required   bool
}

type installNameContextKey struct{}
type deliveredEnvContextKey struct{}

// WithInstallName returns a context that tells NewClient which install directory
// name owns plugins.<name> in the config file. The RPC name can differ.
func WithInstallName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, installNameContextKey{}, name)
}

// InstallNameFromContext returns the install directory name, if one was set.
func InstallNameFromContext(ctx context.Context) (string, bool) {
	name, ok := ctx.Value(installNameContextKey{}).(string)
	if !ok || name == "" {
		return "", false
	}
	return name, true
}

// WithDeliveredEnv attaches KEY=VALUE pairs for the plugin process.
// Pairs already present in the parent environment are not applied.
func WithDeliveredEnv(ctx context.Context, pairs []string) context.Context {
	copied := append([]string(nil), pairs...)
	return context.WithValue(ctx, deliveredEnvContextKey{}, copied)
}

// DeliveredEnvFromContext returns pairs attached by WithDeliveredEnv.
func DeliveredEnvFromContext(ctx context.Context) ([]string, bool) {
	pairs, ok := ctx.Value(deliveredEnvContextKey{}).([]string)
	if !ok {
		return nil, false
	}
	return pairs, true
}

// ResolveDeliveredEnv chooses the environment a plugin receives.
// Precedence for each key is parent environment, project file, global file,
// then the descriptor default. Descriptor keys use descriptor.Env.
// Keys with no descriptor use FINFOCUS_PLUGIN_<NAME>_<KEY>.
// injected lists only pairs that are not already in parent.
// digest is a hash of every effective pair, including parent values.
// An empty effective set returns an empty digest.
// Secret values are never written to the log.
func ResolveDeliveredEnv(
	ctx context.Context,
	plugin string,
	parent []string,
	project, global map[string]string,
	settings []SettingDesc,
) ([]string, string) {
	var injected []string
	parentSet, parentValues := parentEnvironment(parent)
	effective := map[string]string{}
	described := map[string]bool{}

	for _, setting := range settings {
		if setting.Key == "" || setting.Env == "" {
			continue
		}
		described[setting.Key] = true
		value, ok := pickSetting(parentSet, parentValues, project, global, setting)
		if ok {
			effective[setting.Env] = value
		}
	}
	for _, key := range configKeys(project, global) {
		if described[key] {
			continue
		}
		envName := genericPluginEnv(plugin, key)
		value, ok := pickGeneric(parentSet, parentValues, project, global, key, envName)
		if ok {
			effective[envName] = value
		}
	}

	names := make([]string, 0, len(effective))
	for envName := range effective {
		names = append(names, envName)
	}
	slices.Sort(names)
	for _, envName := range names {
		if parentSet[envName] {
			continue
		}
		injected = append(injected, envName+"="+effective[envName])
	}

	log := logging.FromContext(ctx)
	log.Debug().
		Str("component", "pluginhost").
		Str("operation", "resolve_settings").
		Str("plugin", plugin).
		Int("count", len(names)).
		Strs("env", names).
		Msg("resolved plugin settings")

	if len(names) == 0 {
		return injected, ""
	}
	return injected, hashDelivered(plugin, names, effective)
}

func parentEnvironment(parent []string) (map[string]bool, map[string]string) {
	set := make(map[string]bool, len(parent))
	values := make(map[string]string, len(parent))
	for _, entry := range parent {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		set[key] = true
		values[key] = value
	}
	return set, values
}

func pickSetting(
	parentSet map[string]bool,
	parentValues, project, global map[string]string,
	setting SettingDesc,
) (string, bool) {
	if parentSet[setting.Env] {
		return parentValues[setting.Env], true
	}
	if value, ok := project[setting.Key]; ok {
		return value, true
	}
	if value, ok := global[setting.Key]; ok {
		return value, true
	}
	if setting.HasDefault {
		return setting.Default, true
	}
	return "", false
}

func pickGeneric(
	parentSet map[string]bool,
	parentValues, project, global map[string]string,
	key, envName string,
) (string, bool) {
	if parentSet[envName] {
		return parentValues[envName], true
	}
	if value, ok := project[key]; ok {
		return value, true
	}
	if value, ok := global[key]; ok {
		return value, true
	}
	return "", false
}

func configKeys(project, global map[string]string) []string {
	seen := map[string]bool{}
	var keys []string
	for key := range project {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for key := range global {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

func genericPluginEnv(plugin, key string) string {
	name := strings.ToUpper(strings.ReplaceAll(plugin, "-", "_"))
	configKey := strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
	return "FINFOCUS_PLUGIN_" + name + "_" + configKey
}

// GenericPluginEnv is the process variable for a stored key that has no descriptor.
func GenericPluginEnv(plugin, key string) string {
	return genericPluginEnv(plugin, key)
}

func hashDelivered(plugin string, names []string, values map[string]string) string {
	sum := sha256.New()
	_, _ = sum.Write([]byte(plugin))
	_, _ = sum.Write([]byte{0})
	for _, name := range names {
		_, _ = sum.Write([]byte(name))
		_, _ = sum.Write([]byte{0})
		_, _ = sum.Write([]byte(values[name]))
		_, _ = sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func resolveInjected(
	ctx context.Context,
	plugin string,
	settings []*pbc.PluginSetting,
) ([]string, string) {
	project, global := pluginFileMaps(ctx, plugin)
	return ResolveDeliveredEnv(ctx, plugin, os.Environ(), project, global, settingDescs(settings))
}

func pluginFileMaps(ctx context.Context, plugin string) (map[string]string, map[string]string) {
	log := logging.FromContext(ctx)
	global, err := config.PluginConfigValues(filepath.Join(config.ResolveConfigDir(), "config.hujson"), plugin)
	if err != nil {
		log.Warn().
			Str("component", "pluginhost").
			Str("operation", "read_plugin_settings").
			Str("plugin", plugin).
			Err(err).
			Msg("reading global plugin settings")
		global = map[string]string{}
	}
	projectDir := config.GetResolvedProjectDir()
	if projectDir == "" {
		projectDir = config.ResolveProjectDir(ctx, "", "")
	}
	if projectDir == "" {
		return map[string]string{}, global
	}
	project, err := config.PluginConfigValues(config.ProjectConfigPath(projectDir), plugin)
	if err != nil {
		log.Warn().
			Str("component", "pluginhost").
			Str("operation", "read_plugin_settings").
			Str("plugin", plugin).
			Err(err).
			Msg("reading project plugin settings")
		project = map[string]string{}
	}
	return project, global
}

func settingDescs(settings []*pbc.PluginSetting) []SettingDesc {
	out := make([]SettingDesc, 0, len(settings))
	for _, setting := range settings {
		if setting == nil || setting.GetKey() == "" || setting.GetEnv() == "" {
			continue
		}
		out = append(out, SettingDesc{
			Key:        setting.GetKey(),
			Env:        setting.GetEnv(),
			Default:    setting.GetDefaultValue(),
			HasDefault: setting.GetDefaultValue() != "",
			Secret:     setting.GetSecret(),
			Required:   setting.GetRequired(),
		})
	}
	return out
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	a := append([]string(nil), left...)
	b := append([]string(nil), right...)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

var (
	envDigestMu     sync.Mutex            //nolint:gochecknoglobals // process-wide projected cache-key input
	envDigests      = map[string]string{} //nolint:gochecknoglobals // process-wide projected cache-key input
	combinedEnvHash string                //nolint:gochecknoglobals // process-wide projected cache-key input
)

func notePluginEnvDigest(plugin, digest string) {
	envDigestMu.Lock()
	defer envDigestMu.Unlock()
	if digest == "" {
		delete(envDigests, plugin)
	} else {
		envDigests[plugin] = digest
	}
	combinedEnvHash = combineEnvDigests(envDigests)
}

func combineEnvDigests(digests map[string]string) string {
	if len(digests) == 0 {
		return ""
	}
	names := make([]string, 0, len(digests))
	for name := range digests {
		names = append(names, name)
	}
	slices.Sort(names)
	sum := sha256.New()
	for _, name := range names {
		_, _ = sum.Write([]byte(name))
		_, _ = sum.Write([]byte{0})
		_, _ = sum.Write([]byte(digests[name]))
		_, _ = sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

// ReplacePluginEnvDigests records the settings of the clients a cost query will
// keep. A settings or validate probe must not call it: that probe's environment
// can differ from the plugins still answering cost queries, and the cache key
// would then store the old price under the new digest.
func ReplacePluginEnvDigests(clients []*Client) {
	envDigestMu.Lock()
	defer envDigestMu.Unlock()
	envDigests = map[string]string{}
	for _, client := range clients {
		if client == nil || client.Name == "" || client.DeliveredDigest == "" {
			continue
		}
		envDigests[client.Name] = client.DeliveredDigest
	}
	combinedEnvHash = combineEnvDigests(envDigests)
}

// PluginEnvDigest returns the combined digest of settings delivered to plugins
// launched in this process. An empty string means no plugin reported settings.
func PluginEnvDigest() string {
	envDigestMu.Lock()
	defer envDigestMu.Unlock()
	return combinedEnvHash
}

// SetPluginEnvDigestForTest records one plugin's digest and rebuilds the combined value.
func SetPluginEnvDigestForTest(plugin, digest string) {
	notePluginEnvDigest(plugin, digest)
}

// ResetPluginEnvDigestsForTest clears the process-wide digest table.
func ResetPluginEnvDigestsForTest() {
	envDigestMu.Lock()
	defer envDigestMu.Unlock()
	envDigests = map[string]string{}
	combinedEnvHash = ""
}
