# HTTP Contract: Stack List and Activation

Both routes exist on the root server only, behind the same Host check,
authentication, and Origin hardening as every route (spec 2012, FR-002).
Neither registers on a stack's own tree.

## Go methods the hook calls

| Endpoint | Go method |
| --- | --- |
| `GET /api/stacks` | `(*webui.Server).stackEntries`, reading `(*webui.Session).IsReady`, `(*webui.Session).Failed` (a failure is recorded and the pipeline has ended, set through `(*webui.Session).SetPipelineEnded`), and `(*webui.Session).PassphraseRequired` |
| `POST /api/stacks/{name}/activate` | `(*webui.Server).activateStack`, which runs `webui.StackOptions.Activate` once; the CLI supplies `(*cli.webStack).start` |
| CLI validation | `pulumi.ListStacks`, `pulumi.ValidateStacks`, `webui.ValidStackName` |
| Shell stack list | `(*webui.Server).stacksMeta`, written into the root page by `handleRoot` |
| Registration | `(*webui.Server).AddStack` (existing) |

No engine logic is added. Each stack's views call the same methods as before,
through its own session.

## Shell

The root page carries `<meta name="finfocus-stacks" content="prod,dev,test">`,
the registered names in registration order, only when two or more stacks are
registered. A stack's own page (`/s/<name>/`) and a server with fewer than two
stacks carry no such tag, so the browser shows no tabs. The browser reads it at
boot and makes no request to decide.

## `GET /api/stacks`

```json
{ "stacks": [ { "name": "dev", "state": "ready" }, { "name": "prod", "state": "idle" } ] }
```

`state` is one of `idle`, `loading`, `locked`, `ready`, `failed`. With no stack
registered: `{ "stacks": [] }`. Order is registration order. The body is
written with `writePlainJSON`: it holds only stack names and fixed state words,
so no redaction applies, and a name is already restricted to
`[A-Za-z0-9_.-]` by `webui.ValidStackName`.

## `POST /api/stacks/{name}/activate`

- Empty or `{}` body. Idempotent.
- `202` `{ "name": "prod", "state": "loading" }` (or the current state).
- `404` `{ "error": "unknown stack", "code": "not_found" }` for an
  unregistered name, including an invalid segment.

## Errors

Standard `{error, code}` shape. Failure text on a stack's own events is the
existing safe labels ("Overview loading failed", or "Unable to decrypt stack.
Try the passphrase again." for a rejected passphrase); no plugin or Pulumi
text.
