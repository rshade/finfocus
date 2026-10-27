# Validation: Shared Stack Loader

**Feature**: `1733-shared-stack-loader` | **Quickstart**: [quickstart.md](quickstart.md)

Run on 2026-10-26 against `initial-30` `dev` (Pulumi Cloud, empty
passphrase). "Before" is `main` at the merge of #1907, without this feature;
"after" is this branch. Both binaries used the same installed plugins
(aws-public v0.2.2; scorer installed, scoring off).

## Environment limit

`pulumi preview` on `initial-30` `dev` fails inside the stack's own program:
`tailscale:index/getDevice` returns `API token invalid (401)`. Both binaries
fail the same way on every `--stack dev` run. `pulumi stack export` does not run
the program and works. The preview still writes its partial JSON, which
reports `changeSummary` `{same: 57, update: 1}`; the `--show-sames` file lists
58 steps. The scenarios therefore use those saved previews, read inside the
project so the state is loaded, and the live `--stack` runs are recorded only
as failing the same way before and after.

## Results

| Scenario | Result |
| --- | --- |
| 1. One resource set everywhere | Pass with saved previews. `cost estimate` lists 62 resources and the web Estimate picker ("Show all") offers the same 62; web Overview shows 61 rows, the same set without the root stack. `cost recommendations` and the web Recommendations view return the same 6 resources. Before: the web Estimate picker offered 2 and Recommendations 0. |
| 2. Totals keep their meaning | Pass. The plan total is identical before and after: `$0.00` for the default preview, `$141.03` for `--show-sames`. The stack total is `$141.03` for both previews. Resource counts differ (54 and 57) because the plan maps 3 component resources (`TailscaleDeviceNode`) that state mapping skips; they price at `$0`. The budget exit-code case is covered by `TestProjectedBudgetUsesPlanTotalWithCompleteStack`, not run live. |
| 3. An unchanged stack is explained | Pass. `unchanged-stack-preview.json` gives `emptyPlan: true` and the v0.5.2 warning when no complete stack total is available. |
| 4. A saved plan file says what it omits | Pass. Outside a project, the default preview prints `Note: the preview omits 56 unchanged resources, ...` and `stack.incompleteReason: not_requested`, `omittedUnchanged: 56`; the `--show-sames` file prints no note. |
| 5. State that cannot be read | Pass. With `PULUMI_BACKEND_URL=http://127.0.0.1:9`, exit 0, `complete: false`, `incompleteReason: backend_unreachable`, and `Warning: stack state unavailable (backend_unreachable): 56 unchanged resources are not included in the stack total.` No URL or backend text appears. |
| 6. Billing-period history survives | Covered by `TestActualDeployedInventoryKeepsDeletedHistory`; no deleted resource in this stack's history to run live. `cost actual --stack dev` exits 0 before and after. |
| 7. Timing | See below. |

## Timing (SC-005)

| Run | Before | After |
| --- | --- | --- |
| `cost projected --stack dev` (fails in the program, after the full preview) | 12.33 s | 11.16 s |
| `cost projected --pulumi-json default.json` inside the project (median of 8 after, 3 before) | 1.3 s | 2.9 s |
| `pulumi stack export --stack dev` alone | | 0.5 to 0.8 s |

With a live preview, the state export runs beside an 11 to 13 s preview and
adds no measurable time. With a saved preview inside a project, the export
plus pricing of the 53 unchanged resources adds about 1.6 s at the median,
within the 3 s budget; one of eight runs took 6.5 s.

## Found during validation and fixed

- The `--show-sames` preview printed `the preview does not match the stack
  state for 3 resources`: the three were component resources, present in the
  state but not custom. The snapshot now records component URNs, and mismatch
  detection treats them as present (T073).
- `cost recommendations` with no input flag in a Pulumi project exited 2
  (`--pulumi-json is required`), although the loader and the docs supported
  it. It now loads the project's live inventory, as `cost projected` and
  `cost estimate` do (T072).

## Automated gates

`make test`, `make lint`, `make validate`, `make docs-lint`, and
`make test-frontend` (97 tests) pass. `go test -race -shuffle=on -count=3`
over `internal/stackload`, `internal/engine`, and `internal/cli` passes.
Coverage: `internal/stackload` 99.6%; changed statements in
`internal/engine/diff*.go` 99.0%; changed statements elsewhere 90.0%.
