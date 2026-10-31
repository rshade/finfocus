package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/tailscale/hujson"
	"gopkg.in/yaml.v3"

	"github.com/rshade/finfocus-spec/sdk/go/pluginsdk"
)

const (
	// SourceDefault is a built-in value, used when no file or environment sets the key.
	SourceDefault = "default"
	// SourceGlobal is the user-global configuration file.
	SourceGlobal = "global"
	// SourceProject is the project configuration file.
	SourceProject = "project"
	// SourceEnv is an environment variable.
	SourceEnv = "env"

	projectUnavailableReason = "no project directory is available"
)

// TierValue is one tier's stored value. Secret values omit Value.
type TierValue struct {
	Set   bool `json:"set"`
	Value any  `json:"-"`
	show  bool
}

// MarshalJSON omits Value when the tier has no displayable value.
func (t TierValue) MarshalJSON() ([]byte, error) {
	if !t.show {
		return json.Marshal(struct {
			Set bool `json:"set"`
		}{Set: t.Set})
	}
	return json.Marshal(struct {
		Set   bool `json:"set"`
		Value any  `json:"value"`
	}{Set: t.Set, Value: t.Value})
}

// EffectiveValue is the value the process would use, and which tier supplied it.
type EffectiveValue struct {
	Source string `json:"source"`
	Value  any    `json:"-"`
	show   bool
}

// MarshalJSON omits Value for secrets and unset values.
func (e EffectiveValue) MarshalJSON() ([]byte, error) {
	if !e.show {
		return json.Marshal(struct {
			Source string `json:"source"`
		}{Source: e.Source})
	}
	return json.Marshal(struct {
		Source string `json:"source"`
		Value  any    `json:"value"`
	}{Source: e.Source, Value: e.Value})
}

// SettingView is one known setting with its effective value and each tier.
type SettingView struct {
	Setting

	Effective EffectiveValue       `json:"effective"`
	Tiers     map[string]TierValue `json:"tiers"`
}

// ReadOnlyRow is an alert, routing, or analyzer value the page shows but does not edit.
type ReadOnlyRow struct {
	Key    string `json:"key"`
	Secret bool   `json:"secret"`
	Set    bool   `json:"set"`
	Source string `json:"source"`
	Value  any    `json:"-"`
	show   bool
}

// MarshalJSON omits Value for secrets.
func (r ReadOnlyRow) MarshalJSON() ([]byte, error) {
	if !r.show {
		return json.Marshal(struct {
			Key    string `json:"key"`
			Secret bool   `json:"secret"`
			Set    bool   `json:"set"`
			Source string `json:"source"`
		}{Key: r.Key, Secret: r.Secret, Set: r.Set, Source: r.Source})
	}
	return json.Marshal(struct {
		Key    string `json:"key"`
		Secret bool   `json:"secret"`
		Set    bool   `json:"set"`
		Source string `json:"source"`
		Value  any    `json:"value"`
	}{Key: r.Key, Secret: r.Secret, Set: r.Set, Source: r.Source, Value: r.Value})
}

// Page is the configuration document served to the dashboard.
type Page struct {
	ProjectAvailable         bool              `json:"projectAvailable"`
	ProjectUnavailableReason string            `json:"projectUnavailableReason"`
	Versions                 map[string]string `json:"versions"`
	Settings                 []SettingView     `json:"settings"`
	ReadOnly                 []ReadOnlyRow     `json:"readOnly"`
}

// BuildPage loads the global and project files and reports each known setting.
// projectDir is the project's .finfocus directory, or empty when none resolves.
// Environment values come from the process environment.
func BuildPage(globalPath, projectDir string) (Page, error) {
	return buildPage(globalPath, projectDir, os.Environ())
}

func buildPage(globalPath, projectDir string, environ []string) (Page, error) {
	globalDoc, globalVersion, err := loadDocument(globalPath)
	if err != nil {
		return Page{}, err
	}
	page := Page{
		Versions: map[string]string{TierGlobal: globalVersion, TierProject: AbsentVersion},
		Settings: []SettingView{},
		ReadOnly: []ReadOnlyRow{},
	}
	var projectDoc map[string]any
	if projectDir == "" {
		page.ProjectUnavailableReason = projectUnavailableReason
	} else {
		page.ProjectAvailable = true
		projectDoc, page.Versions[TierProject], err = loadDocument(ProjectConfigPath(projectDir))
		if err != nil {
			return Page{}, err
		}
	}
	envValues := collectEnv(environ)
	settings := pageSettings(globalDoc, projectDoc, envValues)
	for _, setting := range settings {
		page.Settings = append(page.Settings, settingView(setting, globalDoc, projectDoc, envValues))
	}
	page.ReadOnly = readOnlyRows(globalDoc, projectDoc)
	return page, nil
}

