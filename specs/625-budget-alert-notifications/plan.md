# Implementation Plan: Budget Alert Notifications

**Branch**: `625-budget-alert-notifications` | **Date**: 2026-10-04 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `specs/625-budget-alert-notifications/spec.md`

## Summary

Budget alert thresholds (#217) gain optional notification destinations of
type `slack` or `webhook`. When a budget-evaluating command (`cost
projected`, `cost actual`, `overview`) runs with `--notify` or
`FINFOCUS_NOTIFY=true` and a threshold is exceeded, a new
`internal/notification` package sends one HTTPS request per destination,
concurrently, each bounded to 10 seconds. Failures only warn on stderr;
output and exit codes are unchanged. Secrets come from `${NAME}` references,
are redacted from every error, and are masked in `config get`/`config list`.
No state is kept between runs.

## Technical Context

**Language/Version**: Go 1.27.1 (see `go.mod`)
**Primary Dependencies**: stdlib `net/http`, `encoding/json`; existing Cobra,
ax-go (`ax.DryRunFromContext`), zerolog, testify. No new modules.
**Storage**: N/A (no persistent state; FR-004)
**Testing**: `go test` with testify; `httptest.NewTLSServer` for HTTPS
destinations; integration tests on the built binary with `SSL_CERT_FILE`
**Target Platform**: Linux, macOS, Windows CLI
**Project Type**: single Go module (CLI)
**Performance Goals**: an unresponsive destination adds at most 10 seconds
per run (SC-003); destinations are sent concurrently
**Constraints**: HTTPS only, no redirects, no secrets in output, nothing on
stdout, exit code unchanged
**Scale/Scope**: a handful of destinations per run; 2 destination types

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] **Plugin-First Architecture**: Principle I governs cost data sources.
  Notifications are output delivery from the orchestration layer, not a cost
  data source, and use no provider pricing. Slack and webhook are generic
  HTTPS sinks with no cloud-provider knowledge.
- [x] **Test-Driven Development**: tests are written first per task (see
  tasks.md); new package targeted at 95% (it sits on the CLI budget path).
  No TUI changes.
- [x] **Cross-Platform Compatibility**: stdlib only. The `SSL_CERT_FILE`
  integration tests skip outside Linux; unit tests run everywhere.
- [x] **Documentation Integrity**: `guides/budgets.md`,
  `reference/config-reference.md`, `schemas/config-schema.json`, README
  budget section, CLAUDE.md (removes "There is no notifications section"),
  and godoc on all exported symbols are updated in the same PR.
- [x] **Protocol Stability**: no proto changes.
- [x] **Implementation Completeness**: no stubs. Email and PagerDuty are
  tracked as separate issues, not placeholders.
- [x] **Persistence Model**: no new store.
- [x] **Quality Gates**: `make validate`, `make test`, `make test-race`,
  `make lint`, `make docs-lint` before the PR.
- [x] **Multi-Repo Coordination**: none needed.

**Violations Requiring Justification**: none.

Post-design re-check: the design keeps the hook in the CLI layer
(`internal/cli`), the engine only copies configuration onto
`ThresholdStatus` with `json:"-"`, and no persistent state is introduced.
The gate still passes.

## Project Structure

### Documentation (this feature)

```text
specs/625-budget-alert-notifications/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── config.md
│   ├── slack-message.md
│   └── webhook-event.schema.json
├── checklists/requirements.md
└── tasks.md            # /speckit.tasks
```

### Source Code (repository root)

```text
internal/config/
├── notification.go          # NotificationDestination, validation, HTTPS rule
├── notification_test.go
├── budget.go                # AlertConfig.Notifications; Validate calls destination validation
├── validate.go              # paths and known keys for notifications
└── config.go / mask.go      # display masking for Get/List

internal/engine/
└── budget_cli.go            # ThresholdStatus.Notifications (json:"-"), filled in evaluateAlerts

internal/notification/        # new package
├── doc.go
├── event.go                 # BudgetAlertEvent, webhook body, Slack body
├── expand.go                # ${NAME} expansion
├── redact.go                # secret redaction of error text
├── sender.go                # HTTP client (no redirects), slack + webhook senders
├── dispatch.go              # concurrent delivery, per-destination timeout, DeliveryResult
└── *_test.go

internal/cli/
├── root.go                  # --notify persistent flag on cost
├── overview.go              # --notify local flag
├── budget_notify.go         # opt-in resolution, collect exceeded alerts, dry-run, hint, warnings
├── common_execution.go      # call budget_notify from evaluateBudgetStatusWithRender
├── config_get.go / config_list.go  # masked display
└── *_test.go

test/integration/
└── budget_notifications_test.go

docs/src/content/docs/guides/budgets.md
docs/src/content/docs/reference/config-reference.md
docs/src/content/docs/schemas/config-schema.json
```

**Structure Decision**: one new internal package for delivery
(`internal/notification`), configuration types in `internal/config` next to
`AlertConfig`, and CLI wiring at the single budget evaluation call site
(research R1).

## Key Design Points

1. Hook point: `evaluateBudgetStatusWithRender`, before
   `checkBudgetExitFromResult`, never altering its return value (R1).
2. Exceeded alerts are collected from `LegacyStatus.Alerts` or every scope in
   `ScopedResult.AllScopes()`, in a stable order (global, providers sorted,
   tags in priority order, types sorted).
3. Masking is display-only because `Config.Save` marshals the struct (R9).
4. `ThresholdStatus.Notifications` is `json:"-"` because scoped results are
   serialized into JSON output (R2).
5. Only `${NAME}` is expanded; unset variables skip the destination (R8).

## Complexity Tracking

No constitution violations to justify.
