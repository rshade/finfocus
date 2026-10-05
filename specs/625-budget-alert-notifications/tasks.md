---

description: "Task list for budget alert notifications"
---

# Tasks: Budget Alert Notifications

**Input**: Design documents from `specs/625-budget-alert-notifications/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Per Constitution Principle II, tests are written before the code
they cover. Each test task must fail before its implementation task starts.
Coverage targets are 80% overall, and 95% for `internal/notification` and
the CLI budget path.

**Completeness**: Per Constitution Principle VI, there are no stubs or TODO
comments. Email and PagerDuty are follow-up issues, not placeholders.

**Documentation**: Per Constitution Principle IV, docs are updated in this
PR.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependency on an
  unfinished task)
- **[Story]**: US1 (Slack, P1), US2 (generic webhook, P2), US3 (failures
  never break the command, P1)

All paths are relative to the repository root. Unit tests sit beside the
code. Tests that use `t.Setenv`, build a root command, or change
package-level state are not parallel and carry
`//nolint:paralleltest // <reason>`. Every other test calls `t.Parallel()`.
Tests that run cost commands set `FINFOCUS_HOME` to `t.TempDir()`.

---

## Phase 1: Setup

- [x] T001 Create the `internal/notification` package with a package doc
  comment describing budget alert delivery in `internal/notification/doc.go`

---

## Phase 2: Foundational (blocks every story)

**Purpose**: configuration types, validation, the engine copy of
destinations, `${NAME}` expansion, redaction, the alert event, and the
dispatcher shell that every destination type plugs into.

### Tests first (Foundational)

- [x] T002 [P] Write table tests for `NotificationDestination.Validate`:
  types `slack` and `webhook` are accepted; any other type fails and lists
  the supported types; an empty `url` fails; a literal `http://` or hostless
  URL fails with "HTTPS is required"; a URL containing `${NAME}` passes
  without a scheme check; `method` accepts `POST`/`PUT` in any case and
  rejects others; `channel` on a webhook fails; `method` or `headers` on a
  Slack destination fails; an empty header name fails. File:
  `internal/config/notification_test.go`
- [x] T003 [P] Add `AlertConfig.Validate` and `ValidateConfigSource` cases
  for destinations in global, provider, tag, and type scopes. Errors carry
  paths such as `cost.budgets.global.alerts[0].notifications[1].url`, and
  no unknown-key warning appears for `notifications`, `type`, `url`,
  `channel`, `method`, or `headers`. A reference to a variable without the
  `FINFOCUS_NOTIFY_` prefix in `url` or a header value is an error naming
  the variable. Files: `internal/config/budget_test.go`,
  `internal/config/validate_test.go`
- [x] T004 [P] Write project-source tests: after `ShallowMergeYAML` applies a
  project overlay whose `cost` section has destinations, every destination
  reports `FromProject() == true` and global-only destinations report false;
  a JSON field named `source` in the file does not set it; validating a
  project config document (`ValidateConfigSource` with project context)
  rejects any `${...}` in a destination with a hint to move it to the
  global config, while literal HTTPS destinations pass. Files:
  `internal/config/merge_test.go`, `internal/config/validate_test.go`
- [x] T005 [P] Write tests that `evaluateAlerts` copies an alert's
  destinations onto `ThresholdStatus.Notifications`, and that marshaling a
  `ScopedBudgetResult` to JSON contains no `notifications` key and no
  destination URL. File: `internal/engine/budget_cli_test.go`
- [x] T006 [P] Write tests for `Expand`: `${FINFOCUS_NOTIFY_X}` is replaced;
  several references in one value work; `$NAME`, a bare `$`, and `$$` stay
  literal; `${AWS_SECRET_ACCESS_KEY}`, `${GITHUB_TOKEN}`, and `${HOME}` return
  a "not a FINFOCUS_NOTIFY_ variable" error and the lookup function is never
  called for them (assert with a recording lookup); an unset or empty allowed
  variable returns an error that names the variable and contains no other
  part of the value; `IsSingleReference` and `HasReference` report whether a
  value is exactly one `${NAME}` or contains any `${`. File:
  `internal/notification/expand_test.go`
