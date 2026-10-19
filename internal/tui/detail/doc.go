// Package detail loads cost history for a TUI detail view when that view opens.
//
// The list view does not call the loader. Activate moves the model to the
// loading state before the fetch runs, so the detail view can show
// "Loading cost history..." immediately. A failed fetch stays on the detail
// view; r retries that fetch. ErrHistorySkipped means there is nothing to
// show, and the section is omitted.
package detail
