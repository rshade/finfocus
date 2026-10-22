# Research: Terraform Plan Ingestion

Decisions for [spec.md](spec.md). The owner invoked planning without
changing the three assumptions recorded there, so those assumptions are
binding.

## Decision: Split detection, running, and parsing the way Pulumi already is

**Rationale**: `internal/pulumi` finds the project and runs the CLI.
`internal/ingest` parses the JSON. Terraform state already lives in
`internal/ingest/terraform_state.go` and `terraform_map.go`. A new
`internal/terraform` package runs the binary and reports identity. The
parser stays in `internal/ingest` so descriptors, property conversion, and
provider extraction stay next to the state mapper.

**Alternatives considered**: One `internal/terraform` package that also
parses JSON. Rejected because it would fork the state mapper's key
conversion and provider rules. A plugin `CostParseService` implementation.
Rejected by the spec: Terraform and Pulumi stay host-native.

## Decision: Accept plan JSON major version 1, from Terraform >= 1.1.0 or OpenTofu >= 1.6.0

**Rationale**: `tofu show -json` on a plan file is the stable JSON plan
format. HashiCorp versions that format with `format_version`. Minor bumps
are backward compatible; a major bump is not. `resource_changes`,
`change.actions`, `change.after`, and `change.after_unknown` are part of
that 1.x document. Terraform 1.1.0 is the oldest release this plan requires
to run. OpenTofu 1.6.0 is the first stable OpenTofu release and emits the
same 1.x document. A newer 1.x minor is accepted. `format_version` whose
major is not 1 is rejected.

The checked-in files `test/fixtures/plans/*-plan.json` are
`planned_values` excerpts. They are not `tofu show -json` plan
documents and are not fixtures for this parser.

**Alternatives considered**: Terraform 0.12, which introduced an earlier JSON
plan. Rejected because the 1.x `format_version` contract and
`terraform version -json` are the combination this feature can classify.
Terraform 1.5 only, to match one fixture's `terraform_version`. Rejected
because a 1.1 plan document is the same shape for the fields this feature
reads.

## Decision: Map actions onto the operation strings cost diff already accepts

**Rationale**: `ParseResponse` has no operation field. That is the other
finding of this first test. `engine.ResourceDescriptor.Operation` is the
Pulumi step op the cost diff already reads. `normalizeDiffOperation`
treats `replace` and `create-replacement` as an update for the summary,
and `same` as unchanged. An empty operation is a create. State ingestion
sets `same` for that reason.

`Parse` returns only the proto response, so the operation cannot ride
inside it without failing the "use the shipped message" rule. The host
wrapper that calls `Parse` also returns `map[source_id]operation` built
from `change.actions` in the same pass. The cost diff reads that map.
Tests assert both: `ValidateParseResponse` accepts the response, and the
map has the operations. A caller that kept only the response would price
every row as a create.

| Plan `change.actions` | Descriptor operation |
| --- | --- |
| `["no-op"]` | `same` |
| `["create"]` | `create` |
| `["update"]` | `update` |
| `["delete"]` | `delete` |
| `["delete","create"]` or `["create","delete"]` | `replace` |

`mode` `data` is skipped. Any other action list is skipped. Neither case
fails the plan. Only `resource_changes` is read, so `resource_drift` entries
never add a row; a plan with only drift is the empty successful plan. `before` becomes `OldProperties` when it is an object.
`after` becomes `Properties` after unknowns are removed. Both use the
existing snake_case to camelCase conversion, then the existing
`terraform:` metadata keys (`terraform:id` is the address).

**Alternatives considered**: A new diff summary bucket for replace.
Rejected because Pulumi replace already collapses to the update bucket, and
the spec requires the same diff treatment. Leaving the operation empty for
no-op. Rejected because an empty operation is priced as a create.

## Decision: Ingestion is a `CostParseProvider` call, not a local mirror

**Rationale**: This is the first host use of the parser contract that
shipped in finfocus-spec v0.7.7 and is in the v0.7.9 module core already
depends on. The Terraform parser implements
`pluginsdk.CostParseProvider`:

```text
Parse(ctx, *finfocus.v1.ParseRequest) (*finfocus.v1.ParseResponse, error)
```

The host builds the request, calls `pluginsdk.ValidateParseRequest` with
the limits below, calls `Parse`, then `pluginsdk.ValidateParseResponse`.
A response that fails that validator is a parser bug, not a plan hint.
Core does not register or serve `CostParseService`. The call is in-process.
Pulumi ingestion is unchanged and does not go through this contract.

Limits the validator is given:

| Field | Value |
| --- | --- |
| `Formats` | `terraform-plan` |
| `InputModes` | `content` only |
| `MaxInputBytes` | 32 MiB |

`path` is not advertised. A `root_path` request fails validation with
`UNSUPPORTED_INPUT_MODE`, so the temp plan file is never an input path.
Input over `MaxInputBytes` fails request validation: for `--terraform-plan`
that is a user-input error naming the limit (exit 2); for a plan FinFocus ran
itself it is the plan-failed hint, which names the limit.
The file set has one file, path `plan.json`, whose content is the
`show -json` bytes. `options` is empty.

