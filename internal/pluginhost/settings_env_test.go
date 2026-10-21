package pluginhost

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/esc"
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

	first := ResolveDeliveredEnv(ctx, "jev",
		[]string{"TYPESAFE_API_KEY=" + secret, "FINFOCUS_PLUGIN_PROMETHEUS_TOKEN=other"},
		map[string]string{"api_key": "from-project", "extra": "from-project"},
		map[string]string{"api_key": "from-global", "extra": "from-global", "only_global": "g"},
		settings, esc.Delivery{})
	injected, parentDigest := first.Pairs, first.Digest
	assert.NotContains(t, injected, "TYPESAFE_API_KEY="+secret)
	assert.Contains(t, injected, "JEV_MODEL=jev-1.13.0")
	assert.Contains(t, injected, "FINFOCUS_PLUGIN_JEV_EXTRA=from-project")
	assert.Contains(t, injected, "FINFOCUS_PLUGIN_JEV_ONLY_GLOBAL=g")
	assert.NotContains(t, injected, "FINFOCUS_PLUGIN_PROMETHEUS_TOKEN=other")
	assert.NotContains(t, buf.String(), secret)
	assert.Equal(t, "ce1c7185d071223843db76391ba50321b4463afdab9f1a2790b14aa91a35435d", parentDigest)

	changed := ResolveDeliveredEnv(ctx, "jev", []string{"TYPESAFE_API_KEY=other-secret"},
		map[string]string{"extra": "from-project"},
		map[string]string{"only_global": "g"},
		settings, esc.Delivery{})
	assert.NotEqual(t, parentDigest, changed.Digest)

	injected = ResolveDeliveredEnv(ctx, "jev", nil,
		map[string]string{"api_key": "from-project"},
		map[string]string{"api_key": "from-global"},
		[]SettingDesc{{Key: "api_key", Env: "TYPESAFE_API_KEY", Secret: true}},
		esc.Delivery{}).Pairs
	assert.Equal(t, []string{"TYPESAFE_API_KEY=from-project"}, injected)

	injected = ResolveDeliveredEnv(ctx, "jev", nil, nil,
		map[string]string{"api_key": "from-global"},
		[]SettingDesc{{Key: "api_key", Env: "TYPESAFE_API_KEY"}},
		esc.Delivery{}).Pairs
	assert.Equal(t, []string{"TYPESAFE_API_KEY=from-global"}, injected)

	injected = ResolveDeliveredEnv(ctx, "jev", []string{"TYPESAFE_API_KEY="},
		map[string]string{"api_key": "from-project"}, nil,
		[]SettingDesc{{Key: "api_key", Env: "TYPESAFE_API_KEY"}},
		esc.Delivery{}).Pairs
	assert.Empty(t, injected)

	digest := ResolveDeliveredEnv(ctx, "recorder",
		[]string{"FINFOCUS_PLUGIN_JEV_TOKEN=nope"}, nil, nil, nil, esc.Delivery{}).Digest
	assert.Empty(t, digest)

	injected = ResolveDeliveredEnv(ctx, "recorder", nil,
		map[string]string{"mock_response": "body"}, nil,
		[]SettingDesc{{Key: "mock_response", Env: "FINFOCUS_RECORDER_MOCK_RESPONSE"}},
		esc.Delivery{}).Pairs
	assert.Equal(t, []string{"FINFOCUS_RECORDER_MOCK_RESPONSE=body"}, injected)
	assert.NotContains(t, buf.String(), "body")
}

