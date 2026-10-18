---
title: "Recommendation Scorer Plugin"
description: "Opt-in recommendation ranking with Jev and Clef backends"
layout: "docs"
---

Rates recommendations that other plugins produced. For each recommendation it
returns how risky acting is, whether the resource is probably in that state on
purpose, whether the change is worth an engineer's time, an expected priority,
whether the evidence is thin, and which recommendations duplicate each other.
It implements `RecommendationScorerService` from `finfocus-spec` on top of
a selectable Jev or Clef backend. Jev remains the default.

**A score is a ranking signal, not approval.** The plugin reports
`SCORE_CALIBRATION_RANKING_ONLY` and never claims a recommendation is safe to
apply unattended. Use scores to sort work and to route items to human review.

## Opt-in and data handling

The plugin is off unless an operator installs it and sets `SCORER_API_KEY`.
Without a key it still starts and declares the scoring capability, and every
scoring call returns `UNAUTHENTICATED`. FinFocus core makes no network calls
for scoring; this plugin makes the only outbound requests, to the selected backend. Hosted Jev sends recommendation data to TypeSafe AI
at `https://api.typesafe.ai` (`POST /v1/systemone`); hosted Clef sends it to
Cloudflare at `https://api.cloudflare.com` through its account-scoped Workers AI endpoint.

**What is sent to the selected backend.** Every field of each recommendation the host
sends, except its `id` and `created_at`. With the current spec that is:

- Category, action type, description, reasoning, source, priority, plugin
  confidence and metadata.
- The resource's provider, type, region, SKU, tags and utilization.
- The resource `id` and `name` exactly as the host presented them in the
  request. The host's `identifier_mode` decides whether those are raw,
  pseudonymized tokens or absent; the plugin sends what it receives.
- The impact figures (savings, current and projected cost, effort).
- `action_detail`: right-size targets, termination detail, commitment terms,
  Kubernetes adjustments (cluster, namespace, controller and container names)
  and config changes.
- The `primary_reason` and `secondary_reasons` codes.
- Fixed question text and the pinned model id.

A field added to the spec later is sent too. To narrow what reaches either backend,
set `scoring.field_allowlist` in FinFocus; the host drops every field not on
the list before the plugin sees it.

**What is not sent:** the recommendation `id` and creation time, credentials,
Pulumi state, and anything outside the recommendations in the request.
Question keys carry only a signal name and the item's position in the batch
(`risk:#3`), and answers are matched back to recommendation ids inside the
plugin. The plugin does not write recommendation text to logs. Logs carry
counts, token usage, the model, provider request ids and timings only. The API
key is never logged and never appears in errors.

Free text is untrusted. The plugin removes control, invisible and
bidirectional characters, caps every string, list and map before sending, and
never places recommendation text in the question instructions: which questions
are asked does not depend on the recommendation's content. A test asserts this.
Even so, a model can be influenced by text in its input, which is one more
reason a score must not gate an irreversible action.

Retention and processing terms depend on your selected provider and agreement.
Review them before enabling hosted scoring on sensitive data, and keep the
host's default `identifier_mode: pseudonymized`. Hosted Jev and Clef both send
recommendation data to a third party. A compatible local endpoint may avoid
hosted calls; Clef self-hosting and vLLM compatibility have not been tested and
are outside this change's scope.

## Install

Build and install from the repository root while the first scorer release is
pending:

```bash
make install-scorer
```

This installs into `~/.finfocus/plugins/scorer/<version>/`. Once scorer release
assets are published and the registry entry is available, the public command
will be:

```bash
finfocus plugin install scorer
```

The existing `jev` registry entry remains until scorer assets exist.
`finfocus plugin install jev` continues to install the last legacy Jev release
separately; it does not migrate configuration or install `scorer`. Historical
`jev-v*` tags and assets remain available. If Jev is already installed,
`finfocus plugin install jev` reports `already installed`; it does not switch
to scorer. Install scorer explicitly and change `scoring.plugin` to migrate.
Neither plugin is installed by
`finfocus setup`.

## Enable in FinFocus

Scoring is off in FinFocus until you turn it on. Set the plugin before
enabling, because every config change is validated:

```bash
export SCORER_BACKEND=jev
export SCORER_API_KEY=...   # backend token; keep it out of history
finfocus config set scoring.plugin scorer
finfocus config set scoring.enabled true

# Check exactly what would be sent, and send nothing
finfocus cost recommendations --pulumi-json plan.json --scoring-dry-run

# Score for real
finfocus cost recommendations --pulumi-json plan.json --sort risk:asc
```

Without the key the plugin still starts, but every scoring call returns
`UNAUTHENTICATED`, which FinFocus shows as a scoring warning while listing the
recommendations unscored.

## Configuration

All configuration is environment variables read at startup.

| Variable | Default | Meaning |
| --- | --- | --- |
| `SCORER_BACKEND` | `jev` | Backend: `jev` or `clef`. |
| `SCORER_API_KEY` | none | Token for the selected backend; TypeSafe API key for Jev or Cloudflare API token for Clef. |
| `SCORER_MODEL` | `jev-1.13.0` for Jev; `clef` for Clef | Backend model id. |
| `SCORER_BASE_URL` | `https://api.typesafe.ai` for Jev; `https://api.cloudflare.com` for Clef | Optional API origin; plain HTTP is accepted only for loopback hosts. The Clef adapter appends the account-scoped path. |
| `SCORER_CLEF_ACCOUNT_ID` | none | Cloudflare account id: 32 hexadecimal characters, required when Clef has an API key. |
| `SCORER_TIMEOUT` | `60s` | Timeout for each backend attempt. |
| `SCORER_BATCH_SIZE` | `25` | Recommendations per batched backend request, 1 to 80; `priority` is always one per request. |
| `SCORER_DUPLICATE_THRESHOLD` | `0.5` | Yes probability at which two recommendations count as duplicates. |
| `FINFOCUS_LOG_LEVEL` | `info` | Set to `debug` for verbose logs. |

For hosted Clef, set the following before starting FinFocus (supply your token
through your secret manager or environment):

```bash
export SCORER_BACKEND=clef
export SCORER_CLEF_ACCOUNT_ID=0123456789abcdef0123456789abcdef
export SCORER_MODEL=clef
# SCORER_API_KEY must hold your Cloudflare token
```

Leave `SCORER_BASE_URL` unset for the hosted default. Do not include
`/client/v4/accounts/.../ai/run/@cf/cloudflare/clef` in it: the adapter adds that
path using `SCORER_CLEF_ACCOUNT_ID`. For Clef the origin must have no path,
query, user info or fragment, and `SCORER_MODEL` must be `clef`; a mismatched
model id is rejected.

### Migration from the legacy Jev plugin

The scorer does not read `TYPESAFE_API_KEY` or any `JEV_*` settings. Rename
settings explicitly; no credential or configuration migration is automatic.

| Removed setting | Replacement |
| --- | --- |
| `TYPESAFE_API_KEY` | `SCORER_API_KEY` |
| `JEV_MODEL` | `SCORER_MODEL` |
| `JEV_BASE_URL` | `SCORER_BASE_URL` |
| `JEV_TIMEOUT` | `SCORER_TIMEOUT` |
| `JEV_BATCH_SIZE` | `SCORER_BATCH_SIZE` |
| `JEV_DUPLICATE_THRESHOLD` | `SCORER_DUPLICATE_THRESHOLD` |
| `scoring.plugin: jev` | `scoring.plugin: scorer` |

Use `SCORER_BACKEND=jev` to retain the original backend. Replace old source
commands with `make build-scorer`, `make install-scorer`, `make test-scorer`
and `make lint-scorer`. A legacy `jev` installation can coexist with `scorer`;
select `scorer` in configuration after installing it.

FinFocus's `scoring.timeout_seconds` (default 30) bounds each whole scoring
call, including every backend request and retry the plugin makes for it.
`SCORER_TIMEOUT` can only shorten a single attempt inside that. When a retry wait
(from `Retry-After` or backoff) would outlast the remaining time, the plugin
fails at once with the rate-limit or outage error instead of timing out. If
you hit rate limits often, raise `scoring.timeout_seconds`.

Keep the key out of shell history and committed files. Do not put it in a
`.env` that is tracked by git.