- [x] T007 [P] Write tests for `Redactor`: every registered secret is
  replaced by `[REDACTED]` in error text, including the secret inside a
  `*url.Error`; empty secrets are ignored; longer secrets are replaced
  before shorter ones that they contain. File:
  `internal/notification/redact_test.go`
- [x] T008 [P] Write tests for `NewBudgetAlertEvent`: an actual alert uses
  current spend and percentage; a forecasted alert uses forecasted spend and
  forecast percentage; `ThresholdValue` is amount × threshold / 100; legacy
  budgets map to scope `global`; scope keys pass through for provider, tag,
  and type; the version comes from `version.GetVersion()`. File:
  `internal/notification/event_test.go`
- [x] T009 [P] Write dispatcher tests using a fake `Sender` registered for a
  test type: every destination is attempted once; sends run concurrently
  (two senders that block until both have started complete without
  deadlock); results come back in input order; an expansion error yields
  `skipped` with the variable name; a non-HTTPS URL after expansion yields
  `skipped`; a project-sourced destination containing any reference yields
  `skipped` with a "project config cannot use variables" reason and the
  lookup function is never called; a project-sourced literal destination is
  sent; dry-run makes no `Send` call and yields `dry-run`. File:
  `internal/notification/dispatch_test.go`

### Implementation (Foundational)

- [x] T010 [P] Implement `NotificationDestination` (fields `Type`, `URL`,
  `Channel`, `Method`, `Headers` with `yaml`/`json` tags), the
  `NotificationTypeSlack`/`NotificationTypeWebhook` constants, sentinel
  errors, the `FINFOCUS_NOTIFY_` prefix rule for references, the unexported
  `source` field with `FromProject()` and an unexported setter used by the
  loader, and `Validate` per data-model.md. File:
  `internal/config/notification.go`
- [x] T011 Mark destinations as project-sourced when `ShallowMergeYAML`
  applies a `cost` section, and make `ValidateConfigSource` reject `${...}`
  in destinations of a project config document. Both callers pass whether
  the file is the resolved project config: `config validate`
  (`internal/cli/config_validate.go`) and the cost pre-run validation in the
  `cost` command's `PersistentPreRunE` (`internal/cli/root.go`), which
  validates the global file and, when one resolves, the project file.
  Files: `internal/config/merge.go`, `internal/config/validate.go`,
  `internal/cli/config_validate.go`, `internal/cli/root.go`
- [x] T012 Add `Notifications []NotificationDestination` to `AlertConfig`
  and validate each destination from `AlertConfig.Validate`, wrapping
  errors with the destination index. File: `internal/config/budget.go`
- [x] T013 Map the new sentinel errors and the
  `notifications[<i>].<field>` path segment in `ValidateConfigSource`, and
  register the new keys so they are not reported as unknown. Add the
  hints. File: `internal/config/validate.go`
- [x] T014 Add `Notifications []config.NotificationDestination` with
  `json:"-"` to `ThresholdStatus` and copy it in `evaluateAlerts`. File:
  `internal/engine/budget_cli.go`
- [x] T015 [P] Implement `Expand(value string, lookup func(string) (string,
  bool)) (string, error)`, `IsSingleReference`, and `HasReference`,
  expanding only `${FINFOCUS_NOTIFY_[A-Za-z0-9_]+}` and refusing every
  other `${...}` without calling `lookup`. File:
  `internal/notification/expand.go`
- [x] T016 [P] Implement `Redactor` with `Add(secret string)` and
  `Redact(text string) string`. File: `internal/notification/redact.go`
- [x] T017 [P] Implement `BudgetAlertEvent` and `NewBudgetAlertEvent`. File:
  `internal/notification/event.go`
- [x] T018 Implement the `Sender` interface (`Send(ctx, dest Resolved,
  event BudgetAlertEvent) error`), the `Resolved` destination (expanded URL,
  channel, method, headers), the `Outcome` constants, `DeliveryResult`, and
  `Dispatcher.Deliver(ctx, []Delivery, Options) []DeliveryResult`. It
  skips a project-sourced destination that has any reference before
  expansion, expands each other destination with a per-delivery `Redactor`,
  checks HTTPS
  after expansion, applies a per-destination `context.WithTimeout` (default
  10s, `Options.Timeout`), runs deliveries concurrently, and redacts every
  reason. File: `internal/notification/dispatch.go`
