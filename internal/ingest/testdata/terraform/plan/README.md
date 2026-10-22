# OpenTofu plan fixtures

`show.json` and `drift.json` are the unmodified stdout of `tofu show -json`
on saved plan files. OpenTofu v1.13.2 (MPL-2.0) produced them. No Terraform
executable or Terraform core source code was used to regenerate these fixtures.
The JSON retains the compatibility field `terraform_version` and the built-in
resource name `terraform_data`; these names are emitted by OpenTofu itself.

The reproducible configurations and generation script are in `generate/`.
With OpenTofu and Python 3 installed, run from the repository root:

```bash
TOFU_BIN=/path/to/tofu bash internal/ingest/testdata/terraform/plan/generate/generate.sh
```

`TOFU_BIN` defaults to `tofu`. Provider versions are pinned in `main.tf` and
`baseline.tf.txt`. `tofu init` installs these providers from the OpenTofu
registry; an existing local provider mirror can be selected using
`TF_CLI_CONFIG_FILE`. The generated plan files, state, provider binaries, and
local output files remain in a temporary directory, which the script removes.
Only the two JSON files are copied into this directory.

The baseline apply targets local-only resources: `terraform_data`, local files,
random values, time, archive creation, and a local module. The TLS resource is
planned only, so no private key is generated or included in the fixture.
It never applies the AWS, Cloudflare, or Helm resources. The `aws` provider uses
fake keys and sets `skip_credentials_validation`, `skip_requesting_account_id`,
`skip_metadata_api_check`, and `skip_region_validation`. AWS, Cloudflare, Helm,
and HTTP endpoints point to loopback. Cloudflare uses a dummy fixture token.
The final plan disables refresh. No cloud account, login, or remote backend is
needed; no real cloud resource is applied.

The final configuration changes the sensitive input, replaces a local resource,
removes another, leaves a resource unchanged, and adds fresh resources. Both
HTTP and external data sources depend on a changed resource, so OpenTofu emits
genuine deferred `read` actions in `resource_changes`. Their entries were not
copied or constructed after generation. The source attributes, nulls, sensitive
markers, and unknown list elements are all emitted by the pinned providers and
OpenTofu. `show.json` covers all five managed actions, a module resource, and
`random`, `null`, `helm`, `time`, `tls`, `local`, `archive`, `external`, `http`,
and Cloudflare providers.

`drift.json` is produced by applying a separate local-file configuration,
editing its output file, then running `tofu plan -refresh-only` followed by
`tofu show -json`. It contains `resource_drift` and no `resource_changes`.
FinFocus product code never runs init or apply; those commands are only part
of this fixture-generation script.