func pageSettings(globalDoc, projectDoc map[string]any, envValues map[string]any) []Setting {
	settings := KnownSettings()
	var dynamic []string
	dynamic = append(dynamic, clusterKeys(globalDoc)...)
	dynamic = append(dynamic, clusterKeys(projectDoc)...)
	dynamic = append(dynamic, pluginKeys(globalDoc)...)
	dynamic = append(dynamic, pluginKeys(projectDoc)...)
	for key := range envValues {
		if strings.HasPrefix(key, keyPluginsPrefix) || strings.HasPrefix(key, keyClusterContexts+".") {
			dynamic = append(dynamic, key)
		}
	}
	slices.Sort(dynamic)
	dynamic = slices.Compact(dynamic)
	for _, key := range dynamic {
		if setting, ok := LookupSetting(key); ok {
			settings = append(settings, setting)
		}
	}
	return settings
}

func settingView(setting Setting, globalDoc, projectDoc, envValues map[string]any) SettingView {
	secret := setting.Secret || IsSecretSetting(setting.Key)
	tiers := map[string]TierValue{
		SourceDefault: tierFrom(defaultValue(setting.Key), secret),
		SourceGlobal:  tierFrom(storedValue(globalDoc, setting.Key), secret),
		SourceProject: tierFrom(storedValue(projectDoc, setting.Key), secret),
		SourceEnv:     tierFrom(envValue(envValues, setting.Key), secret),
	}
	source, value, set := effective(setting.Key, projectDoc, tiers)
	effectiveValue := EffectiveValue{Source: source}
	if set && !secret {
		effectiveValue.Value = resolvedValue(setting.Key, value)
		effectiveValue.show = true
	}
	return SettingView{Setting: setting, Effective: effectiveValue, Tiers: tiers}
}

// resolvedValue applies the sentinels that the runtime resolvers treat as
// "use the default", so the page shows what consumers actually use. The stored
// value stays visible in its tier.
func resolvedValue(key string, value any) any {
	if key == keyScoringTimeout {
		if seconds, ok := value.(int); ok && seconds <= 0 {
			return DefaultScoringTimeoutSeconds
		}
	}
	return value
}

// SectionOf returns the top-level config section that holds key.
func SectionOf(key string) string {
	return sectionName(CanonicalKey(key))
}

// ProjectShadowsKey reports whether the project config in projectDir defines
// the top-level section of key. A project section replaces the whole global
// section (see ShallowMergeYAML), so a value written to the global file at
// globalPath for a key in that section does not apply in the project. It
// returns the project config file that decides this.
//
// Nothing is shadowed when the project file is missing, unreadable, or rejected
// as a whole (NewWithProjectDir then uses the global config), or when it is the
// same file as the global one (FINFOCUS_HOME inside the project).
func ProjectShadowsKey(globalPath, projectDir, key string) (string, bool) {
	if projectDir == "" {
		return "", false
	}
	path := ProjectConfigPath(projectDir)
	if sameFile(globalPath, path) {
		return "", false
	}
	doc, _, err := loadDocument(path)
	if err != nil {
		return "", false
	}
	if _, defined := doc[sectionName(CanonicalKey(key))]; !defined {
		return "", false
	}
	if err = ShallowMergeYAML(&Config{}, path); err != nil {
		return "", false
	}
	return path, true
}

func sameFile(a, b string) bool {
	infoA, errA := os.Stat(a)
	infoB, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(infoA, infoB)
}

func effective(key string, projectDoc map[string]any, tiers map[string]TierValue) (string, any, bool) {
	if tiers[SourceEnv].Set {
		return SourceEnv, tiers[SourceEnv].Value, true
	}
	if projectDoc != nil {
		if _, shadowed := projectDoc[sectionName(key)]; shadowed {
			if tiers[SourceProject].Set {
				return SourceProject, tiers[SourceProject].Value, true
			}
			return SourceDefault, tiers[SourceDefault].Value, tiers[SourceDefault].Set
		}
	}
	if tiers[SourceGlobal].Set {
		return SourceGlobal, tiers[SourceGlobal].Value, true
	}
	return SourceDefault, tiers[SourceDefault].Value, tiers[SourceDefault].Set
}

func tierFrom(value any, secret bool) TierValue {
	if !presentValue(value, secret) {
		return TierValue{}
	}
	shown := TierValue{Set: true, Value: normalizeJSON(value)}
	if !secret {
		shown.show = true
	}
	return shown
}

func presentValue(value any, secret bool) bool {
	if value == nil {
		return false
	}
	if !secret {
		return true
	}
	text, ok := value.(string)
	return !ok || text != ""
}

func sectionName(key string) string {
	switch {
	case strings.HasPrefix(key, keyPluginsPrefix):
		return keyPlugins
	case strings.HasPrefix(key, keyOverview+"."):
		return keyOverview
	default:
		name, _, _ := strings.Cut(key, ".")
		return name
	}
}

