package stackload

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Error describes an input failure with safe categories and local file diagnostics.
// State backend errors are not retained or exposed through Unwrap, because a
// state failure becomes a category in JSON output. A failed preview is fatal
// and keeps Pulumi's error for the terminal, as before the shared loader.
type Error struct {
	// Operation identifies the failed input, state or plan.
	Operation string
	// Reason is a safe availability category.
	Reason       string
	cancellation error
	inputPath    string
	detail       string
	missing      bool
	cause        error
}

// Error returns Pulumi's preview error, or a redacted diagnostic for any other failure.
func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("loading Pulumi %s: %v", e.Operation, e.cause)
	}
	if e.detail != "" {
		return fmt.Sprintf("loading Pulumi %s %q: %s (%s)", e.Operation, e.inputPath, e.detail, e.Reason)
	}
	return fmt.Sprintf("loading Pulumi %s: %s", e.Operation, e.Reason)
}

// Unwrap exposes a failed preview's Pulumi error; state failures expose nothing.
func (e *Error) Unwrap() error {
	return e.cause
}

// Is preserves safe context and missing-file identities without exposing causes.
func (e *Error) Is(target error) bool {
	return e.cancellation != nil && target == e.cancellation || e.missing && target == os.ErrNotExist
}

func classifiedError(operation string, err error) *Error {
	reason := ReasonReadFailed
	text := strings.ToLower(err.Error())
	switch {
	case strings.Contains(text, "passphrase"), strings.Contains(text, "decrypt"), strings.Contains(text, "password"):
		reason = ReasonPassphraseRequired
	case strings.Contains(text, "network"),
		strings.Contains(text, "connection"),
		strings.Contains(text, "dial tcp"),
		strings.Contains(text, "login"),
		strings.Contains(text, "log in"),
		strings.Contains(text, "unauthorized"),
		strings.Contains(text, "forbidden"),
		strings.Contains(text, "unreachable"),
		strings.Contains(text, "timeout"),
		strings.Contains(text, "timed out"),
		strings.Contains(text, "no such host"):
		reason = ReasonBackendUnreachable
	}
	result := &Error{Operation: operation, Reason: reason}
	switch {
	case errors.Is(err, context.Canceled):
		result.cancellation = context.Canceled
	case errors.Is(err, context.DeadlineExceeded):
		result.cancellation = context.DeadlineExceeded
	}
	return result
}

// fileReadError adds only the caller-supplied path and a fixed local I/O diagnostic.
func fileReadError(operation, path string, err error) *Error {
	result := classifiedError(operation, err)
	if path == "" {
		return result
	}
	result.Reason = ReasonReadFailed
	result.inputPath = path
	result.detail = "could not read input"
	if errors.Is(err, os.ErrNotExist) {
		result.detail = "no such file"
		result.missing = true
	}
	return result
}

// invalidInputError never copies offending input or parser error text.
func invalidInputError(operation, path string, err error) *Error {
	result := classifiedError(operation, err)
	if path == "" || result.cancellation != nil {
		return result
	}
	result.Reason = ReasonReadFailed
	result.inputPath = path
	result.detail = "invalid JSON"
	return result
}
