# Tasks: Daily Actual-Cost Series

**Input**: Design documents from `/specs/1924-daily-actuals/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Constitution Principle II requires tests before implementation.
Store and mapper tests land with the store. Command tests land with the
command.

**Completeness**: No stubs or TODOs. The store can write, replace, list, and
prune.

## Phase 1: Store

- [x] T001 Add `DailyPoint`, file names, and `PointsForSeries` in
  `internal/history/daily.go`, with tests in `internal/history/daily_test.go`
  for a dense series, a zero day, two currencies, a short series, and an
  empty series.
- [x] T002 Add `DailyDB` in `internal/history/daily_db.go` and tests in
  `internal/history/daily_db_test.go`: replace one day, leave a day outside
  the new write, reject a symlink, refuse a projected timeline, and refuse to
  open the daily file as a timeline. `costSchemaVersion` stays 1.
- [x] T003 Extend `historyDatabaseFilesSafe` in `internal/cli/history_record.go`
  to guard `*.daily.db`, and add those names to `TestHistoryDir_ProjectTierSymlinkedDatabase`.

## Phase 2: User Story 1 — Record each resource's days

- [x] T004 Add `NewCostHistoryCollectDailyCmd` and `collectDailyPoints` in
  `internal/cli/cost_history_collect_daily.go`. Register it from
  `NewCostHistoryCmd`. Default window is the last 30 complete UTC days.
- [x] T005 Test the mapper and the command with a fake pricer in
  `internal/cli/cost_history_collect_daily_test.go`: dense series, summary
  counts, JSON summary, and unknown `--output` in `outputFormatCases`.

## Phase 3: User Story 2 — Correct a day without erasing the rest

- [x] T006 Command test: a second collect replaces one day, leaves another,
  and a plugin error on one resource stores nothing new for it while the
  other resource is stored.

## Phase 4: User Story 3 — Absence is not zero

- [x] T007 Mapper tests: missing `DailyCosts` stores nothing and does not
  split `TotalCost`; a length mismatch is an error; a zero day is stored;
  two currencies are two series.

## Phase 5: User Story 4 — Projected timeline and jail

- [x] T008 Command test seeds `<project>@<stack>.history.db` and asserts its
  snapshots after a daily collect. MCP exclusion covers `collect-daily` and
  still covers `collect`. The command stays out of `tools.golden`.

## Phase 6: Retention and docs

- [x] T009 Apply `cost.history.retention` age prune when `auto_prune` is set
  and `max_age_days` is positive. Do not apply `max_snapshots`.
- [x] T010 Document the command in `docs/src/content/docs/guides/user-guide.md`,
  `docs/src/content/docs/guides/ci-cd-cost-tracking.md`, and
  `docs/src/content/docs/reference/cli-commands.md`. Note the file in
  `CONTEXT.md` and the jail plus alignment rule in `CLAUDE.md`.

## Phase 7: Web parity

Not applicable. No `internal/webui` task. No follow-up issue.
