# Research: Cost Anomalies Command

## Decision 1: Reuse the overview pipeline, not the overview command

- **Decision**: Build `newOverviewPipeline` with `plainOverviewLoader`, no
  `ConfirmEnrichment`, `wantBudget` false, and read `OnReady`'s
  `OverviewPipelineResult.Rows`.
- **Rationale**: The pipeline owns plugin startup, merge, filter, enrichment,
  and `enrichCostDrift`. The issue forbids shelling out to the overview
  command. A nil `ConfirmEnrichment` skips the table pre-flight summary line,
  which would otherwise pollute stdout.
- **Alternatives**: Calling `executeOverview` (prints its own output and
  evaluates budgets); a new enrichment path (would duplicate drift rules).

## Decision 2: State-only when no plan file is given

- **Decision**: Set `stateOnly` unless `--pulumi-json` is supplied.
- **Rationale**: Drift compares actual spend with the deployed resources'
  projection. A preview adds creating rows that carry no drift, and the
  plain loader would otherwise prompt for a preview on a terminal.
- **Alternatives**: Run the preview (slower, extra Pulumi call, no extra
  anomalies).

## Decision 3: Filter keys are the overview's

- **Decision**: `--filter` accepts `type`, `status`, `provider`
  (`validateAndApplyOverviewFilters`). The issue's `tag:env=prod` example is
  not supported by the pipeline.
- **Rationale**: Widening overview filters is outside this issue and would
  change overview behavior.
- **Alternatives**: A second filter implementation (divergence risk).

## Decision 4: `--stack` is inherited

- **Decision**: Do not declare `--stack`; the `cost` parent already declares it
  as a persistent flag (also `--profile`, `--budget-scope`). Read it with
  `cmd.Flags().GetString("stack")`.
- **Rationale**: Redeclaring shadows the parent and confuses help output.

## Decision 5: Output flag handling

- **Decision**: Local `--output` defaulting to `""`, resolved with
  `resolveOutputFormatWithConfig`; reject anything but table, json, ndjson
  before loading. Exit 2 through `toValidationError`.
- **Rationale**: Matches `cost actual`; avoids reading the config singleton
  while building flags (CLAUDE.md gotcha). `--format json` and `AGENT_MODE`
  therefore select JSON for MCP calls.

## Decision 6: Ordering and totals

- **Decision**: Sort by absolute deviation descending, ties by URN. Print no
  total.
- **Rationale**: Stable output for tests and agents; the spec only forbids
  cross-currency sums, and omitting totals satisfies it trivially.

## Decision 7: Row source for `considered`

- **Decision**: `considered` is `len(rows)` after filter and expansion, as the
  pipeline returns them.
- **Rationale**: "Rows the overview produced".
