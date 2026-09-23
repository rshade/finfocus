# Real Terraform State Fixture (aws-realistic)

`terraform.tfstate` and `terraform-tainted.tfstate` are real state files
written by OpenTofu and the real `hashicorp/aws` provider, not hand-written
JSON. The project in this directory was applied against a local
[moto](https://github.com/getmoto/moto) AWS emulator, so no cloud account
was touched. The account ID `123456789012` and the resource IDs are fake
values from the emulator.

## Contents

| Address | Notes |
| --- | --- |
| `data.aws_ami.amazon_linux`, `data.aws_availability_zones.available` | Data sources; ingestion must skip them |
| `aws_vpc.main`, `aws_subnet.a`, `aws_security_group.web` | Networking |
| `aws_instance.web` | t3.micro in us-east-1a |
| `module.app.aws_instance.app[0..1]` | Module-nested `count = 2`, m5.large |
| `aws_instance.worker["small"]`, `aws_instance.worker["large"]` | `for_each`, t3.small and c5.xlarge |
| `aws_ebs_volume.data`, `aws_volume_attachment.data` | gp3, 100 GiB |
| `aws_s3_bucket.assets`, `aws_iam_role.app`, `aws_dynamodb_table.sessions` | Unpriced or usage-priced |
| `aws_db_instance.main` | db.t3.micro, postgres 16.3, 20 GiB gp3 |

`terraform-tainted.tfstate` is the same state after
`tofu taint 'aws_instance.worker["small"]'`. It exercises dropping tainted
instances.

## Regenerating

Requires Docker and [mise](https://mise.jdx.dev/). OpenTofu runs through
`mise exec`, so it is not a dependency of `make test`.

```bash
make gen-terraform-goldens
UPDATE_GOLDEN=1 go test ./internal/ingest/ ./internal/cli/ -run 'RealTerraformState'
```

`scripts/gen-terraform-goldens.sh` starts `motoserver/moto` on a free local
port and runs `tofu init` and `tofu apply` in a temporary copy of this
directory. It then copies the state and `.terraform.lock.hcl` back, taints one
instance to produce the second state, and runs `tofu destroy`. A trap removes
the container and the temporary directory. The script refuses to copy a state
that contains the host's home directory, working directory, or username.
Override the pinned versions with `MOTO_IMAGE` and `TOFU_VERSION`.

Regenerating changes the lineage and cloud IDs. The goldens in
`internal/ingest/testdata/terraform/` and `internal/cli/testdata/terraform/`
record only addresses and pricing-relevant properties, so they change only
when the mapping or pricing behavior changes.
