# Research: Analyzer Language Plugin

**Feature**: [spec.md](spec.md) | **Date**: 2026-10-22

Sources: pulumi/pulumi#24784 (diff of `pkg/resource/plugin/analyzer_plugin.go`),
`pkg/resource/plugin/plugin.go` at v3.270.0, `pulumi-policy-opa` v1.1.2
(`cmd/pulumi-analyzer-policy-opa/{main,language}.go`, `.goreleaser.yml`), a
throwaway spike run on 2026-10-22 against Pulumi 3.268.0 and 3.271.0, and the
finfocus code named below.

## R1. How current Pulumi starts a directory policy pack

**Finding**: `NewPolicyAnalyzer` calls `newPlugin` with the policy pack
directory as the binary path. `newPlugin` sees a directory, reads
`PulumiPolicy.yaml`, loads the language runtime named by `runtime`
(`ctx.Host.LanguageRuntime("finfocus")`), and calls `RunPlugin`. The engine
reads the analyzer's port from the first line of the `RunPlugin` stdout stream,
then dials it. There is no fallback after 3.270.0.

The spike logged the request Pulumi sends, identical on 3.268.0 and 3.271.0:
`kind=analyzer`, `args=[<engine address>]`, `pwd=<the Pulumi project directory>`,
`info.programDirectory=<the policy pack directory>`. `env` carries what the
engine sets for plugins.

**Consequence**: `pulumi-language-finfocus` must implement `RunPlugin` and
stream the analyzer's stdout unchanged, so its first line is still the port.

## R2. Which LanguageRuntime methods to implement

**Decision**: Implement `GetPluginInfo`, `Handshake`, `GetRequiredPlugins`,
`GetRequiredPackages`, `GetProgramDependencies`, `InstallDependencies`,
`About`, and `RunPlugin`. Leave the program-oriented methods (`Run`,
`GenerateProgram`, `GenerateProject`, `GeneratePackage`, `Pack`, `Link`,
`Template`, `RuntimeOptionsPrompts`, `Cancel`) to the embedded
`UnimplementedLanguageRuntimeServer`.

**Rationale**: The spike needed only the first four plus `RunPlugin` for
`pulumi preview`. `pulumi-policy-opa` v1.1.2 also answers
`GetProgramDependencies`, `InstallDependencies`, and `About`, which
`pulumi install` and `pulumi about` call on a policy pack; a FinFocus pack has
no dependencies, so each answers empty. The remaining methods apply to Pulumi
programs written in a language, which a `finfocus` runtime never hosts.
Pulumi treats `Unimplemented` as "this runtime does not do that", so this is
the protocol's own answer, not a stub (Principle VI).

**Alternatives considered**: Implementing every method with an error, which
reads as a failure instead of a capability gap.

## R3. One binary or two

**Decision**: A third start-up name of the existing `finfocus` binary.
`cmd/finfocus/main.go` already selects analyzer mode from `os.Args[0]`
(`pulumi-analyzer-policy-finfocus`, `pulumi-analyzer-finfocus`);
`pulumi-language-finfocus` selects language host mode. No release artifact is
added (FR-002).

**Rationale**: OPA ships one `main` under two names for the same reason: the
analyzer and the language host cannot drift apart in version. `analyzer
install` already creates the pack entries as links (Unix) or copies (Windows)
of the running binary.

**Alternatives considered**: A separate `pulumi-language-finfocus` build in
the release, rejected by the owner (spec FR-002); it would need its own
archive, checksums, and install path.

## R4. How the language host starts the analyzer

**Decision**: `RunPlugin` re-executes its own binary with an explicit
`Args[0]` of `pulumi-analyzer-policy-finfocus` (with `.exe` on Windows), the
arguments `[<engine address>, "."]`, the policy pack directory as working
directory (R6), and the parent environment plus `req.env`.

**Rationale**: On Linux, `os.Executable()` reads `/proc/self/exe`, which
resolves symlinks. The spike confirmed it: a binary started through a
`pulumi-language-finfocus` symlink saw `os.Args[0]=./pulumi-language-finfocus`
but `os.Executable()=<link target>`. Re-executing that path with its default
`Args[0]` would start `finfocus` under its real name, which `main.go` routes to
the general CLI. Setting `Args[0]` explicitly makes the dispatch independent
of links and copies. Re-executing itself (rather than the sibling legacy entry)
also guarantees the analyzer is the same build as the language host and keeps
the legacy entry optional for current Pulumi.

