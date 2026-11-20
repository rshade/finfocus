package engine

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterInputError(t *testing.T) {
	t.Parallel()

	inner := errors.New("invalid --selector \"x\": want key=value")
	var err error = &ClusterInputError{Err: inner}
	assert.Equal(t, inner.Error(), err.Error(), "the CLI and the web show the inner message as written")
	require.ErrorIs(t, err, inner)
	got, ok := errors.AsType[*ClusterInputError](err)
	require.True(t, ok)
	assert.Same(t, inner, got.Err)
}

func TestClusterPluginMissingError(t *testing.T) {
	t.Parallel()

	err := &ClusterPluginMissingError{Capability: "usage_stats"}
	assert.Equal(t,
		"no installed plugin provides usage_stats; install one with: finfocus plugin install kubernetes",
		err.Error())
	got, ok := errors.AsType[*ClusterPluginMissingError](error(err))
	require.True(t, ok)
	assert.Equal(t, "usage_stats", got.Capability)
}
