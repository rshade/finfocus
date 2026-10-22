---
title: Recommendation Scorer Plugin
description: Walkthrough of the Jev and Clef recommendation scorer, with real output from a dry run, an unauthenticated run, and the JSON scoring summary.
layout: docs
parent: Plugins
nav_order: 20
---

## Overview

The `scorer` plugin rates cost recommendations that other plugins produce. It sends them to the selected
[Jev](https://typesafe.ai/) (from TypeSafe AI) or
[Clef](https://blog.cloudflare.com/clef-decision-models/) backend and returns a risk, false-positive,
worth-acting, priority and thin-evidence score for each one, plus duplicate groups. Core uses the scores to
sort, filter and flag recommendations for review. **A score never approves, dismisses or applies anything.**

This page walks through a run end to end. The configuration reference is the
[Recommendation Scoring Guide](../guides/recommendation-scoring.md), and the data-handling details are in the
[plugin README](https://github.com/rshade/finfocus/tree/main/plugins/scorer).

## What You Need

- `finfocus` and two plugins: `aws-public` for the recommendations and `scorer` for the scores
- A `SCORER_API_KEY` for the selected backend (needed only for step 4)
- The demo plan `examples/plans/aws-legacy-plan.json`. It holds an `m4.large`, a `t2.medium` and a 200 GB `gp2`
  volume in `us-east-1`, so `aws-public` produces one recommendation for each

```bash
finfocus plugin install aws-public
make install-scorer  # from the repository root; public scorer release pending
finfocus config set scoring.plugin scorer
finfocus config set scoring.enabled true
```

Set `scoring.plugin` before `scoring.enabled`. Every config change is validated, and scoring cannot be enabled
without a plugin.

## 1. The Recommendations Without Scores

`--no-scoring` skips the scorer for one run:

```bash
finfocus cost recommendations --pulumi-json examples/plans/aws-legacy-plan.json --no-scoring --output table
```

```text
RECOMMENDATIONS SUMMARY
=======================
Total Recommendations: 3
Total Potential Savings: 10.42 USD

By Action Type:
  RECOMMENDATION_ACTION_TYPE_MODIFY: 3 (10.42 USD)

TOP 3 RECOMMENDATIONS BY SAVINGS
----------------------------------------
RESOURCE                                                             ACTION TYPE                        DESCRIPTION                                         SAVINGS
--------                                                             -----------                        -----------                                         -------
urn:pulumi:dev::legacy-app::aws:ebs/volume:Volume::data-volume       RECOMMENDATION_ACTION_TYPE_MODIFY  Upgrade 200GB gp2 volume to gp3 for ~20% cost s...  4.00 USD
urn:pulumi:dev::legacy-app::aws:ec2/instance:Instance::batch-worker  RECOMMENDATION_ACTION_TYPE_MODIFY  Upgrade from t2.medium to t3.medium for better ...  3.50 USD
urn:pulumi:dev::legacy-app::aws:ec2/instance:Instance::api-server    RECOMMENDATION_ACTION_TYPE_MODIFY  Upgrade from m4.large to m5.large for better pe...  2.92 USD
```

The list is ordered by savings, which is the only ranking available without a scorer.

## 2. See What Would Be Sent (Dry Run)

Run this before you set an API key. `--scoring-dry-run` prints the scorer requests exactly as core would send them,
after identifier handling and `field_allowlist`, and sends nothing. The plugin must be installed, because core starts
it to check the scoring capability, but no scoring request reaches it and no key is needed.

```bash
finfocus cost recommendations --pulumi-json examples/plans/aws-legacy-plan.json --scoring-dry-run
```

Output, trimmed to one of the three recommendations:

```json
{
  "dry_run": true,
  "identifier_mode": "pseudonymized",
  "note": "These requests are exactly what would be sent to the scorer plugin. Nothing was sent. Recommendations already in the score cache are not included.",
  "requests": [
    {
      "recommendations": [
        {
          "id": "rec-2",
          "category": "RECOMMENDATION_CATEGORY_COST",
          "action_type": "RECOMMENDATION_ACTION_TYPE_MODIFY",
          "resource": {
            "id": "res-e2e9c3f63a174894",
            "provider": "aws",
            "resource_type": "ebs",
            "region": "us-east-1",
            "sku": "gp2"
          },
          "modify": {
            "modification_type": "volume_type_upgrade",
            "current_config": {
              "size_gb": "200",
              "volume_type": "gp2"
            },
            "recommended_config": {
              "size_gb": "200",
              "volume_type": "gp3"
            }
          },
          "impact": {
            "estimated_savings": 4,
            "currency": "USD",
            "projection_period": "monthly",
            "current_cost": 20,
            "projected_cost": 16,
            "savings_percentage": 20
          },
          "priority": "RECOMMENDATION_PRIORITY_MEDIUM",
          "confidence_score": 0.9,
          "description": "Upgrade 200GB gp2 volume to gp3 for ~20% cost savings",
          "reasoning": [
            "gp3 volumes are ~20% cheaper than gp2",
            "gp3 provides better baseline performance (3000 IOPS, 125 MB/s)",
            "API-compatible change with no data migration required"
          ],
          "source": "aws-public",
          "metadata": {
            "baseline_iops": "gp2: 100 IOPS/GB, gp3: 3000 IOPS (included)",
            "baseline_throughput": "gp2: 128-250 MB/s, gp3: 125 MB/s (included)"
          }
        }
      ],
      "identifier_mode": "IDENTIFIER_MODE_PSEUDONYMIZED"
    }
  ]
}
```

In this output:

- **The recommendation id is `rec-2`**, not the plugin's id. Ids are replaced with per-request indexes because plugin
  ids can contain resource ids.
- **The resource id is `res-e2e9c3f63a174894`**, not the Pulumi URN. The token is an HMAC with a random per-request key,
  so it cannot be reversed or matched across runs. Within one request the same resource always gets the same token,
  which is what lets the scorer find duplicates.
- **The resource name is absent.** In `pseudonymized` mode core replaces the id and name and removes them from free
  text. Tags and metadata that hold other sensitive values are still sent; narrow them with `scoring.field_allowlist`.
- **Everything else is sent as is**: description, reasoning, metadata, cost impact and the `modify` action detail. The
  plugin forwards these fields to the selected backend.

## 3. Run Without an API Key

If the plugin is enabled but `SCORER_API_KEY` is not set, the plugin still starts, and every scoring call fails with
`UNAUTHENTICATED`. Scoring never fails the command: the recommendations are listed unscored and the exit code is 0.

```bash
finfocus cost recommendations --pulumi-json examples/plans/aws-legacy-plan.json --output table
```

```text
RECOMMENDATIONS SUMMARY
=======================
Total Recommendations: 3
Total Potential Savings: 10.42 USD

By Action Type:
  RECOMMENDATION_ACTION_TYPE_MODIFY: 3 (10.42 USD)

Scoring: scorer (unspecified): scored 0 of 3, 0 need review. Scores rank work for review; they never dismiss or apply a recommendation.

TOP 3 RECOMMENDATIONS BY SAVINGS
----------------------------------------
RESOURCE                                                             ACTION TYPE                        DESCRIPTION                                         SAVINGS
--------                                                             -----------                        -----------                                         -------
urn:pulumi:dev::legacy-app::aws:ebs/volume:Volume::data-volume       RECOMMENDATION_ACTION_TYPE_MODIFY  Upgrade 200GB gp2 volume to gp3 for ~20% cost s...  4.00 USD
urn:pulumi:dev::legacy-app::aws:ec2/instance:Instance::batch-worker  RECOMMENDATION_ACTION_TYPE_MODIFY  Upgrade from t2.medium to t3.medium for better ...  3.50 USD
urn:pulumi:dev::legacy-app::aws:ec2/instance:Instance::api-server    RECOMMENDATION_ACTION_TYPE_MODIFY  Upgrade from m4.large to m5.large for better pe...  2.92 USD
Warning: scoring: scorer call failed (Unauthenticated: SCORER_API_KEY is not set, so the scorer cannot call the backend); affected recommendations are unscored
```

The warning goes to stderr. `(unspecified)` is the scorer's calibration, which is unknown because no call succeeded.
JSON output carries the same failure in the `scoring` summary:

```bash
finfocus cost recommendations --pulumi-json examples/plans/aws-legacy-plan.json --output json | jq .scoring
```

```json
{
  "scorer": "scorer",
  "requested": 3,
  "scored": 0,
  "unscored": 3,
  "warnings": [
    "scorer call failed (Unauthenticated: SCORER_API_KEY is not set, so the scorer cannot call the backend); affected recommendations are unscored"
  ]
}
```

In CI, check `.scoring.unscored` (or `.scoring.warnings`) to catch a missing or expired key.

## 4. Score for Real

Set the key in the environment the plugin starts from, and keep it out of shell history and committed files:

```bash
export SCORER_BACKEND=jev  # or clef; see backend configuration below
export SCORER_API_KEY=...
finfocus cost recommendations --pulumi-json examples/plans/aws-legacy-plan.json --sort risk:asc
```

With scores, the table gains `RISK`, `FALSE POS`, `WORTH`, `PRIORITY`, `REVIEW` and `GROUP` columns. In JSON, each
recommendation gains a `scores` object and `needs_review`, and the `scoring` summary reports the model and
`ranking_only` calibration. Useful variations:

```bash
# Highest priority first, only recommendations at or below 0.3 risk
finfocus cost recommendations --pulumi-json examples/plans/aws-legacy-plan.json --sort priority --filter "risk<=0.3"

# Only items worth an engineer's time
finfocus cost recommendations --pulumi-json examples/plans/aws-legacy-plan.json --filter "worth_acting>=0.6"
```

Both backends report `ranking_only`. Use scores to order work for review;
a risk of 0.3 is not a calibrated 30 percent chance. Scores are cached in the
`scores` bucket, so the same recommendations can reuse prior results. No Clef
accuracy, price or calibration claim is established by the recorded spike.

## Backend Configuration and Migration

Jev is the default (`SCORER_BACKEND=jev`, model `jev-1.13.0`). Hosted Clef uses
`SCORER_BACKEND=clef`, model `clef`, a Cloudflare token in `SCORER_API_KEY`, and
`SCORER_CLEF_ACCOUNT_ID` containing a 32-character hexadecimal account id.
`SCORER_BASE_URL` is an optional origin: `https://api.typesafe.ai` for Jev or
`https://api.cloudflare.com` for Clef. The Clef adapter appends the account path.
For Clef, the origin must have no path, query, user info or fragment, and
`SCORER_MODEL` must be `clef`; other model ids are rejected.
`SCORER_TIMEOUT`, `SCORER_BATCH_SIZE` and `SCORER_DUPLICATE_THRESHOLD` default to
`60s`, `25` and `0.5` respectively.

The scorer does not read the legacy settings. Migrate them explicitly:

| Removed setting | Replacement |
| --- | --- |
| `TYPESAFE_API_KEY` | `SCORER_API_KEY` |
| `JEV_MODEL` | `SCORER_MODEL` |
| `JEV_BASE_URL` | `SCORER_BASE_URL` |
| `JEV_TIMEOUT` | `SCORER_TIMEOUT` |
| `JEV_BATCH_SIZE` | `SCORER_BATCH_SIZE` |
| `JEV_DUPLICATE_THRESHOLD` | `SCORER_DUPLICATE_THRESHOLD` |
| `scoring.plugin: jev` | `scoring.plugin: scorer` |

The first public scorer release and registry entry are pending; use
`make install-scorer` from source now. Once release assets exist,
`finfocus plugin install scorer` will be the registry command. The retained
`finfocus plugin install jev` command installs the last legacy release
separately and does not migrate settings. If Jev is already installed, the
command reports `already installed`; install scorer explicitly and change
`scoring.plugin` to migrate. Historical Jev tags remain available.

Hosted Jev and Clef both send recommendation data to third parties. Use the dry
run and `field_allowlist` to inspect and narrow that data. A compatible local
endpoint may avoid hosted calls; Clef self-hosting and vLLM compatibility remain
untested and outside this change's scope.

[Cloudflare's Clef announcement](https://blog.cloudflare.com/clef-decision-models/)
and the [go-decide v0.1.0 adapter](https://github.com/rshade/go-decide/blob/v0.1.0/clefclient/transport.go)
provide the upstream context. The
[recorded spike](https://github.com/rshade/finfocus/tree/main/specs/1728-clef-spike)
passed `noul` and `score` decoding but rejected `choice`; the scorer sends no
`choice` questions. No live calls were made and no credentials were accessed.
This evidence does not validate live Clef scores.

## Troubleshooting

| Symptom                                                    | Cause and fix                                                                                                    |
| ---------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `--scoring-dry-run requires scoring`                       | Set `scoring.plugin` and `scoring.enabled` first                                                                 |
| `SCORER_API_KEY is not set` warning                      | Export the key in the shell that runs `finfocus`; the plugin inherits its environment                            |
| `requests: []` in the dry run                              | No recommendations to score, or they are all already in the score cache. Check the output with `--no-scoring` |
| Rate-limit or outage warnings                              | Raise `scoring.timeout_seconds`. It bounds the whole call, including the plugin's retries                        |
| Sorting or filtering by a score is an error                | Scoring is disabled, or `--no-scoring` was passed                                                                |

## See Also

- [Recommendation Scoring Guide](../guides/recommendation-scoring.md)
- [Recommendations Guide](../guides/recommendations.md)
- [Scorer plugin README](https://github.com/rshade/finfocus/tree/main/plugins/scorer)