`ParsedResource.unknown_properties` is the unknown channel. Paths are
absent from `resource.attributes`, which is what
`ValidateParseResponse` checks. They are not copied into attributes under
another key inside the response.

`resource.provider` must be one of `aws`, `azure`, `gcp`, `kubernetes`,
or `custom`, because `ValidateResourceDescriptor` rejects anything else
(`sdk/go/pricing/validate.go`). The state mapper keeps every provider, so
skipping all providers outside that set would make a plan and a state of
the same config disagree. The rule is therefore:

- Provider from `ExtractProvider` is `aws`, `azure`, `gcp`, or `kubernetes`:
  kept as is.
- Provider `random`, `null`, `helm`, `time`, `tls`, `local`, `archive`, `external`, `http`: skipped with `resource_skipped` at INFO, zero cost, not a
  failed plan. These are utility providers with no billing relation, and the
  set is nine provider names, not a type table. A new utility provider is a
  one-line addition to the set; until then it is sent as `custom` and a plugin
  answers no cost data.
- Any other provider (for example `cloudflare`, `datadog`, `github`): kept
  with provider `custom`, so a plugin that learns to price it needs no core
  change, and one that does not answers no cost data, as for state.

The host prints one stderr note per skipped provider with a count. It never
touches stdout or the exit code. `resource_type` stays the Terraform type
(`aws_instance`). The proto comment says a parser returns a Pulumi token
and does not need `ResolveResourceTypes`. The validator does not enforce
that comment. This host does not add a type table. `ResolveResourceTypes`
still runs after a valid response, as it does for Terraform state. That
comment-versus-validator gap is a finding of this first test, not a spec
bump.

**Alternatives considered**: Hand-written Go structs with the same field
names. Rejected. The point of this feature is to exercise the shipped
messages and the SDK validators. Serving the parser as a gRPC plugin.
Rejected. Terraform stays host-native. Calling `ResolveResourceTypes`
inside `Parse` so the response already holds Pulumi tokens. Rejected.
That would hide the finding and pull plugin calls into the parser.

## Decision: Unknown names reach pricing only after the response validates

**Rationale**: `GetProjectedCost`'s `ResourceDescriptor` has `attributes`
and no unknown list. After `ValidateParseResponse` succeeds, the host
copies each `unknown_properties` list into the engine properties under
`terraform:unknownProperties` so `BuildAttributes` sends it and the
attributes cache digest changes. `ConvertToProto` skips that key so it
does not become a comma-joined tag. The parse response itself does not
contain that key.

Paths follow the shipped validator: unique, case-sensitive, dot-separated,
no empty segment, no `[` or `]`. List indexes in `after_unknown` are
dropped, so an unknown inside a list is the list path or the nested field
path without an index. A null `after` value is kept and is not an unknown.
A `true` node records that path and does not walk children. Each segment is
camel-cased with the same function as property keys, so a name lines up
with the attribute the plugin sees.

Sensitive leaves are dropped in the parser. `before_sensitive` and
`after_sensitive` mirror the shape of `before` and `after`; a `true` node
removes that leaf (or the whole subtree) from the attributes and from the
old values before the response is built. `IsPulumiSecret` cannot recognize
Terraform's marker, so without this a sensitive value with a benign name
would reach a plugin and break the rule that no credential or secret does.
A removed sensitive leaf is not listed as unknown. Credential-like names
still drop out in `BuildAttributes` as well; there is no second name list.
The plan document and sensitive leaves are never written to logs.

**Alternatives considered**: Bumping finfocus-spec to add
`unknown_properties` on the cost descriptor. Rejected for this release. The
gap is recorded in the plan for a later spec discussion. Omitting the names
entirely. Rejected because the spec requires a plugin to be able to tell
unknown from absent.

## Decision: History identity is a versioned hash, with stack `unscoped` unless the workspace is opted in

**Rationale**: `CostFileNameFor` stores `<project>@<stack>.history.db` and
rejects `@`, slashes, and spaces. The project part is the hex SHA-256 of a
canonical string. The readable label is a new optional meta key
`project_label` on the existing cost database. The schema version stays 1.
Old readers ignore the key. History still works if the key is absent.

Canonical string when `git -C <dir> remote get-url origin` succeeds:

```text
tf-id-v1
<host>/<path>
<relative path using forward slashes, or "." at the git root>
```

`host/path` is the remote with userinfo, query, fragment, and a trailing
`.git` removed. `git@host:org/repo.git` and
`https://github.com/org/repo` both become `github.com/org/repo`. The host
is lowercased. The path case is kept. Only `origin` is consulted.

The stack part is `unscoped` when the workspace opt-in is off. Terraform is
not asked for the workspace. When the opt-in is on, the stack part is the
name from `tofu workspace show` (or `tofu`), and that name is also
mixed into the hash on its own line so two workspaces do not share a file
through a stack-name collision. A workspace named `unscoped` is rejected
while the opt-in is on, because that name is the opted-out stack part.

