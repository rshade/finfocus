# Research: Cost History on the Web

## Where the payload types live

- **Decision**: Move the CLI's `costHistoryView` and `costDiffJSON` (with
  `newCostDiffJSON`) into `internal/history` as `ViewDocument`,
  `DiffDocument`, and `NewDiffDocument`. The CLI and the web both serialize
  those types.
- **Rationale**: `internal/webui` must not import `internal/cli`. `RedactJSON`
  needs the Go value, not decoded JSON, to see `datakey` struct tags, so the
  web cannot receive pre-marshaled bytes. `internal/history` already owns the
  snapshot and diff types, imports neither `engine` nor `ingest`, and both
  packages already import it.
- **Alternatives considered**: Defining the types in `internal/webui` would
  make `cost history view` depend on the web package. Returning
  `json.RawMessage` would skip the data-keyed walk and drop breakdown keys
  such as `aws:secretsmanager/secret:Secret`.

## Data-keyed breakdown maps

- **Decision**: Tag `CostSnapshot.ByProvider`, `CostSnapshot.ByType`, and
  `CostAnnotation.ResourceChanges` with `datakey:"true"`.
- **Rationale**: Their keys are provider names, resource type tokens, and
  change kinds. `IsHiddenPropertyKey` would drop a type token that contains a
  credential word. The tag is the repository's existing mark (CLAUDE.md,
  `RedactJSON`). Values still pass `history.IsPulumiSecret`.
- **Alternatives considered**: `engine.DataKeyed` would make `history` import
  `engine`, which is forbidden.

## Read-only open for the web

- **Decision**: The web opens the database with `history.OpenCostDBRead` after
  a stat check, then checks the stored stack name with `diffStackMatches`.
  The CLI keeps `openHistoryTargetRead` / `openHistoryTargetReadExisting`.
- **Rationale**: For a Terraform target the CLI's opener uses
  `OpenCostDBFor` and `SetProjectLabel`, which write. The dashboard is
  read-only toward the history store (FR-012). The label does not change any
  snapshot, annotation, or diff value, so the values stay identical.
- **Alternatives considered**: Reusing the CLI opener would write to the
  store from a GET request.

## Shared, cmd-free query functions

- **Decision**: Extract `historyViewBounds(from, to string)`,
  `selectViewCurrency(snapshots, annotations, choice)`, `checkDiffInputs`, and
  the read functions `readHistoryView(db, stack, query)` and
  `readHistoryDiff(db, stack, query)`. `viewBounds`, `applyViewCurrency`,
  `readDiffFlags`, `runView`, and `runDiff` call them.
- **Rationale**: The issue requires one implementation. The CLI keeps its
  flag reading, `requireKnownProvider`, and chart rendering, which shape only
  terminal output.

## Error classes

- **Decision**: `internal/history` gains `ErrNoCostHistory` (missing
  database), `ErrNoSnapshots` (returned by `ResolveCostSnapshot` for an empty
  set; message unchanged), and `QueryError` (wraps a caller mistake, message
  unchanged). The shared read functions wrap date, selector, threshold, and
  currency failures in `QueryError`. Open, stats, and read failures stay plain.
- **Rationale**: The handler maps `QueryError` to 400 with the message,
  `ErrNoCostHistory`/`ErrNoSnapshots` to a 200 `no_history` status, and the
  rest to a fixed 502. CLI messages and exit codes do not change, because
  wrapping keeps `Error()` and the CLI's exit mapping only checks ax errors.
- **Alternatives considered**: Matching error strings in the handler is
  fragile. Error types in `internal/webui` would force the CLI read path to
  import the web package.

## Resource series

- **Decision**: The web calls `loadDetailHistory(ctx, "", stack, id)`, the
  function behind `overviewHistoryLoader` and `costActualHistoryLoader`, and
  maps `detail.ErrHistorySkipped` to `history.ErrNoCostHistory`.
- **Rationale**: FR-004 asks for the TUI's points. That function already reads
  with `OpenCostDBRead` and never writes.

## Session stack and budget

- **Decision**: The stack is `webData.forecastStack()` (the loaded snapshot's
  stack, else `--stack`), read under the query gate and released before any
  file I/O. The budget is `globalBudget(d.params.cfg)`, the function the CLI
  chart uses.
- **Rationale**: The overview TUI passes the same stack name to its detail
  loader, and the forecast hook already reads history for it.

## Input syntax for non-flag values

- **Decision**: `strict` uses `strconv.ParseBool` and `threshold` uses
  `strconv.ParseFloat`. A parse failure is a `QueryError` that names the
  parameter. Every other message is the CLI's.
- **Rationale**: pflag's own parse message names a `--flag` that a query
  string does not have; once parsed, the CLI's checks run unchanged.
