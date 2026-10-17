package cli

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/registry"
	"github.com/rshade/finfocus/internal/webui"
)

func settingText(value string) *string { return &value }

func TestFilterPluginChanges(t *testing.T) {
	t.Parallel()
	descriptors := []webui.PluginSettingDescriptor{
		{Key: "api_key", Type: "string", Required: true, Secret: true, Env: "TYPESAFE_API_KEY"},
		{Key: "batch_size", Type: "int", Env: "JEV_BATCH_SIZE", Default: "25", DefaultSet: true},
		{Key: "enabled", Type: "bool", Env: "EXAMPLE_ENABLED"},
		{Key: "level", Type: "enum", Env: "EXAMPLE_LEVEL", EnumValues: []string{"low", "high"}},
		{Key: "base_url", Type: "url", Env: "JEV_BASE_URL", Default: "https://api.typesafe.ai", DefaultSet: true},
		{Key: "timeout", Type: "duration", Env: "JEV_TIMEOUT"},
	}
	tests := []struct {
		name    string
		request config.WriteRequest
		project map[string]string
		global  map[string]string
		environ []string
		want    int
		wantErr string
	}{
		{
			name: "secret empty is omitted",
			request: config.WriteRequest{Tier: config.TierGlobal, Changes: []config.Change{
				{Key: "plugins.jev.api_key", Value: settingText("")},
				{Key: "plugins.jev.batch_size", Value: settingText("10")},
			}},
			environ: []string{"TYPESAFE_API_KEY=present"},
			want:    1,
		},
		{
			name: "secret replacement is kept",
			request: config.WriteRequest{Tier: config.TierGlobal, Changes: []config.Change{
				{Key: "plugins.jev.api_key", Value: settingText("new-key")},
			}},
			want: 1,
		},
		{
			name: "masked placeholder is rejected",
			request: config.WriteRequest{Tier: config.TierGlobal, Changes: []config.Change{
				{Key: "plugins.jev.api_key", Value: settingText(config.RedactedValue)},
			}},
			wantErr: "masked placeholder",
		},
		{
			name: "int rejects a fraction",
			request: config.WriteRequest{Tier: config.TierGlobal, Changes: []config.Change{
				{Key: "plugins.jev.batch_size", Value: settingText("2.0")},
			}},
			environ: []string{"TYPESAFE_API_KEY=present"},
			wantErr: "integer",
		},
		{
			name: "bool accepts only true and false",
			request: config.WriteRequest{Tier: config.TierGlobal, Changes: []config.Change{
				{Key: "plugins.jev.enabled", Value: settingText("yes")},
			}},
			environ: []string{"TYPESAFE_API_KEY=present"},
			wantErr: "true or false",
		},
		{
			name: "enum must be listed",
			request: config.WriteRequest{Tier: config.TierGlobal, Changes: []config.Change{
				{Key: "plugins.jev.level", Value: settingText("medium")},
			}},
			environ: []string{"TYPESAFE_API_KEY=present"},
			wantErr: "allowed value",
		},
		{
			name: "url must be absolute",
			request: config.WriteRequest{Tier: config.TierGlobal, Changes: []config.Change{
				{Key: "plugins.jev.base_url", Value: settingText("api.typesafe.ai")},
			}},
			environ: []string{"TYPESAFE_API_KEY=present"},
			wantErr: "absolute URL",
		},
		{
			name: "duration uses Go syntax",
			request: config.WriteRequest{Tier: config.TierGlobal, Changes: []config.Change{
				{Key: "plugins.jev.timeout", Value: settingText("soon")},
			}},
			environ: []string{"TYPESAFE_API_KEY=present"},
			wantErr: "duration",
		},
		{
			name: "required rejects an empty effective value",
			request: config.WriteRequest{Tier: config.TierGlobal, Changes: []config.Change{
				{Key: "plugins.jev.batch_size", Value: settingText("4")},
			}},
			wantErr: "required",
		},
		{
			name: "parent env satisfies required",
			request: config.WriteRequest{Tier: config.TierGlobal, Changes: []config.Change{
				{Key: "plugins.jev.batch_size", Value: settingText("4")},
			}},
			environ: []string{"TYPESAFE_API_KEY="},
			want:    1,
		},
		{
			name: "clear removes a secret",
			request: config.WriteRequest{Tier: config.TierProject, Changes: []config.Change{
				{Key: "plugins.jev.api_key", Clear: true},
			}},
			global: map[string]string{"api_key": "kept"},
			want:   1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := filterPluginChanges("jev", descriptors, tt.project, tt.global, tt.environ, tt.request)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				var status *webui.StatusError
				require.ErrorAs(t, err, &status)
				assert.Equal(t, http.StatusBadRequest, status.Status)
				return
			}
			require.NoError(t, err)
			assert.Len(t, got.Changes, tt.want)
		})
	}
}

func TestEarlierWithAssetsPicksHighestOlderRelease(t *testing.T) {
	t.Parallel()
	latest := registry.GitHubRelease{TagName: "v2.0.0"}
	releases := []registry.GitHubRelease{
		{TagName: "v2.0.0"},
		{TagName: "v1.0.0", Assets: []registry.ReleaseAsset{{Name: "bin"}}},
		{TagName: "v1.2.0", Assets: []registry.ReleaseAsset{{Name: "bin"}}},
		{TagName: "v0.9.0", Assets: []registry.ReleaseAsset{{Name: "bin"}}},
	}
	got, ok := earlierWithAssets(releases, "", latest)
	require.True(t, ok)
	assert.Equal(t, "v1.2.0", got.TagName)
}

func TestNewestPlugin(t *testing.T) {
	t.Parallel()
	plugins := []registry.PluginInfo{
		{Name: "demo", Version: "v1.0.0"},
		{Name: "demo", Version: "v1.2.0"},
		{Name: "other", Version: "v9.0.0"},
	}
	got, ok := newestPlugin(plugins, "demo")
	require.True(t, ok)
	assert.Equal(t, "v1.2.0", got.Version)
	_, ok = newestPlugin(plugins, "missing")
	assert.False(t, ok)
}

func TestCatalogHostErrorHidesTheCause(t *testing.T) {
	t.Parallel()
	err := catalogHostError(context.Background(), "aws-public", assert.AnError)
	var status *webui.StatusError
	require.ErrorAs(t, err, &status)
	assert.Equal(t, http.StatusBadGateway, status.Status)
	assert.Equal(t, "catalog host returned an error", status.Text)
	assert.NotContains(t, status.Text, "assert.AnError")
	rate := catalogHostError(
		context.Background(),
		"aws-public",
		errors.New(githubRateLimitMessage+" at https://api.github.com"),
	)
	require.ErrorAs(t, rate, &status)
	assert.Equal(t, githubRateLimitMessage, status.Text)
}
