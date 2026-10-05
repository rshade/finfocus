// Package notification delivers budget alert notifications.
//
// When a budget-evaluating cost command finds an exceeded alert threshold that
// has destinations attached, and the run opted in with --notify or
// FINFOCUS_NOTIFY, the CLI builds one [BudgetAlertEvent] per exceeded threshold
// and hands a [Delivery] per destination to a [Dispatcher]. The dispatcher
// expands ${FINFOCUS_NOTIFY_*} references, checks HTTPS, and sends every
// delivery concurrently with a per-destination time limit.
//
// Two destination types are built in: Slack incoming webhooks and generic
// HTTPS webhooks that receive the budget.threshold.exceeded JSON event.
//
// Delivery never fails the command. Each outcome is reported as a
// [DeliveryResult] whose reason has every resolved URL and header value
// replaced by [RedactedText], and response bodies are never read into a
// reason. No state is kept between runs.
package notification
