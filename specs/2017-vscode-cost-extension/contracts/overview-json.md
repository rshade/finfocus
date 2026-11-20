# Contract: JSON Overview Additions

Command: `finfocus overview --output json`. Existing fields retain their
names and values, except for the approved config-budget fallback (FR-006).

## Per resource (`resources[]`)

```json
{
  "urn": "urn:pulumi:dev::shop::aws:ec2/instance:Instance::web",
  "type": "aws:ec2/instance:Instance",
  "status": "updating",
  "projectedCost": { "monthlyCost": 61.32, "currency": "USD" },
  "modifiedAt": "2026-11-16T10:00:00Z",
  "location": { "file": "lib/web.ts", "line": 12, "column": 5 },
  "display": {
    "projected": "$61.32",
    "actual": "$18.40",
    "delta": "+$30.66",
    "status": "~ updating"
  }
}
```

- `location`: omitted when unresolved (FR-003). `column` omitted when the
  runtime records none.
- `modifiedAt`: optional deployed-resource modification time, RFC3339. Copy
  from state through the overview pipeline, not provider properties. Omitted
  for missing timestamps and preview-only creates. The consumer treats an
  absent/invalid timestamp as freshness unknown for a deployed result.
- `display.*`: the same strings the overview table prints for that row
  (`OverviewRowResult.ProjectedDisplay`, `ActualDisplay`, `DeltaDisplay`,
  `StatusDisplay`); `-` where the table prints `-`.
- `display.projected` equal to `-` on a row whose raw `status` is not
  `deleting` is the extension's "unpriced" signal (FR-012b). The placeholder
  is part of this contract and is pinned by a parser test. The JSON does not
  say whether the gap is a type with no cost data or a failed price (in the
  partial run of validation.md Finding 1b, `errors[]` was empty); the
  extension decides by stability across runs. A row whose price failed with
  an explained error prints `ERR` and has an `errors[]` entry; it is not a
  silent gap and is outside this rule.

## Metadata consumed by the extension

Existing keys are `metadata.stackName`, `metadata.generatedAt`, and
`metadata.isStateOnly`. `isStateOnly` may be omitted for preview results;
absence means false. No `metadata.stack` or `metadata.stateOnly` key is added.

## Summary (`summary.display`)

```json
{ "projected": "$1,240.18", "actual": "$611.20", "delta": "+$30.66" }
```

From the same formatters as the table footer.

## Budgets (`budgetDisplay`, top level)

The object `viewmodel.BuildBudgetDisplay` returns, identical to the web's
`GET /api/overview/budget` `display`: `healthDisplay`, `amountsDisplay`,
`footerDisplay`, `details[]`. Present (with empty `details`) when no budget
exists. `budgets[]` now includes config budgets when no plugin returns one
(FR-006).

## Failures

Unchanged ax-go envelope on **stderr** with `--format json` (stdout is
empty on failure): `error_code`, `message`, `version`. The extension reads the
`(passphrase_required)` category token at the end of `message`.

## Pinning (FR-026)

`packages/vscode-finfocus/contract/overview-fields.json` lists every JSON path
above that the extension reads. `internal/cli/overview_json_contract_test.go`
renders a fixture overview and a passphrase failure and fails if any listed
required path is missing. Include state-only and preview fixtures, timestamp
present/absent cases, a Go location with no column, and no-budget/config-budget
cases. Optional paths are asserted when applicable and allowed to be absent
otherwise; pin names and types as well as presence. The extension parser tests
must use the same field list so an unused inventory cannot hide schema drift.
