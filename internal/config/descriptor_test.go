package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKnownSettingsMatchSchema(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../docs/src/content/docs/schemas/config-schema.json")
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(data, &schema))

	for _, setting := range KnownSettings() {
		node := schemaNode(t, schema, setting.Key)
		switch setting.Kind {
		case KindEnum:
			assert.Equal(t, "string", node["type"], setting.Key)
			assert.Equal(t, stringList(node["enum"]), setting.Enum, setting.Key)
		case KindInt:
			assert.Equal(t, "integer", node["type"], setting.Key)
			assertBounds(t, setting, node)
		case KindNumber:
			assert.Equal(t, "number", node["type"], setting.Key)
			assertBounds(t, setting, node)
		case KindBool:
			assert.Equal(t, "boolean", node["type"], setting.Key)
		case KindString, KindPlugin, KindClusterContext:
			assert.Equal(t, "string", node["type"], setting.Key)
			if setting.Pattern != "" {
				assert.Equal(t, setting.Pattern, node["pattern"], setting.Key)
			}
		case KindStringList:
			assert.Equal(t, "array", node["type"], setting.Key)
		case kindPluginNumber:
			require.Failf(t, "write-time kind in the settings list", "%s", setting.Key)
		default:
			require.Failf(t, "unexpected kind", "%s for %s", setting.Kind, setting.Key)
		}
	}

	flat := schemaNode(t, schema, "cost.budgets.amount")
	description, _ := flat["description"].(string)
	assert.Contains(t, description, "Not applied")
	assert.Equal(t, "cost.budgets.global.amount", CanonicalKey("cost.budgets.amount"))
	_, known := LookupSetting("cost.budgets.amount")
	assert.True(t, known)
	for _, setting := range KnownSettings() {
		assert.NotEqual(t, "cost.budgets.amount", setting.Key)
	}
}

func TestCanonicalBudgetAliasAndPluginSecret(t *testing.T) {
	t.Parallel()
	setting, ok := LookupSetting("plugins.aws.secret_key")
	require.True(t, ok)
	assert.True(t, setting.Secret)
	setting, ok = LookupSetting("plugins.aws.region")
	require.True(t, ok)
	assert.False(t, setting.Secret)
	pointer, err := pointerFor("overview.cluster_contexts.prod/west")
	require.NoError(t, err)
	assert.Equal(t, "/overview/cluster_contexts/prod~1west", pointer)
	_, err = encodeSetting(mustSetting(t, "logging.file"), "relative.log")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "absolute")
}

func schemaNode(t *testing.T, schema map[string]any, key string) map[string]any {
	t.Helper()
	node := schema
	for _, part := range splitSchemaKey(key) {
		properties, _ := node["properties"].(map[string]any)
		next, ok := properties[part].(map[string]any)
		require.Truef(t, ok, "schema missing %s at %s", part, key)
		node = resolveRef(t, schema, next)
	}
	return node
}

func splitSchemaKey(key string) []string {
	switch key {
	case "cost.budgets.global.amount":
		return []string{"cost", "budgets", "global", "amount"}
	case "cost.budgets.global.currency":
		return []string{"cost", "budgets", "global", "currency"}
	case "cost.budgets.global.period":
		return []string{"cost", "budgets", "global", "period"}
	case "cost.budgets.amount":
		return []string{"cost", "budgets", "amount"}
	default:
		return splitDot(key)
	}
}

func splitDot(key string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(key); i++ {
		if i == len(key) || key[i] == '.' {
			parts = append(parts, key[start:i])
			start = i + 1
		}
	}
	return parts
}

func resolveRef(t *testing.T, root, node map[string]any) map[string]any {
	t.Helper()
	ref, _ := node["$ref"].(string)
	if ref == "" {
		return node
	}
	current := any(root)
	for _, part := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
		object, ok := current.(map[string]any)
		require.Truef(t, ok, "schema ref %s", ref)
		current = object[part]
	}
	resolved, ok := current.(map[string]any)
	require.Truef(t, ok, "schema ref %s", ref)
	return resolved
}

func stringList(value any) []string {
	raw, _ := value.([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text, _ := item.(string)
		out = append(out, text)
	}
	return out
}

func assertBounds(t *testing.T, setting Setting, node map[string]any) {
	t.Helper()
	if setting.Min != nil {
		assert.InDelta(t, *setting.Min, numberField(node, "minimum"), 0.0001, setting.Key)
	}
	if setting.Max != nil {
		assert.InDelta(t, *setting.Max, numberField(node, "maximum"), 0.0001, setting.Key)
	}
}

func numberField(node map[string]any, name string) float64 {
	switch typed := node[name].(type) {
	case float64:
		return typed
	default:
		return -1
	}
}

func mustSetting(t *testing.T, key string) Setting {
	t.Helper()
	setting, ok := LookupSetting(key)
	require.True(t, ok)
	return setting
}
