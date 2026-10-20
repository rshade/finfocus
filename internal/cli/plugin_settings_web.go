package cli

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/logging"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/proto"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/webui"
)

const pluginSettingDescriptionLimit = 1024

const (
	settingTypeString   = "string"
	settingTypeInt      = "int"
	settingTypeBool     = "bool"
	settingTypeEnum     = "enum"
	settingTypeURL      = "url"
	settingTypeDuration = "duration"
	textFalse           = "false"
	pluginProbeTimeout  = 20 * time.Second
	pluginInfoTimeout   = 5 * time.Second
	pluginReasonLimit   = 400
)

type pluginSettingState struct {
	view        webui.PluginSettingsView
	descriptors []webui.PluginSettingDescriptor
	project     map[string]string
	global      map[string]string
	origins     map[string]esc.Ref
}

func pluginSettingsView(
	ctx context.Context, name, globalPath, projectDir string,
) (webui.PluginSettingsView, error) {
	state, err := loadPluginSettingState(ctx, name, globalPath, projectDir)
	if err != nil {
		return webui.PluginSettingsView{}, err
	}
	config.NotePluginSecretKeys(secretSettingKeys(name, state.descriptors))
	_ = config.PersistPluginSecretKeys()
	return state.view, nil
}

func preparePluginSettings(
	ctx context.Context, name, globalPath, projectDir string, request config.WriteRequest,
) (config.WriteRequest, error) {
	state, err := loadPluginSettingState(ctx, name, globalPath, projectDir)
	if err != nil {
		return config.WriteRequest{}, err
	}
	config.NotePluginSecretKeys(secretSettingKeys(name, state.descriptors))
	_ = config.PersistPluginSecretKeys()
	return filterPluginChanges(
		name, state.descriptors, state.project, state.global, os.Environ(), state.origins, request,
	)
}

func secretSettingKeys(name string, descriptors []webui.PluginSettingDescriptor) []string {
	var keys []string
	for _, descriptor := range descriptors {
		if descriptor.Secret && descriptor.Key != "" {
			keys = append(keys, "plugins."+name+"."+descriptor.Key)
		}
	}
	return keys
}

func loadPluginSettingState(
	ctx context.Context, name, globalPath, projectDir string,
) (pluginSettingState, error) {
	entries, err := registry.GetAllPluginEntries()
	if err != nil {
		return pluginSettingState{}, err
	}
	installed, err := registry.NewDefault().ListPlugins()
	if err != nil {
		return pluginSettingState{}, err
	}
	inCatalog := false
	for _, entry := range entries {
		if entry.Name == name {
			inCatalog = true
			break
		}
	}
	info, installedOK := newestPlugin(installed, name)
	if !inCatalog && !installedOK {
		return pluginSettingState{}, pluginStatus(http.StatusNotFound, "not_found", "plugin not found")
	}
	globalValues, projectValues := readPluginConfig(ctx, name, globalPath, projectDir)
	versions, projectAvailable, err := pluginConfigVersions(globalPath, projectDir)
	if err != nil {
		return pluginSettingState{}, err
	}
	internal, mode := pluginDescriptors(ctx, name, info, installedOK)
	keys := pluginSettingKeys(internal, projectValues, globalValues)
	delivered := resolvePluginDelivery(ctx, name, internal, projectValues, globalValues)
	view := webui.PluginSettingsView{
		Name:             name,
		Mode:             mode,
		Descriptors:      publicDescriptors(internal),
		Versions:         versions,
		ProjectAvailable: projectAvailable,
		Values: pluginSettingValues(
			name, keys, internal, projectValues, globalValues, os.Environ(), delivered,
		),
	}
	return pluginSettingState{
		view: view, descriptors: internal, project: projectValues, global: globalValues,
		origins: delivered.Origins,
	}, nil
}

func resolvePluginDelivery(
	ctx context.Context,
	name string,
	descriptors []webui.PluginSettingDescriptor,
	project, global map[string]string,
) pluginhost.Delivered {
	var from esc.Delivery
	if src, ok := esc.SourceFromContext(ctx); ok {
		from = src.Delivery(ctx, name)
	}
	return pluginhost.ResolveDeliveredEnv(
		ctx, name, os.Environ(), project, global, webSettingDescs(descriptors), from,
	)
}

func webSettingDescs(descriptors []webui.PluginSettingDescriptor) []pluginhost.SettingDesc {
	out := make([]pluginhost.SettingDesc, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if descriptor.Key == "" || descriptor.Env == "" {
			continue
		}
		out = append(out, pluginhost.SettingDesc{
			Key:        descriptor.Key,
			Env:        descriptor.Env,
			Default:    descriptor.Default,
			HasDefault: descriptor.DefaultSet,
			Secret:     descriptor.Secret,
			Required:   descriptor.Required,
		})
	}
	return out
}

