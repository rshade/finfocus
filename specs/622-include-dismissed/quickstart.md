# Quickstart: include_dismissed on cost recommendations

```bash
finfocus cost recommendations --pulumi-json plan.json --include-dismissed
```

The plugin request sets `include_dismissed` and still lists locally dismissed IDs in
`excluded_recommendation_ids`. The table merges local dismissed and snoozed rows as it does today.
A plugin that stores its own dismissals also returns those, except IDs the host excluded.

```bash
go test ./internal/engine/ ./internal/proto/ ./internal/cli/ -count=1 \
  -run 'IncludeDismissed|GetRecommendations'
make test
make lint
```

The module pin follows finfocus-spec branch `599-include-dismissed`. Move it to the release tag
when that release ships, and add an upgrade hop only if `pluginsdk.SpecVersion` changes.
