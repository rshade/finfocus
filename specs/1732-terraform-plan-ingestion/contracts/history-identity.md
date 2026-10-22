# Contract: Terraform history identity

Pulumi history keys are unchanged. This contract applies only when the
detected project is Terraform.

## Inputs

| Input | Source |
| --- | --- |
| Directory | The Terraform root from detection |
| Include workspace | `--include-workspace`, else `cost.terraform.include_workspace`, else false |
| Remote | `git -C <dir> remote get-url origin` after resolving the git root. No other remote |
| Workspace | `tofu workspace show`, and only when include-workspace is true |

The workspace command is not `init`, `apply`, or `import`. It is not run
when the opt-in is off.

## Hash

UTF-8, SHA-256, lowercase hex. The project part of
`<project>@<stack>.history.db`.

With `origin`:

```text
tf-id-v1
<host>/<path>
<relative-path>
<workspace line only when opted in>
```

`<host>/<path>` drops userinfo, query, fragment, and one trailing `.git`.
SCP form `git@host:org/repo` is accepted. `https://github.com/org/repo` and
`git@github.com:org/repo.git` produce the same host and path. The host is
lowercased.

`<relative-path>` uses forward slashes and is `.` when the Terraform root
is the git root. A relative path that escapes the git root uses the
absolute-path identity instead.

Without a git root or without `origin`:

```text
tf-id-abspath-v1
<cleaned absolute path, forward slashes>
```

The raw remote URL is not logged and is not stored.

## Stack and label

| Opt-in | Stack part | Label |
| --- | --- | --- |
| Off | `unscoped` | The relative path, or `path:` plus the absolute path |
| On | The workspace name | The same label, then a space and the workspace name |

Workspace name `unscoped` with the opt-in on is a user-input error. The
name is reserved for the opted-out stack part.

`project_label` is written into the cost database meta bucket when the file
is created or opened. Schema version stays 1. Readers that do not know the
key skip it. The filename, not the label, is the identity.

## Commands

`cost history view`, `list`, `diff`, `export`, and `prune` resolve this
identity and open that file. They do not run a plan.

Snapshots recorded from a Terraform projection use this project and stack.

`cost history collect` does not create a Terraform timeline. See
[cli.md](cli.md).
