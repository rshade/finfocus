#!/usr/bin/env bash
#
# Regenerates the real Terraform state goldens under
# test/fixtures/terraform/aws-realistic/ by applying the fixture project with
# OpenTofu and the real hashicorp/aws provider against a local moto emulator.
# No cloud account is touched: every provider endpoint points at the emulator.
# Writes terraform.tfstate (as applied) and terraform-tainted.tfstate (the same
# state after `tofu taint` on one for_each instance), and refreshes
# .terraform.lock.hcl.
#
# Requirements: docker, mise (OpenTofu is run through `mise exec`).
#
# Usage: scripts/gen-terraform-goldens.sh
#   MOTO_IMAGE      emulator image (default motoserver/moto:5.2.3)
#   TOFU_VERSION    OpenTofu version resolved by mise (default 1.12.6)

set -euo pipefail

MOTO_IMAGE="${MOTO_IMAGE:-motoserver/moto:5.2.3}"
TOFU_VERSION="${TOFU_VERSION:-1.12.6}"

ROOT="$(git rev-parse --show-toplevel)"
FIXTURE_DIR="$ROOT/test/fixtures/terraform/aws-realistic"

for cmd in docker mise python3; do
    command -v "$cmd" >/dev/null 2>&1 || { echo "Error: $cmd is required" >&2; exit 1; }
done

WORK_DIR="$(mktemp -d)"
CONTAINER=""

cleanup() {
    local status=$?
    if [ -n "$CONTAINER" ]; then
        docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
    fi
    rm -rf "$WORK_DIR"
    exit "$status"
}
trap cleanup EXIT INT TERM

tofu() {
    mise exec "opentofu@$TOFU_VERSION" -- tofu "$@"
}

free_port() {
    python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()'
}

PORT="$(free_port)"
ENDPOINT="http://127.0.0.1:$PORT"

echo "Starting $MOTO_IMAGE on port $PORT..."
CONTAINER="$(docker run -d --rm -p "127.0.0.1:$PORT:5000" "$MOTO_IMAGE")"

for _ in $(seq 1 60); do
    if curl -fsS "$ENDPOINT/moto-api/" >/dev/null 2>&1; then
        break
    fi
    sleep 1
done
curl -fsS "$ENDPOINT/moto-api/" >/dev/null || { echo "Error: emulator did not become ready" >&2; exit 1; }

cp -R "$FIXTURE_DIR/." "$WORK_DIR/"
rm -rf "$WORK_DIR/.terraform" "$WORK_DIR/terraform.tfstate" "$WORK_DIR"/terraform.tfstate.*

export TF_IN_AUTOMATION=1
export TF_INPUT=0

tofu -chdir="$WORK_DIR" init -no-color
tofu -chdir="$WORK_DIR" apply -no-color -auto-approve -var "endpoint=$ENDPOINT"

cp "$WORK_DIR/.terraform.lock.hcl" "$FIXTURE_DIR/.terraform.lock.hcl"

copy_state() {
    local dest="$1"
    if grep -qE "$WORK_DIR|$HOME|$(id -un)" "$WORK_DIR/terraform.tfstate"; then
        echo "Error: generated state leaks a host path or username; refusing to copy it" >&2
        exit 1
    fi
    cp "$WORK_DIR/terraform.tfstate" "$FIXTURE_DIR/$dest"
    echo "Wrote ${FIXTURE_DIR#"$ROOT"/}/$dest"
}

copy_state terraform.tfstate

tofu -chdir="$WORK_DIR" taint -no-color 'aws_instance.worker["small"]'
copy_state terraform-tainted.tfstate

tofu -chdir="$WORK_DIR" destroy -no-color -auto-approve -var "endpoint=$ENDPOINT"
