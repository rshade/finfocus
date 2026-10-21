# Research: Pulumi ESC Credentials for Plugins

Each decision below resolves an unknown from the plan's Technical Context.
Evidence was gathered on 2026-10-19 from the esc-sdk source and swagger at
`sdk/v0.14.0`, the pulumi/pulumi source, this repository, and the local
Pulumi files on a development machine. Only key names were inspected.

## R1. ESC client library

- **Decision**: `github.com/pulumi/esc-sdk/sdk` v0.14.0 (package `sdk/go`,
  imported as `esc`). Build `esc.Configuration` by hand by setting `Servers`
  to `<backend>/api/esc` and `HTTPClient` to a client with no redirects.
  Derive the auth context from the caller's context through
  `context.WithValue(ctx, esc.ContextAPIKeys, …)`.
- **Rationale**: the user chose the SDK while issue #1865 was drafted. It
  adds two modules (`esc-sdk/sdk` and `gopkg.in/ghodss/yaml.v1`). finfocus
  already has `gopkg.in/yaml.v2`. A probe at Go 1.27.2 builds, and
  `govulncheck` reports no vulnerabilities. The license is Apache-2.0. It
  honors context deadlines through `req.WithContext`.
- **Gotchas found**:
  - `NewCustomBackendConfiguration` drops the port and path because it uses
    `Hostname()`.
  - `NewDefaultClient` reads only `PULUMI_BACKEND_URL`.
  - `NewAuthContext` starts from `context.Background()` and loses
    cancellation.
  - Therefore none of these three helpers is used.
- **Alternatives considered**:
  - Shelling out to `pulumi env open`. This was rejected by the user's
    choice and would add a CLI dependency (FR-011).
  - Importing `pulumi/sdk/v3/go/common/workspace` to reuse Pulumi's account
    and backend helpers. That adds about 108 packages to `cmd/finfocus`
    (go-keyring, dbus, go-git, Charm v1, OTel collector).

## R2. Opening the stack's `environment:` imports

- **Decision**:
  - When the stack file's `environment` is a list of strings, or a map whose
    only key is `imports` with a list of strings, open each reference by name
    in list order with `OpenAndReadEnvironment(AtVersion)`. Merge the
    `environmentVariables` maps, and the later reference wins for each
    variable.
  - Every other form is an `unsupported_definition` failure for that stack.
    Such forms include inline `values:`, an import written as
    `{name: {merge: false}}`, and any other key. The plugins then fall back
    (FR-009).
- **Rationale**:
  - Pulumi opens the whole block as one anonymous environment
    (`POST /api/esc/environments/{org}/yaml/open`, `pkg/cmd/pulumi/config/io.go:238`).
    The SDK has no call for that endpoint.
  - For the common list form, opening the named environments in order gives
    the same `environmentVariables` result, because ESC's default import merge
    is per key, with later entries winning.
- **Alternatives considered**:
  - Calling `/yaml/open` directly over `net/http`. That would give exact
    semantics, but it bypasses the chosen SDK and reaches an endpoint the SDK
    does not wrap. It is recorded as a possible follow-up if inline
    definitions turn out to be common.
  - Silently ignoring the inline values. This was rejected because it would
    deliver different variables than Pulumi does, with no signal to the
    person.

## R3. Finding the project and the stack before plugins launch

- **Decision**: the Pulumi project root is found with `pulumi.FindProject` from
  the working directory, as `detectPulumiProject` already does. The stack is:
  1. The `--stack` flag when the command has one and it is set.
  2. Otherwise, the stack Pulumi selected for that project, read from
     `<PULUMI_HOME>/workspaces/<project>-<sha1(abs path of Pulumi.yaml)>-workspace.json`.
     The current format is `{"stacks": {"<backend URL>": "org/project/stack"}}`.
     The legacy format is `{"stack": "<name>"}`.
  3. Otherwise there is no stack, and stack imports are not used. This case
     is debug-logged, and no warning is shown.

  The stack file is read with the existing name candidates
  (`stackSettingsNameCandidates` and `readStackSettingsFile`), which this
  work moves from `internal/cli/overview.go` into `internal/pulumi`.