- [x] T019 Implement the shared HTTP client: no redirects
  (`CheckRedirect` returns `http.ErrUseLastResponse`, and a 3xx is a
  failure), `Content-Type: application/json`, a non-2xx response is an
  error naming only the status code, and the response body is drained and
  never echoed. File: `internal/notification/http.go`

**Checkpoint**: `go test ./internal/config/... ./internal/engine/...
./internal/notification/...` passes.

---

## Phase 3: User Story 1 - Slack alert when a threshold is crossed (P1) 🎯 MVP

**Goal**: an opted-in run that crosses a threshold posts one Slack message
per exceeded alert.

**Independent test**: a budget of $100 with an 80% actual alert pointing at
an `httptest.NewTLSServer`; a run with `--notify` at $85 sends one message
with the contract fields; $50 sends none; no `--notify` sends none and
prints the hint.

### Tests first (US1)

- [x] T020 [P] [US1] Write Slack sender tests against
  `httptest.NewTLSServer`: the body matches `contracts/slack-message.md`
  (header block, four fields, `text` fallback); `channel` appears only when
  set; a forecasted alert says "Forecasted spend"; scoped budgets name their
  scope; currency and percentage formatting follow the contract. File:
  `internal/notification/slack_test.go`
- [x] T021 [P] [US1] Write tests for opt-in resolution: `--notify` wins in
  both directions; `FINFOCUS_NOTIFY` fills an unset flag; an invalid
  variable value warns once and counts as false. File:
  `internal/cli/budget_notify_test.go`
- [x] T022 [P] [US1] Write tests for collecting exceeded alerts from a
  legacy `BudgetStatus` and from a `ScopedBudgetResult`. Only `EXCEEDED`
  alerts with destinations are collected (not `APPROACHING` or `OK`), in the
  stable order global, providers sorted, tags in priority order, types
  sorted. Two alerts at 80% and 100% that both crossed yield two deliveries
  per destination. File: `internal/cli/budget_notify_test.go`
- [x] T023 [US1] Write CLI tests that run `cost projected` over
  `examples/plans/aws-simple-plan.json` with a temp `FINFOCUS_HOME` config
  and an injected TLS test client: with `--notify` and a crossed threshold
  one Slack request arrives; under the threshold none arrives; without
  opt-in none arrives and stderr has exactly one hint line; with
  `--output json` stdout is still valid JSON with no `notifications` key or
  URL; two consecutive `--notify` runs each send (no state between runs);
  and `overview --notify --plain` over the same plan sends one request. File: `internal/cli/budget_notify_cmd_test.go`
- [x] T024 [P] [US1] Write `config get`/`config list` tests: a literal
  webhook URL and literal header values are shown as `[REDACTED]`; a value
  that is exactly `${NAME}` is shown as written; `config set` on another
  key followed by a reload keeps the original URL in the file. File:
  `internal/cli/config_mask_test.go`

### Implementation (US1)

- [x] T025 [US1] Implement the Slack `Sender` (body per the contract;
  register for `config.NotificationTypeSlack`). File:
  `internal/notification/slack.go`
- [x] T026 [US1] Add the `--notify` persistent flag to `cost` (next to
  `--exit-on-threshold`) and a local `--notify` to `overview`, with help
  text naming `FINFOCUS_NOTIFY`. Files: `internal/cli/root.go`,
  `internal/cli/overview.go`
- [x] T027 [US1] Implement `budget_notify.go`: resolve opt-in, collect
  exceeded alerts into `notification.Delivery` values, print the hint when
  not opted in, call `Dispatcher.Deliver` when opted in, and allow the HTTP
  client to be replaced in tests through an unexported package variable set
  only from `export_test.go`. File: `internal/cli/budget_notify.go`
- [x] T028 [US1] Call the notification step from
  `evaluateBudgetStatusWithRender` after the budget result is computed and
  before `checkBudgetExitFromResult`, skipping it on an evaluation error,
  and never altering the returned error. File:
  `internal/cli/common_execution.go`