With no git root or no `origin`, the hash input is `tf-id-abspath-v1` plus
the cleaned absolute path in forward slashes. The label is `path:` plus
that path.

**Alternatives considered**: Hashing the absolute path always. Rejected by
the spec. Using the relative path as the project name. Rejected because
`CostFileNameFor` cannot hold every path and the spec asks for a hash.
Reading every git remote. Rejected because `origin` is the remote clones
share.

## Decision: Workspace opt-in is `cost.terraform.include_workspace` and `--include-workspace`

**Rationale**: The spec leaves the control to planning and requires a saved
setting plus a one-run flag that does not reuse `--stack`. The flag
overrides the setting for that command and does not write the file. On a
Pulumi project the flag is ignored, so Pulumi commands do not gain a new
failure. There is no environment variable.

**Alternatives considered**: `FINFOCUS_INCLUDE_WORKSPACE`. Rejected because
the spec's assumption names a setting and a flag only. Reusing `--stack`.
Rejected because that flag is the Pulumi stack.

## Decision: Four hints, and the same exit split as today

**Rationale**: A bad flag combination, a missing plan file, and a plan file
whose `format_version` major is not 1 are user-input errors (`ax.ExitValidation`,
exit 2), matching `--terraform-state` and `--pulumi-json`. A missing binary,
a directory that is not initialized, a credentials or backend failure, and
any other plan failure are operational failures, exit 1, matching a failed
`pulumi preview`. They print one hint and no cost total.

Classification order on the combined stderr of `plan` and `show`:

1. Not initialized, if the text asks for init (`tofu init`, `tofu init`,
   `initialization required`, or `Plugin reinitialization required`).
2. Credentials or backend, if the text contains one of:
   - AWS: `No valid credential sources found`,
     `failed to refresh cached credentials`, `ExpiredToken`,
     `InvalidClientTokenId`.
   - Azure: `Error building ARM Config`, `az login`,
     `unable to build authorizer`.
   - GCP: `could not find default credentials`,
     `google: could not find default credentials`.
   - Backend: `error configuring the backend`, `Error loading state`,
     `Failed to get existing workspaces`, `Error inspecting states`.

   The list is matched on recorded stderr, so a phrase is added only with a
   recorded sample in the runner test.
3. Otherwise the plan-failed hint, including the last 20 stderr lines.

A binary older than the minimum uses the plan-failed hint and names the
minimum. The binary check uses `version -json` and reads `terraform_version`.
If that command cannot be parsed, the run continues and `format_version`
decides.

**Alternatives considered**: Exit 2 for every hint. Rejected because a
failed preview is already exit 1, and the spec only requires a non-zero
exit. A fifth hint for an old binary. Rejected by the four-hint list.

## Decision: Private temp directory, and cancel the plan's process group

**Rationale**: `os.MkdirTemp` creates the directory in the user temp area,
then the mode is set to `0700`. The plan file is `<dir>/plan.bin`. After
`plan` returns it is set to `0600` before `show`. `RemoveAll` runs on every
exit, including cancel and chmod failure. The plan JSON is the stdout of
`show` and is never logged.

The child is started with its own process group on Unix. Cancellation kills
that group so provider processes Terraform spawned do not keep running.
Windows uses `CommandContext`, which kills the process the command started.
The plan file still never sits in the project directory.

Arguments are exactly `plan -input=false -lock=false -no-color -out=<file>`
and `show -json -no-color <file>`. Refresh is not passed.

**Alternatives considered**: Writing the plan into the project under
`.terraform/`. Rejected by the spec. `-refresh=false` to avoid state writes.
Rejected by the spec. Killing only the direct child. Rejected because a
provider process would survive cancellation.

## Decision: One loader, and two commands that do not plan

**Rationale**: `loadProjectedResources` is already the loader for
`cost projected` and `cost forecast`. It grows a Terraform plan arm and a
shared detector. Overview, bare `finfocus`, estimate, and recommendations
call that same detector and the same plan runner instead of copying the
rules. `recommendations dismiss` and `snooze` are not in that set: their
`--pulumi-json` is optional today and only decides whether plugins open
(`createDismissEngine`), so they load no project resources and never plan.
Terraform rows carry no `ref.*` tags, because a plan has no
`propertyDependencies`. `cost actual` keeps state export for Pulumi and, in a Terraform
directory with no input file, returns the state-file hint without planning.
`cost history collect` in a Terraform directory explains that checkpoint
collection is Pulumi-only and does not plan. ESC loading and analyzer
install keep `FindProject` and stay Pulumi-only.

Overview does not call the Pulumi change-detection prompt for a Terraform
directory. It always runs the plan.

**Alternatives considered**: Planning inside `cost actual`. Rejected by the
spec assumption. Synthesizing `cost history collect` from one plan.
Rejected because a plan has no checkpoint list.