func storedValue(doc map[string]any, key string) any {
	if doc == nil {
		return nil
	}
	if name, configKey, ok := splitPluginKey(key); ok {
		return pluginValue(doc, name, configKey)
	}
	if cluster, found := strings.CutPrefix(key, keyClusterContexts+"."); found {
		return clusterValue(doc, cluster)
	}
	value := walkPath(doc, strings.Split(key, "."))
	if key == keyPulumiCloudEnabled {
		if enabled, ok := value.(bool); ok {
			return strconv.FormatBool(enabled)
		}
	}
	return value
}

func pluginValue(doc map[string]any, name, configKey string) any {
	plugins, _ := doc[keyPlugins].(map[string]any)
	plugin, _ := plugins[name].(map[string]any)
	if plugin == nil {
		return nil
	}
	value, ok := plugin[configKey]
	if !ok {
		return nil
	}
	return redactNestedSecrets(value)
}

// redactNestedSecrets removes credential-named leaves from a plugin value.
// The page must not receive those values or the display placeholder.
func redactNestedSecrets(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, child := range typed {
			if IsCredentialName(key) {
				continue
			}
			redacted[key] = redactNestedSecrets(child)
		}
		return redacted
	case []any:
		redacted := make([]any, len(typed))
		for i, child := range typed {
			redacted[i] = redactNestedSecrets(child)
		}
		return redacted
	default:
		return value
	}
}

func clusterValue(doc map[string]any, cluster string) any {
	overview, _ := doc[keyOverview].(map[string]any)
	contexts, _ := overview[overviewKeyClusterContexts].(map[string]any)
	if contexts == nil {
		return nil
	}
	value, ok := contexts[cluster]
	if !ok {
		return nil
	}
	return value
}

func walkPath(doc map[string]any, parts []string) any {
	var current any = doc
	for _, part := range parts {
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		next, exists := object[part]
		if !exists {
			return nil
		}
		current = next
	}
	return current
}

func envValue(env map[string]any, key string) any {
	if env == nil {
		return nil
	}
	value, ok := env[key]
	if !ok {
		return nil
	}
	return value
}

func defaultValue(key string) any {
	switch key {
	case "output.default_format":
		return formatTable
	case "output.precision":
		return defaultPrecision
	case "logging.level":
		return levelInfo
	case keyLoggingFile:
		return filepath.Join(ResolveConfigDir(), "logs", "finfocus.log")
	case "plugin_host.strict_compatibility":
		return false
	case keyBudgetGlobalAmount:
		return float64(0)
	case keyBudgetGlobalPeriod:
		return DefaultBudgetPeriod
	case keyPulumiCloudEnabled:
		return cloudModeFalse
	case "scoring.enabled":
		return false
	case "scoring.identifier_mode":
		return ScoringIdentifierPseudonymized
	case keyScoringTimeout:
		return DefaultScoringTimeoutSeconds
	case "scoring.field_allowlist":
		return []any{}
	case "scoring.needs_review.risk":
		return DefaultScoringReviewRisk
	case "scoring.needs_review.false_positive":
		return DefaultScoringReviewFalsePositive
	case "scoring.needs_review.insufficient_evidence":
		return DefaultScoringReviewEvidence
	case "scoring.needs_review.dead_band":
		return DefaultScoringReviewDeadBand
	default:
		return nil
	}
}

func normalizeJSON(value any) any {
	number, ok := value.(float64)
	if !ok || math.IsInf(number, 0) || math.IsNaN(number) || number != math.Trunc(number) {
		return value
	}
	return int(number)
}

func pluginKeys(doc map[string]any) []string {
	plugins, _ := doc[keyPlugins].(map[string]any)
	var keys []string
	for name, raw := range plugins {
		plugin, _ := raw.(map[string]any)
		for configKey := range plugin {
			keys = append(keys, keyPluginsPrefix+name+"."+configKey)
		}
	}
	return keys
}

func clusterKeys(doc map[string]any) []string {
	overview, _ := doc[keyOverview].(map[string]any)
	contexts, _ := overview[overviewKeyClusterContexts].(map[string]any)
	var keys []string
	for cluster := range contexts {
		keys = append(keys, keyClusterContexts+"."+cluster)
	}
	return keys
}

func loadDocument(path string) (map[string]any, string, error) {
	if path == "" {
		return map[string]any{}, AbsentVersion, nil
	}
	preferred := PreferredConfigPath(path)
	version, err := FileVersion(preferred)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(preferred)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return map[string]any{}, version, nil
		}
		return nil, "", fmt.Errorf("reading configuration: %w", err)
	}
	doc, err := decodeDocument(data)
	if err != nil {
		return nil, "", err
	}
	return doc, version, nil
}

