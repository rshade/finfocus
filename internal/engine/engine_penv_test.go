package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
