package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/ax-go"
)

func TestToValidationError(t *testing.T) {
	t.Run("nil error passes through", func(t *testing.T) {
		assert.NoError(t, toValidationError(context.Background(), nil))
	})

	t.Run("plain error becomes validation_error with exit code 2", func(t *testing.T) {
		cause := errors.New("--from is required when using --terraform-state")
		err := toValidationError(context.Background(), cause)
		require.Error(t, err)

		assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err))

		var axErr *ax.Error
		require.ErrorAs(t, err, &axErr)
		assert.Equal(t, "validation_error", axErr.ErrorCode)
		assert.Equal(t, cause.Error(), axErr.Error())
		require.ErrorIs(t, err, cause)
	})

	t.Run("existing ax error is unchanged", func(t *testing.T) {
		original := ax.NewError(context.Background(), "budget_exceeded", "over budget",
			ax.WithErrorExitCode(42))
		err := toValidationError(context.Background(), original)
		require.Error(t, err)
		assert.Same(t, original, err)
		assert.Equal(t, 42, ax.ErrorExitCode(err))
	})
}
