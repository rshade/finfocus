package esc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	sdk "github.com/pulumi/esc-sdk/sdk/go"
)

// Category is why an ESC environment was skipped.
type Category string

const (
	// CategoryNotLoggedIn means no usable Pulumi Cloud token or organization.
	CategoryNotLoggedIn Category = "not_logged_in"
	// CategoryUnsupportedBackend means the state backend cannot host ESC.
	CategoryUnsupportedBackend Category = "unsupported_backend"
	// CategoryNotFound means the environment is missing or hidden.
	CategoryNotFound Category = "not_found"
	// CategoryAccessDenied means Pulumi Cloud returned 403.
	CategoryAccessDenied Category = "access_denied"
	// CategoryTimedOut means the ESC wait budget or the deadline ran out.
	CategoryTimedOut Category = "timed_out"
	// CategoryUnsupportedDefinition means the stack environment is not a list of imports.
	CategoryUnsupportedDefinition Category = "unsupported_definition"
	// CategoryUnavailable means the service, the transport, or the SDK failed.
	CategoryUnavailable Category = "unavailable"
)

// Failure is a redacted ESC error. It has no wrapped cause.
//
//nolint:errname // The contract names this Failure; it is not a Go XxxError type.
type Failure struct {
	Ref      Ref
	Category Category
}

// Error returns the fixed warning text. It never includes a value or an SDK error.
func (f Failure) Error() string {
	switch {
	case f.Category == CategoryNotLoggedIn && f.Ref.Name == "":
		return "ESC skipped (not logged in): set PULUMI_ACCESS_TOKEN or run `pulumi login`"
	case f.Category == CategoryNotLoggedIn:
		return fmt.Sprintf(
			"ESC environment %s skipped (not_logged_in): set PULUMI_ACCESS_TOKEN or run `pulumi login`",
			f.Ref.String(),
		)
	case f.Ref.Name == "":
		return fmt.Sprintf("ESC skipped (%s); plugins use shell and config values", f.detail())
	default:
		return fmt.Sprintf(
			"ESC environment %s skipped (%s); plugins use shell and config values",
			f.Ref.String(),
			f.detail(),
		)
	}
}

func (f Failure) detail() string {
	if f.Category == CategoryNotFound {
		return "not found or no access"
	}
	return string(f.Category)
}

type panicError struct{}

func (panicError) Error() string { return "esc call failed" }

func classify(ctx context.Context, err error) Failure {
	if err == nil {
		return Failure{}
	}
	if errors.Is(err, context.DeadlineExceeded) || deadlineExceeded(ctx) {
		return Failure{Category: CategoryTimedOut}
	}
	if _, ok := errors.AsType[panicError](err); ok {
		return Failure{Category: CategoryUnavailable}
	}
	if _, ok := errors.AsType[*url.Error](err); ok {
		return Failure{Category: CategoryUnavailable}
	}
	if apiErr, ok := errors.AsType[*sdk.GenericOpenAPIError](err); ok {
		return classifyModel(apiErr.Model())
	}
	return Failure{Category: CategoryUnavailable}
}

func deadlineExceeded(ctx context.Context) bool {
	return ctx != nil && errors.Is(ctx.Err(), context.DeadlineExceeded)
}

func classifyModel(model any) Failure {
	switch typed := model.(type) {
	case sdk.Error:
		return categoryForStatus(typed.Code)
	case *sdk.Error:
		if typed == nil {
			return Failure{Category: CategoryUnavailable}
		}
		return categoryForStatus(typed.Code)
	default:
		return Failure{Category: CategoryUnavailable}
	}
}

func categoryForStatus(code int32) Failure {
	switch code {
	case http.StatusUnauthorized:
		return Failure{Category: CategoryNotLoggedIn}
	case http.StatusForbidden:
		return Failure{Category: CategoryAccessDenied}
	case http.StatusNotFound:
		return Failure{Category: CategoryNotFound}
	default:
		return Failure{Category: CategoryUnavailable}
	}
}
