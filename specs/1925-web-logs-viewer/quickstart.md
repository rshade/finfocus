# Quickstart: Web UI Log Viewer

Use this to see the feature work end to end once it is built. Paths are
relative to the repository root; `<port>` is any free port.

## Build

```bash
make build
```

## 1. Off by default

```bash
cd ../finfocus-demo
PULUMI_CONFIG_PASSPHRASE='' ../finfocus/bin/finfocus --web --no-browser --port <port>
```

Open the printed URL. Expect no Logs entry in the navigation, and no failed
request in the browser console. From the same shell, with the session cookie,
`GET /api/logs/status` returns `200` with `"enabled": false`, and
`GET /api/logs/stream` returns `404`.

## 2. On through the environment

```bash
FINFOCUS_WEB_LOGS=1 PULUMI_CONFIG_PASSPHRASE='' \
  ../finfocus/bin/finfocus --web --no-browser --port <port>
```

Expect a Logs entry. Open it and confirm:

- Lines from `core` and from each plugin, oldest first, then live.
- Each line has a text level badge as well as colour.
- Source chips filter by plugin; the level select and search narrow further.
- Pause stops scrolling and counts new lines; Clear empties the view;
  Download saves the visible lines as `.ndjson`.

Add `--debug` (or `FINFOCUS_LOG_LEVEL=debug`) for debug lines.

## 3. On through config, and precedence

```bash
finfocus config set web.logs.enabled true
```

The view appears with no environment variable. Then:

```bash
FINFOCUS_WEB_LOGS=0 finfocus --web ...      # view absent: env wins
FINFOCUS_WEB_LOGS=maybe finfocus --web ...  # exit 2, validation error
```

## 4. Redaction

Run a plugin or fixture that logs a field named `api_token` and a Pulumi
secret value. In the page, in the stream
(`curl -N --cookie <session cookie> http://127.0.0.1:<port>/api/logs/stream`),
and in the downloaded file, the value is `"[redacted]"`.

## 5. Log file unchanged

With `logging.file` set, run once with and once without `FINFOCUS_WEB_LOGS=1`.
The file keeps receiving core and plugin lines in both runs.

## 6. Viewer package on its own

```bash
node --test packages/zerolog-viewer/test/*.test.mjs
```

Passes with no finfocus code involved. Adding an import that leaves
`packages/zerolog-viewer/src/` makes the boundary test fail.

## Stop

```bash
pkill -f '^<path>/finfocus --web'
ps -ef | grep '[f]infocus-plugin'    # expect no orphans
```