## Signals

| Signal | Backend question | Range |
| --- | --- | --- |
| `risk` | Yes/no: real risk of downtime, data loss, performance regression, or a hard-to-undo financial commitment | 0 to 1 |
| `false_positive` | Yes/no: probably in this state on purpose | 0 to 1 |
| `worth_acting` | Yes/no: worth an engineer's time this week | 0 to 1 |
| `priority` | Four-level rating: Ignore, Low, Medium, High | 0 to 3 |
| `insufficient_evidence` | Yes/no: record too thin or contradictory | 0 to 1 |
| `duplicate_group_id` | Pairwise yes/no within each resource, then connected components | string |

Duplicate grouping compares recommendations that share `resource.id` as the
host presented it, asks whether each pair describes the same change, and
assigns a group id only to components with more than one member. It is skipped
when the request says identifiers were omitted. At most 10 recommendations per
resource are compared.

## How requests are run

- A request holds up to 100 recommendations (`max_batch_size`). The plugin
  splits it into backend requests of up to 25 recommendations, each asking
  risk, false positive, worth acting and insufficient evidence as named
  questions `<signal>:#<position>` about an ordered list. `priority` goes in
  its own request per recommendation. Up to 8 requests are in flight.
- A retry whose wait would outlast the caller's deadline is not attempted; the
  call fails at once with the underlying rate-limit or outage error.
- Results are index-aligned with the request and echo each recommendation id.
  An item the backend could not answer is returned as a `ResourceError`
  without failing the rest.
- 429 and 529 responses and connection failures are retried with exponential
  backoff and jitter, honouring `Retry-After`. Authentication (401/403),
  exhausted rate limits and outages fail the whole call with `UNAUTHENTICATED`,
  `PERMISSION_DENIED`, `RESOURCE_EXHAUSTED` or `UNAVAILABLE`.
- The response reports `scorer.name = "scorer"`, the model id the API returned,
  ranking-only calibration and the provider request ids.

## Clef compatibility and evidence

[Cloudflare describes Clef's decision API](https://blog.cloudflare.com/clef-decision-models/).
The plugin uses the released
[go-decide v0.1.0 transport adapter](https://github.com/rshade/go-decide/blob/v0.1.0/clefclient/transport.go)
for account-path rewriting and response-envelope unwrapping.
The [recorded compatibility spike](../../specs/1728-clef-spike/README.md)
passed `noul` and `score` response decoding. It rejected `choice`, which this
scorer does not send. That spike made no live calls and accessed no credentials;
it does not establish live recommendation quality or calibration for Clef.

Both backends return ranking signals. Do not auto-approve or auto-dismiss from
a score, interpret risk as a calibrated probability, or transfer Jev evaluation
results to Clef. Validate any threshold with labelled real recommendations and
recheck it when changing backend or model. No Clef price, latency or accuracy
claim follows from the recorded replay.

## Testing

```bash
make test-scorer    # unit tests, including the SDK scorer conformance runner
make lint-scorer
```

Live tests require explicit `SCORER_LIVE_TEST=1` as well as the selected backend's
`SCORER_API_KEY`. They send synthetic recommendation data to the provider and
are for an operator to run; agents must not read credentials or run live tests.
Select the backend explicitly:

```bash
SCORER_LIVE_TEST=1 SCORER_BACKEND=jev go -C plugins/scorer test -run 'TestLive' -v -count=1
SCORER_LIVE_TEST=1 SCORER_BACKEND=clef go -C plugins/scorer test -run 'TestLive' -v -count=1
```

Clef also needs `SCORER_CLEF_ACCOUNT_ID`. Model listing is Jev-only. Evaluation
is also Jev-only and requires the additional `SCORER_EVAL=1` gate:

```bash
SCORER_LIVE_TEST=1 SCORER_BACKEND=jev SCORER_EVAL=1 go -C plugins/scorer test -run TestEvaluation -v -count=1
```

`TestEvaluation` scores the labelled dataset in `testdata/` and logs AUC per
signal, the risk Brier score and duplicate accuracy, plus token usage. The data
is synthetic; replace it with sanitised real recommendations before publishing
any threshold guidance. These results do not validate Clef.