- **Rationale**:
  - The overview pipeline starts plugins before the stack is known
    (`overview_pipeline.go:288`).
  - `pulumi stack ls` costs a CLI process and network time, and #692 exists
    to keep that time off the plugin path.
  - The workspace file was verified on the development machine. For
    `../finfocus-demo` the file named by the hash rule holds
    `richard-shade-gmail-com/finfocus-demo/dev`, which agrees with
    `pulumi stack ls` (`dev`, `current: true`).
  - The fully qualified name also supplies the organization.
- **Alternatives considered**:
  - Moving the plugin open after `detectPulumiProject`. That would undo
    #692.
  - Calling `pulumi stack ls` early. That is the same cost moved earlier,
    and it needs the CLI.

## R4. Organization, backend, and token

- **Decision**:
  - **Backend URL**: `PULUMI_BACKEND_URL`, then `backend.url` in
    `Pulumi.yaml`, then `current` in `credentials.json`, then
    `https://api.pulumi.com` when a token comes from the environment. A
    `file`, `s3`, `gs`, or `azblob` scheme is `unsupported_backend`. A scheme
    other than `https` is accepted only for loopback hosts, so a token never
    crosses the network in clear text.
  - **Token**: `PULUMI_ACCESS_TOKEN` first. Otherwise the plaintext
    `credentials.json` in `PULUMI_CREDENTIALS_PATH`, else `PULUMI_HOME`, else
    `~/.pulumi`, using `accounts[<url>].accessToken` then
    `accessTokens[<url>]`. These cases are `not_logged_in`, and the warning
    tells the person to set `PULUMI_ACCESS_TOKEN`:
    - The file is an encrypted envelope (Pulumi v3.270+ with an OS keychain).
    - The file is missing.
    - The token is empty.
  - **Organization**:
    - Stack imports use the stack's org: the org segment of a qualified stack
      name, otherwise the default-org chain.
    - A configured reference without an org always uses the default-org
      chain and never the stack's org, as the spec's Key Entities say.
    - The default-org chain is `PULUMI_DEFAULT_ORGANIZATION`, then
      `backends[<url>].defaultOrg` in `config.json`, then
      `accounts[<url>].username`. With none of these, the category is
      `not_logged_in`.
  - **Trust (FR-020)**:
    - `PULUMI_ACCESS_TOKEN` is sent only to a backend URL from
      `PULUMI_BACKEND_URL`, from `credentials.json` `current`, or the default
      `https://api.pulumi.com`.
    - A URL taken from `Pulumi.yaml` `backend.url` is repository content. It
      gets only a token saved in `credentials.json` for exactly that URL, or
      else `unsupported_backend`.
    - This follows the codebase's existing distrust of project-sourced
      values: `${…}` in a project notification destination is skipped, and a
      project `cost.history.directory` is gated.
    - Without the rule, cloning a repository whose `Pulumi.yaml` names a
      hostile host and running any read-only `cost` command would send the
      person's token there.
- **Rationale**: this is the same order as Pulumi's
  `GetCurrentCloudURLWithSource`, `httpstate.Current`, and
  `pkg/backend/organizations.go`, without the extra package graph. The Pulumi
  files are read only. FinFocus never creates or refreshes them, and an
  expired OAuth token surfaces as `401`, which maps to `not_logged_in`.
- **Alternatives considered**: decrypting the keychain envelope. That needs
  platform keyring libraries (Principle III risk) and is the same package
  graph as R1.

## R5. Failure classification and redaction

