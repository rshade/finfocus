package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rshade/finfocus/plugins/carbon"
)

// Startup configuration reads the process environment.
func TestRunRejectsInvalidConfiguration(t *testing.T) {
	t.Setenv(carbon.EnvUtilization, "invalid")
	assert.Equal(t, 1, run())
}
