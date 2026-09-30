# Jev Recommendation Scorer Plugin

Rates recommendations that other plugins produced. For each recommendation it
returns how risky acting is, whether the resource is probably in that state on
purpose, whether the change is worth an engineer's time, an expected priority,
whether the evidence is thin, and which recommendations duplicate each other.
It implements `RecommendationScorerService` from `finfocus-spec` on top of
TypeSafe AI's Jev (System One) model.

**A score is a ranking signal, not approval.** The plugin reports
`SCORE_CALIBRATION_RANKING_ONLY` and never claims a recommendation is safe to
apply unattended. Use scores to sort work and to route items to human review.

## Opt-in and data handling

The plugin is off unless an operator installs it and sets `TYPESAFE_API_KEY`.
Without a key it still starts and declares the scoring capability, and every
scoring call returns `UNAUTHENTICATED`. FinFocus core makes no network calls
for scoring; this plugin makes the only outbound requests, to
`https://api.typesafe.ai` (`POST /v1/systemone`).

**What is sent to TypeSafe AI.** Every field of each recommendation the host
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

A field added to the spec later is sent too. To narrow what reaches TypeSafe,
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

Retention and processing terms are set by your agreement with TypeSafe AI. The
Master Customer Agreement and data-processing terms have not been reviewed for
third-party plugin use, and zero data retention is documented as an enterprise
plan feature. Review them before enabling the plugin on sensitive data, and
keep the host's default `identifier_mode: pseudonymized`.

## Install

```bash
finfocus plugin install jev
```

This installs the latest `jev-v*` release to
`~/.finfocus/plugins/jev/<version>/`. The registry does not install it during
`finfocus setup`; it is always an explicit choice.

### From source

From the `finfocus` repository root:

```bash
make install-jev
```

This builds the plugin and installs it the same way as `make install-recorder`.

## Enable in FinFocus

Scoring is off in FinFocus until you turn it on. Set the plugin before
enabling, because every config change is validated:

```bash
export TYPESAFE_API_KEY=...   # from the TypeSafe console; keep it out of history
finfocus config set scoring.plugin jev
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
| `TYPESAFE_API_KEY` | none | API key from the TypeSafe console. The only place the key is read from. |
| `JEV_MODEL` | `jev-1.13.0` | Model id. The default is a pinned version, not the moving `jev-latest` alias, so scores stay comparable. |
| `JEV_BASE_URL` | `https://api.typesafe.ai` | API origin. Plain `http` is accepted only for loopback hosts. |
| `JEV_TIMEOUT` | `60s` | Timeout for each backend attempt. |
| `JEV_BATCH_SIZE` | `25` | Recommendations per backend request for the batched signals, 1 to 80. `priority` is always one per request. |
| `JEV_DUPLICATE_THRESHOLD` | `0.5` | Yes probability at which two recommendations count as duplicates. |
| `FINFOCUS_LOG_LEVEL` | `info` | Set to `debug` for verbose logs. |

FinFocus's `scoring.timeout_seconds` (default 30) bounds each whole scoring
call, including every backend request and retry the plugin makes for it.
`JEV_TIMEOUT` can only shorten a single attempt inside that. When a retry wait
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
- The response reports `scorer.name = "jev"`, the model id the API returned,
  ranking-only calibration and the provider request ids.

## Cost and limits

As of 2026-09, input is billed at about $0.042 per million tokens, and a
batched record costs about 430 input tokens, so scoring 1,000 recommendations
costs roughly two cents. Output tokens are free. Latency is around 20 ms per
record in batches.

Scoring `priority` next to other records was measured to lower its rank
correlation with labellers on the synthetic set (Spearman 0.77 for one record
per request against about 0.55 at batch sizes of 5 to 25), while risk and
false-positive ranking held. So `priority` is always requested one record per
backend request, still up to 8 requests in flight. The other signals stay
batched, at `JEV_BATCH_SIZE` records per request. Asking priority costs one
extra request per record, which adds roughly a cent per 1,000 records.

## Reading the numbers (non-normative)

On the 80 synthetic recommendations in `testdata/`, labelled by two
independent blind labellers, Jev ranked risk, false positive and worth-acting
at AUC of roughly 0.9 or better, but the values are not calibrated (risk Brier
score about 0.2, close to uninformative) and a combined auto-approve gate had
precision 0.25. Do not auto-approve or auto-dismiss from a score. Any threshold
you pick is unvalidated until you check it against labelled real data, and
should be re-checked when the model version changes. Per-record noise averages
about 0.03 and peaks near 0.07, so leave a dead band of about 0.1 around a
threshold.

## Testing

```bash
make test-jev    # unit tests, including the SDK scorer conformance runner
make lint-jev
```

Two tests call the real API and are skipped unless `TYPESAFE_API_KEY` is set:

```bash
go -C plugins/jev test -run 'TestLive' -v -count=1
JEV_EVAL=1 go -C plugins/jev test -run TestEvaluation -v -count=1
```

`TestEvaluation` scores the labelled dataset in `testdata/` and logs AUC per
signal, the risk Brier score and duplicate accuracy, plus the tokens and cost
of the run (a few cents). The data is synthetic; replace it with sanitised real
recommendations before publishing any threshold guidance.
