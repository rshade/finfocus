# Quickstart: Cost History on the Web

## Command line

```bash
finfocus cost history collect --stack dev
finfocus cost history view --stack dev --output json
finfocus cost history diff --stack dev --from v1 --output json
```

## Browser session

Start `finfocus --web --no-browser --port <port>` from the project, open the
printed URL, and wait for the overview to load. Then request:

```text
GET /api/history/view?from=2026-01-01&currency=USD
GET /api/history/diff?from=v1&to=v3&threshold=1
GET /api/history/resource?id=<urn>
```

`view` and `diff` match the command-line JSON plus `status` and `display`.
A stack that was never collected answers `{"status":"no_history"}`. A bad
selector or date is `400 invalid_history_query` with the command line's
message. A damaged database is `502 history_unavailable`.

The page does not draw this data yet; that is the `web-ui` follow-up.

## Checks

```bash
go test -count=1 -run 'History' ./internal/history/ ./internal/webui/ ./internal/cli/
make test
make lint
make validate
make test-e2e-web
```
