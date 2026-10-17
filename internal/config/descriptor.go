package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// SettingKind is the control a known configuration setting uses.
type SettingKind string

const (
	KindEnum           SettingKind = "enum"
	KindInt            SettingKind = "int"
	KindNumber         SettingKind = "number"
	KindBool           SettingKind = "bool"
	KindString         SettingKind = "string"
	KindStringList     SettingKind = "string_list"
	KindPlugin         SettingKind = "plugin"
	KindClusterContext SettingKind = "cluster_context"
)

// Setting describes one known configuration key.
type Setting struct {
	Key      string      `json:"key"`
	Kind     SettingKind `json:"kind"`
	Help     string      `json:"help"`
	Secret   bool        `json:"secret"`
	ReadOnly bool        `json:"readOnly"`
	Enum     []string    `json:"enum,omitempty"`
	Min      *float64    `json:"min,omitempty"`
	Max      *float64    `json:"max,omitempty"`
	Pattern  string      `json:"pattern,omitempty"`
}

const (
	keyClusterContexts    = "overview.cluster_contexts"
	keyPluginsPrefix      = "plugins."
	keyLoggingFile        = "logging.file"
	keyBudgetGlobalAmount = "cost.budgets.global.amount"
	keyBudgetGlobalPeriod = "cost.budgets.global.period"
	keyScoringTimeout     = "scoring.timeout_seconds"

	isoCurrencyLength = 3
)

// KnownSettings returns the fixed settings the page and config set can edit.
// Plugin settings and cluster contexts are discovered from the files.
func KnownSettings() []Setting {
	zero := 0.0
	ten := 10.0
	one := 1.0
	return []Setting{
		{
			Key:  "output.default_format",
			Kind: KindEnum,
			Help: "Default command output",
			Enum: []string{"table", formatJSON, "ndjson"},
		},
		{Key: "output.precision", Kind: KindInt, Help: "Decimal places for cost amounts", Min: &zero, Max: &ten},
		{Key: "logging.level", Kind: KindEnum, Help: "Log level", Enum: []string{"debug", "info", "warn", "error"}},
		{Key: keyLoggingFile, Kind: KindString, Help: "Absolute path of the log file"},
		{
			Key:  "plugin_host.strict_compatibility",
			Kind: KindBool,
			Help: "Refuse plugins whose spec version does not match",
		},
		{
			Key:  keyBudgetGlobalAmount,
			Kind: KindNumber,
			Help: "Global budget amount. Zero disables the budget.",
			Min:  &zero,
		},
		{
			Key:     "cost.budgets.global.currency",
			Kind:    KindString,
			Help:    "Global budget currency, three uppercase letters",
			Pattern: `^[A-Z]{3}$`,
		},
		{Key: keyBudgetGlobalPeriod, Kind: KindEnum, Help: "Global budget period", Enum: []string{"monthly"}},
		{Key: "scoring.enabled", Kind: KindBool, Help: "Score recommendations. Requires scoring.plugin."},
		{Key: "scoring.plugin", Kind: KindString, Help: "Plugin that scores recommendations"},
		{
			Key:  "scoring.identifier_mode",
			Kind: KindEnum,
			Help: "How resource identifiers are sent to the scorer",
			Enum: []string{"pseudonymized", "raw", "omitted"},
		},
		{Key: keyScoringTimeout, Kind: KindInt, Help: "Scorer timeout in seconds", Min: &zero},
		{Key: "scoring.field_allowlist", Kind: KindStringList, Help: "Recommendation fields the scorer may receive"},
		{
			Key:  "scoring.needs_review.risk",
			Kind: KindNumber,
			Help: "Risk threshold for needs-review",
			Min:  &zero,
			Max:  &one,
		},
		{
			Key:  "scoring.needs_review.false_positive",
			Kind: KindNumber,
			Help: "False-positive threshold for needs-review",
			Min:  &zero,
			Max:  &one,
		},
		{
			Key:  "scoring.needs_review.insufficient_evidence",
			Kind: KindNumber,
			Help: "Evidence threshold for needs-review",
			Min:  &zero,
			Max:  &one,
		},
		{
			Key:  "scoring.needs_review.dead_band",
			Kind: KindNumber,
			Help: "Dead band for needs-review",
			Min:  &zero,
			Max:  &one,
		},
	}
}

// CanonicalKey maps a config set alias onto the field that is stored.
// The flat budget keys are accepted by the command and ignored if written
// literally, so they are rewritten to the global scope.
func CanonicalKey(key string) string {
	switch key {
	case "cost.budgets.amount":
		return keyBudgetGlobalAmount
	case "cost.budgets.currency":
		return "cost.budgets.global.currency"
	case "cost.budgets.period":
		return keyBudgetGlobalPeriod
	default:
		return key
	}
}

