---
name: finfocus-plugin-dev
description: >
  Implement and maintain a FinFocus cost plugin in a repository scaffolded by
  `finfocus plugin init`. Use when adding or changing pricing logic in a
  plugin, adding a resource type, writing the Supports handler, returning
  GetProjectedCost or GetActualCost results, testing a plugin locally against
  a Pulumi plan, or installing a plugin build into `~/.finfocus/plugins`.
  Triggers on: "implement GetProjectedCost", "add a resource type to my
  plugin", "plugin Supports", "finfocus plugin", "pluginsdk", "finfocus-spec
  SDK", "test my plugin locally", "plugin pricing logic", "plugin
  expires_at", or any task inside a FinFocus plugin repo. For SDK version
  bumps use the finfocus-plugin-upgrade skill instead.
---
<!-- Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0. -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# FinFocus Plugin Development

A FinFocus plugin is a standalone binary that serves the `CostSourceService`
gRPC API from `github.com/rshade/finfocus-spec/sdk/go/pluginsdk`. The
`finfocus` CLI launches it, asks `Supports`, then calls `GetProjectedCost` (and
optionally `GetActualCost`, `GetRecommendations`, and others) per resource.
This repository was scaffolded by `finfocus plugin init`.

## Rules

- **Never hand-edit SDK-version state** (the `finfocus-spec` line in
  `go.mod`, `SpecVersion`, workflow pins). Run `finfocus plugin upgrade` and
  follow the `finfocus-plugin-upgrade` skill.
- **Never fabricate prices.** If the plugin cannot price a resource, return
  "not supported" from `Supports` or an error from `GetProjectedCost`. Do not
  guess, and do not return a default. The scaffold's example rates in
  `calculateEC2InstanceCost` are placeholders: replace them. Scaffolds from
  finfocus v0.4.0 and earlier also default a missing instance type to
  `t3.micro` and price unknown types at the `t3.micro` rate; remove both.
- Price from real data: a local pricing table with a documented source and
  date, or the provider's pricing API. Record the source in `billing_detail`.
- Do not run `git commit` or `git add` unless the user asks.
- `SpecVersion` must be `v`-prefixed (`v0.7.2`). Use `pluginsdk.SpecVersion`,
  as the scaffold does. Older scaffolds wrote a bare literal such as `"0.6.1"`;
  `finfocus plugin upgrade` rewrites it.
- Never write to stdout from the plugin: the SDK prints a single
  `PORT=<n>` handshake line there. Log to stderr or the SDK log writer.

## Repo layout (from `finfocus plugin init`)

| Path | Purpose | Edit? |
| --- | --- | --- |
| `cmd/plugin/main.go` | Builds `pricing.NewCalculator()` and calls `pluginsdk.Serve`; reads `--port` and `FINFOCUS_PLUGIN_PORT`; optional health server | Rarely |
| `internal/pricing/calculator.go` | `Calculator` embeds `*pluginsdk.BasePlugin`; `GetPluginInfo`, `Supports`, `GetProjectedCost`, `GetActualCost` | Yes, main work |
| `internal/pricing/data.go` | `PricingData` load/save helpers and example data | Yes, if you use local tables |
| `internal/client/client.go` | Provider API client stub (`GetResourceCost`, `ValidateCredentials`, `GetSupportedRegions`) | If you call live APIs |
| `internal/pricing/calculator_test.go` | Example tests using `pluginsdk.NewTestPlugin` | Yes, extend |
| `manifest.yaml`, `manifest.json` | Plugin metadata (name, version, providers, capabilities) | Keep in sync |
| `Makefile` | `build`, `test`, `lint`, `install`, `cover`, `security` | Rarely |
| `.golangci-lint.yml` | Lint config used by `make lint` | No |
| `.github/workflows/` | CI, release, release-please (docker, claude review when generated) | Via upgrade only |
| `docs/`, `docker/`, `issues.md`, `examples/` | Generated docs, image, TODO list | As needed |

Binary name is `finfocus-plugin-<name>`; `make build` writes it to `bin/`.
Details on the SDK are in `references/sdk-api.md`.

## Add a resource type end to end

1. **Find the real input.** Run a plan (see Testing loop) and read what the
   plugin receives. Core sends the Pulumi type token
   (`aws:ec2/instance:Instance`), `provider`, `sku`, `region`, and the
   resource inputs as `tags`. Scaffolds from v0.4.0 and earlier match
   `aws:ec2:Instance`, which core never sends; fix that case. Normalize the
   token in one function and test it.
2. **Register support.** Add the provider or resource type to the matcher in
   `NewCalculator` (`base.Matcher().AddProvider`, `AddResourceType`) and extend
   `Supports` (see below).
