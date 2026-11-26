# Research: Per-Request Credentials for Pooled Plugins

## D1. Names come from configuration

**Decision**: `esc.plugins.<name>.credentials: {credential-name: ESC_VARIABLE}`.

**Rationale**: The contract says names are caller-chosen tokens. Two plugins
already consume the channel with names unrelated to their variable names, and
one returns `InvalidArgument` for any name it does not know.

**Rejected**: lowercased variable names (breaks both); a name table in core
(plugin knowledge in core); a new advertised field (changes finfocus-spec).

## D2. First launch carries the full environment

**Decision**: Support is known only after `GetPluginInfo`. The first launch of
a binary gets today's environment. When the answer advertises support and the
pool has a usable plan, the existing one-time advertised-settings restart
delivers the environment without the declared ESC values. The tenant's secret
process is gone before anything can share it. Later launches are predicted
from what the pool learned, and start without the values.

**Rejected**: withholding on the first launch. A plugin without support that
exits when its credential is missing would stop working.

**Fallback**: if a process launched without the values does not advertise
support (including a failed `GetPluginInfo`), it is relaunched with the full
environment and treated as isolated.

## D3. Withholding is resolution without those ESC values

**Decision**: Resolve with the ESC offer minus the declared variables.
Precedence then works unchanged: a shell, config, or default value remains as
the ambient value, and the digest of the process key excludes the credential.

## D4. Keys

The final key gains a `channel` flag: the running process advertises support
and was launched under a plan. Without it, a process with no credentials in
its environment that does not support the channel could match a stack whose
credentials were withheld. The offer key hashes the declared map, so two
requests with the same offer and different maps never join one launch.

## D5. Attach at the connection, per holder

**Decision**: `Client.Conn` becomes `grpc.ClientConnInterface`. A holder of a
channel process gets a wrapper that adds its credentials with
`pluginsdk.WithCredentials` and writes the headers with
`pluginsdk.CredentialUnaryClientInterceptor`. `API` is rebuilt over it.

**Rationale**: The usage, allocator, scorer, and carbon services are built
from `Conn`. Wrapping `API` alone would miss them. No launcher changes, so a
client without credentials cannot send headers.

**Rejected**: setting the value on engine contexts (every call site must
remember); a second dial per holder (launchers own dialing).

## D6. Digest per holder

The holder's `DeliveredDigest` starts from the digest of its full resolution,
which hashes each ESC reference and revision, so stacks never share projected,
actual, recommendation, or Supports entries. When its calls carry credentials
the declared map is added: remapping a name between two variables the plugin
already receives changes the account and nothing else.

## D7. Masking

`TraceInterceptor` wraps every plugin connection and logs the error. It reads
the set from the call context, so it masks values before logging and before
returning. The launch calls have no set, but the first launch has the values in
its environment, so their errors are masked with the same values as the excerpt
of a plugin that exits early. Status details that contain a value are dropped. Names are not
masked in plugin text: they are the plugin's public vocabulary, and the host
never writes them.

The OpenTelemetry handler records a call's error before any interceptor runs,
and `AX_OTEL_DEBUG` prints it. Launchers therefore dial through `dialPlugin`,
which wraps that handler (`credentialStats`), instead of `ax.GRPCDial`, whose
handler cannot be wrapped. `otelgrpc` was already in the build.

## D8. Re-dial after exit

A gRPC connection re-dials its address. Once the process has exited, another
program may hold that port, and calls now carry secrets. The dialer checks the
process before and after it connects, and refuses when it is gone. The check is
signal 0, which reports `ErrProcessDone` (on Windows `EINVAL`, since `Wait`
releases the handle there) as soon as the process is reaped. The
`exited` channel is later: `cmd.Wait` also waits for output that a surviving
child holds open. TCP has no peer credentials (`SO_PEERCRED` is for Unix
sockets), so the instant between death and reaping remains.

## D9. Port proof

A port is free for a moment between allocation and the plugin's bind, so
another program can answer `GetPluginInfo` on it and claim support. Nothing
secret crossed that connection before; per-request credentials do.

**Decision**: before the host connects to a process that will receive
credentials, that process must have written `PORT=<port>` to its output.
`pluginsdk` writes it right after it listens on 127.0.0.1, and only the launched
process and its children can write to that pipe. A squatter makes the real
plugin exit with a collision, which the launcher retries on a new port. A
plugin that announces nothing within the bind timeout is relaunched with its
credentials in its environment, and the pool remembers it.

**Rejected**: an inherited listener, a Unix socket, or a per-launch certificate
(each changes the plugin contract); a liveness check alone (a squatter answers
while the real plugin is still starting).

## D10. Unusable set

`pluginsdk.NewCredentials` is the one rule. An error makes the plan unusable
for that holder only, with a warning that carries a fixed text and the plugin
name. Values that commonly do not fit: JSON key files (newlines).
