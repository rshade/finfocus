package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pbc "github.com/rshade/finfocus-spec/sdk/go/proto/finfocus/v1"
	"github.com/rshade/finfocus/internal/pluginhost"
	"github.com/rshade/finfocus/internal/webui"
)

func TestPluginSettingSecretMatrix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		kind       pbc.PluginSettingType
		secretFlag bool
		key        string
		wantSecret bool
	}{
		{
			name:       "bool credential-like name is not secret",
			kind:       pbc.PluginSettingType_PLUGIN_SETTING_TYPE_BOOL,
			key:        "cors_allow_credentials",
			wantSecret: false,
		},
		{
			name:       "bool explicit secret stays secret",
			kind:       pbc.PluginSettingType_PLUGIN_SETTING_TYPE_BOOL,
			secretFlag: true,
			key:        "cors_allow_credentials",
			wantSecret: true,
		},
		{
			name:       "enum credential-like name is not secret",
			kind:       pbc.PluginSettingType_PLUGIN_SETTING_TYPE_ENUM,
			key:        "token_mode",
			wantSecret: false,
		},
		{
			name:       "enum explicit secret stays secret",
			kind:       pbc.PluginSettingType_PLUGIN_SETTING_TYPE_ENUM,
			secretFlag: true,
			key:        "token_mode",
			wantSecret: true,
		},
		{
			name:       "string api_token stays secret",
			kind:       pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING,
			key:        "api_token",
			wantSecret: true,
		},
		{
			name:       "string region is not secret",
			kind:       pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING,
			key:        "region",
			wantSecret: false,
		},
		{
			name:       "url api_key stays secret",
			kind:       pbc.PluginSettingType_PLUGIN_SETTING_TYPE_URL,
			key:        "api_key",
			wantSecret: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			setting := &pbc.PluginSetting{Key: tt.key, Type: tt.kind, Secret: tt.secretFlag}
			if tt.kind == pbc.PluginSettingType_PLUGIN_SETTING_TYPE_ENUM {
				setting.EnumValues = []string{"off", "on"}
			}
			got := descriptorsFromInfo([]*pbc.PluginSetting{setting})
			require.Len(t, got, 1)
			assert.Equal(t, tt.wantSecret, got[0].Secret)
			assert.Equal(t, tt.wantSecret, settingIsSecret(got[0], true, tt.key))
		})
	}
}

func TestPluginSettingValuesOmitSecretText(t *testing.T) {
	t.Parallel()

	descriptors := descriptorsFromInfo([]*pbc.PluginSetting{
		{Key: "cors_allow_credentials", Type: pbc.PluginSettingType_PLUGIN_SETTING_TYPE_BOOL},
		{Key: "api_token", Type: pbc.PluginSettingType_PLUGIN_SETTING_TYPE_STRING},
	})
	values := pluginSettingValues(
		"aws-public",
		[]string{"cors_allow_credentials", "api_token"},
		descriptors,
		nil,
		map[string]string{
			"cors_allow_credentials": "true",
			"api_token":              "super-secret",
		},
		nil,
		pluginhost.Delivered{},
	)
	byKey := map[string]webui.PluginSettingValue{}
	for _, value := range values {
		byKey[value.Key] = value
	}
	assert.False(t, byKey["cors_allow_credentials"].Secret)
	assert.Equal(t, "true", byKey["cors_allow_credentials"].Global.Value)
	assert.True(t, byKey["api_token"].Secret)
	assert.Empty(t, byKey["api_token"].Global.Value)
	assert.True(t, byKey["api_token"].Global.Set)

	body, err := json.Marshal(values)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "super-secret")
	assert.Contains(t, string(body), "true")
}
