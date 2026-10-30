package history

import "errors"

var (
	// ErrNoCostHistory means no cost history database exists for the stack.
	ErrNoCostHistory = errors.New("no cost history")
	// ErrNoSnapshots means a cost history database holds no snapshots to read.
	ErrNoSnapshots = errors.New("no cost history snapshots")
)

// QueryError is a cost history request the caller got wrong: a bad date,
// range, selector, threshold, or currency choice. Its message is the
// underlying error's, so callers can show it unchanged.
type QueryError struct {
	Err error
}

func (e *QueryError) Error() string { return e.Err.Error() }

func (e *QueryError) Unwrap() error { return e.Err }

// IsQueryError reports whether err is, or wraps, a QueryError.
func IsQueryError(err error) bool {
	_, ok := errors.AsType[*QueryError](err)
	return ok
}

// IsNoHistory reports whether err means there is no cost history to read yet,
// as opposed to history that could not be read.
func IsNoHistory(err error) bool {
	return errors.Is(err, ErrNoCostHistory) || errors.Is(err, ErrNoSnapshots)
}
