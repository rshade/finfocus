package ingest_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/ingest"
)

func TestParseTerraformState(t *testing.T) {
	t.Run("valid v4 state", func(t *testing.T) {
		data, err := os.ReadFile("../../examples/plans/terraform-simple-state.json")
		require.NoError(t, err)
		state, err := ingest.ParseTerraformState(data)
		require.NoError(t, err)
		assert.Equal(t, 4, state.Version)
		assert.Equal(t, "1.9.0", state.TerraformVersion)
		require.Len(t, state.Resources, 3)
	})

	t.Run("rejects version 3", func(t *testing.T) {
		_, err := ingest.ParseTerraformState([]byte(`{"version": 3, "resources": []}`))
		require.Error(t, err)
		assert.ErrorIs(t, err, ingest.ErrUnsupportedStateVersion)
	})

	t.Run("detects encrypted state", func(t *testing.T) {
		data, err := os.ReadFile("../../examples/plans/terraform-encrypted-state.json")
		require.NoError(t, err)
		_, err = ingest.ParseTerraformState(data)
		require.Error(t, err)
		assert.ErrorIs(t, err, ingest.ErrEncryptedState)
		assert.Contains(t, err.Error(), "encryption_version")
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		_, err := ingest.ParseTerraformState([]byte(`{not json`))
		require.Error(t, err)
	})
}

func TestGetManagedResources(t *testing.T) {
	t.Run("filters data sources", func(t *testing.T) {
		data, err := os.ReadFile("../../examples/plans/terraform-simple-state.json")
		require.NoError(t, err)
		state, err := ingest.ParseTerraformState(data)
		require.NoError(t, err)
		managed := state.GetManagedResources()
		require.Len(t, managed, 2)
		for _, r := range managed {
			assert.Equal(t, "managed", r.Mode)
		}
	})

	t.Run("filters deposed and tainted instances", func(t *testing.T) {
		data, err := os.ReadFile("../../examples/plans/terraform-deposed-state.json")
		require.NoError(t, err)
		state, err := ingest.ParseTerraformState(data)
		require.NoError(t, err)
		managed := state.GetManagedResources()
		require.Len(t, managed, 1)
		require.Len(t, managed[0].Instances, 1)
		attrs := managed[0].Instances[0].Attributes
		assert.Equal(t, "i-0new", attrs["id"])
	})
}

func TestLoadTerraformState(t *testing.T) {
	t.Run("file not found", func(t *testing.T) {
		_, err := ingest.LoadTerraformState("does/not/exist.tfstate")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reading terraform state file")
	})

	t.Run("valid file", func(t *testing.T) {
		state, err := ingest.LoadTerraformState("../../examples/plans/terraform-simple-state.json")
		require.NoError(t, err)
		assert.Equal(t, 4, state.Version)
	})
}
