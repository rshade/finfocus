#!/usr/bin/env bash
# Generate genuine OpenTofu plan JSON without applying cloud resources.
set -euo pipefail

fixture_source="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fixture_output="$(dirname "$fixture_source")"
fixture_work="$(mktemp -d "${TMPDIR:-/tmp}/finfocus-tofu-fixtures.XXXXXX")"
tofu_binary="${TOFU_BIN:-tofu}"
trap 'rm -rf "$fixture_work"' EXIT

"$tofu_binary" version | head -n 1
mkdir -p "$fixture_work/plan" "$fixture_work/drift"
cp -R "$fixture_source/mod" "$fixture_source/chart" "$fixture_work/plan/"
cp "$fixture_source/echo.py" "$fixture_work/plan/echo.py"
cp "$fixture_source/baseline.tf.txt" "$fixture_work/plan/main.tf"

"$tofu_binary" -chdir="$fixture_work/plan" init -input=false -no-color
# These targets touch only local files, generated keys/random values, and local state.
"$tofu_binary" -chdir="$fixture_work/plan" apply -input=false -lock=false -no-color -auto-approve \
  -target=random_pet.name -target=time_static.stamp \
  -target=local_file.plain -target=local_sensitive_file.secret -target=archive_file.zip \
  -target=terraform_data.item -target=terraform_data.stable -target=terraform_data.swap \
  -target=terraform_data.holder -target=terraform_data.gone -target=module.net \
  -target=null_resource.marker

cp "$fixture_source/main.tf" "$fixture_work/plan/main.tf"
# No cloud resource has been applied, and refresh is disabled for this plan.
"$tofu_binary" -chdir="$fixture_work/plan" plan -input=false -lock=false -no-color \
  -refresh=false -out=plan.bin
"$tofu_binary" -chdir="$fixture_work/plan" show -json plan.bin > "$fixture_output/show.json"

cp "$fixture_source/drift.tf.txt" "$fixture_work/drift/main.tf"
"$tofu_binary" -chdir="$fixture_work/drift" init -input=false -no-color
"$tofu_binary" -chdir="$fixture_work/drift" apply -input=false -lock=false -no-color -auto-approve
printf 'changed outside OpenTofu\n' > "$fixture_work/drift/only.txt"
"$tofu_binary" -chdir="$fixture_work/drift" plan -refresh-only -input=false -lock=false \
  -no-color -out=drift.bin
"$tofu_binary" -chdir="$fixture_work/drift" show -json drift.bin > "$fixture_output/drift.json"
