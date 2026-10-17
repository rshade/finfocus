# Tasks: Web Config Page

**Input**: Design documents from `/specs/628-web-config-page/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/http.md

**Tests**: Tests are written with the behavior they lock, in the same change
as the code. TUI goldens do not apply.

**Completeness**: No stubs or TODOs. Live reload, alert editors, and plugin
install are out of scope and have no placeholder commands.

**Documentation**: Web UI docs, the configuration reference, spec 627, and
`CLAUDE.md` update with the page.

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [x] T001 Confirm branch `628-web-config-page` and spec directory `specs/628-web-config-page/`

## Phase 2: Foundational

- [x] T002 Move the credential-name fragment list to `config.IsCredentialName` and delegate `engine.isCredentialKey` in `internal/engine/flatten.go`
- [x] T003 Add `acquirePathLock` in `internal/config/lock.go` and point `DismissalStore.acquireFileLock` at it
- [x] T004 Add the known-setting descriptor in `internal/config/descriptor.go` and a test that it matches `docs/src/content/docs/schemas/config-schema.json`

## Phase 3: User Story 1 — See the effective configuration (P1)

**Independent test**: Global precision 2, project precision 4, `FINFOCUS_LOG_LEVEL=debug`. The view reports those sources and does not require a browser.

- [x] T005 [US1] Build the effective view in `internal/config/view.go` (tiers, source, project availability, read-only sections, secret status)
- [x] T006 [US1] Test the view in `internal/config/view_test.go`, including an unresolved project directory

## Phase 4: User Story 2 — Edit a chosen tier (P1)

**Independent test**: Patch precision in a commented project file. Comments, unknown keys, and the global file stay. Precision 11 is rejected and the file is unchanged. Preview writes nothing.

- [x] T007 [US2] Implement HuJSON patch, validation, and dry-run diff in `internal/config/patch.go`
- [x] T008 [US2] Test comments, unknown keys, validation, empty preview, and YAML migration in `internal/config/patch_test.go`

## Phase 5: User Story 3 — Secrets stay write-only (P1)

**Independent test**: A stored plugin credential and notification URL are absent from the view JSON and from `config get` / `config list`. `[REDACTED]` does not replace the stored credential.

- [x] T009 [US3] Mask plugin credential values in `MaskedForDisplay` (`internal/config/mask.go`)
- [x] T010 [US3] Skip omitted and `[REDACTED]` secret changes in the patcher, covered by `internal/config/patch_test.go`

## Phase 6: User Story 4 — Conflicts and crashes (P2)

**Independent test**: A stale `baseVersion` leaves the file unchanged. Two overlapping patches both persist. A failed rename leaves the previous file.

- [x] T011 [US4] Lock, re-check the hash, and atomically rename inside `internal/config/patch.go`
- [x] T012 [US4] Test conflict and overlapping saves in `internal/config/patch_test.go`

## Phase 7: User Story 5 — Command line matches the page (P2)

**Independent test**: `config set` with `FINFOCUS_LOG_LEVEL` set does not write that level. A flat budget amount lands on `cost.budgets.global.amount`. A legacy YAML file becomes HuJSON.

- [x] T013 [US5] Point `internal/cli/config_set.go` at the patcher, including the flat budget aliases
- [x] T014 [US5] Test `config set` and masked `config get` / `config list` under `internal/cli/`

## Phase 8: Dashboard page

- [x] T015 Add `ConfigView`, `ConfigPreview`, and `ConfigSave` to `internal/webui/session.go` and register routes in `internal/webui/handlers_config.go` and `internal/webui/server.go`
- [x] T016 Wire callbacks, project resolution, and key-only audit logging in `internal/cli/web.go`
- [x] T017 Test GET redaction, Origin, content type, unknown fields, and log omission in `internal/webui/handlers_config_test.go`
- [x] T018 Add `internal/webui/static/views/config.js`, the route in `app.js`, the nav link in `index.html`, and `internal/webui/frontendtest/config.test.mjs`
- [x] T019 Add a browser case in `test/e2e/web_browser_test.go` that sets project precision and checks the file comments

## Phase 9: Docs and spec 627

- [x] T020 [P] Update `docs/src/content/docs/architecture/web-ui.md`, `reference/web-ui.md`, `guides/web-ui.md`, and the configuration reference
- [x] T021 [P] Amend `specs/627-web-ui/spec.md` FR-019 and the out-of-scope list, and note the page in `CLAUDE.md` and `CONTEXT.md`

## Dependencies

T002 and T003 block the patcher. T004 blocks the view. T005–T012 block the HTTP handlers. T015 blocks the page. T018 can follow the contract without waiting for the browser test.

## Parallel opportunities

T020 and T021 touch different docs and can land after the behavior is settled.

## Implementation strategy

Ship the config package first so the page and `config set` share one writer. The page is the MVP once GET and POST work. Command-line masking and the browser test complete the issue's acceptance list.
