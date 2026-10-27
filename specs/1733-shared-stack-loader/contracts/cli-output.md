# Contract: command output

**Feature**: `1733-shared-stack-loader` | **Spec**: [../spec.md](../spec.md)

Everything here is additive. A consumer that ignores the new keys sees the
v0.5.2 output for every input (SC-007).

## `cost projected --output json`

New key `stack` under `finfocus`, present for Pulumi sources and absent for
Terraform and OpenTofu sources.

```json
{
  "finfocus": {
    "summary": { "totalMonthly": 15.59 },
    "resources": [],
    "diff": { "totalAfter": 15.59, "unchanged": 0 },
    "emptyPlan": true,
    "stack": {
      "totalMonthly": 141.03,
      "currency": "USD",
      "resources": 60,
      "complete": true,
      "unchanged": [
        { "resourceType": "aws:ec2/instance:Instance",
          "resourceId": "urn:pulumi:dev::demo::aws:ec2/instance:Instance::web",
          "monthly": 7.59, "currency": "USD" }
      ]
    }
  }
}
```

| Key | Meaning |
| --- | --- |
| `summary.totalMonthly`, `diff.totalAfter` | The plan total. Same name, same value as v0.5.2. Budgets and exit codes read it. |
| `resources[]`, `summary` breakdowns, `diff.unchanged` | Plan-origin entries only. |
| `emptyPlan` | True when no plan-origin resource is a priceable type. Same meaning as v0.5.2. |
| `stack.totalMonthly`, `stack.currency` | The stack total. |
| `stack.resources` | How many resources the stack total covers. |
| `stack.complete` | False when the state could not be read. |
| `stack.incompleteReason` | One of `not_requested` (a saved preview with no state, or a saved preview of a different stack or project than the surrounding one), `backend_unreachable`, `passphrase_required`, `read_failed`; omitted when complete. |
| `stack.omittedUnchanged` | Unchanged resources the preview left out, from its change summary; omitted when zero. |
| `stack.unchanged[]` | State-added unchanged resources with their monthly cost; omitted when empty. |
| `stack.errors[]` | Pricing failures of state-origin resources, with `resourceType`, `resourceId`, optional `pluginName`, and `message`; omitted when empty. `stack.complete` describes inventory availability, so pricing failures can occur even when it is true. |
| `errors[]` | Pricing failures of plan-origin resources only, as in v0.5.2. State-origin failures are reported separately in `stack.errors[]`. |

When `stack.complete` is false, `stack.totalMonthly` equals the plan total and
`stack.unchanged` is omitted.

## `--output ndjson`

No new lines. The line set is the v0.5.2 line set, so a reader that treats
every line as a resource entry keeps working. Stack-level information goes to
stderr (below).

## Table output

After the existing table, when the state was read and there are unchanged
resources, a captioned section:

```text
UNCHANGED RESOURCES (not in the plan total)
<same columns as the plan table>
Stack total: $141.03/mo across 60 resources (plan total $15.59/mo)
```

When no plan-origin resource changes and the stack is complete:

```text
No changes. Projected monthly cost is zero.
Stack total: $141.03/mo across 60 unchanged resources.
```

The first line is the Terraform and OpenTofu sentence, unchanged.

## stderr notices

Written to stderr in every output format. Text is fixed so tests can pin it.

| Condition | Text |
| --- | --- |
| Plan holds nothing to price and no complete stack total is available | The v0.5.2 warning: ``Warning: the plan has no resources to price, so the $0 total is not an estimate. A default `pulumi preview --json` omits unchanged resources; make the plan with `--show-sames` or run `finfocus overview` to see the whole stack.`` |
| No changes in the plan, complete stack total, `--output ndjson` (table output prints it on stdout) | ``No changes. Projected monthly cost is zero.`` then ``Stack total: $X/mo across N unchanged resources.`` |
| Saved preview with no state (`not_requested`) | ``Note: the preview omits N unchanged resources, so they are not priced. Make the preview with `--show-sames` or run `finfocus overview`.`` Printed only when `N` is above zero. |
| State unavailable (`backend_unreachable`, `passphrase_required`, `read_failed`) | `Warning: stack state unavailable (<reason>): N unchanged resources are not included in the stack total.` `N` is `omittedUnchanged`; the clause is dropped when it is zero. |
| Preview and state disagree | `Warning: the preview does not match the stack state for N resources; they are priced as the preview lists them.` |

No notice contains a URL, backend name, token, or raw error text.

## Other commands

| Command | Change |
| --- | --- |
| `cost estimate` | The resource picker and `--resource` lookup cover the live inventory. With no input flag in a Pulumi project, it loads that project's live inventory, as it already did for a Terraform directory. |
| `cost recommendations` | Recommendations are fetched for the live inventory. Counts and savings totals can grow, which is intended. |
| `cost forecast` | Inventory from the snapshot; total over plan-origin resources, unchanged from v0.5.2 (research R14). |
| `overview` | Same rows as today, loaded through the shared step. |
| `cost actual`, `cost history collect` | `DeployedDescriptors()` plus the existing history enrichment. Figures unchanged. `cost actual --pulumi-json` uses `ActualDescriptors()`: the deployed resources when the surrounding project's state can be read, otherwise the plan's resources, as in v0.5.2. |

## Exit codes

Unchanged. A budget result is computed from the plan total.
