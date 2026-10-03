# Budget configuration

`cost.budgets` lives in `config.hujson`. `finfocus config init` creates the
project file (`$PROJECT/.finfocus/config.hujson` plus `.gitignore`) when a
`Pulumi.yaml` is found, and the global file otherwise. Resolution order is
`--project-dir`, `FINFOCUS_PROJECT_DIR`, the nearest `Pulumi.yaml`, then
`~/.finfocus/`. Project keys replace global keys. Missing keys are inherited.

## Scopes

| Scope | Key | Match |
| --- | --- | --- |
| Global | `global` | Required when any other scope is set |
| Provider | `providers.<name>` | Case-insensitive provider name |
| Tag | `tags[]` | Selector `key:value` or `key:*`, plus `priority` |
| Type | `types.<pulumi-type>` | Exact, case-sensitive type token |

Each scope uses the same fields:

| Field | Rule |
| --- | --- |
| `amount` | Required to enable. `0` disables. Negative is rejected |
| `currency` | ISO code. Empty inherits the global currency. A mismatch is an error |
| `period` | Empty or `monthly`. Anything else is rejected |
| `alerts` | `{ "threshold": number, "type": "actual" or "forecasted" }`. Empty uses 50%, 80%, and 100% actual |
| `exit_on_threshold` | Optional bool. Nil inherits the parent `cost.budgets` value |
| `exit_code` | Optional integer 0-255. Nil inherits the parent, which defaults to 1 |

Parent `cost.budgets` may also set `exit_on_threshold` and `exit_code` for
scopes that do not set their own.

Tag selectors match `^[a-zA-Z0-9_-]+:(\*|[a-zA-Z0-9_-]+)$`. When one resource
matches several tag budgets, only the highest `priority` receives the cost.
Equal priorities are ordered by selector, and a warning is emitted.

## Examples

Global gate:

```hujson
{
  "cost": {
    "budgets": {
      "global": {
        "amount": 500,
        "currency": "USD",
        "exit_on_threshold": true,
        "exit_code": 2,
        "alerts": [{ "threshold": 100, "type": "forecasted" }]
      }
    }
  }
}
```

Providers, with one shared exit setting:

```hujson
{
  "cost": {
    "budgets": {
      "global": { "amount": 15000, "currency": "USD" },
      "providers": {
        "aws": { "amount": 8000 },
        "gcp": { "amount": 5000 },
        "azure": { "amount": 2000 }
      },
      "exit_on_threshold": true,
      "exit_code": 1
    }
  }
}
```

Tags and a type. Tag spend stays `$0` until cost results carry tags. The
validator warns:

```text
tag-based budgets are configured but tag allocation is not yet fully implemented; tag budgets will show $0 spend until tag data is available in cost results
```

```hujson
{
  "cost": {
    "budgets": {
      "global": { "amount": 10000, "currency": "USD" },
      "tags": [
        { "selector": "team:platform", "priority": 100, "amount": 3000 },
        { "selector": "env:prod", "priority": 50, "amount": 5000 },
        { "selector": "cost-center:*", "priority": 10, "amount": 1000 }
      ],
      "types": {
        "aws:ec2/instance:Instance": { "amount": 2000 },
        "aws:rds/instance:Instance": {
          "amount": 3000,
          "exit_on_threshold": true,
          "exit_code": 6
        }
      }
    }
  }
}
```

## Environment

| Variable | Effect |
| --- | --- |
| `FINFOCUS_BUDGET_EXIT_ON_THRESHOLD` | `strconv.ParseBool` value stored on the global scope |
| `FINFOCUS_BUDGET_EXIT_CODE` | Integer stored on the global scope |

These write the global scope. CLI flags on that same scope override them.
They leave `cost.budgets.exit_on_threshold` and provider, tag, and type exit
fields unchanged, so they do not control those exit decisions. There is no `FINFOCUS_BUDGET_AMOUNT`
or `FINFOCUS_BUDGET_CURRENCY` reader.

## Errors the loader returns

- `global budget is required when scoped budgets are defined`
- `scoped budget currency must match global budget currency`
- `invalid tag selector format`
- `budget amount cannot be negative`
- `budget period must be 'monthly'`
- `exit code must be between 0 and 255`
