# Data Model: Pass include_dismissed

## proto.GetRecommendationsRequest.IncludeDismissed

| Property | Value |
| --- | --- |
| Type | `bool` |
| Set true | `cost recommendations --include-dismissed` only |
| Set false | Every other recommendations fetch |
| Copied to | `pbc.GetRecommendationsRequest.include_dismissed` |

## Cache key

| Request | Key |
| --- | --- |
| Default | `recommendations/multi/{types}/{hash}` (unchanged) |
| Flagged | that key plus `/include-dismissed` |

`hash` is `HashRecommendationInputs`, which already includes excluded IDs.

## Unchanged

Local dismissal records, the merge that appends them, and `excluded_recommendation_ids` contents.
No new file, bucket, or recommendation status.