- **Decision**: one closed set of categories:

  | Category | Cause |
  | --- | --- |
  | `not_logged_in` | No token, an encrypted credentials file, no organization, or a `401` |
  | `unsupported_backend` | A DIY backend scheme, or a non-loopback URL that is not `https` |
  | `not_found` | `404`. Pulumi Cloud also returns `404` for an org or environment the token cannot see (R12), so the warning says "not found or no access" |
  | `access_denied` | `403` (rare in practice; kept for completeness) |
  | `timed_out` | The context deadline expired |
  | `unsupported_definition` | An inline stack definition (R2) |
  | `unavailable` | Anything else: transport errors, other statuses, an unclassifiable error, or a recovered panic |

  - The status is read from `*esc.GenericOpenAPIError` and
    `Model().(esc.Error).Code`, using `errors.As`. An error whose status
    cannot be read is `unavailable`.
  - `Error()`, `Body()`, `Model().Message`, and `*url.Error` text are never
    logged or returned.
  - Every SDK call runs under `recover()`, because `getValue` and `getRange`
    make unchecked type assertions that panic on some traces.
- **Rationale**: `Body()` and `Model()` carry the raw response. A `*url.Error`
  names the org, project, and environment, which is allowed in a warning,
  but the whole string is not trusted. The probe hit a panic in
  `ReadOpenEnvironment`.
- **Alternatives considered**: logging the SDK error at debug level. This was
  rejected because FR-013 covers every log level.

## R6. Value handling

- **Decision**:
  - Only the top-level `environmentVariables` property is used.
  - String values pass through unchanged. Numbers and booleans are formatted
    with `strconv`.
  - Objects, lists, and nulls are dropped, and a debug log line records the
    variable name only.
  - The secret flag of each value is kept from the returned `*Environment`
    model so that R7 can use it.
  - Values are never stored in a type that has a `String` or `MarshalJSON`
    method that could print them.
- **Rationale**: the SDK returns plain values, including plaintext secrets.
  The probe returned a number as `float64`. `mapValues` also drops `null`
  entries before finfocus sees them. `Trace.Def` is empty in read responses
  (R12), so the origin of a variable is the reference that was opened, not a
  per-value source.

## R7. Cache fingerprint (FR-014)

- **Decision**: for every variable that ESC delivers, `hashDelivered` writes
  `name\0esc\0<ref>\0<revision-key>\0` in place of `name\0value\0`. No ESC
  value of any kind enters the digest. Values from every other source keep
  today's hashing. `<revision-key>` depends on the form of the reference:

  | Reference | `<revision-key>` | Extra call |
  | --- | --- | --- |
  | No version, or `@latest` | The ETag from `GetEnvironmentETag` (`HEAD`) | One `HEAD` |
  | `@<number>` | The number itself. That revision is immutable | None |
  | `@<tag>` (any other text) | The revision the tag points to, from `GetEnvironmentRevisionTag` | One `GET` |

  A failed extra call leaves the key empty for that reference, which only
  costs cache hits.
- **Why the reference form matters**: the spike (R12) showed that the ETag
  describes the environment's latest definition, whatever version was
  requested.
  - For `@1` it returned an ETag even though revision 1 never changes, so
    using it there would only cause cache misses.
  - Moving a tag such as `stable` to another revision does not edit the
    definition, so the ETag would not change. Stale answers would then be
    reused until the TTL.

  Resolving the tag closes that hole. The tag case is a design inference. The
  spike did not move a tag, so a unit test against `httptest` pins it.
- **Rationale**:
  - Open and read responses carry no revision. The `Environment` and
    `OpenEnvironment` models have only `id`, `diagnostics`, `exprs`,
    `properties`, `schema`, and `executionContext`.
  - The ETag changes whenever the environment definition is edited, and that
    is the only way a static value in it can change.
  - Every value that changes without an edit comes from a provider
    (`fn::open::aws-login` and similar) and rotates by design.
  - The spike (R12) showed that the secret flag cannot separate the two
    kinds. `AWS_ACCESS_KEY_ID` from an OIDC login is **not** marked secret
    and still changed on every open.
  - Hashing names, the reference, and the ETag is therefore both stable
    under rotation (SC-007) and sensitive to edits (FR-014).
