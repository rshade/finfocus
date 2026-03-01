package tui

import (
	"testing"
	"time"

	"charm.land/bubbles/v2/spinner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadingState_Update(t *testing.T) {
	loading := NewLoadingState()
	assert.NotNil(t, loading.spinner)
	assert.Equal(t, "Querying cost data from plugins...", loading.message)

	// Test Update with nil msg (spinner update usually requires specific msg, but returns cmd)
	// We just want to ensure it doesn't panic. cmd might be nil.
	_ = loading.Update(nil)

	// Check Init returns a command
	initCmd := loading.Init()
	assert.NotNil(t, initCmd)
}

func TestLoadingState_SpinnerTick(t *testing.T) {
	loading := NewLoadingState()

	// Test that spinner tick messages are handled without panic.
	initCmd := loading.Init()
	require.NotNil(t, initCmd)

	// Simulate a spinner tick message.
	tickMsg := spinner.TickMsg{Time: time.Now(), ID: 0}
	cmd := loading.Update(tickMsg)
	// The spinner should return another tick command.
	assert.NotNil(t, cmd)
}

func TestLoadingState_MessageUpdate(t *testing.T) {
	loading := NewLoadingState()
	assert.Equal(t, "Querying cost data from plugins...", loading.message)

	// Test updating the message.
	loading.message = "Processing results..."
	assert.Equal(t, "Processing results...", loading.message)

	output := RenderLoading(loading)
	assert.Contains(t, output, "Processing results...")
}

func TestRenderLoading(t *testing.T) {
	loading := NewLoadingState()
	output := RenderLoading(loading)
	assert.Contains(t, output, "Querying cost data from plugins...")
}

func TestRenderLoading_NilLoading(t *testing.T) {
	output := RenderLoading(nil)
	assert.Equal(t, "Loading...", output)
}
