---
title: Recommendation Scoring Guide
description: Rate cost recommendations with an optional scorer plugin to rank review work, without automating any decision.
---

## Overview

Cost recommendations come from cost-source plugins. Scoring is an optional extra step that hands those recommendations
to a **scorer plugin**, which rates each one: how risky acting is, whether the resource is probably in that state on
purpose, whether the recommendation is worth an engineer's time, how thin the evidence is, and which recommendations
duplicate each other. Core then sorts, filters and flags the list with those ratings.

Scoring is **off by default**. With it off, behavior and network activity are unchanged. Core never calls a model or
external API itself: the scorer is a plugin that implements `RecommendationScorerService` from
[finfocus-spec](https://github.com/rshade/finfocus-spec), and the plugin owns any model, key or network access.

**Scores route work to review. They never approve, dismiss or apply a recommendation.** Nothing in FinFocus dismisses,
snoozes, hides or applies a recommendation because of a score.

**Prerequisites**:

- A scorer plugin installed under `~/.finfocus/plugins/<name>/<version>/` that advertises the
  `recommendation_scoring` capability. The registry provides one: `finfocus plugin install jev` installs the
  Jev-backed scorer, which needs `TYPESAFE_API_KEY` and sends recommendation data to TypeSafe AI (pseudonymized by
  default). See the [plugin README](https://github.com/rshade/finfocus/tree/main/plugins/jev).
- At least one cost-source plugin that returns recommendations

## Enable Scoring

Opt in per scorer in `config.hujson`:

```json
{
  "scoring": {
    "enabled": true,
    "plugin": "my-scorer",
    "identifier_mode": "pseudonymized",
    "field_allowlist": ["category", "action_type", "resource", "impact", "description"],
    "timeout_seconds": 30,
    "needs_review": {
      "risk": 0.3,
      "false_positive": 0.3,
      "insufficient_evidence": 0.5,
      "dead_band": 0.1
    }
  }
}
```

| Key                                          | Meaning                                                       | Default         |
| -------------------------------------------- | ------------------------------------------------------------- | --------------- |
| `scoring.enabled`                            | Turn the scoring step on                                      | `false`         |
| `scoring.plugin`                             | Scorer plugin name (required when enabled)                    | none            |
| `scoring.identifier_mode`                    | `pseudonymized`, `omitted` or `raw`                           | `pseudonymized` |
| `scoring.field_allowlist`                    | Recommendation fields sent to the scorer; empty sends all     | all fields      |
| `scoring.timeout_seconds`                    | Timeout for each scorer call                                  | `30`            |
| `scoring.needs_review.risk`                  | Flag for review at or above this risk                         | `0.3`           |
| `scoring.needs_review.false_positive`        | Flag for review at or above this false-positive score         | `0.3`           |
| `scoring.needs_review.insufficient_evidence` | Flag for review at or above this thin-evidence score          | `0.5`           |
| `scoring.needs_review.dead_band`             | Width of the band around each threshold that is still flagged | `0.1`           |

Valid `field_allowlist` names are `category`, `action_type`, `resource`, `impact`, `priority`, `confidence_score`,
`description`, `reasoning`, `source`, `created_at`, `metadata`, `action_detail`, `primary_reason` and
`secondary_reasons`. The `resource` field carries tags and utilization; `action_detail` carries the provider-specific
action detail (right-size targets, termination detail, commitment terms, Kubernetes adjustments or config changes).
The recommendation id is always sent, as an opaque value (see [Data handling](#data-handling)).

You can also set values with `finfocus config set`. Set `scoring.plugin` before `scoring.enabled`, because the
configuration is validated on every change.

`scoring.timeout_seconds` bounds the whole scorer call, including any retries the plugin makes against a hosted model.
If a hosted scorer reports rate limits often, raise it.

### Quick start with Jev

The registry's `jev` plugin scores with TypeSafe AI's Jev model. It needs an API key from the TypeSafe console and sends
recommendation data to TypeSafe, so read its
[data-handling notes](https://github.com/rshade/finfocus/tree/main/plugins/jev#opt-in-and-data-handling) first.

```bash
finfocus plugin install jev
export TYPESAFE_API_KEY=...
finfocus config set scoring.plugin jev
finfocus config set scoring.enabled true

# See exactly what would be sent, and send nothing
finfocus cost recommendations --pulumi-json plan.json --scoring-dry-run

# Score for real, lowest risk first
finfocus cost recommendations --pulumi-json plan.json --sort risk:asc
```

Without `TYPESAFE_API_KEY` the plugin starts but every scoring call returns `UNAUTHENTICATED`, which appears as a scoring
warning while the recommendations are listed unscored.

For a step-by-step run with real output, see the [Jev Scorer Plugin walkthrough](../plugins/jev.md).

## Use Scores

| Signal                  | Scale  | Higher means                            |
| ----------------------- | ------ | --------------------------------------- |
| `risk`                  | 0 to 1 | Acting is riskier                       |
| `false_positive`        | 0 to 1 | More likely in this state on purpose    |
| `worth_acting`          | 0 to 1 | More worth an engineer's time           |
| `priority`              | 0 to 3 | 0 ignore, 1 low, 2 medium, 3 high       |
| `insufficient_evidence` | 0 to 1 | The record is thinner or contradictory  |

```bash
# Lowest-risk recommendations first
finfocus cost recommendations --pulumi-json plan.json --sort risk:asc

# Highest priority first, only recommendations at or below 0.3 risk
finfocus cost recommendations --pulumi-json plan.json --sort priority --filter "risk<=0.3"

# Medium or high priority only (priority is on a 0 to 3 scale)
finfocus cost recommendations --pulumi-json plan.json --filter "priority>=2"

# JSON with scores and a scoring summary
finfocus cost recommendations --pulumi-json plan.json --output json

# Skip scoring for one run
finfocus cost recommendations --pulumi-json plan.json --no-scoring
```

- `--sort` accepts `risk`, `false_positive`, `worth_acting`, `priority` and `insufficient_evidence`, with an optional
  `:asc` or `:desc` (default `desc`). Recommendations without that score always sort last. Ties break by savings.
- `--filter` accepts `signal<op>value` expressions with `<`, `<=`, `>`, `>=` and `=`, for example `risk<=0.3` or
  `worth_acting>=0.6`. Several filters combine with AND, and they combine with `action=` filters. Recommendations
  without the filtered score are dropped. A value outside the signal's scale, such as `risk<=30` or `priority>=4`, is an
  error.
- Sorting or filtering by a score without scoring enabled is an error, not a silent no-op.

The table gains `RISK`, `FALSE POS`, `WORTH`, `PRIORITY`, `REVIEW` and `GROUP` columns when scores exist, JSON and NDJSON
gain a `scores` object per recommendation and a `scoring` summary, and the interactive view shows the risk score, a
`REVIEW` marker and the duplicate group.

### The needs-review marker

`REVIEW` (JSON `needs_review`) marks a recommendation whose risk, false-positive or thin-evidence score reaches its
threshold. Each threshold is widened downward by half the dead band, so with a risk threshold of 0.3 and a dead band of
0.1, a risk of 0.25 is still flagged. Scores from a model move slightly between identical calls (about 0.03 on average and
0.07 at most in the reference probe), so a recommendation that lands near a threshold should go to a person rather than
be decided by a decimal.

Scorers say in `calibration` whether their numbers are probabilities. Most are `ranking_only`: the values order
recommendations, but a 0.3 is not a 30 percent chance. Tune the thresholds on your own data, and again when the scorer's
model version changes. The reference probe was 80 synthetic recommendations, so treat it as a starting point only.

### Duplicate groups

Recommendations that describe the same change to the same resource share a `duplicate_group_id` (`dup-1`, `dup-2`, and so
on). The list is not merged or reduced: every recommendation stays visible, and the group id is only a label. Groups are
found within one scorer response, so recommendations split across batches are not compared. With `identifier_mode:
omitted`, scorers usually cannot group.

## Data Handling

Scoring is the one place where core sends recommendation content to a plugin that might forward it off the host, for
example to a hosted model. Treat the scorer as you would any service that receives your infrastructure metadata.

**What is sent.** A `ScoreRecommendations` request carries the recommendations as plugins returned them, limited by
`field_allowlist`: category, action type, impact and cost detail, resource information (provider, type, region, SKU, tags,
utilization), description, reasoning, source, metadata, priority, confidence, creation time, the provider-specific
`action_detail`, and the `primary_reason` and `secondary_reasons` codes.

**Identifiers.** Core applies `identifier_mode` before the request leaves the host, so a scorer never has to be trusted
to do it:

| Mode                      | What the scorer receives                                                                                                                                 |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pseudonymized` (default) | `resource.id` and `resource.name` replaced by opaque tokens. The same resource gets the same token within a request, so duplicate detection still works. |
| `omitted`                 | `resource.id` and `resource.name` removed. Duplicate grouping becomes unreliable.                                                                        |
| `raw`                     | Cloud identifiers exactly as plugins returned them. Use only as an explicit opt-in.                                                                      |

In `pseudonymized` and `omitted` modes core also:

- Replaces the plugin's recommendation id with a per-request index (`rec-0`, `rec-1`), because plugin ids can embed
  resource ids.
- Replaces occurrences of the raw resource id and name inside the description, reasoning, tags and metadata values, and
  inside free text in `action_detail` (such as a termination reason or modify config values).
- Applies the same mode to identifiers inside `action_detail`: a Kubernetes `cluster_id`, `namespace`,
  `controller_name` and `container_name` are pseudonymized or removed exactly like `resource.id` and `resource.name`.
- Derives tokens with HMAC-SHA256 over the normalized (trimmed, lower-case) resource id, using a random key generated for
  each request and never stored. Tokens are not comparable across requests.

**What is not protected.** Tags, metadata and free text can still contain sensitive values that are not the resource id
or name, such as team names, account names or hostnames. Identifier handling alone does not make a request safe to send.
Use `field_allowlist` to drop fields you do not want a scorer to see, and read the scorer plugin's own data-handling
documentation, which must say what leaves the host.

**Dry run.** `--scoring-dry-run` prints exactly the requests that would be sent, after identifier handling and the
allowlist, and sends nothing:

```bash
finfocus cost recommendations --pulumi-json plan.json --scoring-dry-run
```

Recommendations already in the score cache are not part of the output, because they would not be sent.

**Score cache.** Scores are cached in the `scores` bucket of the existing BoltDB cache (`~/.finfocus/cache/cache.db`),
keyed on a hash of the recommendation content plus the scorer name, plugin version and model. Only the extracted score
values are stored. Raw scorer requests, responses and provider request ids are never persisted, and the resource id and
name are not part of the hashed content in `pseudonymized` and `omitted` modes. Deleting the cache is always safe. The cache
follows the normal `cost.cache` settings, so with the cache disabled, nothing is stored.

## When Scoring Is Unavailable

Scoring never fails the command. If the scorer is not installed, does not advertise the capability, times out, returns an
error or returns an invalid response, FinFocus prints a warning to stderr, lists it under `scoring.warnings` in JSON
output, and shows the recommendations unscored. Errors for individual recommendations leave only those unscored.

## How It Runs

1. Plugins return recommendations, and dismissed recommendations are excluded as usual.
2. The scorer plugin is looked up by `scoring.plugin` and must advertise `recommendation_scoring`. Scorer-only plugins are
   not queried for recommendations.
3. Scores found in the cache are applied. The rest are sent in batches. The first call is made alone with up to 20
   recommendations to learn the scorer's `max_batch_size`; the remaining batches then run up to 8 at a time, each with the
   configured timeout.
4. Every response is validated against the scorer contract before any score is used.

Dismissed and snoozed recommendations shown with `--include-dismissed` are not scored.

## See Also

- [Recommendations Guide](./recommendations.md)
- [Cache Configuration Guide](./cache.md)
- [Configuration Reference](../reference/config-reference.md)
- [CLI Commands](../reference/cli-commands.md#cost-recommendations)
