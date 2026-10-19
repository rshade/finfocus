# Contract: `cost recommendations history`

```text
finfocus cost recommendations history <recommendation-id> [--output table|json|ndjson]
```

The command does not accept a Pulumi plan and does not require a plugin name.
It opens every installed plugin, queries history, and closes those connections.

## Results

- Table columns: timestamp, action, reason, note, expires, actor.
- Actor is blank when absent. JSON and NDJSON include `actor` only when set.
- No events and table output print the existing "no history" line.
- JSON is one object with `recommendation_id`, `events`, and `event_count`.
- NDJSON is one object per event, with `recommendation_id` and `event`.
- An empty recommendation id is a command error.
- An unsupported `--output` value is a command error before plugins open.

## Failures

- Plugin unsupported, empty, unreachable, timed out, or any plugin status,
  including a plugin's own cancellation or deadline: stderr or log diagnostic,
  local and other plugin events still print, exit success when the local read
  succeeded.
- The caller's context being canceled or past its deadline: the command
  returns that error and does not print a successful history listing.
- The dismissal file is not rewritten on any of these paths.