func TestResolveDeliveredEnv_ESC(t *testing.T) {
	t.Parallel()
	settings := []SettingDesc{
		{Key: "api_key", Env: "TYPESAFE_API_KEY", Secret: true},
		{Key: "model", Env: "JEV_MODEL", Default: "jev-1.13.0", HasDefault: true},
	}
	ref := esc.Ref{Org: "acme", Project: "billing", Name: "prod"}
	base := esc.Delivery{
		Values: map[string]string{
			"TYPESAFE_API_KEY":     "from-esc",
			"UNRELATED":            "hidden-value",
			"FINFOCUS_PLUGIN_PORT": "9",
		},
		Origins: map[string]esc.Ref{"TYPESAFE_API_KEY": ref, "UNRELATED": ref, "FINFOCUS_PLUGIN_PORT": ref},
		Fingerprints: map[string]string{
			"TYPESAFE_API_KEY":     "etag-1",
			"UNRELATED":            "etag-1",
			"FINFOCUS_PLUGIN_PORT": "etag-1",
		},
	}
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)
	ctx := logger.WithContext(context.Background())
	got := ResolveDeliveredEnv(ctx, "jev",
		[]string{"TYPESAFE_API_KEY=from-parent"},
		map[string]string{"api_key": "from-project"},
		map[string]string{"api_key": "from-global"},
		settings, base)
	assert.Contains(t, got.Pairs, "TYPESAFE_API_KEY=from-esc")
	assert.True(t, got.Overrides["TYPESAFE_API_KEY"])
	assert.Equal(t, ref, got.Origins["TYPESAFE_API_KEY"])
	assert.NotContains(t, got.Origins, "UNRELATED")
	assert.NotContains(t, got.Pairs, "UNRELATED=hidden-value")
	assert.NotContains(t, got.Pairs, "FINFOCUS_PLUGIN_PORT=9")
	assert.Contains(t, got.Pairs, "JEV_MODEL=jev-1.13.0")
	assert.NotContains(t, buf.String(), "hidden-value")
	assert.NotContains(t, buf.String(), "from-esc")
	assert.Contains(t, buf.String(), "TYPESAFE_API_KEY")
	assert.Contains(t, buf.String(), "acme/billing/prod")

	rotated := base
	rotated.Values = map[string]string{
		"TYPESAFE_API_KEY":     "rotated-value",
		"UNRELATED":            "hidden-value",
		"FINFOCUS_PLUGIN_PORT": "9",
	}
	again := ResolveDeliveredEnv(ctx, "jev",
		[]string{"TYPESAFE_API_KEY=from-parent"}, nil, nil, settings, rotated)
	assert.Equal(t, got.Digest, again.Digest)

	edited := base
	edited.Fingerprints = map[string]string{"TYPESAFE_API_KEY": "etag-2"}
	changedKey := ResolveDeliveredEnv(ctx, "jev",
		[]string{"TYPESAFE_API_KEY=from-parent"}, nil, nil, settings, edited)
	assert.NotEqual(t, got.Digest, changedKey.Digest)

	without := ResolveDeliveredEnv(ctx, "jev",
		[]string{"TYPESAFE_API_KEY=from-parent"},
		map[string]string{"api_key": "from-project"},
		map[string]string{"api_key": "from-global"},
		settings, esc.Delivery{})
	assert.NotContains(t, without.Pairs, "TYPESAFE_API_KEY=")
	assert.Empty(t, without.Origins)
}

func TestResolveDeliveredEnv_ConfiguredVariables(t *testing.T) {
	t.Parallel()
	allowed := []string{"AWS_ACCESS_KEY_ID"}
	src := esc.NewSource(esc.Options{
		PerPlugin: map[string]esc.PluginOptions{
			"aws": {Variables: allowed},
		},
		Budget: time.Second,
	})
	ctx := esc.WithSource(context.Background(), src)
	ref := esc.Ref{Org: "acme", Project: "billing", Name: "prod"}
	from := esc.Delivery{
		Values: map[string]string{
			"AWS_ACCESS_KEY_ID":     "from-esc",
			"AWS_SECRET_ACCESS_KEY": "hidden",
		},
		Origins: map[string]esc.Ref{
			"AWS_ACCESS_KEY_ID":     ref,
			"AWS_SECRET_ACCESS_KEY": ref,
		},
		Fingerprints: map[string]string{"AWS_ACCESS_KEY_ID": "etag", "AWS_SECRET_ACCESS_KEY": "etag"},
	}
	got := ResolveDeliveredEnv(ctx, "aws", nil, nil, nil, nil, from)
	assert.Equal(t, []string{"AWS_ACCESS_KEY_ID=from-esc"}, got.Pairs)
	assert.Equal(t, ref, got.Origins["AWS_ACCESS_KEY_ID"])
	other := ResolveDeliveredEnv(ctx, "scorer", nil, nil, nil, nil, from)
	assert.Empty(t, other.Pairs)
	assert.Empty(t, other.Origins)
}

