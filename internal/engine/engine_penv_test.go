package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/esc"
	"github.com/rshade/finfocus/internal/pluginhost"
)

func TestCacheKey_EngineSettingsDigest(t *testing.T) {
	t.Parallel()

	resource := ResourceDescriptor{Provider: "aws", Type: "aws:ec2:Instance", ID: "i-1"}
	request := ActualCostRequest{From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	bare := New(nil, nil)
	before, err := bare.projectedCostCacheKey(resource)
	require.NoError(t, err)
	assert.NotContains(t, before, "/penv-")
	assert.NotContains(t, bare.actualCostCacheKey(request), "/penv-")

	withSettings := New([]*pluginhost.Client{{Name: "recorder", DeliveredDigest: "aaa"}}, nil)
	after, err := withSettings.projectedCostCacheKey(resource)
	require.NoError(t, err)
	assert.Contains(t, after, "/penv-")
	assert.NotEqual(t, before, after)
	assert.NotEqual(t, bare.actualCostCacheKey(request), withSettings.actualCostCacheKey(request))

	changedEngine := New([]*pluginhost.Client{{Name: "recorder", DeliveredDigest: "bbb"}}, nil)
	changed, err := changedEngine.projectedCostCacheKey(resource)
	require.NoError(t, err)
	assert.NotEqual(t, after, changed)

	// Another engine keeps the key it was built with.
	again, err := withSettings.projectedCostCacheKey(resource)
	require.NoError(t, err)
	assert.Equal(t, after, again)
}

func TestCacheKey_ESCRotationKeepsDigest(t *testing.T) {
	t.Parallel()
	settings := []pluginhost.SettingDesc{{Key: "token", Env: "AWS_ACCESS_KEY_ID"}}
	ref := esc.Ref{Org: "acme", Project: "billing", Name: "prod"}
	first := pluginhost.ResolveDeliveredEnv(t.Context(), "aws", nil, nil, nil, settings, esc.Delivery{
		Values:       map[string]string{"AWS_ACCESS_KEY_ID": "AKIA-one"},
		Origins:      map[string]esc.Ref{"AWS_ACCESS_KEY_ID": ref},
		Fingerprints: map[string]string{"AWS_ACCESS_KEY_ID": "etag-1"},
	})
	rotated := pluginhost.ResolveDeliveredEnv(t.Context(), "aws", nil, nil, nil, settings, esc.Delivery{
		Values:       map[string]string{"AWS_ACCESS_KEY_ID": "AKIA-two"},
		Origins:      map[string]esc.Ref{"AWS_ACCESS_KEY_ID": ref},
		Fingerprints: map[string]string{"AWS_ACCESS_KEY_ID": "etag-1"},
	})
	assert.Equal(t, first.Digest, rotated.Digest)
	edited := pluginhost.ResolveDeliveredEnv(t.Context(), "aws", nil, nil, nil, settings, esc.Delivery{
		Values:       map[string]string{"AWS_ACCESS_KEY_ID": "AKIA-two"},
		Origins:      map[string]esc.Ref{"AWS_ACCESS_KEY_ID": ref},
		Fingerprints: map[string]string{"AWS_ACCESS_KEY_ID": "etag-2"},
	})
	assert.NotEqual(t, first.Digest, edited.Digest)

	resource := ResourceDescriptor{Provider: "aws", Type: "aws:ec2:Instance", ID: "i-1"}
	stable := New([]*pluginhost.Client{{Name: "aws", DeliveredDigest: first.Digest}}, nil)
	same := New([]*pluginhost.Client{{Name: "aws", DeliveredDigest: rotated.Digest}}, nil)
	changed := New([]*pluginhost.Client{{Name: "aws", DeliveredDigest: edited.Digest}}, nil)
	stableKey, err := stable.projectedCostCacheKey(resource)
	require.NoError(t, err)
	sameKey, err := same.projectedCostCacheKey(resource)
	require.NoError(t, err)
	changedKey, err := changed.projectedCostCacheKey(resource)
	require.NoError(t, err)
	assert.Equal(t, stableKey, sameKey)
	assert.NotEqual(t, stableKey, changedKey)
	assert.Contains(t, stableKey, "/penv-")
}
