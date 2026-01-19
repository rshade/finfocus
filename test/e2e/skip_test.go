package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldSkip_MissingCredentials(t *testing.T) {
	// Ensure no credentials
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")

	skip, reason := ShouldSkip("aws")
	assert.True(t, skip)
	assert.Contains(t, reason, "missing credentials")
}

func TestShouldSkip_WithCredentials(t *testing.T) {
	// Set credentials
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	skip, reason := ShouldSkip("aws")
	assert.False(t, skip)
	assert.Empty(t, reason)
}