3. **Extract inputs.** Read SKU and region from `req.GetResource()`; fall
   back to `pluginsdk/mapping` helpers (`ExtractAWSSKU`, `ExtractAzureSKU`,
   `ExtractGCPSKU`, and the `*Region` forms) over `Tags`.
4. **Price it.** Compute an hourly rate and call
   `c.Calculator().CreateProjectedCostResponse(currency, hourly, detail)`,
   which sets `CostPerMonth = hourly * 730`. For a monthly price, build the
   response with `pluginsdk.NewGetProjectedCostResponse(
   pluginsdk.WithProjectedCostDetails(unit, currency, monthly, detail))`.
5. **Test.** Add a table-driven case for the new type, plus one unsupported
   and one missing-SKU case.
6. **Run the real loop** (Testing loop) and compare the number with the
   provider's public price page.
7. **Update** `manifest.yaml`/`manifest.json` capabilities if they changed and
   the README resource list.

## Supports contract

`Supports(ctx, *pbc.SupportsRequest) (*pbc.SupportsResponse, error)` is the
optional `SupportsProvider` interface.

- Answer `Supported: false` with a `Reason` rather than returning an error.
  Core treats an RPC error as fail-open (calls the plugin anyway).
- A plugin that does not implement `Supports` gets the SDK default
  (`Supported: false`, `Reason: pluginsdk.DefaultSupportsNotImplementedReason`).
  Core treats that reason as fail-open too, so implement `Supports` for real.
- Core sends provider, resource type, SKU, and region, and caches answers per
  client, provider, type, region, SKU, and feature. A region-bound plugin must
  decline other regions here so the router can pick another plugin.
- Keep it cheap and offline: no API calls.

## Cost response contracts

- `GetProjectedCost` returns `Currency`, `UnitPrice` (hourly), `CostPerMonth`,
  and `BillingDetail`. Use `BillingDetail` to state assumptions (OS, tenancy,
  defaults applied, price source).
- Validate with `pluginsdk.ValidateProjectedCostRequest` (needs provider, type,
  SKU, region). If tags contain `ref.*` keys and SKU is empty, core validates
  with `ValidateProjectedCostRequestLenient`; do the same, then price from the
  `ref.<property>.sku` tags or report what is missing in `BillingDetail`.
- `$0` is a valid priced result and does **not** trigger fallback to the next
  plugin; only an error or empty result does. Return `$0` only when the
  resource is truly free. Otherwise return an error, so another plugin can try.
- Unsupported resource: return `pluginsdk.NotSupportedError(resource)`.
  No billing data: `pluginsdk.NoDataError(id)`.
- Cache hints: set `ExpiresAt` with `pluginsdk.WithProjectedCostExpiresAt`.
  Past timestamps skip caching; core caps TTLs at 7 days.
- Optional extras (implement only if you can do them correctly):
  `GetRecommendations`, `GetPricingSpec`, `EstimateCost`, `BatchCost`,
  `DryRun`. See `references/sdk-api.md`.

## Testing loop

1. `make test` (unit tests, in-process gRPC via `pluginsdk.NewTestPlugin`).
2. Conformance: `pluginsdk.RunBasicConformance(plugin)` (also Standard and
   Advanced) in a test; see `references/testing.md`.
3. `make install` copies the binary to
   `~/.finfocus/plugins/<name>/<version>/`.
4. `finfocus plugin list` then `finfocus plugin validate`.
5. Price a real plan:
   `finfocus cost projected --pulumi-json plan.json` (create the plan with
   `pulumi preview --json > plan.json`). Add `--debug` to see plugin calls.
6. `make lint` before finishing.

More in `references/testing.md` and `references/core-contract.md`.

## Common pitfalls

- Core does not set `PORT`. Use `--port` or `FINFOCUS_PLUGIN_PORT`; the SDK's
  `Serve` handles both and announces the chosen port on stdout.
- Pulumi tokens vary (`aws:ec2/instance:Instance`); never match on a single
  hard-coded spelling.
- Nested inputs arrive as dotted tag keys (`sku.capacity`,
  `rootBlockDevice.0.volumeType`) beside the collapsed key.
- A mismatch between `PluginVersion`, `manifest.*` version, and the install
  directory makes `plugin list` confusing. Keep them equal.
- A `region` key in `plugin.metadata.json` makes core look for
  `finfocus-plugin-<name>-<region>` first. Region-per-binary plugins should
  follow that name.
- `make install` must write `plugin.manifest.json` as flat JSON whose
  `name` and `version` match the install directory, or `finfocus plugin
  validate` fails. Scaffolds from v0.4.0 and earlier copy `manifest.yaml`
  there instead; replace that line with the current scaffold's `printf`.
