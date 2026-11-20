// Package notification delivers budget alert notifications.
//
// When a cost command finds an exceeded threshold with destinations and the run
// opts in, the CLI builds one [BudgetAlertEvent] per threshold and hands a
// [Delivery] per destination to a [Dispatcher]. The dispatcher expands allowed
// notification variables and sends each delivery concurrently with a deadline.
//
// Slack, generic webhooks, and PagerDuty use HTTPS. Email uses SMTP.
// Delivery never fails the cost command. [DeliveryResult] masks credentials
// and may include bounded, sanitized PagerDuty diagnostics. The package keeps
// no state between runs.
package notification