- [x] T029 [US1] Mask `url` and header values under `notifications` in the
  `config get` and `config list` display path only (`Config.Save` must keep
  writing real values). Files: `internal/config/mask.go`,
  `internal/cli/config_get.go`, `internal/cli/config_list.go`

**Checkpoint**: US1 tests pass; a dry manual run against a TLS mock shows
one Slack request.

---

## Phase 4: User Story 3 - Failures never break the cost check (P1)

**Goal**: delivery problems only produce secret-free stderr warnings; output
and exit code are identical to a run without notifications.

**Independent test**: destinations returning 500, 302, hanging past the
timeout, or using an unset variable leave stdout and the exit code
unchanged and print one warning each with no secret.

### Tests first (US3)

- [x] T030 [P] [US3] Write dispatcher failure tests: HTTP 500 → `failed`
  with "status 500"; 302 → `failed` and the redirect target is never
  requested; a server that blocks past a 100 ms `Options.Timeout` →
  `failed` with a timeout reason within the limit; a reason never contains
  the URL, a header value, or the response body. File:
  `internal/notification/dispatch_failure_test.go`
- [x] T031 [US3] Write the exfiltration CLI test (SC-006): with
  `t.Setenv` for `AWS_SECRET_ACCESS_KEY`, `GITHUB_TOKEN`, and
  `FINFOCUS_NOTIFY_SLACK_URL`, a project config (via `--project-dir`)
  whose destinations reference each variable in the URL and a header, and
  `--notify`, the run fails pre-run validation, the TLS mock receives no
  request, and stderr contains none of those values. File:
  `internal/cli/budget_notify_cmd_test.go`. The send-time guard (no
  validation) is covered at dispatcher level in T009 with a recording
  lookup that must never see those variable names.
- [x] T032 [US3] Write CLI tests comparing stdout and exit code of a run
  with failing destinations against a run without notifications, including
  `--exit-on-threshold` with an exceeded budget (exit code still the budget
  exit code). Assert the warning format from `contracts/config.md` and that
  no secret appears in stderr. File:
  `internal/cli/budget_notify_cmd_test.go`
- [x] T033 [P] [US3] Write dry-run tests: `--notify --dry-run` makes no
  request and prints one `dry-run: would notify ...` line per delivery with
  no URL. File: `internal/cli/budget_notify_cmd_test.go`

### Implementation (US3)

- [x] T034 [US3] Print one warning per `failed` or `skipped` result and one
  dry-run line per `dry-run` result to stderr in the contract format, and
  log the redacted reason at WARN with zerolog. File:
  `internal/cli/budget_notify.go`
- [x] T035 [US3] Pass `ax.DryRunFromContext` into `Options.DryRun`, and
  confirm the timeout default is 10s and the parent context comes from the
  command. File: `internal/cli/budget_notify.go`

**Checkpoint**: US1 and US3 tests pass together.

---

## Phase 5: User Story 2 - Structured webhook event (P2)

**Goal**: a `webhook` destination receives the `budget.threshold.exceeded`
event with configured method and headers.

**Independent test**: a TLS mock receives a body that validates against
`contracts/webhook-event.schema.json` and the `Authorization` header from
`${FINFOCUS_NOTIFY_API_TOKEN}`.

### Tests first (US2)

- [x] T036 [P] [US2] Write webhook sender tests: the body has exactly the
  schema's fields and values for actual and forecasted alerts and for each
  scope; `budget.name` is `global` or `<scope>:<scope_key>`; the default
  method is POST and PUT is honored; headers are sent with expanded
  values; a configured `Content-Type` is not overridden. File:
  `internal/notification/webhook_test.go`
- [x] T037 [US2] Write a CLI test for a webhook destination with a header
  from `t.Setenv("FINFOCUS_NOTIFY_API_TOKEN", ...)`, asserting the received header
  and that the token never appears in stderr. A second case references
  `${API_TOKEN}` and asserts pre-run validation fails naming the variable. File:
  `internal/cli/budget_notify_cmd_test.go`

### Implementation (US2)

- [x] T038 [US2] Implement the webhook `Sender` and register it for
  `config.NotificationTypeWebhook`. File:
  `internal/notification/webhook.go`

**Checkpoint**: all three stories pass.

