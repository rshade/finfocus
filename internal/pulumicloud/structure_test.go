package pulumicloud

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportRejectsMissingDeployment(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{}`, `null`, `{"deployment":null}`} {
		_, err := DecodeIdentities(strings.NewReader(raw))
		require.Error(t, err, raw)
	}
	for _, raw := range []string{`{"deployment":{}}`, `{"deployment":{"resources":null}}`} {
		ids, err := DecodeIdentities(strings.NewReader(raw))
		require.NoError(t, err, raw)
		assert.Empty(t, ids)
	}
	ids, err := DecodeIdentities(strings.NewReader(`{"deployment":{"resources":[]}}`))
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestUpdatePageRejectsMissingUpdates(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{}`, `null`} {
		_, err := decodeUpdatePage(strings.NewReader(raw))
		require.Error(t, err, raw)
	}
	// Pulumi Cloud sends null, not [], on the page after the last update.
	updates, err := decodeUpdatePage(strings.NewReader(`{"updates":null}`))
	require.NoError(t, err)
	assert.Empty(t, updates)
	updates, err = decodeUpdatePage(strings.NewReader(`{"updates":[]}`))
	require.NoError(t, err)
	assert.Empty(t, updates)
}
