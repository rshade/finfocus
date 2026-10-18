package webui

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunStatus_Transitions(t *testing.T) {
	t.Parallel()

	stateOnly := RunStatus{}.WithStateOnly(true)
	assert.Equal(t, RunStatus{StateOnly: true, DetectFailed: true}, stateOnly)
	assert.True(t, stateOnly.PreviewAllowed())

	loaded := stateOnly.WithPreviewLoaded()
	assert.Equal(t, RunStatus{PreviewLoaded: true}, loaded)
	assert.False(t, loaded.PreviewAllowed())
	assert.Equal(t, loaded, loaded.WithStateOnly(false))
}

func TestRunStatus_WireShape(t *testing.T) {
	t.Parallel()

	event, err := json.Marshal(RunStatusEvent{RunStatus: RunStatus{StateOnly: true}, Footnote: "n"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"stateOnly":true,"previewLoaded":false,"detectFailed":false,"footnote":"n"}`, string(event))

	snap, err := json.Marshal(OverviewSnapshot{RunStatus: RunStatus{PreviewLoaded: true}})
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(snap, &decoded))
	assert.Equal(t, false, decoded["stateOnly"])
	assert.Equal(t, true, decoded["previewLoaded"])
	assert.Equal(t, false, decoded["detectFailed"])
	assert.NotContains(t, decoded, "footnote")
}
