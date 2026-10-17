package pluginhost

import (
	"bytes"
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveDeliveredEnv_Precedence(t *testing.T) {
	t.Parallel()

	secret := "super-secret-value"
	settings := []SettingDesc{
		{Key: "api_key", Env: "TYPESAFE_API_KEY", Secret: true, Required: true},
		{Key: "model", Env: "JEV_MODEL", Default: "jev-1.13.0", HasDefault: true},
	}
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())

	injected, parentDigest := ResolveDeliveredEnv(ctx, "jev",
		[]string{"TYPESAFE_API_KEY=" + secret, "FINFOCUS_PLUGIN_PROMETHEUS_TOKEN=other"},
		map[string]string{"api_key": "from-project", "extra": "from-project"},
		map[string]string{"api_key": "from-global", "extra": "from-global", "only_global": "g"},
		settings)
	assert.NotContains(t, injected, "TYPESAFE_API_KEY="+secret)
	assert.Contains(t, injected, "JEV_MODEL=jev-1.13.0")
	assert.Contains(t, injected, "FINFOCUS_PLUGIN_JEV_EXTRA=from-project")
	assert.Contains(t, injected, "FINFOCUS_PLUGIN_JEV_ONLY_GLOBAL=g")
	assert.NotContains(t, injected, "FINFOCUS_PLUGIN_PROMETHEUS_TOKEN=other")
	assert.NotContains(t, buf.String(), secret)
	assert.NotEmpty(t, parentDigest)

	_, changed := ResolveDeliveredEnv(ctx, "jev", []string{"TYPESAFE_API_KEY=other-secret"},
		map[string]string{"extra": "from-project"},
		map[string]string{"only_global": "g"},
		settings)
	assert.NotEqual(t, parentDigest, changed)

	injected, _ = ResolveDeliveredEnv(ctx, "jev", nil,
		map[string]string{"api_key": "from-project"},
		map[string]string{"api_key": "from-global"},
		[]SettingDesc{{Key: "api_key", Env: "TYPESAFE_API_KEY", Secret: true}})
	assert.Equal(t, []string{"TYPESAFE_API_KEY=from-project"}, injected)

	injected, _ = ResolveDeliveredEnv(ctx, "jev", nil, nil,
		map[string]string{"api_key": "from-global"},
		[]SettingDesc{{Key: "api_key", Env: "TYPESAFE_API_KEY"}})
	assert.Equal(t, []string{"TYPESAFE_API_KEY=from-global"}, injected)

	injected, _ = ResolveDeliveredEnv(ctx, "jev", []string{"TYPESAFE_API_KEY="},
		map[string]string{"api_key": "from-project"}, nil,
		[]SettingDesc{{Key: "api_key", Env: "TYPESAFE_API_KEY"}})
	assert.Empty(t, injected)

	_, digest := ResolveDeliveredEnv(ctx, "recorder",
		[]string{"FINFOCUS_PLUGIN_JEV_TOKEN=nope"}, nil, nil, nil)
	assert.Empty(t, digest)

	injected, _ = ResolveDeliveredEnv(ctx, "recorder", nil,
		map[string]string{"mock_response": "body"}, nil,
		[]SettingDesc{{Key: "mock_response", Env: "FINFOCUS_RECORDER_MOCK_RESPONSE"}})
	assert.Equal(t, []string{"FINFOCUS_RECORDER_MOCK_RESPONSE=body"}, injected)
	assert.NotContains(t, buf.String(), "body")
}

func TestResolveDeliveredEnv_EmptySetOmitsDigest(t *testing.T) {
	t.Parallel()
	injected, digest := ResolveDeliveredEnv(context.Background(), "recorder", nil, nil, nil, nil)
	assert.Empty(t, injected)
	assert.Empty(t, digest)
	require.NotContains(t, digest, "recorder")
}
