package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
)

func TestEmbeddedCatalogNamesSatisfyESCValidation(t *testing.T) {
	t.Setenv("FINFOCUS_HOME", t.TempDir())
	assert.Contains(t, embeddedCatalogNames(), "aws-public")

	config.SetESCCatalogNames(embeddedCatalogNames)
	t.Cleanup(func() { config.SetESCCatalogNames(nil) })
	result, _ := config.ValidateConfigSource("config.hujson", []byte(`{
		"esc": {"plugins": {"aws-public": {"environments": ["billing/aws"]}}}
	}`))
	require.True(t, result.Valid)
	for _, warning := range result.Warnings {
		assert.NotEqual(t, "esc.plugins.aws-public", warning.Path)
	}
}
