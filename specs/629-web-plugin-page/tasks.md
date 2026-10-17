# Tasks: Web Plugin Page

**Input**: Design documents from `/specs/629-web-plugin-page/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/http.md

**Tests**: Write the failing test named in the task before the production
change it locks. Use testify. Tests that set environment variables or build
a plugin binary are `//nolint:paralleltest` with the reason.

**Completeness**: No stubs and no TODO comments. Custom registries, signatures,
init, upgrade, conformance, certify, and MCP install tools are not started.

## Phase 1: Foundational policy and delivery

- [x] T001 Add `ProgressEvent` and `InstallOptions.OnEvent` in `internal/registry/progress.go`. Keep every existing `progress func(string)` sentence. Lock the current CLI sentences with a test in `internal/registry/progress_test.go` that installs from the `httptest` fake in `installer_api_test.go` and compares the string list with `OnEvent` phases.
- [x] T002 Add `CatalogOnly`, `RequireChecksumFile`, `AcceptMissingChecksum`, and `AcceptFallbackVersion` to `InstallOptions`. `CatalogOnly` rejects URL and `owner/repo` specifiers and ignores `SkipChecksum`. A missing checksum file returns `ErrChecksumUnavailable` unless accepted. A mismatch still returns `ErrChecksumMismatch`. A no-asset release returns `*NoAssetsError` (`errors.As`) unless `AcceptFallbackVersion` equals the fallback. Tests in `internal/registry/install_policy_test.go`. CLI default fields stay zero so warn-and-continue and the interactive fallback prompt stay.
- [x] T003 Correct the `getGitHubToken` comment in `internal/registry/github.go` so it documents `GITHUB_TOKEN` only. Rate-limit errors stay the existing sentence.
- [x] T004 Add `ResolveDeliveredEnv` in `internal/pluginhost/settings_env.go`. Precedence is parent environment, project file, global file, descriptor default. Descriptor keys use `descriptor.env` only. Keys without a descriptor use `FINFOCUS_PLUGIN_<NAME>_<KEY>` only. Another plugin's keys are absent. Tests cover each precedence step and prove a secret is not formatted into a log line.
- [x] T005 Inject that environment from `ProcessLauncher.startPlugin` via a context value. `NewClient` in `internal/pluginhost/host.go` resolves env from the install directory name, calls `GetPluginInfo`, and restarts the process once when the advertisement requires variables that were not injected. The digest of delivered pairs is available to the engine. Tests use a script plugin and `//nolint:paralleltest` because they exec.
- [x] T006 Append `/penv-<digest>` in `internal/engine/engine.go` for projected and actual cache keys and for `checkPluginSupports` when the digest is non-empty. An empty digest keeps today's key. A test changes one delivered value and expects a different projected key.

## Phase 2: User Story 1 — catalog (P1)

- [x] T007 [US1] Add a catalog merge that lists every `registry.json` entry plus installed directories outside the catalog, with installed versions, security level, and update-available. Unit test the merge with a temp plugin dir and no network (`internal/registry/catalog_test.go` or the callback's package).
- [x] T008 [US1] Add `GET /api/plugins/catalog` through a `SessionOptions` callback wired in `internal/cli/web.go`. `internal/webui` does not import `cli`, `registry`, or `pluginhost`. Handler test covers the merge and an origin rejection on a later mutating route's middleware.

## Phase 3: User Story 2 and 3 — install, update, remove, trust (P1)

- [x] T009 [US2] Implement `POST /api/plugins/{name}/plan`, install, update, and `DELETE`, plus the operation record and `GET /api/plugins/operations/{id}/stream` in `internal/webui/handlers_plugins.go`, reusing `sse.go`. The plan does not download. Confirmation fields that do not match a fresh plan return 409. Busy returns 409. Tests use a fake callback, not the network.
- [x] T010 [US3] Handler tests: non-catalog name, checksum mismatch, missing checksum without the second flag, and fallback without `acceptFallbackFrom` leave the plugin directory unchanged. The callback receives `CatalogOnly` and `RequireChecksumFile`.
- [x] T011 [US2] `internal/webui/static/views/plugins.js`, nav link in `index.html`, `ROUTES` entry in `app.js`. Cards show installed, update-available, outside-catalog, and security level. The confirm dialog names plugin, version, repository, and security level. Progress renders phase, bytes, and warnings. Frontend test `internal/webui/frontendtest/plugins.test.mjs`.

## Phase 4: User Story 4 — validate (P2)

- [x] T012 [US4] `POST /api/plugins/{name}/validate` runs the file checks from `plugin validate` and a launch plus `GetPluginInfo`. Response is pass or fail with a redacted reason. Not-installed is 404. The page does not offer conformance, certify, init, or upgrade. Handler test with a fake callback.

## Phase 5: User Story 5 and 6 — settings and delivery (P1)

- [x] T013 [US5] Settings GET and POST in `handlers_plugins.go`. GET never includes a secret value (assert on the raw body). POST preview and save call `ConfigPreview` and `ConfigSave` and reject keys outside `plugins.<name>.*` and values that disagree with the descriptor. Generic mode treats `IsCredentialName` keys as secrets.
- [x] T014 [US5] The settings form in `plugins.js`: typed controls, enum select, write-only secret replace, generic rows, tier choice, and the next-launch sentence. Frontend test covers the descriptor form, the generic fallback, and the secret replace flow.
- [x] T015 [US6] `plugins/jev`, `plugins/prometheus`, `plugins/kubernetes`, and `plugins/recorder` advertise `WithSettings` for the variables they already read. Floats that have no proto type stay strings. Plugin tests assert the env names and the secret flags.
- [x] T016 [US6] Browser e2e: install the recorder from a stand-in catalog, set `FINFOCUS_RECORDER_MOCK_RESPONSE` through the form, run a query, and assert the recorder received it. An exported parent value wins in the pluginhost test from T005, not only in the browser.

## Phase 6: Docs and spec amendments

- [x] T017 [US1] Update `docs/src/content/docs/guides/plugin-system.md`, the plugin configuration section of `docs/src/content/docs/reference/cli-commands.md`, and `docs/src/content/docs/architecture/web-ui.md`, `reference/web-ui.md`, and `guides/web-ui.md`. Amend `specs/627-web-ui/spec.md` and `specs/628-web-config-page/spec.md` so install, update, and remove are allowed local side effects and the per-plugin form is this page. Update the plugin-host and cache-key notes in `CLAUDE.md`.
- [x] T018 Run `make test-frontend`, the focused Go tests, `make check-plugin-boundaries`, `make validate`, `make test`, `make test-race`, `make lint`, and `make docs-lint`. Fix failures before calling the issue done.

## Dependencies

T001–T006 block the page. T007–T008 are the catalog slice. T009–T011 depend on T002 and T008. T013 depends on the config callbacks that already exist. T015 can proceed beside T004. T016 depends on T013 and T015. T017 follows the behavior it describes.