func readPluginConfig(ctx context.Context, name, globalPath, projectDir string) (map[string]string, map[string]string) {
	globalValues, err := config.PluginConfigValues(globalPath, name)
	if err != nil {
		logPluginConfigRead(ctx, name)
		globalValues = map[string]string{}
	}
	if projectDir == "" {
		return globalValues, map[string]string{}
	}
	projectValues, err := config.PluginConfigValues(config.ProjectConfigPath(projectDir), name)
	if err != nil {
		logPluginConfigRead(ctx, name)
		projectValues = map[string]string{}
	}
	return globalValues, projectValues
}

func logPluginConfigRead(ctx context.Context, name string) {
	logging.FromContext(ctx).Warn().
		Str("component", "cli").
		Str("operation", "plugin_settings").
		Str("plugin", name).
		Msg("plugin configuration could not be read")
}

func pluginConfigVersions(globalPath, projectDir string) (map[string]string, bool, error) {
	versions := map[string]string{
		config.TierGlobal:  config.AbsentVersion,
		config.TierProject: config.AbsentVersion,
	}
	globalVersion, err := config.FileVersion(globalPath)
	if err != nil {
		return nil, false, err
	}
	versions[config.TierGlobal] = globalVersion
	if projectDir == "" {
		return versions, false, nil
	}
	projectVersion, err := config.FileVersion(config.ProjectConfigPath(projectDir))
	if err != nil {
		return nil, false, err
	}
	versions[config.TierProject] = projectVersion
	return versions, true, nil
}

func pluginDescriptors(
	ctx context.Context, name string, info registry.PluginInfo, installed bool,
) ([]webui.PluginSettingDescriptor, string) {
	if !installed {
		return nil, "generic"
	}
	descriptors, err := loadPluginDescriptors(ctx, name, info)
	if err != nil || len(descriptors) == 0 {
		if err != nil {
			logging.FromContext(ctx).Warn().
				Str("component", "cli").
				Str("operation", "plugin_settings").
				Str("plugin", name).
				Msg("plugin settings could not be read from the plugin")
		}
		return nil, "generic"
	}
	return descriptors, "descriptor"
}