// LookupSetting reports the descriptor for key, including dynamic plugin
// settings and cluster contexts.
func LookupSetting(key string) (Setting, bool) {
	key = CanonicalKey(key)
	for _, setting := range KnownSettings() {
		if setting.Key == key {
			return setting, true
		}
	}
	if cluster, found := strings.CutPrefix(key, keyClusterContexts+"."); found {
		if cluster == "" {
			return Setting{}, false
		}
		return Setting{
			Key:  key,
			Kind: KindClusterContext,
			Help: "Kubeconfig context for cluster " + cluster,
		}, true
	}
	if setting, ok := pluginSetting(key); ok {
		return setting, true
	}
	return Setting{}, false
}

func pluginSetting(key string) (Setting, bool) {
	name, configKey, ok := splitPluginKey(key)
	if !ok {
		return Setting{}, false
	}
	return Setting{
		Key:    key,
		Kind:   KindPlugin,
		Help:   "Setting " + configKey + " for plugin " + name,
		Secret: IsCredentialName(configKey) || IsCredentialName(name),
	}, true
}

func splitPluginKey(key string) (string, string, bool) {
	if !strings.HasPrefix(key, keyPluginsPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(key, keyPluginsPrefix)
	name, configKey, ok := strings.Cut(rest, ".")
	if !ok || name == "" || configKey == "" {
		return "", "", false
	}
	return name, configKey, true
}

// pointerFor returns the RFC 6901 pointer for a setting key.
func pointerFor(key string) (string, error) {
	setting, ok := LookupSetting(key)
	if !ok {
		return "", invalidChange("unknown configuration section: %s", strings.Split(key, ".")[0])
	}
	var segments []string
	switch {
	case strings.HasPrefix(setting.Key, keyClusterContexts+"."):
		cluster := strings.TrimPrefix(setting.Key, keyClusterContexts+".")
		segments = []string{"overview", "cluster_contexts", cluster}
	case setting.Kind == KindPlugin:
		name, configKey, _ := splitPluginKey(setting.Key)
		segments = []string{"plugins", name, configKey}
	default:
		segments = strings.Split(setting.Key, ".")
	}
	var pointer strings.Builder
	for _, segment := range segments {
		pointer.WriteByte('/')
		pointer.WriteString(strings.NewReplacer("~", "~0", "/", "~1").Replace(segment))
	}
	return pointer.String(), nil
}

// encodeSetting converts a text value into the JSON literal stored for setting.
func encodeSetting(setting Setting, raw string) (any, error) {
	switch setting.Kind {
	case KindEnum:
		return encodeEnum(setting, raw)
	case KindInt:
		return encodeBoundedInt(setting, raw)
	case KindNumber:
		return encodeBoundedNumber(setting, raw)
	case KindBool:
		return encodeBool(setting, raw)
	case KindStringList:
		return encodeStringList(raw), nil
	case KindString, KindPlugin, KindClusterContext:
		return encodeText(setting, raw)
	default:
		return nil, fmt.Errorf("unknown configuration setting: %s", setting.Key)
	}
}

func encodeEnum(setting Setting, raw string) (any, error) {
	if slices.Contains(setting.Enum, raw) {
		return raw, nil
	}
	return nil, fmt.Errorf("invalid %s %q (must be one of: %s)", setting.Key, raw, strings.Join(setting.Enum, ", "))
}

func encodeBoundedInt(setting Setting, raw string) (any, error) {
	number, err := strconv.Atoi(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be a number: %w", lastSegment(setting.Key), err)
	}
	if setting.Min != nil && float64(number) < *setting.Min || setting.Max != nil && float64(number) > *setting.Max {
		return nil, fmt.Errorf("invalid %s: %d", setting.Key, number)
	}
	return number, nil
}

func encodeBoundedNumber(setting Setting, raw string) (any, error) {
	number, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("%s must be a number: %w", lastSegment(setting.Key), err)
	}
	if setting.Min != nil && number < *setting.Min || setting.Max != nil && number > *setting.Max {
		return nil, fmt.Errorf("invalid %s: %v", setting.Key, number)
	}
	return number, nil
}

func encodeBool(setting Setting, raw string) (any, error) {
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be a boolean: %w", lastSegment(setting.Key), err)
	}
	return value, nil
}

func encodeStringList(raw string) []string {
	var fields []string
	for field := range strings.SplitSeq(raw, ",") {
		if field = strings.TrimSpace(field); field != "" {
			fields = append(fields, field)
		}
	}
	if fields == nil {
		return []string{}
	}
	return fields
}

func encodeText(setting Setting, raw string) (any, error) {
	if setting.Key == keyLoggingFile && raw != "" && !filepath.IsAbs(raw) {
		return nil, errors.New("logging.file must be an absolute path")
	}
	if setting.Pattern != "" && raw != "" && !matchesPattern(setting.Pattern, raw) {
		return nil, fmt.Errorf("invalid %s %q", setting.Key, raw)
	}
	return raw, nil
}

func lastSegment(key string) string {
	_, last, ok := strings.CutLast(key, ".")
	if ok {
		return last
	}
	return key
}

func matchesPattern(pattern, value string) bool {
	if pattern == `^[A-Z]{3}$` {
		if len(value) != isoCurrencyLength {
			return false
		}
		for _, r := range value {
			if r < 'A' || r > 'Z' {
				return false
			}
		}
		return true
	}
	return true
}
