# internal/esc

`esc` opens Pulumi ESC environments and returns the environment variables a
plugin may receive. Callers decide which names a plugin is allowed to see.
This package does not start plugins and does not read finfocus config files.

## API

- `ParseRef` / `Ref.String` parse `[[org/]project/]name[@version]`.
- `Failure` is a redacted error. `Error` returns fixed warning text and never
  includes a variable value or an SDK error body.
- `Source` resolves one command's environments. `Delivery` returns names,
  values, the winning reference, and a revision key. A failure is reported
  through `Options.Warn` and yields an empty delivery.
- `WithSource` / `SourceFromContext` store the `Source` on a context.
- `(*Source).Options` and `Fresh` copy the options into a new resolver.
- `(*Source).Variables` returns the extra names configured for one plugin.

`Delivery` is safe to call from more than one goroutine. One reference is
opened once. The wait budget (five seconds in the CLI) is charged only while
a call is in flight.

## Import rules

This package may import `internal/logging`, `internal/pulumi`, the plugin
SDK, and `github.com/pulumi/esc-sdk/sdk`. It must not import `cli`, `engine`,
`pluginhost`, `registry`, `config`, or `webui`.

## Redaction

An ESC value stays in memory for the life of the `Source` that read it. It
is not logged, placed in an error, or written to disk. Cache callers hash
`name`, the reference, and the revision key. They do not hash the value.

The SDK client is built by hand: `Servers` is `<backend>/api/esc`, redirects
are disabled, and the token is `Authorization: token <value>` on the request
context. Do not use `NewDefaultClient` or `NewAuthContext`. Open calls recover
from SDK panics. Do not read `GenericOpenAPIError.Body()`.

## Tests

`internal/esc/esctest` serves the ESC HTTP paths on a loopback `httptest`
server. Point `PULUMI_BACKEND_URL` at `Server.URL` and set
`PULUMI_ACCESS_TOKEN` to the token passed to `esctest.New`. The server
records open, HEAD, and tag reads so a test can assert that a disabled
selection makes no requests.

## Session discovery

`DiscoverSession(getenv, projectBackend, stackName)` exposes the existing
backend, token, and organization rules to other core clients. Environment
credentials override saved credentials when the backend is selected through
`PULUMI_BACKEND_URL` or the current saved account. A project `backend.url`
uses only credentials saved for that URL, and never receives an environment
token. `Session.Token`
must never be logged or persisted.
