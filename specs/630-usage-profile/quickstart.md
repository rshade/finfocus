# Quickstart: Usage Profile

## One projection

```bash
finfocus cost projected --pulumi-json plan.json --profile=dev
finfocus cost projected --pulumi-json plan.json --profile=prod --output json
```

The text result starts with `Profile: dev` or `Profile: prod`. The JSON
document's `finfocus.profile` is the same label. A recorder plugin's
`GetProjectedCost` request has `usage_profile` set to `DEV` or `PROD`.

## A saved default

In the project or global `config.hujson`:

```json
{
  "cost": {
    "profile": "dev"
  }
}
```

```bash
finfocus cost projected --pulumi-json plan.json
FINFOCUS_PROFILE=burst finfocus cost projected --pulumi-json plan.json
FINFOCUS_PROFILE=burst finfocus cost projected --pulumi-json plan.json --profile=prod
```

The three runs send `dev`, then `burst`, then `prod`. The file still says
`dev`.

## What does not change

```bash
finfocus cost actual --pulumi-json plan.json --profile=dev
finfocus cost history collect --profile=dev
```

Those plugin requests do not carry a usage profile.

## A bad value

```bash
finfocus cost projected --profile=staging
```

The command exits 2 and names the accepted values. No plugin starts.

```bash
finfocus config validate
```

fails the same way when `cost.profile` is `staging` or empty.
