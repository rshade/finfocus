package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseMetadataFlags(t *testing.T) {
	t.Run("valid key-value pair", func(t *testing.T) {
		m, warnings := parseMetadataFlags([]string{"region=us-east-1"})
		assert.Equal(t, map[string]string{"region": "us-east-1"}, m)
		assert.Empty(t, warnings)
	})

	t.Run("empty key is skipped with warning", func(t *testing.T) {
		m, warnings := parseMetadataFlags([]string{"=value", "good=ok"})
		assert.Equal(t, map[string]string{"good": "ok"}, m)
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "empty key")
	})

	t.Run("whitespace-only key is skipped with warning", func(t *testing.T) {
		m, warnings := parseMetadataFlags([]string{"  =value"})
		assert.Nil(t, m)
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "empty key")
	})

	t.Run("missing equals sign warns", func(t *testing.T) {
		m, warnings := parseMetadataFlags([]string{"noequals"})
		assert.Nil(t, m)
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "missing '='")
	})

	t.Run("empty input returns nil", func(t *testing.T) {
		m, warnings := parseMetadataFlags([]string{})
		assert.Nil(t, m)
		assert.Nil(t, warnings)
	})

	t.Run("last occurrence wins for duplicate keys", func(t *testing.T) {
		m, warnings := parseMetadataFlags([]string{"k=first", "k=second"})
		assert.Equal(t, map[string]string{"k": "second"}, m)
		assert.Empty(t, warnings)
	})
}
