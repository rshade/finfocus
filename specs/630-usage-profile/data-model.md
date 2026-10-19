# Data Model: Usage Profile

## Usage profile

One value for a run.

| Label shown | Wire value | Meaning |
| --- | --- | --- |
| `default` | unset / `USAGE_PROFILE_UNSPECIFIED` | Plugin uses its ordinary assumptions |
| `dev` | `USAGE_PROFILE_DEV` | Development workload |
| `prod` | `USAGE_PROFILE_PROD` | Production workload |
| `burst` | `USAGE_PROFILE_BURST` | Short, high-intensity workload |

Accepted input, any case: `dev`, `development`, `prod`, `production`,
`burst`, `default`, `unspecified`.

Validation:

- A missing flag, an unset variable, and an omitted config field are no
  preference.
- A present empty value is a user-input error.
- Any other word is a user-input error.
- Config validation reports the same error for `cost.profile`.

## Profile source

Resolved once, in order:

1. `--profile` when the flag was set on the command.
2. `FINFOCUS_PROFILE` when the variable is set.
3. `cost.profile` when the configured string is non-empty.
4. No preference.

The flag does not write the file or the variable.

## Where it is stored for a run

`Engine.usageProfile` (`pbc.UsageProfile`). The zero value is no
preference. `WithUsageProfile` sets it. It is not serialized into
`config.hujson` from the environment.

## Requests

| Request | Carries the profile |
| --- | --- |
| `GetProjectedCost` from an opted-in engine | Yes |
| `GetRecommendations` from an opted-in engine | Yes |
| Projected `BatchCost` | No. Skipped when a profile is set |
| `GetActualCost` and historical `GetWindowCost` | No |
| Hourly-rate lookup inside actual cost | No, even if the engine has a profile |
| `EstimateCost` | No field. Estimate still uses `GetProjectedCost`, which carries it |
| History collection's projected pricing | No. That engine is not opted in |

## Cache

Projected key gains `/profile-<label>` after the existing suffixes when
the label is `dev`, `prod`, or `burst`. Recommendations key gains the same
suffix. No preference keeps the current key.

## Output

Projected JSON document and recommendations JSON include `profile` with
the display label, including `default`. Table output prints the same label
once for the run. Forecast, cluster, overview, and the dashboard do not
add a new profile control; they still send the hint.
