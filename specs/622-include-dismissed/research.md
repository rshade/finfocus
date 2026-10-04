# Research: Pass include_dismissed From cost recommendations

## R1. Keep sending excluded IDs

**Decision**: `--include-dismissed` sets the proto field and still sends
`excluded_recommendation_ids`.

**Rationale**: Plugins built before the field ignore it. If core stopped sending excluded IDs,
those plugins would return host-dismissed recommendations. `annotateActiveStatus` would label them
Active, and the local merge would skip them because the ID is already present. Sending the IDs
preserves today's list. The new field adds only the plugin's own dismissals, which are not in the
host exclusion list. finfocus-spec research R2 is the same rule.

**Alternatives considered**: Clear the exclusion list when the flag is set (mislabels rows from old
plugins). Send the field only (same failure).

## R2. New method, same signature for the default path

**Decision**: `GetRecommendationsForResources` stays as it is and calls an unexported function with
the flag false. `GetRecommendationsForResourcesWithDismissed` is the flagged entry. Overview and
the analyzer keep the default method.

**Rationale**: The calculator interface and the test doubles use the two-argument method. Adding a
parameter would churn every mock. A context value would hide the flag.

**Alternatives considered**: An options struct on the existing method (breaks every caller). A field
on `Engine` (races if two calls share an engine).

## R3. Cache suffix

**Decision**: When the flag is true, append `/include-dismissed` to the existing recommendations
cache key. When it is false, the key is unchanged.

**Rationale**: The hash already covers excluded IDs. It does not cover the new field. A suffix
avoids changing `HashRecommendationInputs` and leaves stored default entries valid.

**Alternatives considered**: Add a bool to `HashRecommendationInputs` (changes a tested helper and
would rewrite default keys if the encoding changes). One cache entry for both (wrong answers).
