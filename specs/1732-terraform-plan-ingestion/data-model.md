# Data Model: Terraform Plan Ingestion

Entities from [spec.md](spec.md). Field names below are the internal model.
They are not a proto change. See [research.md](research.md) for why.

## Project marker

The result of looking at one working directory.

| Field | Rule |
| --- | --- |
| Kind | `pulumi`, `terraform`, or none |
| Directory | Absolute path of the winning marker |
| Pulumi stack | Set only for `pulumi`, from the current stack or `--stack`, as today |

Validation:

- A Pulumi marker is `Pulumi.yaml` or `Pulumi.yml` found by walking upward.
- A Terraform marker is a regular file in the working directory whose name
  ends in `.tf` or `.tf.json`. Subdirectories, `.tfvars`, and `.terraform`
  do not count.
- Both in the same directory: kind `pulumi`.
- Terraform files in the working directory and a Pulumi file only in a
  parent: kind `terraform`.
- Neither: an error that names Pulumi and Terraform.

## Plan document

In-memory `terraform show -json` output. The plan file on disk is temporary
and is not a stored entity.

| Field | Rule |
| --- | --- |
| FormatVersion | Required. Major version must be 1 |
| TerraformVersion | Informational. The binary's own version is checked before the run |
| ResourceChanges | The only list that is priced |

Validation:

- A user-supplied file that is missing, unreadable, or not JSON is a
  user-input error.
- A major `format_version` other than 1 is a user-input error for
  `--terraform-plan`, and the plan-failed hint for a run this process started.
- Output changes, variables, and `prior_state` are ignored.

## Planned resource

One `resource_changes` element with `mode` `managed`, a known action list,
and a provider outside the skipped set (`random`, `null`, `helm`, `time`, `tls`, `local`, `archive`, `external`, `http`). The provider sent is
`aws`, `azure`, `gcp`, `kubernetes`, or `custom` (every other cloud-less or
unrecognized provider). The stored form is `finfocus.v1.ParsedResource`.

| Field | Rule |
| --- | --- |
| `source_id` | `address`. Unique in one response. Also `resource.id` |
| `resource.resource_type` | Terraform type, unchanged (`aws_instance`) |
| `resource.provider` | Cloud from the type prefix via `resourcetype.ExtractProvider`; a value outside `aws`, `azure`, `gcp`, `kubernetes` becomes `custom`. the skipped set is skipped (see research) |
| Operation | Not on the proto. Companion `map[source_id]operation`: `create`, `update`, `delete`, `replace`, or `same`. See research |
| `resource.attributes` | `change.after` with unknown and sensitive leaves removed, keys camel-cased, plus the existing `terraform:` metadata except the unknown list |
| `old` values | `change.before` with sensitive leaves removed, when it is an object. Not a parse field. The host keeps them beside the response for the cost diff. Nil for a create |
| `unknown_properties` | Dot paths, camel-cased, unique, no list indexes, absent from attributes |

Relationships:

- Attributes and `unknown_properties` describe the same tree. A path is
  listed only when that attribute was removed. `ValidateParseResponse`
  rejects a path that is still present, duplicated, or indexed.
- After validation, the host copies `unknown_properties` to
  `terraform:unknownProperties` on the engine descriptor so the projected
  request and its cache digest include the list. That key is not part of
  the parse response. String tag conversion skips it.
- Module addresses stay in `source_id`. No separate module entity.

Skipped rows are not planned resources. Each one is a warning.

## Plan warning

A `finfocus.v1.ParseWarning`.

| Field | Rule |
| --- | --- |
| `code` | `resource_skipped` |
| `severity` | `INFO` for `mode` `data` and for a resource from a skipped provider. `WARNING` for an unrecognized action list |
| `source_id` | The address. Required when `code` is `resource_skipped` |
| `message` | Human text. Callers match on `code`, not `message` |

A warning does not fail the plan and does not add a cost row.

## Failure hint

One operational failure. No cost result is attached.

| Kind | When | Exit |
| --- | --- | --- |
| Binary missing | `terraform` and `tofu` are both absent from PATH | 1 |
| Not initialized | Stderr asks for init, and the binary is new enough | 1 |
| Credentials or backend | Stderr matches the credential phrase list, and it did not ask for init | 1 |
| Plan failed | Any other non-zero plan or show, including a binary below the minimum | 1 |

User-input failures are a different state: exit 2, no hint kind. They cover
flag combinations, a missing `--terraform-plan` file, and a plan file whose
format major is not 1.

## Project identity

The history key for a Terraform directory. Pulumi history keys are unchanged.

| Field | Rule |
| --- | --- |
| Project | Lowercase hex SHA-256 of the canonical string in research |
| Stack | `unscoped`, or the workspace name when the opt-in is on |
| Label | Relative path, or `path:` plus the absolute path. The workspace name is appended only when the opt-in is on |
| IncludeWorkspace | False unless `--include-workspace` or `cost.terraform.include_workspace` |

The database file is `<Project>@<Stack>.history.db` in the existing history
directory. `project_label` is optional meta. Commands that only read history
still open the file when the label is missing.

State transitions:

- Opt-in off, then on: a different stack part and a different hash, so a
  different file. The opted-out file is left in place.
- Opt-in on for workspace `unscoped`: rejected before a file is opened.

## What is not stored

- The plan file, the plan JSON, credentials, and the raw git remote URL.
- A Terraform checkpoint list. `cost history collect` does not write one.
- A web session field. The dashboard does not learn this model.