**Alternatives considered**:

- Start the sibling `<programDirectory>/pulumi-analyzer-policy-finfocus`. The
  spike did this and it works, but it couples the new path to the legacy
  entry, and on Windows the two copies can be different builds after a
  partial sync.
- Serve the analyzer inside the language host process. Avoids a child
  process, but `RunAnalyzerServe` owns process-global state
  (`FINFOCUS_ANALYZER_MODE`, signal handlers, plugin launch) and logs to the
  process stderr, so it would need restructuring. The analyzer code that works
  today stays as it is instead.

## R5. Stopping the analyzer (FR-007, FR-008)

**Finding**: Before 3.270.0 Pulumi stopped the analyzer itself:
`InterruptChildren`, five seconds, then `KillChildren` and `Process.Kill`.
Now Pulumi cancels the `RunPlugin` stream and stops the language host process
the same way. `exec.CommandContext` would SIGKILL the analyzer on cancel,
which skips its deferred plugin cleanup.

**Decision**:

- Unix: the child's `Cancel` sends `os.Interrupt` (SIGINT), the signal Pulumi
  sent the analyzer before, which `RunAnalyzerServe` already handles. `WaitDelay`
  is 3 seconds, inside Pulumi's 5-second grace period.
- The language host traps SIGINT and SIGTERM and stops serving gracefully,
  which cancels any running `RunPlugin` and waits for the child, instead of
  exiting at once and leaving the analyzer behind. Its own shutdown is capped
  at 4 seconds (`WaitDelay` plus 1 second) before it calls `Stop`, so it
  finishes before Pulumi's 5-second force-kill of the language host.
- Windows: `Process.Signal(os.Interrupt)` is not supported, so `Cancel` falls
  back to `Process.Kill`. This matches Pulumi's own forcible path on Windows.
  Isolated in `languagehost_unix.go` / `languagehost_windows.go` (Principle
  III). **Limit**: a killed analyzer runs no cleanup, so a cost plugin it
  started can outlive it on Windows. The spec scopes SC-005 accordingly and
  leaves graceful Windows cleanup (for example a job object) out of scope.
- A non-zero child exit is sent as `RunPluginResponse.exitcode`; a start
  failure is returned as an error. Pulumi then reports it (FR-008).

**Verification**: An integration subtest installs the existing router-like
fixture (`test/integration/testdata/orphanplugin`), runs a preview, and
checks that the recorded plugin PIDs are gone afterwards (SC-005). The spike
showed no leftover analyzer or language host processes, but with no plugins,
so it did not exercise plugin cleanup.

## R6. Working directory (FR-009)

**Decision**: The child runs in `req.pwd` (the Pulumi project) when that is an
absolute existing directory, and in `info.programDirectory` (the policy pack
directory) otherwise. The language host also sets `FINFOCUS_PROJECT_DIR` to
`req.pwd` when nothing set the variable already. The first answer (option C,
keep the pack directory) was reversed on 2026-10-28; running in the project
rather than only exporting the variable followed a review finding that a
relative `cost.history.directory` resolved against the pack directory and fell
back to global history.

**Finding behind the change**: `RunAnalyzerServe` resolves the project from
`os.Getwd()` (`internal/cli/analyzer_serve.go:92-97`). In the pack directory
there is no `Pulumi.yaml` above it, so the first implementation read only the
global tier and wrote the summary under `FINFOCUS_HOME`, unlike Pulumi before
3.270.0, which starts the analyzer in the project. A preview of
`finfocus-demo` on Pulumi 3.272.0 confirmed that Pulumi sends `pwd` as the
project, and the summary then lands in the project's `.finfocus/`.

**Why the environment variable**: `config.ResolveProjectDir` already reads
`--project-dir`, then `FINFOCUS_PROJECT_DIR`, then the nearest `Pulumi.yaml`.
Reusing it adds no second project-directory rule and needs no change to
`analyzer serve`.

## R7. Environment variables Pulumi no longer sets

