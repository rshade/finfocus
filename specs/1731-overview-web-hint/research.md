# Research: Overview Web Hint

All unknowns from the spec's Assumptions are resolved below. Each entry cites
the code it was read from.

## R1: Is `w` free in the overview?

- **Decision**: Bind `w` in `handleListKeypress` (`internal/tui/overview_model.go`).
- **Rationale**: The list handler binds `q`, `ctrl+c`, `enter`, `e`, left/right,
  `/`, `s`, `p`, `esc`, `pgup`, `pgdown`. `w` is unbound there. Keys it does
  not match fall through to `bubbles/table`, whose default keymap uses
  `j k b f u d g G` and arrows, not `w`. The filter input
  (`handleFilterInput`) and passphrase prompt (`handlePassphraseInput`) take
  every key before the list handler runs, so `w` is already typed text there
  (FR-009 holds with no extra guard). The detail view has its own handler
  (`handleDetailUpdate`), which does not get the binding.
- **Alternatives considered**: `W` or `ctrl+w` (rejected: `ctrl+w` is
  delete-word in many terminals and Windows consoles); `o` for "open"
  (rejected: reads as opening a resource).

## R2: How does the hand-off reach `runWebSession`?

- **Decision**: The model records a request and quits with `tea.Quit`;
  `runInteractiveOverviewWithInit` (`internal/cli/overview.go`) reads it from
  the final model, runs its normal teardown, then calls `runWebSession`.
- **Rationale**:
  - `p.Run()` returns the final model. The function already inspects it for
    `Err()`, so a `WebRequested()` accessor follows the same pattern.
  - Teardown already runs between `p.Run()` and the return:
    `enrichCancel()`, then `finishInteractiveOverview` (bounded wait on the
    enrichment goroutine, then the plugin cleanup). Calling the web session
    after that point means the overview's plugins are closed before the web
    session opens its own, which is FR-005 with no new code path. It also
    avoids two plugin sets running at once.
  - `runWebSession` already owns the server, the browser open, the printed
    URL, and `ServeUntilInterrupt` (SIGINT/SIGTERM, plugin group kill). Reusing
    it gives FR-003 and FR-004 for free.
  - The terminal is restored by Bubble Tea when `p.Run()` returns, before the
    web URL prints.
- **Alternatives considered**:
  - Start the web server inside the TUI process while it runs (rejected:
    two pipelines, two plugin sets, and signal handling owned by Bubble Tea).
  - `syscall.Exec` into `finfocus --web` (rejected: not portable to Windows,
    loses the audit context).
  - Run `runWebSession` from a `tea.Cmd` (rejected: it blocks until
    interrupt).

## R3: What values does the hand-off pass?

- **Decision**: Build a `webFlagValues` from the overview's `overviewParams`:
  `pulumiJSON`, `pulumiState`, `stack`, `from`, `to`, `adapter`, `filter`;
  `port` 0 and `noBrowser` false.
- **Rationale**: `webFlagValues` and `overviewParams` carry the same seven
  source fields (`runWebSession` builds `overviewParams` from them), so
  FR-010 is a field copy. `config.New()` is what `runWebSession` already uses;
  the project overlay is process-wide state set in root's pre-run, which has
  already run for this process.
- **Alternatives considered**: Re-parse flags from `cmd` (rejected: the
  delegated path calls `overviewCmd.RunE` without flag parsing, so the values
  live in params).

## R4: Where does the hint render, and how does it stay on one line?

- **Decision**: A new line under the status bar in `renderListView`
  (`internal/tui/overview_view.go`), rendered by a helper that picks a long
  form (`[w] web dashboard · finfocus --web`), a short form (`[w] web`), or
  nothing, by measured width.
- **Rationale**: `renderStatusBar` is a single `SubtleStyle.Render` string with
  no width handling; at 80 columns with a stack prefix it already approaches
  the limit, so appending to it would wrap. A separate line keeps the existing
  golden lines intact and only adds one. Width comes from `m.width`
  (`tea.WindowSizeMsg`); `lipgloss.Width` measures it.
- **Alternatives considered**: Append to the status bar (rejected: wraps);
  show only in the help overlay (rejected: no help overlay exists and the
  goal is discovery).

## R5: Which view states show the hint and bind the key?

- **Decision**: Only the list view (`renderListView` / `handleListKeypress`).
  Not the initializing phase checklist, the passphrase prompt, the error
  view, or the detail view.
- **Rationale**: The list view is where the person lands and where keys act.
  It is also visible while rows are still enriching (`loadedCount <
  totalCount`), which covers the spec's "still loading" case: cancel goes
  through the same `enrichCancel()` and `finishInteractiveOverview`. The
  initializing view has no table and no way to act, and the error view is
  terminal.
- **Spec impact**: Story 2 scenario 3 and FR-005 say "while overview data is
  still loading". Tighten them to "while rows are still loading in the list
  view" so the requirement matches a state the key exists in.

## R6: Non-interactive safety

- **Decision**: No new gate.
- **Rationale**: The TUI is only constructed when `overviewInteractive`
  returns true (`DetectResolvedOutputMode` equals interactive). The hint and
  key live inside that model, so every plain, JSON, piped, `AGENT_MODE`, and
  MCP run never builds them (FR-006, FR-007). The plain path
  (`loadPlainOverviewData` onward) is untouched, which the existing
  plain-output tests pin.

## R7: Testing seam for the hand-off

- **Decision**: Test the model by sending a `tea.KeyPressMsg` for `w` and
  asserting the quit command and `WebRequested()`; test the CLI hand-off with
  a package-level function variable for the web start, following the
  `webPipelineHook` precedent in `web.go`.
- **Rationale**: Per the constitution, mocks are for external systems; the web
  session is a long-running server, so the seam replaces only its start. The
  real TUI runs in a golden render test and a pseudo-terminal check recorded
  in `quickstart.md`.
- **Alternatives considered**: A full pty integration test in CI (rejected for
  now: no pty harness exists in `test/integration/`; revisit if one lands).

## R8: Spec Kit script limitation

- **Finding**: `.specify/scripts/bash/common.sh` validates branches with
  `^[0-9]{3}-`, so it rejects `1731-...` and `setup-plan.sh` aborts. It also
  numbered this feature 1730, the number of the ESC feature that exists only
  on its own branch. Plan files here were created by hand.
- **Action**: Out of scope for this feature. Worth a separate issue.
