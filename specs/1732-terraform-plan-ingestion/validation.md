---
title: "Terraform Plan Ingestion Validation"
description: "Implementation checks and credential-free quickstart results"
layout: "docs"
---

Validated on Linux on 2026-10-21. The implementation covers all seven user
stories in `spec.md` and all forty original tasks and the five follow-up tasks in `tasks.md`.

## Initial automated checks

| Check | Result |
| --- | --- |
| `make test` | Passed, including frontend and bundled plugin suites |
| `go test ./...` | Passed, including integration packages |
| `make lint` | Passed, including Go, Markdown, and workflow checks |
| `make validate` | Passed; module tidy diff empty and vet clean |
| `make docs-lint` | Passed; 107 Markdown files |
| `make build` | Passed |
| Race detector on ingest, Terraform runner, and history packages | Passed |
| Race detector on Terraform CLI and overview regressions | Passed |
| Windows amd64 and macOS arm64 CLI builds | Passed |
| `git diff --check` | Passed |

The MCP tool golden was regenerated with
`UPDATE_GOLDEN=1 go test ./internal/cli -run '^TestMCPSchemaToolAllowList$'`.
Its contents remain unchanged because it records tool names. The generated
MCP schemas for projected, forecast, estimate, recommendations, and overview
were also checked directly: all expose `terraform-plan` and
`include-workspace`.

## Coverage

Measured with `go test ./internal/ingest ./internal/terraform -count=1
-coverprofile=<temporary-file>` and statement counts from the profile.

| Scope | Covered statements | Coverage | Critical-path target |
| --- | ---: | ---: | ---: |
| `internal/terraform` | 212 / 217 | 97.70% | 95% |
| `internal/ingest/terraform_plan*.go` | 305 / 320 | 95.31% | 95% |
| Entire `internal/ingest` package | Go test package result | 95.7% | 95% |

## Credential-free quickstart

The built CLI was run in a temporary Terraform directory with an executable
fake Terraform binary. Projected, forecast, estimate, recommendations,
overview, and bare `finfocus` each planned and printed the exact no-change
message. Saved plan input exited successfully without invoking an
infrastructure binary. Conflicting inputs, actual cost without a state file,
and history collection each exited 2 without invoking that binary.

Automated tests additionally exercised the existing Pulumi preview path,
Pulumi/Terraform marker precedence, ESC's parent Pulumi lookup, all failure
hints, temporary-file cleanup and Unix process-group cancellation, identity
across temporary git repositories, workspace overrides, history readers,
and projection snapshot persistence. Plugin resolver tests cover mapped
current and old properties, Terraform address selectors, custom-provider
routing, and real cloud IDs in overview actual-cost requests.

The parser passes the shipped SDK request and response validators. Regression
tests cover sensitive values changing classification between snapshots and
unknown values inside lists. A list containing a public unknown is omitted
and advertised as unknown at the list path because the SDK validator cannot
traverse list elements in unknown-property paths.

## Checks requiring another environment

No live cloud-account projection, credentialed backend access, real Pulumi
stack preview, or live actual-cost query was performed. The committed genuine
show JSON fixture and fake process/plugin tests cover their integration
boundaries. Windows cancellation is documented as untested at runtime;
Windows and macOS were checked by compilation.

## Extension hooks

`.specify/extensions.yml` is absent. No before-implement or after-implement
hooks required dispatch.

## Fresh real Terraform plan check

A follow-up check on 2026-10-21 used Terraform 1.16.1 and AWS provider 6.66.0
to initialize and plan a new configuration. It proposed three creates:
`aws_instance.web` (`t3.micro`), `aws_ebs_volume.data` (100 GiB gp3), and
`aws_s3_bucket.assets`. Dummy credentials, disabled credential checks, and
loopback service endpoints kept the test independent of a cloud account.
No apply was run.

The configuration, binary plan, JSON plan, and individual command logs remain
in `/tmp/finfocus-real-terraform-k5t2h6dl/`. `results.json` summarizes the
observed command outcomes.

Generation used real executables:

```bash
terraform init -input=false -no-color
terraform plan -input=false -lock=false -no-color -out=plan.bin
terraform show -json plan.bin > plan.json
finfocus cost projected --terraform-plan plan.json --output json
finfocus cost projected --output json
finfocus overview --output json
finfocus --web --no-browser
```

