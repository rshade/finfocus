# Tasks: Per-Stack-Safe Web Data Path

Tests first in every phase. Web UI Fit is `Not applicable`: no web-parity
phase and no `web-ui` follow-up. Test files use testify only; tests that
touch process-wide state (`t.Setenv`, root command) are not parallel.

## Phase 1: Setup

- [X] T001 Baseline: record `go vet ./...` and `go test ./internal/cli/ ./internal/pulumi/ ./internal/webui/...` results on the untouched branch in the working notes

## Phase 2: Foundational — stack listing (US4, independent of the rest)

- [X] T002 [P] [US4] Write `internal/pulumi/list_stacks_test.go`: `ListStacks` parses names and the current marker via the stubbed runner; runner failure is wrapped; canceled context is preserved; malformed JSON errors; `ValidateStacks` accepts present names, names every unknown stack and lists the available ones; `GetCurrentStack` keeps its error text and `NoCurrentStackError`
- [X] T003 [US4] Implement `ListStacks` and `ValidateStacks` in `internal/pulumi/pulumi.go` and rewrite `GetCurrentStack` on top of `ListStacks` without changing its messages

## Phase 3: Shared stores (US1)

**Goal**: one cache and one history open per process, injected everywhere.
**Independent test**: two engines and two pipelines in one process both hold
live stores.

- [X] T004 [US1] Write `internal/cli/web_stores_test.go` with a temp `FINFOCUS_HOME`, cache TTL and history enabled: `openSharedStores` returns live stores; two engines built through `newEngineWithStores` both use the same cache instance and a value written through one is read through the other; a pipeline given the stores records history without opening the file (a second `history.NewBoltStore` on the same directory must still time out, proving no extra handle was taken or held); `Close` is idempotent; running the first engine's cleanup leaves the second engine's cache and history working; disabled and failing opens leave nil fields while the other store stays live; a parallel `-race` use of both engines. Also add the regression case that builds two engines through the old `newEngineWithCacheAndHistory` and asserts the second has nil stores (documents why the shared path exists)
- [X] T005 [US1] Create `internal/cli/web_stores.go`: `sharedStores`, `openSharedStores` (reuses `initCacheFromConfig` and `initHistoryFromConfig` with `config.GetResolvedProjectDir()`), `Close` (history then cache, `sync.Once`)
- [X] T006 [US1] In `internal/cli/common_execution.go` split the engine construction out of `newEngineWithCache` into a builder both it and the new `newEngineWithStores` call, so non-web callers keep identical behavior (router, pricing-spec fallback, cache wiring)
- [X] T007 [US1] Add `stores *sharedStores` to `overviewPipelineConfig` in `internal/cli/overview_pipeline.go` and route the three history sites (state snapshot, plan lineage after load, plan lineage after on-demand preview) through one helper that returns the injected store with a no-op cleanup, or opens and closes per call when nil
- [X] T008 [US1] Update `webData` in `internal/cli/web_data.go`: hold `*sharedStores`, make `newEngine` build through `newEngineWithStores` (no `newEngineWithCacheAndHistory`), set `d.cache`/`d.history` from the stores, and let `reloadPlugins` keep reusing `d.cache`; the returned engine cleanup no longer closes stores
- [X] T009 [US1] Wire `runWebSession` in `internal/cli/web.go` to open the stores once, pass them to `newWebData` and the pipeline config, and close them after the pipeline cleanup (after `closeReloaded`, `pipe.Cleanup`)

## Phase 4: Per-stack ESC (US2)

**Goal**: a stack's plugins see that stack's environments only.
**Independent test**: two stacks with different `environment:` lists resolve
their own.

- [X] T010 [US2] Write `internal/cli/esc_stack_test.go` with a temp Pulumi project (`Pulumi.yaml`, `Pulumi.dev.yaml` with six imports, `Pulumi.tailscale.yaml` with five): `stackESCSource` for each stack carries only its own list; an absent settings file or no `environment:` key gives an absent environment and no error; an empty stack returns the root source unchanged; global and per-plugin options match the root's; a malformed settings file for one stack logs at debug and gives an absent environment while another stack still resolves; refreshing one stack's source leaves the other untouched; a root source built from `--stack dev,tailscale` is not used to pick the stack
- [X] T011 [US2] Implement `stackESCSource` in `internal/cli/esc_source.go`: take `root.Options()`, set `Stack` from `pulumidetect.LoadStackContext(projectDir, stack)`, return a new `esc.Source`; on load error log at debug and return an absent-stack source like `loadESCStack` does today
- [X] T012 [US2] In `internal/cli/web_data.go` give `webData` a stack-scoped ESC source: when `params.stack` is non-empty, `newWebData` derives it with `stackESCSource` (project dir from `pulumidetect.FindProject(cwd)`, as `loadESCStack` does; not found keeps the root source) and `noteESC`/`freshESC` use it; with an empty stack keep the context's root source. Make `runWebSession` attach it to `pipeCtx` via `esc.WithSource` so `openPlugins` sees it

