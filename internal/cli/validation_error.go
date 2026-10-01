package cli

import (
	"context"
	"errors"

	"github.com/rshade/ax-go"
)

// toValidationError wraps user-input errors (invalid flag combinations,
// unreadable or malformed --terraform-state/--pulumi-json/--pulumi-state
// input files, unparseable date ranges) in an *ax.Error carrying
// ax.ExitValidation so ax.Execute exits with code 2 instead of the default
// internal-error bucket (exit 1). Errors that are already *ax.Error are
// returned unchanged; nil is returned unchanged.
func toValidationError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	var axErr *ax.Error
	if errors.As(err, &axErr) {
		return err
	}
	return ax.NewError(ctx, "validation_error", err.Error(),
		ax.WithErrorExitCode(ax.ExitValidation),
		ax.WithErrorCause(err))
}
