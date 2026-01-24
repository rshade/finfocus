package fixtures_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/test/fixtures"
)

func TestLoadPlan_Success(t *testing.T) {
	// Use existing fixture
	path := filepath.Join("aws", "simple.json")

	plan, err := fixtures.LoadPlan(path)
	require.NoError(t, err)
	assert.NotNil(t, plan)

	// Check for either resources or steps (deployment plan)
	if plan["resources"] == nil {
		assert.NotEmpty(t, plan["steps"], "Plan should have steps if no resources")
	} else {
		assert.NotEmpty(t, plan["resources"])
	}
}

func TestLoadPlan_NotFound(t *testing.T) {
	_, err := fixtures.LoadPlan("nonexistent.json")
	require.Error(t, err)
}

func TestLoadConfig_Success(t *testing.T) {
	path := filepath.Join("configs", "test-config.yaml")

	var cfg map[string]interface{}
	err := fixtures.LoadYAML(path, &cfg)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}