func loadPluginDescriptors(
	ctx context.Context, name string, info registry.PluginInfo,
) ([]webui.PluginSettingDescriptor, error) {
	launchCtx, cancel := context.WithTimeout(ctx, pluginProbeTimeout)
	defer cancel()
	client, err := pluginhost.NewClient(
		pluginhost.WithInstallName(launchCtx, name), pluginhost.NewProcessLauncher(), info.Path,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	infoCtx, infoCancel := context.WithTimeout(ctx, pluginInfoTimeout)
	defer infoCancel()
	response, err := client.API.GetPluginInfo(infoCtx, &proto.Empty{})
	if err != nil {
		return nil, err
	}
	return descriptorsFromInfo(response.GetSettings()), nil
}

func descriptorsFromInfo(settings []*pbc.PluginSetting) []webui.PluginSettingDescriptor {
	out := make([]webui.PluginSettingDescriptor, 0, len(settings))
	for _, setting := range settings {
		if setting == nil || setting.GetKey() == "" {
			continue
		}
		typeName := settingTypeName(setting.GetType())
		item := webui.PluginSettingDescriptor{
			Key:         setting.GetKey(),
			Type:        typeName,
			Required:    setting.GetRequired(),
			Secret:      settingSecret(setting.GetSecret(), typeName, setting.GetKey(), false),
			Default:     setting.GetDefaultValue(),
			DefaultSet:  setting.GetDefaultValue() != "",
			Description: clipSettingDescription(setting.GetDescription()),
			Env:         setting.GetEnv(),
		}
		if setting.GetType() == pbc.PluginSettingType_PLUGIN_SETTING_TYPE_ENUM {
			item.EnumValues = append([]string{}, setting.GetEnumValues()...)
		}
		out = append(out, item)
	}
	return out
}

func publicDescriptors(descriptors []webui.PluginSettingDescriptor) []webui.PluginSettingDescriptor {
	if len(descriptors) == 0 {
		return []webui.PluginSettingDescriptor{}
	}
	out := make([]webui.PluginSettingDescriptor, len(descriptors))
	for i, descriptor := range descriptors {
		out[i] = descriptor
		if descriptor.Secret {
			out[i].Default = ""
		}
	}
	return out
}

func pluginSettingKeys(
	descriptors []webui.PluginSettingDescriptor, project, global map[string]string,
) []string {
	seen := map[string]struct{}{}
	for _, descriptor := range descriptors {
		seen[descriptor.Key] = struct{}{}
	}
	for key := range project {
		seen[key] = struct{}{}
	}
	for key := range global {
		seen[key] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func pluginSettingValues(
	name string,
	keys []string,
	descriptors []webui.PluginSettingDescriptor,
	project, global map[string]string,
	environ []string,
	delivered pluginhost.Delivered,
) []webui.PluginSettingValue {
	byKey := descriptorMap(descriptors)
	if keys == nil {
		keys = []string{}
	}
	values := make([]webui.PluginSettingValue, 0, len(keys))
	for _, key := range keys {
		descriptor, described := byKey[key]
		secret := settingIsSecret(descriptor, described, key)
		envName := pluginhost.GenericPluginEnv(name, key)
		if described && descriptor.Env != "" {
			envName = descriptor.Env
		}
		envSet := environSet(environ, envName)
		projectSide := valueSide(project, key, secret)
		globalSide := valueSide(global, key, secret)
		origin, fromESC := delivered.Origins[envName]
		row := webui.PluginSettingValue{
			Key:     key,
			Secret:  secret,
			Project: projectSide,
			Global:  globalSide,
			EnvSet:  envSet,
			EffectiveSource: effectiveSource(
				fromESC,
				envSet,
				projectSide.Set,
				globalSide.Set,
				described && descriptor.DefaultSet,
			),
		}
		if fromESC {
			row.ESCEnvironment = origin.String()
		}
		values = append(values, row)
	}
	return values
}

func valueSide(values map[string]string, key string, secret bool) webui.PluginValueSide {
	text, ok := values[key]
	if !ok {
		return webui.PluginValueSide{}
	}
	side := webui.PluginValueSide{Set: true}
	if !secret {
		side.Value = text
	}
	return side
}

func effectiveSource(escSet, envSet, projectSet, globalSet, defaultSet bool) string {
	switch {
	case escSet:
		return "esc"
	case envSet:
		return "env"
	case projectSet:
		return "project"
	case globalSet:
		return "global"
	case defaultSet:
		return "default"
	default:
		return ""
	}
}

func filterPluginChanges(
	name string,
	descriptors []webui.PluginSettingDescriptor,
	project, global map[string]string,
	environ []string,
	origins map[string]esc.Ref,
	request config.WriteRequest,
) (config.WriteRequest, error) {
	byKey := descriptorMap(descriptors)
	filtered := make([]config.Change, 0, len(request.Changes))
	applied := map[string]config.Change{}
	for _, change := range request.Changes {
		key := strings.TrimPrefix(change.Key, "plugins."+name+".")
		descriptor, known := byKey[key]
		if change.Value != nil && *change.Value == config.RedactedValue {
			return config.WriteRequest{}, pluginStatus(
				http.StatusBadRequest, "invalid_request", "masked placeholder is not a value",
			)
		}
		secret := settingIsSecret(descriptor, known, key)
		text := ""
		if change.Value != nil {
			text = *change.Value
		}
		if secret && text == "" && !change.Clear {
			continue
		}
		if !change.Clear {
			if err := validateSettingText(descriptor, known, text); err != nil {
				return config.WriteRequest{}, err
			}
		}
		change.Secret = secret
		filtered = append(filtered, change)
		applied[key] = change
	}
	if err := requirePluginSettings(
		name, byKey, project, global, environ, origins, request.Tier, applied,
	); err != nil {
		return config.WriteRequest{}, err
	}
	request.Changes = filtered
	return request, nil
}

func requirePluginSettings(
	name string,
	byKey map[string]webui.PluginSettingDescriptor,
	project, global map[string]string,
	environ []string,
	origins map[string]esc.Ref,
	tier string,
	applied map[string]config.Change,
) error {
	for key, descriptor := range byKey {
		if !descriptor.Required {
			continue
		}
		envName := descriptor.Env
		if envName == "" {
			envName = pluginhost.GenericPluginEnv(name, key)
		}
		envSet := environSet(environ, envName)
		if envSet {
			continue
		}
		if _, fromESC := origins[envName]; fromESC {
			continue
		}
		effective := descriptor.Default
		if text, ok := tieredValue(key, tier, project, global, applied); ok {
			effective = text
		}
		if effective == "" && descriptor.Default == "" {
			return pluginStatus(http.StatusBadRequest, "invalid_request", "setting "+key+" is required")
		}
	}
	return nil
}

func tieredValue(
	key, tier string, project, global map[string]string, applied map[string]config.Change,
) (string, bool) {
	proj, projSet := project[key]
	glob, globSet := global[key]
	if change, ok := applied[key]; ok {
		if tier == config.TierProject {
			proj, projSet = appliedText(change)
		} else {
			glob, globSet = appliedText(change)
		}
	}
	if projSet {
		return proj, true
	}
	if globSet {
		return glob, true
	}
	return "", false
}

func appliedText(change config.Change) (string, bool) {
	if change.Clear {
		return "", false
	}
	if change.Value == nil {
		return "", false
	}
	return *change.Value, true
}

func validateSettingText(descriptor webui.PluginSettingDescriptor, known bool, text string) error {
	kind := settingTypeString
	if known {
		kind = descriptor.Type
	}
	if text == "" {
		if kind == settingTypeString || kind == "" {
			return nil
		}
		return pluginStatus(
			http.StatusBadRequest,
			"invalid_request",
			"setting "+descriptor.Key+" does not match its type",
		)
	}
	switch kind {
	case settingTypeString, "":
		return nil
	case settingTypeInt:
		if _, err := strconv.ParseInt(text, 10, 64); err != nil {
			return pluginStatus(
				http.StatusBadRequest,
				"invalid_request",
				"setting "+descriptor.Key+" must be an integer",
			)
		}
	case settingTypeBool:
		if text != textTrue && text != textFalse {
			return pluginStatus(
				http.StatusBadRequest,
				"invalid_request",
				"setting "+descriptor.Key+" must be true or false",
			)
		}
	case settingTypeEnum:
		if !slices.Contains(descriptor.EnumValues, text) {
			return pluginStatus(
				http.StatusBadRequest,
				"invalid_request",
				"setting "+descriptor.Key+" is not an allowed value",
			)
		}
	case settingTypeURL:
		parsed, err := url.Parse(text)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return pluginStatus(
				http.StatusBadRequest,
				"invalid_request",
				"setting "+descriptor.Key+" must be an absolute URL",
			)
		}
	case settingTypeDuration:
		if _, err := time.ParseDuration(text); err != nil {
			return pluginStatus(
				http.StatusBadRequest,
				"invalid_request",
				"setting "+descriptor.Key+" must be a duration",
			)
		}
	default:
		return pluginStatus(http.StatusBadRequest, "invalid_request", "setting "+descriptor.Key+" has an unknown type")
	}
	return nil
}

func descriptorMap(descriptors []webui.PluginSettingDescriptor) map[string]webui.PluginSettingDescriptor {
	byKey := make(map[string]webui.PluginSettingDescriptor, len(descriptors))
	for _, descriptor := range descriptors {
		byKey[descriptor.Key] = descriptor
	}
	return byKey
}

func settingIsSecret(descriptor webui.PluginSettingDescriptor, known bool, key string) bool {
	if !known {
		return settingSecret(false, "", key, true)
	}
	return settingSecret(descriptor.Secret, descriptor.Type, key, true)
}

// settingSecret honors an explicit plugin secret flag. Bool and enum settings
// skip the credential-name heuristic; string and URL keys still use it. With
// anySegment, a credential name in any dotted segment of the key counts.
func settingSecret(explicit bool, typeName, key string, anySegment bool) bool {
	if explicit {
		return true
	}
	if !credentialNameApplies(typeName) {
		return false
	}
	if anySegment {
		return config.IsCredentialName(key) || config.IsSecretSetting(key)
	}
	return config.IsCredentialName(key)
}

func credentialNameApplies(typeName string) bool {
	return typeName != settingTypeBool && typeName != settingTypeEnum
}

func environSet(environ []string, key string) bool {
	if key == "" {
		return false
	}
	prefix := key + "="
	for _, pair := range environ {
		if strings.HasPrefix(pair, prefix) {
			return true
		}
	}
	return false
}

func settingTypeName(kind pbc.PluginSettingType) string {
	switch kind {
	case pbc.PluginSettingType_PLUGIN_SETTING_TYPE_INT:
		return settingTypeInt
	case pbc.PluginSettingType_PLUGIN_SETTING_TYPE_BOOL:
		return settingTypeBool
	case pbc.PluginSettingType_PLUGIN_SETTING_TYPE_ENUM:
		return settingTypeEnum
	case pbc.PluginSettingType_PLUGIN_SETTING_TYPE_URL:
		return settingTypeURL
	case pbc.PluginSettingType_PLUGIN_SETTING_TYPE_DURATION:
		return settingTypeDuration
	case pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING, pbc.PluginSettingType_PLUGIN_SETTING_TYPE_UNSPECIFIED:
		return settingTypeString
	default:
		return settingTypeString
	}
}

func clipSettingDescription(text string) string {
	if len(text) <= pluginSettingDescriptionLimit {
		return text
	}
	return text[:pluginSettingDescriptionLimit]
}
