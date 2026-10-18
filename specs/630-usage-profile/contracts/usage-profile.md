# Contract: Usage Profile

This feature adds no RPC. It fills fields finfocus-spec v0.7.6 already
defines.

## CLI

```text
finfocus cost projected --profile=dev
finfocus cost recommendations --profile=prod
finfocus cost forecast --profile=burst
finfocus cost estimate --profile=dev
finfocus cost cluster --profile=dev
finfocus overview --profile=dev
```

`--profile` is a persistent flag of `cost`, and a local flag of
`overview`. Values are `dev`, `development`, `prod`, `production`,
`burst`, `default`, and `unspecified`, case-insensitive.

An unknown or empty value exits 2 through `toValidationError` and does
not open plugins.

`cost actual`, `cost history`, and a historical cluster window accept the
inherited flag and do not put it on their plugin requests. Their output
does not claim the profile changed measured spend.

## Environment

`FINFOCUS_PROFILE` overrides `cost.profile` and yields to an explicit
flag. It is not written back to the config file. `PULUMICOST_PROFILE` is
not read.

## Configuration

```yaml
cost:
  profile: dev
```

Project config overrides global config by the usual shallow merge of the
`cost` object. `finfocus config validate` rejects an unknown or empty
`cost.profile`.

## Plugin requests

`GetProjectedCostRequest.usage_profile` and
`GetRecommendationsRequest.usage_profile` receive the enum. No preference
leaves the zero value. Core does not change `monthly_cost` because of the
profile.

## Results

Projected JSON:

```json
{
  "finfocus": {
    "profile": "dev",
    "summary": {},
    "resources": []
  }
}
```

Recommendations JSON includes top-level `"profile": "dev"`. The NDJSON
summary line includes the same field. Table output prints `Profile: dev`
before the table.
