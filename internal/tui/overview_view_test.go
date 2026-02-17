package tui

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestOverviewView_InitializingRender verifies the initializing view output.
func TestOverviewView_InitializingRender(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0)
	assert.Equal(t, ViewStateInitializing, model.state)

	// Set a phase message
	model.progressMsg = "Loading stack state..."

	output := model.View()
	assert.Contains(t, output, "Loading stack state...")
	// Should not contain table elements
	assert.NotContains(t, output, "Resource")
	assert.NotContains(t, output, "Type")
}

// TestOverviewView_InitializingDefaultMsg verifies default message when progressMsg is empty.
func TestOverviewView_InitializingDefaultMsg(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0)
	model.progressMsg = ""

	output := model.View()
	assert.Contains(t, output, "Initializing...")
}

// TestOverviewView_ErrorStateRender verifies the error view output.
func TestOverviewView_ErrorStateRender(t *testing.T) {
	ctx := context.Background()

	model, _ := NewOverviewModel(ctx, nil, 0)
	model.state = ViewStateError
	model.err = assert.AnError

	output := model.View()
	assert.Contains(t, output, assert.AnError.Error())
}