func decodeDocument(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return map[string]any{}, nil
	}
	if isYAMLDocument(data) {
		var doc map[string]any
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("parsing configuration: %w", err)
		}
		if doc == nil {
			doc = map[string]any{}
		}
		return doc, nil
	}
	value, err := hujson.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parsing configuration: %w", err)
	}
	value.Standardize()
	var doc map[string]any
	if err = json.Unmarshal(value.Pack(), &doc); err != nil {
		return nil, fmt.Errorf("parsing configuration: %w", err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

func collectEnv(environ []string) map[string]any {
	env := environMap(environ)
	out := map[string]any{}
	compat := env["FINFOCUS_COMPAT"] == "1" || env["FINFOCUS_COMPAT"] == envBoolTrue
	if compat {
		putEnvString(out, "output.default_format", env["PULUMICOST_OUTPUT_FORMAT"])
		putEnvInt(out, "output.precision", env["PULUMICOST_OUTPUT_PRECISION"])
		putEnvString(out, "logging.level", env["PULUMICOST_LOG_LEVEL"])
		putEnvString(out, keyLoggingFile, env["PULUMICOST_LOG_FILE"])
		scanPluginEnv(environ, "PULUMICOST_PLUGIN_", out)
	}
	putEnvString(out, "output.default_format", env["FINFOCUS_OUTPUT_FORMAT"])
	putEnvInt(out, "output.precision", env["FINFOCUS_OUTPUT_PRECISION"])
	putEnvString(out, "logging.level", env[pluginsdk.EnvLogLevel])
	putEnvString(out, keyLoggingFile, env[pluginsdk.EnvLogFile])
	if raw := env["FINFOCUS_STRICT_COMPATIBILITY"]; raw != "" {
		if value, err := strconv.ParseBool(raw); err == nil {
			out["plugin_host.strict_compatibility"] = value
		}
	}
	scanPluginEnv(environ, "FINFOCUS_PLUGIN_", out)
	return out
}

func environMap(environ []string) map[string]string {
	out := make(map[string]string, len(environ))
	for _, entry := range environ {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			out[key] = value
		}
	}
	return out
}

func putEnvString(out map[string]any, key, value string) {
	if value != "" {
		out[key] = value
	}
}

func putEnvInt(out map[string]any, key, value string) {
	if value == "" {
		return
	}
	number, err := strconv.Atoi(value)
	if err == nil {
		out[key] = number
	}
}

func scanPluginEnv(environ []string, prefix string, out map[string]any) {
	for _, entry := range environ {
		if !strings.HasPrefix(entry, prefix) {
			continue
		}
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(key, prefix), "_")
		if len(parts) < minPluginKeyParts {
			continue
		}
		name := strings.ToLower(parts[0])
		configKey := strings.ToLower(strings.Join(parts[1:], "_"))
		out[keyPluginsPrefix+name+"."+configKey] = value
	}
}

func readOnlyRows(globalDoc, projectDoc map[string]any) []ReadOnlyRow {
	rows := []ReadOnlyRow{}
	for _, section := range []string{keyRouting, keyAnalyzer, keyCost} {
		doc, source, ok := winningSection(section, globalDoc, projectDoc)
		if !ok {
			continue
		}
		walkLeaves(section, doc[section], func(path string, value any) {
			if section == keyCost && !strings.Contains(path, ".alerts") {
				return
			}
			rows = append(rows, readOnlyRow(path, source, value))
		})
	}
	return rows
}

func winningSection(section string, globalDoc, projectDoc map[string]any) (map[string]any, string, bool) {
	if _, ok := projectDoc[section]; ok {
		return projectDoc, SourceProject, true
	}
	if _, ok := globalDoc[section]; ok {
		return globalDoc, SourceGlobal, true
	}
	return nil, "", false
}

func readOnlyRow(path, source string, value any) ReadOnlyRow {
	secret := readOnlySecret(path)
	row := ReadOnlyRow{Key: path, Secret: secret, Set: presentValue(value, secret), Source: source}
	if row.Set && !secret {
		row.Value = normalizeJSON(value)
		row.show = true
	}
	return row
}

func readOnlySecret(path string) bool {
	if strings.HasSuffix(path, ".url") || strings.Contains(path, ".headers.") {
		return true
	}
	cleaned := strings.NewReplacer("[", ".", "]", "").Replace(path)
	return IsSecretSetting(cleaned)
}

func walkLeaves(prefix string, value any, visit func(string, any)) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			next := prefix + "." + key
			walkLeaves(next, typed[key], visit)
		}
	case []any:
		for i, item := range typed {
			walkLeaves(fmt.Sprintf("%s[%d]", prefix, i), item, visit)
		}
	default:
		if value != nil {
			visit(prefix, value)
		}
	}
}
