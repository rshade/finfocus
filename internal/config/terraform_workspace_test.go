package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerraformIncludeWorkspaceDefaultsFalse(t *testing.T) {
	t.Parallel()

	cfg := New()
	assert.False(t, cfg.Cost.Terraform.IncludeWorkspace)

	var decoded CostConfig
	require.NoError(t, json.Unmarshal([]byte(`{"terraform":{"include_workspace":true}}`), &decoded))
	assert.True(t, decoded.Terraform.IncludeWorkspace)
}