- **Residual**: a value edited in an *imported* environment does not change
  the importing environment's ETag. The answer can then be reused until the
  cache TTL expires. This is documented, not hidden.
- **Alternatives considered**:
  - Hashing all values, as today. Rotation would then miss the cache on
    every run (SC-007 fails).
  - Hashing non-secret values and replacing secrets with a marker. This was
    the first plan. The spike showed it misses the cache on every run,
    because the OIDC access key ID is a non-secret value that rotates.
  - Using `Trace.Def` to tell provider outputs from literals. The read
    response leaves `Trace.Def` empty (R12).

## R8. Precedence and the process environment

- **Decision**:
  - `ResolveDeliveredEnv` gains an ESC input. For an allowed name that ESC
    supplies, the effective value is the ESC value.
  - The delivered context carries the set of names that must replace a value
    already in the parent environment. `appendDeliveredEnv` removes the
    inherited entry for those names before appending the ESC pair.
  - The reserved names (`pluginsdk.EnvPort`, `EnvLogFile`, `EnvTraceID`)
    are never overridden.
- **Rationale**: today `startPlugin` builds `os.Environ()` first, and
  `appendDeliveredEnv` skips any key that is already present
  (`process.go:524-550`). Without this change, ESC-over-shell would silently
  do nothing.
- **Notes**:
  - The stdio launcher (`stdio.go`) never applies delivered env and has no
    production caller. It is left unchanged, and the plan records why.
  - The existing one-time restart after `GetPluginInfo` already handles
    descriptor-allowed ESC names that are learned only after the first
    launch.

## R9. Where configuration lives

- **Decision**: a new top-level `esc` section.

  ```jsonc
  "esc": {
    "environments": ["billing/aws-readonly@stable"],
    "plugins": {
      "vantage": {
        "environments": ["billing/vantage"],
        "variables": ["VANTAGE_TOKEN"]
      }
    }
  }
  ```

  - An omitted `environments` means "no preference".
  - A present empty list means "off" for what it covers.
  - The project tier's `esc` section replaces the global one, as every other
    top-level section does (`ShallowMergeYAML`).
