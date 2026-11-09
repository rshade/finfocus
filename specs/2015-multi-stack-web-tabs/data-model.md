# Data Model: Multi-Stack Web Dashboard

## Stack list (CLI)

Ordered, de-duplicated names from `--stack` (values split on commas). Each is a valid URL path segment
(`webui.ValidStackName`) and exists in `pulumi stack ls`.

## Stack entry (server)

| Field | Meaning |
| --- | --- |
| `name` | Stack name; the `/s/<name>/` segment |
| `state` | `idle` (not activated), `loading`, `locked` (passphrase prompt pending), `ready`, `failed` |

State transitions: `idle` → `loading` on first activation → `ready`, `failed`,
or `locked`. `locked` → `loading` when a passphrase is submitted, then `ready`,
`locked` again (rejected), or `failed`. `locked` wins over a recorded failure,
because a rejected passphrase records one and re-prompts. A recorded failure
reads `failed` only after the pipeline has ended (`Session.SetPipelineEnded`,
which also sends a snapshot so the browser re-reads the states); while a retry
is still loading it reads `loading`. A stack that reached `ready` stays `ready`,
even if a later on-demand preview fails. `failed` is terminal for the
process (a restart retries); `ready` stays ready.

## Browser state

| Field | Meaning |
| --- | --- |
| `stacks` | Names from the shell's `finfocus-stacks` meta tag, each `idle` until `GET /api/stacks` reports its state |
| `activeStack` | Name parsed from `#/s/<name>/<view>`; first entry when absent or unknown |
| `multi` | True when the page lists two or more valid stack names and its base is empty |
