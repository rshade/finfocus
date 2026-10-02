// Copyright 2025-2026 Richard Shade. All rights reserved.
// Licensed under the Apache License, Version 2.0. See LICENSE for details.

package tui_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/tui"
)

//nolint:paralleltest // clearEnv uses t.Setenv, which changes the process environment
func TestDetectOutputModeFor_NonTTYWriter(t *testing.T) {
	clearEnv(t, "NO_COLOR", "TERM", "CI")

	var buf bytes.Buffer
	assert.Equal(t, tui.OutputModePlain, tui.DetectOutputModeFor(&buf, false, false, false))
	assert.Equal(t, tui.OutputModeStyled, tui.DetectOutputModeFor(&buf, true, false, false))
	assert.Equal(t, tui.OutputModePlain, tui.DetectOutputModeFor(&buf, true, true, false))
	assert.Equal(t, tui.OutputModePlain, tui.DetectOutputModeFor(&buf, true, false, true))
}

func TestDetectOutputModeFor_NoColorEnv(t *testing.T) {
	clearEnv(t, "NO_COLOR", "TERM", "CI")
	t.Setenv("NO_COLOR", "1")

	var buf bytes.Buffer
	require.Equal(t, tui.OutputModePlain, tui.DetectOutputModeFor(&buf, true, false, false))
	require.Equal(t, tui.OutputModePlain, tui.DetectOutputMode(true, false, false))
}

func TestDetectOutputMode_NoColorWinsOverForceColor(t *testing.T) {
	t.Parallel()
	// Flag precedence does not read the environment, but NO_COLOR in the
	// process would also force plain. Clear it only when this test runs alone
	// from a parent that is not parallel: the flag check returns before getenv
	// when noColor is set, so the result is plain either way.
	assert.Equal(t, tui.OutputModePlain, tui.DetectOutputMode(true, true, false))
}