- **Rationale**: `plugins.<name>` is an open flat map. `PluginConfigValues`
  injects every scalar in it as `FINFOCUS_PLUGIN_<NAME>_<KEY>`, so
  `plugins.<name>.pulumi_env` (the shape issue #1865 proposed) would leak into
  the plugin and appear as a generic web setting.
- **Alternatives considered**:
  - A reserved key inside `plugins.<name>`. That needs an exclusion list in
    three readers.
  - A `pulumi` section. ESC is not the only Pulumi concern finfocus might
    configure later.

## R10. Lifetime and timing

- **Decision**:
  - The root `PersistentPreRunE` attaches a lazy `esc.Source` to the command
    context. The first plugin launch that needs a selection opens its
    environments, once per reference, with a singleflight per reference.
  - Each `Source` has 5 seconds of ESC wait time (SC-004). Time is charged
    only while at least one ESC call is in flight, and overlapping calls are
    charged once. A wall-clock deadline from the first open would be wrong:
    `cost recommendations` opens the scorer through a second `openPlugins`
    call (`cost_recommendations_scoring.go:177`) after the recommendation
    RPCs, which can take longer than 5 seconds. A first-open deadline would
    time it out at once.
  - `reloadPlugins` in `--web` attaches a fresh `Source`, which makes the
    environment reopen (FR-012).
  - `mcp-server` runs `PersistentPreRunE` on every tool call, so each call
    gets a fresh `Source`.
  - `analyzer serve` attaches a `Source` limited to explicit configuration.
    Under `pulumi preview` or `up`, Pulumi already sets the stack
    environment's variables in the process environment, which plugins
    inherit (`plugin.go:500,602`). The stack is also unknown at launch,
    because it arrives through `ConfigureStack`.
- **Rationale**:
  - The open runs inside the plugin-open goroutine, so it overlaps stack
    loading instead of delaying it (#692).
  - Commands with no selection never touch the network.

## R11. Web parity data

- **Decision**:
  - `webui.PluginSettingValue` gains `escEnvironment` (the reference only).
    `effectiveSource` gains the value `esc`.
  - `pluginSettingValues` takes the per-plugin ESC origins from the same
    `esc.Source` the session's last plugin open used.
  - `requirePluginSettings` treats an ESC-supplied value as satisfying a
    required setting.
  - The existing help sentence pattern
    (`plugins.js` `fieldHelp`) gets one ESC sentence. Status-cell design is
    left to the UX follow-up.
- **Rationale**: Principle VII requires the hook and the data. Polishing the
  layout belongs in the `web-ui` issue.

## R12. Spike against real Pulumi Cloud (2026-10-19)

A throwaway Go program (not committed) imported esc-sdk v0.14.0 and ran the
planned flow against `../finfocus-demo`, whose stack imports a real
environment that uses an OIDC `aws-login`. It printed names, types, flags,
categories, and timings only, and never a value or a token. `go vet` was
clean. The dependency footprint matched R1 (two new modules).

| Step | Result |
| --- | --- |
| `Pulumi.yaml` → name, `backend.url` | `finfocus-demo`, no backend URL |
| `credentials.json` | Plaintext. Backend from `current` (`https://api.pulumi.com`). Token from `accounts[url].accessToken` |
| Workspace file → stack | `richard-shade-gmail-com/finfocus-demo/dev`, from the `stacks[<backend>]` format. Matches `pulumi stack ls` |
| Organization | From the qualified stack name |
| Stack `environment:` | List form, 1 import |
| Discovery total | 1 ms, with no Pulumi CLI process |
| ETag `HEAD` and open | 879 ms. ETag present and identical on two opens |
| `environmentVariables` | `map[string]esc.Value`, 6 strings. Secret flag set on 3 entries (the AWS secret key, the session token, and an OAuth client secret). Not set on the AWS access key ID, the region, or the OAuth client ID |
| Rotation | Between two opens, 2 secret values **and 1 non-secret value** (the access key ID) changed. A non-secret-value fingerprint was unstable, which led to the R7 revision |
| `Trace.Def` | Empty for every entry |
| Missing environment in own org | `404` → `not_found` (184 ms) |
| Invalid token | `401` → `not_logged_in` (223 ms) |
| Environment in another org | `404`, not `403` |
| Unroutable host, 2 s deadline | `timed_out` at 2.002 s. The deadline is honored exactly |
| Org shorter than 3 characters | A plain SDK client-side error → `unavailable`. `ParseRef` bounds catch it earlier in configuration |

A second run, started by the user, opened versioned references through
`OpenAndReadEnvironmentAtVersion`:

| Reference | Result |
| --- | --- |
| `…/tailbench-rshade-aws@latest` | Opened in 1.209 s with the same 6 variables. Rotation reproduced: 2 secret values and 1 non-secret value changed. ETag stable |
| `…/tailbench-rshade-aws@1` | Opened in 347 ms with **0** variables (revision 1 is the empty environment as created). An ETag was still returned. That shows the ETag describes the latest definition, not the requested revision, which led to the version-aware key in R7 |

Not exercised:

- **Moving a tag to another revision.** It is covered by a unit test (R7).
- **Encrypted `credentials.json`.** It was not available on this machine.
  Unit tests cover the `not_logged_in` mapping with a fixture envelope.

## R12 follow-up (2026-10-20)

Quickstart steps that rewrite `../finfocus-demo/Pulumi.dev.yaml` were not
run. That file already had local edits, including an `environment:` import,
and checking it out would discard them. The web render used a temporary
`FINFOCUS_HOME`, `esc.plugins.recorder.environments` set to
`acme/billing/recorder`, and a loopback ESC server. Plugins → recorder →
Settings showed the ESC help sentence and no value. The Status cell still
read `not set`, which is the UX follow-up. The demo stack's own import was
skipped as `unavailable` on that loopback server and was not opened against
Pulumi Cloud.