---

## Phase 6: Polish and cross-cutting

- [x] T039 Write the binary-level integration test
  `TestBudgetNotifications`: build-time `finfocus`, a TLS mock whose
  certificate is written to a temp file passed as `SSL_CERT_FILE`
  (skip unless `runtime.GOOS == "linux"`), and checks for one Slack and
  one webhook delivery, no delivery without opt-in, an unchanged exit code
  when the mock returns 500, and a project config referencing
  `${GITHUB_TOKEN}` that is rejected and never reaches the mock. File:
  `test/integration/budget_notifications_test.go`
- [x] T040 [P] Document notifications in the budgets guide: config shape,
  `--notify` and `FINFOCUS_NOTIFY`, the CI recipe, dry-run, failure
  behavior, the HTTPS and no-redirect rules, secrets via
  `${FINFOCUS_NOTIFY_*}` in the global config only, a "Security: project
  config and pull requests" section (why project configs cannot use
  variables, the literal-destination residual risk, and not setting
  `FINFOCUS_NOTIFY` on untrusted pull-request runs), and the Slack
  `channel` caveat. File:
  `docs/src/content/docs/guides/budgets.md`
- [x] T041 [P] Add the `notifications` fields to the config reference and
  the JSON schema. Files:
  `docs/src/content/docs/reference/config-reference.md`,
  `docs/src/content/docs/schemas/config-schema.json`
- [x] T042 [P] Update the README budget section with a short notifications
  example linking to the guide. File: `README.md`
- [x] T043 Update CLAUDE.md: replace "There is no notifications section"
  under `config validate` with the notification validation rules, and add
  an engine/CLI gotcha bullet covering the hook point, opt-in, `json:"-"`
  on `ThresholdStatus.Notifications`, display-only masking, the
  `FINFOCUS_NOTIFY_` prefix rule, and project-sourced destinations never
  expanding variables. File:
  `CLAUDE.md`
- [x] T044 Run `make validate`, `make test`, `make test-race`, `make lint`,
  `make docs-lint`, and `go test -run TestBudgetNotifications
  ./test/integration/...`; check `go test -race -shuffle=on -count=3
  ./internal/notification/... ./internal/cli/...`; and confirm coverage of
  `internal/notification` is at least 95%
- [x] T045 Mark the spec status as Implemented and tick every completed
  task. Files: `specs/625-budget-alert-notifications/spec.md`,
  `specs/625-budget-alert-notifications/tasks.md`

---

## Dependencies & Execution Order

- Phase 1 → Phase 2 → stories. Phase 2 blocks everything.
- US1 (Phase 3) is the MVP and adds the CLI wiring that US3 and US2 reuse.
- US3 (Phase 4) depends on US1's CLI wiring (T027, T028).
- US2 (Phase 5) depends only on Phase 2 for its sender (T038), and on
  US1's wiring for its CLI test (T037).
- Phase 6 follows the stories it documents.

Within a phase, test tasks come first and must fail before the matching
implementation task.

## Parallel Opportunities

- Phase 2 tests T002–T009 touch different files and can be written
  together. Implementations T010, T015, T016, T017 are independent.
- In US1, T020, T021, T022, and T024 can be written in parallel.
- T036 (webhook tests) can start as soon as Phase 2 is done, in parallel
  with US1.
- Docs T040–T042 are independent of each other.

## Parallel Example: User Story 1

```text
T020 slack_test.go  |  T021/T022 budget_notify_test.go  |  T024 config_mask_test.go
→ T025 slack.go, T026 flags, T027 budget_notify.go, T028 hook, T029 masking
→ T023 command tests green
```

## Implementation Strategy

1. MVP: Phases 1–3 (Slack, opt-in, masking) and Phase 4 (failure safety),
   because both P1 stories are needed before anything ships.
2. Add Phase 5 (generic webhook).
3. Finish Phase 6 (integration test, docs, gates) before the PR.

## Notes

- Never write to stdout from the notification path.
- Never print a resolved URL, header value, or response body.
- Never call the environment lookup for a variable outside
  `FINFOCUS_NOTIFY_*`, or for any variable of a project-sourced destination.
- Email and PagerDuty are out of scope (follow-up issues).
