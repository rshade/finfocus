# Contract: pulumi-language-finfocus

**Requirements**: FR-001 to FR-003, FR-007, FR-008 | **Research**: R1 to R5, R10

The FinFocus binary started under the name `pulumi-language-finfocus` serves
Pulumi's `pulumirpc.LanguageRuntime` service
(`github.com/pulumi/pulumi/sdk/v3/proto/go`, `language.proto`). It is a Pulumi
plugin, not a FinFocus plugin, and changes no `finfocus-spec` protocol.

## Process

- Pulumi starts it with the engine address as the only argument.
- It listens on `127.0.0.1:0` and prints only the chosen port and a newline on
  standard output. Logs go to standard error.
- SIGINT or SIGTERM stops it gracefully: running `RunPlugin` calls are
  canceled, their analyzers are stopped as below, then the process exits 0.
  The whole shutdown is capped at 4 seconds (`WaitDelay` + 1s); after that
  the server calls `Stop`. Pulumi force-kills a plugin 5 seconds after
  interrupting it.

## Go entry points

| Symbol | Role |
| --- | --- |
| `cli.RunAnalyzerLanguageHost(cmd *cobra.Command) error` | called by `main.go` for the language host name; sets up stderr logging, listens, prints the port, serves until a signal |
| `analyzer.NewLanguageHost(opts analyzer.LanguageHostOptions) *analyzer.LanguageHost` | the `pulumirpc.LanguageRuntimeServer`; `opts.Executable` and `opts.AnalyzerName` are injectable for tests |
| `analyzer.LanguagePluginBinaryName` | `"pulumi-language-finfocus"` |
| `analyzer.PolicyPackEntryPath(dir, name string) string` | entry path with the platform suffix |

## RPCs

| RPC | Response |
| --- | --- |
| `GetPluginInfo` | `version` = `version.GetVersion()` |
| `Handshake` | empty `LanguageHandshakeResponse` |
| `GetRequiredPlugins` | empty |
| `GetRequiredPackages` | empty |
| `GetProgramDependencies` | empty |
| `InstallDependencies` | no output, success |
| `About` | `executable` = own path, `version` = FinFocus version |
| `RunPlugin` | see below |
| all others | `Unimplemented` (embedded server) |

### RunPlugin

Preconditions, each an `InvalidArgument` status when broken: `args` has at
least one element; `info` is set and `info.programDirectory` is absolute;
`kind` is `analyzer`.

Behavior:

1. Start `opts.Executable` with `Args[0] = opts.AnalyzerName`
   (`pulumi-analyzer-policy-finfocus`, `.exe` on Windows), then
   `args[0]` and `"."`.
2. Working directory: `info.programDirectory`.
3. Environment: the language host's environment, then `req.env`.
4. Stream the child's stdout and stderr as `RunPluginResponse` messages, bytes
   unchanged, so the first stdout line remains the analyzer's port.
5. When the stream's context is canceled: Unix sends SIGINT, Windows kills;
   `WaitDelay` is 3 seconds.
6. Non-zero exit: send `RunPluginResponse{exitcode}` and return nil. Failure
   to start: return the error.

## Invariants (tested)

- Starting through a symlink named `pulumi-language-finfocus` runs analyzer
  mode in the child, never the general CLI (R4).
- Nothing but the port reaches the language host's own standard output.
- After a canceled or finished preview, no analyzer process and, on Linux and
  macOS, no FinFocus cost plugin process it started is still running
  (SC-005).
- The analyzer receives the stack and project from `ConfigureStack` on this
  path: they appear in `last-cost-summary.json` (US1 AS4).
