package cli

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/config"
	"github.com/rshade/finfocus/internal/webui"
)

func TestRefuseMissingChecksumInSecureMode(t *testing.T) {
	t.Run("on refuses with a conflict the page can show", func(t *testing.T) {
		t.Setenv(config.EnvSecurePlugins, "true")

		err := refuseMissingChecksumInSecureMode()

		var status *webui.StatusError
		require.ErrorAs(t, err, &status)
		assert.Equal(t, http.StatusConflict, status.Status)
		assert.Equal(t, "checksum_required", status.Code)
		assert.Contains(t, status.Text, config.EnvSecurePlugins)
	})

	t.Run("off leaves the confirmation to the user", func(t *testing.T) {
		t.Setenv(config.EnvSecurePlugins, "false")

		require.NoError(t, refuseMissingChecksumInSecureMode())
	})

	t.Run("a bad value is an error, not a pass", func(t *testing.T) {
		t.Setenv(config.EnvSecurePlugins, "maybe")

		require.Error(t, refuseMissingChecksumInSecureMode())
	})
}
