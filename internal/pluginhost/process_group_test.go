package pluginhost

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveStderrPassthrough(t *testing.T) {
	t.Run("nil stderr discards", func(t *testing.T) {
		assert.Equal(t, io.Discard, resolveStderrPassthrough(nil))
	})

	t.Run("non-terminal stderr discards", func(t *testing.T) {
		// A pipe stands in for CI/test runners where Core's stderr is not a
		// terminal; plugin output must not be passed through (issue #1231).
		r, w, err := os.Pipe()
		require.NoError(t, err)
		defer func() {
			_ = r.Close()
			_ = w.Close()
		}()

		assert.Equal(t, io.Discard, resolveStderrPassthrough(w))
	})
}
