# Contract: Plan JSON and the parser

Host-native ingestion. This is the first use of the shipped
`CostParseService` contract. Core does not register or serve that RPC.
The parser is called in-process with the SDK types and validators from
finfocus-spec v0.7.9 (`pluginsdk.CostParseProvider`,
`pluginsdk.ValidateParseRequest`, `pluginsdk.ValidateParseResponse`).

## Call

```text
Parse(ctx, *finfocus.v1.ParseRequest) (*finfocus.v1.ParseResponse, error)
```

The host builds the request and validates it before `Parse`. `Parse`
returns a response that passes `ValidateParseResponse`. A response that
fails that check is a bug in the parser.

| Request field | Value |
| --- | --- |
| `format` | `terraform-plan` |
| `files` | One file, path `plan.json`, content the `show -json` bytes |
| `root_path` | Not used. Limits advertise `content` only |
| `options` | Empty |

Advertised limits: format `terraform-plan`, input mode `content`, maximum
32 MiB of raw file bytes. `path` is rejected with `UNSUPPORTED_INPUT_MODE`.
An unknown format is rejected with `UNSUPPORTED_FORMAT`.

| Response field | Rule |
| --- | --- |
| `resources[].source_id` | The resource address. Unique in the response |
| `resources[].resource.resource_type` | The Terraform type, such as `aws_instance`. Not a Pulumi token |
| `resources[].resource.provider` | `aws`, `azure`, `gcp`, `kubernetes`, or `custom`. A provider outside the first four is `custom`, except `random`, `null`, `helm`, `time`, `tls`, `local`, `archive`, `external`, `http`, which are skipped |
| `resources[].resource.attributes` | Planned values, unknown and sensitive leaves removed, keys camel-cased the same way as Terraform state |
| `resources[].unknown_properties` | Dot paths that are absent from attributes. The validator checks this |
| `warnings[]` | Code `resource_skipped`. INFO for data sources and for a skipped provider. WARNING for an unrecognized action |

`ParseResponse` has no operation field. The host wrapper returns
`map[source_id]operation` beside the response. Values are in
[research.md](../research.md). The cost diff uses that map. The map is not
encoded into the proto.

After validation, the host copies `unknown_properties` into engine
properties as `terraform:unknownProperties` for `GetProjectedCost` only.
That key is not part of the parse response. `ResolveResourceTypes` runs
on the Terraform type after parsing, as it does for state. The proto
comment that says a parser returns a Pulumi token, and that the host
does not call `ResolveResourceTypes`, is a finding of this test. This
release does not bump finfocus-spec.

## Runner

Binary choice: only `tofu`, resolved with `LookPath`. Never execute Terraform core.
Then `version -json`. Reject below OpenTofu 1.6.0 with
the plan-failed hint.

Working directory: the detected Terraform directory, or the current
directory when `--terraform-plan` is not used.

```text
<binary> plan -input=false -lock=false -no-color -out=<temp>/plan.bin
<binary> show -json -no-color <temp>/plan.bin
```

The temp directory is from `os.MkdirTemp`, mode `0700`. The plan file is
mode `0600` before `show`. Both are removed before the function returns,
including on error and cancellation. Cancellation kills the child's process
group on Unix and the child process on Windows.

Stderr classification and the phrase list are in [research.md](../research.md).
Tests pin those phrases with recorded stderr. They do not shell out to a
network.

## Parser

`Parse` reads the bytes above, or the file given to `--terraform-plan`,
from `plan.json` in the request. It returns a `ParseResponse` only.
The operation map and the before-values are host state beside that return.

- Read `resource_changes` only.
- Skip `mode` other than `managed`.
- Map actions as in research. Skip unrecognized actions.
- Build unknown paths from `after_unknown`.
- Remove every leaf marked in `before_sensitive` or `after_sensitive` from
  `before` and `after`. A removed sensitive leaf is not an unknown path.
- Never read `resource_drift`.
- Camel-case with the state mapper's function.
- Set `terraform:id` to the address and the other `terraform:` metadata the
  state mapper sets when the values exist (`arn`, cloud id, module, provider
  reference).
- Do not log properties, the document, or stderr beyond the 20 lines inside
  the plan-failed hint.

A document with an empty `resource_changes`, or only `no-op` actions, is a
successful empty plan. It is not an error.