## Phase 5: Per-stack passphrase (US3)

**Goal**: each stack's prompt and answer are isolated.
**Independent test**: with two stacks, a submit reaches only its own pipeline
and session.

- [X] T013 [US3] Write `internal/cli/web_stack_test.go`: two `webStack` values own distinct channels, sessions, and contexts, and canceling one leaves the other running. Add a two-session test where each pipeline's `OnPassphraseRequired` calls its own `session.RequirePassphrase` and a POST to one session's `/api/passphrase` unlocks only that pipeline (use the existing web test harness and stub loader)
- [X] T014 [US3] Create `internal/cli/web_stack.go`: `webStack` with `newWebStack`, `start`, and `stop`; each stack creates its own passphrase channel (no relay type, because `webSessionOptions` already scopes submit to the channel and the stack's context) that builds `webData`, the pipeline (own channel, injected stores, stack ESC), the session, `bindWebPipeline`, and `bindPipelineSource`
- [X] T015 [US3] Refactor `runWebSession` in `internal/cli/web.go` to build exactly one `webStack` through `newWebStack`, keep `webSessionOptions` as is, keep startup order (session, listen, then `pipe.Run`), shutdown order, and every existing seam (`startWebSession`, `webPipelineHook`); `bindWebPipeline` uses the stack's own name for `RequirePassphrase`

## Phase 6: Single-stack parity (US5)

- [X] T016 [US5] Run the existing suites unmodified and fix only genuine regressions: `go test ./internal/cli/... ./internal/webui/... ./internal/pulumi/...`; update only assertions that pin the removed per-engine store opens
- [X] T017 [US5] Add `internal/cli/web_single_stack_test.go`: a one-stack `runWebSession` through the existing test seams opens the cache and history exactly once, closes them once after the pipeline cleanup, and a root-level `--stack dev` still resolves the same ESC environments as before
- [X] T018 [US5] Run `go test -race -shuffle=on -count=3` on `internal/cli`, `internal/pulumi`, and `internal/webui`; fix order-dependence (set `FINFOCUS_HOME` to a temp dir in every new test)

## Phase 7: Polish and verification

- [X] T019 Docs: add a "per-stack runtime" note (shared stores, per-stack ESC and passphrase) to `docs/src/content/docs/architecture/web-ui.md`; add the new invariants to `CLAUDE.md` (Plugin Host / CLI sections): stores are opened once and injected, `stackESCSource` empty-stack rule, `webStack` owns the passphrase channel
- [X] T020 `make validate`, `make test`, `make lint`, `make docs-lint`; `markdownlint` the changed Markdown outside `docs/`
- [X] T021 Real-binary check: `make build`, run `finfocus --web --no-browser` from `../finfocus-demo` (empty passphrase, debug logging), confirm one history init, no "proceeding without" warning, Overview loads; render with Playwright and compare to a pre-change run; stop the server with an anchored `pkill`
- [X] T022 Code review against the recorded base commit (`df5d5f64`); fix substantiated findings; report scout opportunities without expanding scope
- [X] T023 Prepare the handoff: file list, Conventional Commit message (`refactor(web): make stores, ESC and passphrase per-stack-safe`, `Closes #2038`), PR title and body (no follow-up issue: Web UI Fit is `Not applicable`)

## Dependencies

- T001 → everything. T002 → T003 (US4 is independent of US1–US3).
- US1: T004 → T005 → T006 → T007 → T008 → T009.
- US2: T010 → T011 → T012; T012 needs T008 (shared `webData` edits) and
  precedes T015.
- US3: T013 → T014 → T015; T014 needs T005 and T011/T012.
- US5 (T016–T018) after T015. Polish after US5.

## Parallel opportunities

- T002 (pulumi) runs beside T004 and T010 (different files and packages).
- T005/T006 touch different files and may run together once T004 is written.
- Tests T004, T010, T013 are written before their implementations.

## Implementation strategy

MVP is US1 (shared stores): it removes the silent cache and history loss.
Then US2 and US3 (both P1) and US4 (P2, independent). US5 is the safety net
and gates the PR. Ship as one PR; #2039 builds on `newWebStack`.
