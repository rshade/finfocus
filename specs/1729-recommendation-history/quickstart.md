# Quickstart: Recommendation History

## Preconditions

- Repository dependencies are downloaded (`go mod download` if needed).
- finfocus-spec v0.7.7 is the pinned module. No local replace is required.

## Validate the behavior

From the repository root:

```bash
go test -count=1 -timeout 120s ./internal/config/ ./internal/proto/ ./internal/engine/ ./internal/cli/
go test -race -count=1 -timeout 180s -run 'RecommendationHistory|HistoryCmd|DismissalStore_LoadActor' \
  ./internal/config/ ./internal/proto/ ./internal/engine/ ./internal/cli/
```

Repository gates when the change is complete:

```bash
make validate
make test
make lint
make docs-lint
```

## What the tests cover

- A plugin-only history, including actor, in the engine and the command output.
- Chronological merge, duplicate collapse that keeps an actor, and distinct actors.
- Unsupported plugins, backend errors, cancellation, and malformed events.
- An older dismissal file without `actor` still loads, and a history read does
  not change the file.

## Manual check

With no plugins installed, and a dismissal already recorded:

```bash
finfocus cost recommendations history <recommendation-id>
finfocus cost recommendations history <recommendation-id> --output json
```

The table includes an actor column. JSON omits `actor` when the local event
has none. The dismissal file is unchanged.