**Finding**: PR #24784 deleted `constructEnv`, which set `PULUMI_CONFIG`,
`PULUMI_ORGANIZATION`, `PULUMI_PROJECT`, `PULUMI_STACK`, and `PULUMI_DRY_RUN`
for the old path. No finfocus code reads them (`grep` over `internal/`). The
analyzer gets the same data from `ConfigureStack`
(`internal/analyzer/server.go:507`), which the engine calls on both paths.

**Decision**: Nothing to replace.

## R8. Policy pack layout and the Windows suffix

**Finding**: `SetupPolicyPack` and `syncPolicyPackBinary`
(`internal/analyzer/policypack.go`, `install.go:346`) write
`pulumi-analyzer-policy-finfocus` without `.exe`, even on Windows.
`policyPackBinaryPath`, which adds the suffix, is called only by a test.
Windows `PATH` lookup needs the suffix, so the Windows pack entry was most
likely never found. That is a defect in the current code, fixed here because
FR-004 requires the suffix.

**Decision**: One list of pack entries (legacy analyzer, language plugin),
each created through the suffix-aware path. `SetupPolicyPack` creates every
entry on every run; the separate `syncPolicyPackBinary` relink collapses into
it, since both remove and recreate the same paths. `Install` already runs
`SetupPolicyPack` on no-op installs (`install.go:224-233`), so an existing
user's `finfocus analyzer install` adds the new entry (US2).

## R9. What runs the check (FR-010, FR-011)

**Finding**: The nightly `Analyzer E2E Tests` job, which installs Pulumi
`latest`, failed on 2026-10-22 with this exact error
(run 39739635721): `test/e2e/analyzer_e2e_test.go` `ensureAnalyzerConfig`
builds the old layout by hand. A scheduled check against the newest release
therefore already exists; it needs the new layout.

**Decision**:

- `ensureAnalyzerConfig` writes both entries.
- The opt-in integration test `test/integration/analyzer_policy_pack_test.go`
  (`FINFOCUS_TEST_PULUMI`) gains subtests for the legacy-only layout (run only
  on Pulumi releases before 3.270.0) and for plugin cleanup (R5).
- `ci.yml` gets a job with a matrix of `ubuntu-latest` and `macos-latest`
  by Pulumi `3.268.0` and `latest` (`pulumi/actions@v7` `pulumi-version`)
  that runs that test with
  `FINFOCUS_TEST_PULUMI=$(command -v pulumi)`, so PRs see a break too.
  `nightly.yml` runs it with `latest` for releases between PRs.

**Alternatives considered**: Turning the test on whenever `pulumi` is on
`PATH`. GitHub's runner image ships a Pulumi, so every PR's integration job
would pick a version nobody chose.

## R10. Dependencies

**Finding**: `rpcutil.ServeWithOptions` and `rpcutil.MakeRunPluginStreams`
(`sdk/v3/go/common/util/rpcutil`) pull in packages missing from finfocus's
`go.sum` (opentracing, glog, termios, and others).

**Decision**: Serve with plain `google.golang.org/grpc` and write the
`RunPluginResponse` stdout/stderr messages with a small `io.Writer`, as the
spike did. No `go.mod` change. The handshake (print only the port on stdout)
follows `RunAnalyzerServe`.

## R11. Docs and other consumers

Files that describe the old layout, from a repository search:

- `docs/src/content/docs/getting-started/analyzer-setup.md`
- `docs/src/content/docs/analyzer-integration.md`
- `docs/src/content/docs/architecture/analyzer.md`
- `docs/src/content/docs/deployment/deployment.md`
- `docs/src/content/docs/guides/routing.md`
- `docs/src/content/docs/support/faq.md` (adds the error text and the fix)
- `agent-skills/finfocus-analyzer-setup/SKILL.md` and
  `references/troubleshooting.md`; the skill's `.skill` archive is rebuilt
  (`SKILL.md` + `references/`) or the archive test fails
- `agent-skills/finfocus-install/SKILL.md` (checked; changed if it names the
  pack entries)

`CHANGELOG.md` is release-please's and is not edited. `specs/0xx-*` and
`specs/5xx-6xx` are history and stay as written. Outside this repository:
`rshade/finfocus-action` `src/analyze.ts` (`setupAnalyzer`, FR-013) and the
`finfocus-demo` Pulumi pin (SC-003).