Both projected commands exited 0 and reported the same partial monthly total:
USD 15.592, consisting of USD 7.592 for EC2 and USD 8.00 for EBS. Each retained
all three create operations. S3 returned `NO_COST_DATA`: the installed
`finfocus-plugin-aws-public` rejected the request because `resource.sku` was
missing. The total therefore excludes S3; the real-plan run is not a clean
all-resources pricing pass. CLI overview exited 0 with three resources and
the same projected monthly total.

`finfocus --web --no-browser` exited 1 before starting its HTTP server. Its
error was `auto-detecting Pulumi project: find pulumi project: no Pulumi
project found in current or parent directories; use --pulumi-json to provide
input directly`. A Terraform web dashboard could not be rendered. The
feature's original contract explicitly excluded web support; this test
confirms that limitation in the real executable.

## OpenTofu-only and web follow-up

The owner's follow-up supersedes the original binary preference and web
exemption. FinFocus now selects only `tofu` for automatic plans, version
checks, and workspace lookup. Sentinel tests verify that a `terraform`
executable is never called, even when both commands are installed. The
minimum OpenTofu version is 1.6.0, and failed, missing, or malformed version
responses stop before planning.

Source/dependency checks found no Terraform core source import, module
requirement, bundled executable, or BSL license header in the checked Go and
license files. No Terraform/OpenTofu core implementation was copied into
FinFocus. The runner invokes OpenTofu as an external process. OpenTofu's
[upstream license](https://github.com/opentofu/opentofu/blob/main/LICENSE)
is MPL-2.0. The locally installed AWS provider's `LICENSE.txt` also declares
MPL-2.0. The checked files cover this integration and its module manifests.

OpenTofu 1.13.2 was downloaded from its official GitHub release. The Linux
amd64 archive's SHA-256 matched the published checksum:
`723bebb0df8411daff1e5fc3862e28ff5581fe5634087f4c29a9aa41d78208b5`.
The version is pinned in `mise.toml`. Both new committed plan fixtures were
regenerated using that executable; they are unmodified `tofu show -json`
stdout. Their reproducible configurations and generator are committed next
to the fixtures. No Terraform executable was used for regeneration.

A separate fresh OpenTofu project is retained at
`/tmp/finfocus-real-opentofu-6cnqk_mm/`. Initialization and planning succeeded
with AWS provider 6.66.0 from the OpenTofu registry, dummy credentials,
loopback service endpoints, and no cloud apply. It planned EC2, EBS, and S3
creates. The web server started successfully in that directory and the
populated dashboard rendered the same partial USD 15.592 monthly total:
USD 7.592 for EC2 and USD 8.00 for EBS. S3 remains unpriced because the
installed AWS public pricing plugin requires a SKU; this follow-up does not
change that plugin's resource pricing support.

Browser checks confirmed all three Terraform addresses and create statuses,
correct summary/table placement, a live session, and no console errors.
Existing overview, actual-cost, recommendations, estimate-resource, and
estimate-baseline endpoints returned HTTP 200. The real EC2 baseline was
USD 7.592/month. All resources are newly planned, so actual cost correctly
has no deployed resources. Tests verify that tab queries reuse captured
plan descriptors and do not invoke Pulumi or start another plan.

The estimate check exposed and fixed validation of `terraform:` metadata:
those keys now follow the same identifier rules as the existing `pulumi:`
namespace. Arbitrary namespaces and malformed identifiers remain rejected.
Pulumi decryption and retry regressions continue to pass.

Browser render evidence:

- [Desktop overview](validation/opentofu-web-overview.png)
- [Narrow overview](validation/opentofu-web-overview-narrow.png)

The required UX-only follow-up is
[issue #1888](https://github.com/rshade/finfocus/issues/1888). It covers control
copy, address wrapping, missing-price presentation, editable metadata
visibility, and accessibility. It excludes engine, API, redaction, pricing,
and data-shape fixes.

Follow-up verification passed: `make test`, `make lint`, `make validate`,
`make docs-lint`, targeted race tests for OpenTofu execution/parsing/estimates
and web integration, and `shellcheck` for the fixture generator. A coverage
run exposed a timing-sensitive Unix process termination assertion; it now
waits for the signaled child to terminate and passed 20 consecutive runs.
Final statement coverage is 202/207 (97.58%) for `internal/terraform` and
305/320 (95.31%) for the plan parser files. The entire ingest package remains
at 95.7%.
