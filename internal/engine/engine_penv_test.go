package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/pluginhost"
)

//nolint:paralleltest // PluginEnvDigest is process-wide
func TestCacheKey_PluginEnvDigest(t *testing.T) {
	pluginhost.ResetPluginEnvDigestsForTest()
	t.Cleanup(pluginhost.ResetPluginEnvDigestsForTest)

	resource := ResourceDescriptor{Provider: "aws", Type: "aws:ec2:Instance", ID: "i-1"}
	before, err := ProjectedResourceCacheKey(resource)
	require.NoError(t, err)
	assert.NotContains(t, before, "/penv-")

	request := ActualCostRequest{From: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	actualBefore := generateActualCostCacheKey(request)
	assert.NotContains(t, actualBefore, "/penv-")

	pluginhost.SetPluginEnvDigestForTest("recorder", "aaa")
	after, err := ProjectedResourceCacheKey(resource)
	require.NoError(t, err)
	assert.Contains(t, after, "/penv-")
	assert.NotEqual(t, before, after)
	assert.NotEqual(t, actualBefore, generateActualCostCacheKey(request))

	pluginhost.SetPluginEnvDigestForTest("recorder", "bbb")
	changed, err := ProjectedResourceCacheKey(resource)
	require.NoError(t, err)
	assert.NotEqual(t, after, changed)

	pluginhost.ResetPluginEnvDigestsForTest()
	restored, err := ProjectedResourceCacheKey(resource)
	require.NoError(t, err)
	assert.Equal(t, before, restored)
}
