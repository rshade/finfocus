# Contract: Overview Terminal Behavior

This feature has no HTTP surface (the spec's Web UI Fit is Not applicable), so
there is no `http-api.md`. The contract is what the person sees and what the
process does.

## Hint line

- **Where**: Directly below the status bar in the interactive overview list
  view.
- **Text**: `[w] web dashboard · finfocus --web` when it fits;
  `[w] web` when only that fits; no line otherwise.
- **Style**: `SubtleStyle`, like the status bar.
- **Absent from**: the plain table, JSON and NDJSON output, the initializing
  view, the passphrase prompt, the error view, and the detail view.

## Key `w`

| Context | Effect |
| --- | --- |
| List view | Overview closes, then the web session starts. |
| Filter input focused | Typed as text. |
| Passphrase prompt visible | Typed as text. |
| Detail view | No effect. |
| Any non-interactive run | No key is read. |

## Hand-off sequence

1. `w` sets the request and returns `tea.Quit`.
2. `p.Run()` returns; Bubble Tea restores the terminal.
3. The enrichment context is canceled and `finishInteractiveOverview` waits
   (bounded) and closes the overview's plugins.
4. `runWebSession` runs with the overview's source parameters: it prints the
   same URL line, opens the browser, and serves until SIGINT or SIGTERM.
5. On interrupt it shuts down exactly as `finfocus --web` does.

## Exit codes and errors

Identical to `finfocus --web`. A failure to start the server (for example a
pinned port that is taken) returns that error and its exit code. The overview
is not reopened.

## Documentation to update

- `docs/src/content/docs/commands/overview.md`: add `w` to the key table and
  show the hint line.
- Golden files under `internal/tui/testdata/` for the overview list view.
