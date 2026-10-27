# Quickstart: verify the shared stack loader

**Feature**: `1733-shared-stack-loader` | **Spec**: [spec.md](spec.md)

Each scenario names the user story and success criterion it proves. Run them
against a built binary (`make build`). Pulumi Cloud is the backend; the demo
and `initial-30` stacks use an empty passphrase.

```bash
export PULUMI_BACKEND_URL=https://api.pulumi.com PULUMI_CONFIG_PASSPHRASE=''
```

## 1. One resource set everywhere (US1, SC-001, SC-003)

Use a stack with unchanged resources and one pending change
(`initial-30` `dev`: 65 stack resources, 1 pending update).

```bash
cd <initial-30 checkout>
finfocus overview --stack dev --yes --output json \
  | jq '[.resources[].urn] | length'
finfocus cost recommendations --stack dev --output json \
  | jq '[.recommendations[].resource_id] | unique | length'
finfocus cost estimate --stack dev --modify nomatch:size=1   # exits 2 and lists the loaded resources
finfocus cost projected --stack dev --output json \
  | jq '.finfocus.stack.resources'
```

Expect the overview row count and `finfocus.stack.resources` to agree (61,
where v0.5.2 reported 1 resource for `cost projected`), and the resources the
`cost estimate` error lists to be the same set. `cost recommendations` lists
only resources that have recommendations, and every one of them is in that set.
Then
`finfocus --web --no-browser --port 8791` and compare the Overview, Estimate,
and Recommendations views with the command line (Playwright).

## 2. Totals keep their meaning (US6, SC-007, SC-008)

```bash
finfocus cost projected --stack dev --output json \
  | jq '{plan: .finfocus.summary.totalMonthly, after: .finfocus.diff.totalAfter, stack: .finfocus.stack.totalMonthly}'
```

Expect `plan` and `after` to equal the v0.5.2 value for the same stack and
`stack` to be at least that. Repeat with a preview made by
`pulumi preview --json --show-sames > sames.json` and
`--pulumi-json sames.json`: `plan` equals the v0.5.2 value for that file, and
`stack` equals the first run's `stack` (each resource once).

Set a budget threshold between the two totals and run with `--exit-code`:
the exit code is the v0.5.2 result for the plan total.

## 3. An unchanged stack is explained (US2, SC-002)

Use a stack with no pending changes, or the committed fixture
`internal/cli/testdata/pulumi/unchanged-stack-preview.json`.

```bash
finfocus cost projected --pulumi-json internal/cli/testdata/pulumi/unchanged-stack-preview.json --output json
finfocus cost projected --stack <unchanged stack>
```

Expect `finfocus.emptyPlan: true` in JSON, the stderr warning, and for the
table form on a complete stack the sentence `No changes. Projected monthly
cost is zero.` with the stack total line.

## 4. A saved plan file says what it omits (US4)

```bash
pulumi preview --json --stack dev > default.json
finfocus cost projected --pulumi-json default.json --output json | jq '.finfocus.stack'
```

Expect a stderr notice naming the omitted count (59 on `initial-30` `dev` before
the one change), and no notice for a file made with `--show-sames`.

## 5. State that cannot be read (US5, SC-004)

Run with the backend unreachable (for example `PULUMI_BACKEND_URL` set to a
closed local port) and a preview file:

```bash
finfocus cost projected --pulumi-json default.json --output json | jq '.finfocus.stack'
```

Expect exit 0, `complete: false`, `incompleteReason`, and the stderr notice. No
backend name, URL, or raw error appears.

## 6. Billing-period history survives (SC-009)

On a stack where a resource was deleted earlier in the month and the history
store holds it, `finfocus cost actual --from <first of month>` reports that
resource's charges, identical to v0.5.2.

## 7. Timing (SC-005)

```bash
time finfocus cost projected --stack dev --output json >/dev/null
```

Measured baseline on `initial-30` `dev`: `pulumi preview` 11.5 to 13.6 s,
`pulumi stack export` about 0.8 s, both in parallel 10.2 s. Expect the command
within 3 seconds of v0.5.2.

## Automated

```bash
make test           # includes the stackload parity and compatibility tests
make lint
make test-frontend  # web handlers and views
```