//nolint:paralleltest // mutates the process-wide plugin environment digest
func TestPluginEnvDigestRoundTrip(t *testing.T) {
	ResetPluginEnvDigestsForTest()
	t.Cleanup(ResetPluginEnvDigestsForTest)
	assert.Equal(t, "FINFOCUS_PLUGIN_RECORDER_OUTPUT_DIR", GenericPluginEnv("recorder", "output-dir"))
	assert.True(t, sameStringSet([]string{"b", "a"}, []string{"a", "b"}))
	assert.False(t, sameStringSet([]string{"a"}, []string{"a", "b"}))

	assert.Empty(t, PluginEnvDigest())
	ReplacePluginEnvDigests([]*Client{nil, {Name: ""}, {Name: "recorder", DeliveredDigest: ""}})
	assert.Empty(t, PluginEnvDigest())

	clients := []*Client{{Name: "recorder", DeliveredDigest: "aaa"}, {Name: "scorer", DeliveredDigest: "bbb"}}
	ReplacePluginEnvDigests(clients)
	assert.Equal(t, CombinedDigest(clients), PluginEnvDigest())
	assert.NotEmpty(t, PluginEnvDigest())
	SetPluginEnvDigestForTest("recorder", "")
	assert.NotContains(t, PluginEnvDigest(), "aaa")
	ResetPluginEnvDigestsForTest()
	assert.Empty(t, PluginEnvDigest())
	assert.Empty(t, CombinedDigest(nil))
}

func TestSettingsEnvEdgePaths(t *testing.T) {
	set, values := parentEnvironment([]string{"novalue", "=blank", "A=1"})
	assert.Equal(t, map[string]bool{"A": true}, set)
	assert.Equal(t, "1", values["A"])
	_, ok := pickGeneric(map[string]bool{}, map[string]string{}, nil, nil, "missing", "MISSING")
	assert.False(t, ok)

	effective := map[string]string{}
	origins := map[string]esc.Ref{}
	stamps := map[string]escStamp{}
	overrides := map[string]bool{}
	applyESC(
		effective,
		map[string]bool{"FOO": true},
		esc.Delivery{Values: map[string]string{"FOO": "x"}},
		origins,
		stamps,
		overrides,
	)
	assert.Equal(t, "x", effective["FOO"])
	assert.Empty(t, origins)
	logResolved(context.Background(), "recorder", []string{"FOO", "BAR"}, map[string]escStamp{
		"FOO": {ref: "acme/billing/prod"},
		"BAR": {ref: "acme/billing/prod"},
	})

	dir := t.TempDir()
	t.Setenv("FINFOCUS_HOME", dir)
	t.Setenv("FINFOCUS_PROJECT_DIR", "")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.hujson"), []byte("{"), 0o600))
	project, global := pluginFileMaps(context.Background(), "recorder")
	assert.Empty(t, project)
	assert.Empty(t, global)

	ResetPluginEnvDigestsForTest()
	t.Cleanup(ResetPluginEnvDigestsForTest)
	SetPluginEnvDigestForTest("scorer", "ccc")
	assert.NotEmpty(t, PluginEnvDigest())
}

func TestResolveDeliveredEnv_EmptySetOmitsDigest(t *testing.T) {
	t.Parallel()
	got := ResolveDeliveredEnv(context.Background(), "recorder", nil, nil, nil, nil, esc.Delivery{})
	injected, digest := got.Pairs, got.Digest
	assert.Empty(t, injected)
	assert.Empty(t, digest)
	require.NotContains(t, digest, "recorder")
}
