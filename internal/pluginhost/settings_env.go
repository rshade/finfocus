package pluginhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/logging"
)

// SettingDesc is one advertised plugin setting, or a config key with no descriptor.
type SettingDesc struct {
	Key     string
	Env     string
	Default string
	// HasDefault is false when Default is empty. A proto3 string cannot tell
	// an explicit empty default from a default that was not set.
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

// Delivered is the environment a plugin process receives.
type Delivered struct {
	Pairs     []string
	Overrides map[string]bool
	Digest    string
	Origins   map[string]esc.Ref
}

// WithDeliveredEnv attaches the environment for the plugin process.
// Names in Overrides replace a value inherited from the parent.
func WithDeliveredEnv(ctx context.Context, delivered Delivered) context.Context {
	return context.WithValue(ctx, deliveredEnvContextKey{}, copyDelivered(delivered))
}

// DeliveredEnvFromContext returns the environment attached by WithDeliveredEnv.
func DeliveredEnvFromContext(ctx context.Context) (Delivered, bool) {
	delivered, ok := ctx.Value(deliveredEnvContextKey{}).(Delivered)
	if !ok {
		return Delivered{}, false
	}
	return delivered, true
}

// ResolveDeliveredEnv chooses the environment a plugin receives.
// An allowed ESC value wins over the parent, the project file, the global
// file, and the descriptor default. Every other name keeps that order.
// Pairs lists values to add. Overrides names replace a parent value.
// Digest hashes every effective name. ESC names hash the reference and
// revision key, never the value. An empty effective set returns an empty digest.
// Secret values are never written to the log.
func ResolveDeliveredEnv(
	ctx context.Context,
	plugin string,
	parent []string,
	project, global map[string]string,
	settings []SettingDesc,
	fromESC esc.Delivery,
) Delivered {
	parentSet, parentValues := parentEnvironment(parent)
	effective := map[string]string{}
	described := map[string]bool{}
	allowed := map[string]bool{}

	for _, setting := range settings {
		if setting.Key == "" || setting.Env == "" {
			continue
		}
		described[setting.Key] = true
		allowed[setting.Env] = true
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
	if src, ok := esc.SourceFromContext(ctx); ok {
		for _, name := range src.Variables(plugin) {
			if name != "" {
				allowed[name] = true
			}
		}
	}
	return finishDelivered(ctx, plugin, parentSet, effective, allowed, fromESC)
}

func finishDelivered(
	ctx context.Context,
	plugin string,
	parentSet map[string]bool,
	effective map[string]string,
	allowed map[string]bool,
	fromESC esc.Delivery,
) Delivered {
	origins := map[string]esc.Ref{}
	stamps := map[string]escStamp{}
	overrides := map[string]bool{}
	applyESC(effective, allowed, fromESC, origins, stamps, overrides)

	names := make([]string, 0, len(effective))
	for envName := range effective {
		names = append(names, envName)
	}
	slices.Sort(names)
	pairs := make([]string, 0, len(names))
	for _, envName := range names {
		if parentSet[envName] && !overrides[envName] {
			continue
		}
		pairs = append(pairs, envName+"="+effective[envName])
	}
	logResolved(ctx, plugin, names, stamps)
	if len(names) == 0 {
		return Delivered{Pairs: pairs}
	}
	if len(overrides) == 0 {
		overrides = nil
	}
	if len(origins) == 0 {
		origins = nil
	}
	return Delivered{
		Pairs:     pairs,
		Overrides: overrides,
		Digest:    hashDelivered(plugin, names, effective, stamps),
		Origins:   origins,
	}
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

type escStamp struct {
	ref      string
	revision string
}

func applyESC(
	effective map[string]string,
	allowed map[string]bool,
	fromESC esc.Delivery,
	origins map[string]esc.Ref,
	stamps map[string]escStamp,
	overrides map[string]bool,
) {
	names := make([]string, 0, len(fromESC.Values))
	for name := range fromESC.Values {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		if reservedEnv(name) || !allowed[name] {
			continue
		}
		effective[name] = fromESC.Values[name]
		overrides[name] = true
		ref, ok := fromESC.Origins[name]
		if !ok {
			continue
		}
		origins[name] = ref
		stamps[name] = escStamp{ref: ref.String(), revision: fromESC.Fingerprints[name]}
	}
}

func reservedEnv(name string) bool {
	switch name {
	case pluginsdk.EnvPort, pluginsdk.EnvLogFile, pluginsdk.EnvTraceID:
		return true
	default:
		return false
	}
}

func logResolved(ctx context.Context, plugin string, names []string, stamps map[string]escStamp) {
	escNames := make([]string, 0, len(stamps))
	refSeen := map[string]struct{}{}
	var escRefs []string
	for name, stamp := range stamps {
		escNames = append(escNames, name)
		if _, ok := refSeen[stamp.ref]; ok {
			continue
		}
		refSeen[stamp.ref] = struct{}{}
		escRefs = append(escRefs, stamp.ref)
	}
	slices.Sort(escNames)
	slices.Sort(escRefs)
	logging.FromContext(ctx).Debug().
		Str("component", "pluginhost").
		Str("operation", "resolve_settings").
		Str("plugin", plugin).
		Int("count", len(names)).
		Strs("env", names).
		Strs("esc_env", escNames).
		Strs("esc_refs", escRefs).
		Msg("resolved plugin settings")
}

func hashDelivered(plugin string, names []string, values map[string]string, stamps map[string]escStamp) string {
	sum := sha256.New()
	_, _ = sum.Write([]byte(plugin))
	_, _ = sum.Write([]byte{0})
	for _, name := range names {
		_, _ = sum.Write([]byte(name))
		_, _ = sum.Write([]byte{0})
		if stamp, ok := stamps[name]; ok {
			_, _ = sum.Write([]byte("esc"))
			_, _ = sum.Write([]byte{0})
			_, _ = sum.Write([]byte(stamp.ref))
			_, _ = sum.Write([]byte{0})
			_, _ = sum.Write([]byte(stamp.revision))
			_, _ = sum.Write([]byte{0})
			continue
		}
		_, _ = sum.Write([]byte(values[name]))
		_, _ = sum.Write([]byte{0})
	}
	return hex.EncodeToString(sum.Sum(nil))
}

func copyDelivered(delivered Delivered) Delivered {
	delivered.Pairs = append([]string(nil), delivered.Pairs...)
	if len(delivered.Overrides) > 0 {
		overrides := make(map[string]bool, len(delivered.Overrides))
		for name := range delivered.Overrides {
			overrides[name] = true
		}
		delivered.Overrides = overrides
	}
	if len(delivered.Origins) > 0 {
		origins := make(map[string]esc.Ref, len(delivered.Origins))
		maps.Copy(origins, delivered.Origins)
		delivered.Origins = origins
	}
	return delivered
}

func resolveInjected(
	ctx context.Context,
	plugin string,
	settings []*pbc.PluginSetting,
) Delivered {
	project, global := pluginFileMaps(ctx, plugin)
	var fromESC esc.Delivery
	if src, ok := esc.SourceFromContext(ctx); ok {
		fromESC = src.Delivery(ctx, plugin)
	}
	return ResolveDeliveredEnv(ctx, plugin, os.Environ(), project, global, settingDescs(settings), fromESC)
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

// CombinedDigest hashes the settings delivered to clients. An empty result
// means none of them reported settings. The value does not depend on other
// engines in the process.
func CombinedDigest(clients []*Client) string {
	digests := make(map[string]string, len(clients))
	for _, client := range clients {
		if client == nil || client.Name == "" || client.DeliveredDigest == "" {
			continue
		}
		digests[client.Name] = client.DeliveredDigest
	}
	return combineEnvDigests(digests)
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
