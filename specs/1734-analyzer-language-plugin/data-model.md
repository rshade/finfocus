# Data Model: Analyzer Language Plugin

**Feature**: [spec.md](spec.md) | **Research**: [research.md](research.md)

No persisted data changes. The entities are the files of the policy pack
directory, the start-up mode chosen from the process name, and one added field
on the install result.

## Policy pack directory

Location: `$FINFOCUS_HOME/analyzer`, default `~/.finfocus/analyzer`
(`analyzer.ResolvePolicyPackDir`, unchanged).

| Entry | Unix | Windows | Started by |
| --- | --- | --- | --- |
| `PulumiPolicy.yaml` | file, `0600` | file | read by Pulumi; `runtime: finfocus` |
| Legacy analyzer entry | `pulumi-analyzer-policy-finfocus`, symlink to the finfocus binary (copy if the link fails) | `pulumi-analyzer-policy-finfocus.exe`, copy | Pulumi releases before 3.270.0 when no language plugin is found |
| Language plugin entry | `pulumi-language-finfocus`, symlink (copy if the link fails) | `pulumi-language-finfocus.exe`, copy | every Pulumi release that finds it on `PATH` |

Rules:

- `SetupPolicyPack` writes the YAML and recreates **both** entries on every
  call, including no-op and `--force` installs (FR-004, FR-006).
- Entry names come from one list and one suffix-aware path function; no code
  builds an entry name by hand (R8).
- A stale entry is removed with `os.Remove` after `os.Lstat`, so a dangling
  link is replaced, never followed.
- The returned method (`symlink` or `copy`) is the language plugin entry's;
  both entries are created the same way on a given platform.

## Start-up mode

Chosen in `cmd/finfocus/main.go` from `filepath.Base(os.Args[0])`:

| Name contains | Mode | Standard output |
| --- | --- | --- |
| `pulumi-language-finfocus` | language host (new) | its own gRPC port, one line, nothing else |
| `pulumi-analyzer-policy-finfocus` | analyzer serve (unchanged) | the analyzer's port, one line |
| `pulumi-analyzer-finfocus` | analyzer serve (unchanged) | the analyzer's port, one line |
| anything else | CLI through `ax.Execute` (unchanged) | command output |

`.exe` is part of the name on Windows and does not change the match.

## RunPlugin request to analyzer process

| RunPluginRequest field | Use |
| --- | --- |
| `args[0]` | engine address, passed through as the analyzer's first argument |
| `info.programDirectory` | working directory of the analyzer when `pwd` is unusable (FR-009, R6) |
| `env` | appended to the language host's environment |
| `pwd` | working directory of the analyzer, and exported as `FINFOCUS_PROJECT_DIR` when absolute, existing, and not already set (FR-014) |
| `kind` | must be `analyzer`; any other kind is rejected with `InvalidArgument` |

The analyzer process: path = the language host's own executable,
`Args = [<legacy entry name>, <engine address>, "."]`.

| Analyzer outcome | RunPlugin result |
| --- | --- |
| stdout bytes | `RunPluginResponse.stdout`, unchanged and in order |
| stderr bytes | `RunPluginResponse.stderr` |
| exit 0 | stream ends without error |
| exit non-zero | `RunPluginResponse.exitcode` with that code, then the stream ends |
| start failure | `RunPlugin` returns the error |
| stream canceled | graceful stop (Unix SIGINT, Windows kill), wait up to 3s |

## InstallResult (JSON output of `finfocus analyzer install`)

One field is added; existing fields keep their names and meaning.

| Field | Type | Meaning |
| --- | --- | --- |
| `policy_pack_dir` | string | existing: the policy pack directory |
| `policy_pack_method` | string | existing: `symlink` or `copy` |
| `policy_pack_language_plugin` | string, omitted when empty | new: absolute path of the language plugin entry |
