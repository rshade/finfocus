package cli

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/ax-go"
)

func TestToValidationError(t *testing.T) {
	t.Parallel()

	t.Run("nil error passes through", func(t *testing.T) {
		t.Parallel()
		assert.NoError(t, toValidationError(context.Background(), nil))
	})

	t.Run("plain error becomes validation_error with exit code 2", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
		original := ax.NewError(context.Background(), "budget_exceeded", "over budget",
			ax.WithErrorExitCode(42))
		err := toValidationError(context.Background(), original)
		require.Error(t, err)
		assert.Same(t, original, err)
		assert.Equal(t, 42, ax.ErrorExitCode(err))
	})
}

//nolint:paralleltest // NewRootCmd sets the resolved project dir, which is process-wide state.
func TestRootCmdFlagErrorsExitAsValidation(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "unknown flag", args: []string{"cost", "projected", "--web"}, want: "unknown flag: --web"},
		{name: "missing flag argument", args: []string{"--stack"}, want: "flag needs an argument: --stack"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewRootCmdWithArgs("test", tt.args, func(string) (string, bool) { return "", false })
			cmd.SetArgs(tt.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)

			err := cmd.Execute()
			require.Error(t, err)
			assert.Equal(t, ax.ExitValidation, ax.ErrorExitCode(err))
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}
