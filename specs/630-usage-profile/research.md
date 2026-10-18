# Research: Usage Profile

## Decision: Send the existing enum; do not extend the protocol

**Decision**: Map the resolved label onto `pbc.UsageProfile` and set
`GetProjectedCostRequest.usage_profile` and
`GetRecommendationsRequest.usage_profile`.

**Rationale**: finfocus-spec v0.7.6 already defines
`USAGE_PROFILE_UNSPECIFIED`, `PROD`, `DEV`, and `BURST` on those two
messages. Issue #368's cross-repo dependency is done.

**Alternatives considered**: A new core-only string tag. Plugins would not
see the field the spec told them to read. A spec bump. Nothing new is
required.

## Decision: `default` is unspecified, and the SDK aliases are accepted

**Decision**: Accept `dev`, `development`, `prod`, `production`, `burst`,
`default`, and `unspecified`, in any case. `default` and `unspecified` map
to `USAGE_PROFILE_UNSPECIFIED`. Display the resolved value as `dev`,
`prod`, `burst`, or `default`.

**Rationale**: `pluginsdk.ParseUsageProfile` already accepts the first
aliases and `unspecified`. The issue's word for no preference is `default`.
An empty value, when the flag, variable, or config field is present, is a
validation error. The SDK treats an empty string as unspecified, so the
core wrapper rejects empty before calling it.

**Alternatives considered**: Only the four issue words. That rejects
`production`, which the SDK documents. Only the SDK words. That rejects
the issue's `default`.

## Decision: Resolve outside the config loader

**Decision**: Precedence is `--profile`, then `FINFOCUS_PROFILE` when it is
set in the environment, then `cost.profile` when the field is non-empty,
then no preference. The loader does not copy the variable into the struct.

**Rationale**: `Config.Save` rewrites the struct and would persist an
environment override. Budget CLI overrides already stay off the global
config for the same reason. `os.LookupEnv` distinguishes an unset variable
from an empty one.

**Alternatives considered**: Folding `FINFOCUS_PROFILE` in `config.New`,
the way cache settings work. A later save would write the variable into
`config.hujson`. Reading `PULUMICOST_PROFILE`. That name is retired.

## Decision: Opt in per command

**Decision**: `WithUsageProfile` is called by projected cost,
recommendations, forecast, estimate, cluster, overview, the dashboard
engine, and `analyzer serve`. Actual cost and `cost history collect` do
not call it.

**Rationale**: History collection prices with `GetProjectedCost` to store
a baseline. The spec says history must not carry the hint. Actual cost's
state estimate also calls the projected helper for an hourly rate; that
path passes unspecified even when the engine has a profile, so a dev
hint cannot shrink a measured fallback.

**Alternatives considered**: Setting the profile inside `newEngineWithCache`.
That would mark history rows with the hint. A context value. The engine
already carries per-run settings such as `pricingSpecFallback` the same way.

## Decision: Skip projected batching when a profile is set

**Decision**: `GetProjectedCost` uses the per-resource path when the
profile is not unspecified. The per-resource request sets `usage_profile`.
Unspecified runs keep today's batch path.

**Rationale**: `BatchCostRequest` has no usage profile. A batch call would
drop the hint and then cache that answer under a profile key.

**Alternatives considered**: A spec change to add the field to
`BatchCostRequest`. Out of scope for a core issue, and the per-resource
path already exists as the batch fallback. Putting the profile in resource
tags. That would leak a core hint into user properties.

## Decision: Cache suffix only when set

**Decision**: Append `/profile-dev`, `/profile-prod`, or `/profile-burst`
to the projected resource key and the recommendations key. Unspecified
adds nothing.

**Rationale**: Anything sent to a plugin belongs in the projected cache
key. An empty suffix keeps entries written before this feature valid.

**Alternatives considered**: Always appending `/profile-unspecified`.
That misses the current cache for every resource.

## Decision: No dashboard control

**Decision**: The dashboard engine uses the same resolver with no flag, so
a saved default and the environment apply. No new control is added.

**Rationale**: The spec bounds the dashboard to the resolved profile. The
config page can already edit `config.hujson`; it does not need a new form
for this field.

## Constitution re-check

The design adds no protocol, no required store, and no plugin pricing.
Post-design constitution check passes.
