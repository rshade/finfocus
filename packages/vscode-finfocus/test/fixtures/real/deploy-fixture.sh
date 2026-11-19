#!/usr/bin/env bash
# Deploys testdata/sourcepos/nodejs/program to a throwaway local backend so the
# real-CLI tests have a stack to read. Needs pulumi, node and npm.
#
#   project=$(test/fixtures/real/deploy-fixture.sh "$SCRATCH")
#
# prints the project directory on stdout (stack `dev`). The tests then need
#   FINFOCUS_FIXTURE_PROJECT=<that directory> FINFOCUS_FIXTURE_STACK=dev
#   NODE_PATH=<that directory>/node_modules PULUMI_BACKEND_URL=file://$SCRATCH/backend
#   PULUMI_CONFIG_PASSPHRASE=fixture-passphrase
# The passphrase is a throwaway: an empty one does not survive an editor host.
set -euo pipefail

scratch="${1:?usage: deploy-fixture.sh <scratch directory>}"
root="$(git -C "$(dirname "${BASH_SOURCE[0]}")" rev-parse --show-toplevel)"
source_dir="$root/testdata/sourcepos/nodejs"

mkdir -p "$scratch/backend"
cp -R "$source_dir/program" "$scratch/shop"
cp -R "$source_dir/vendor-lib" "$scratch/vendor-lib"
cp -R "$source_dir/pack-lib" "$scratch/pack-lib"

export PULUMI_BACKEND_URL="file://$scratch/backend"
export PULUMI_CONFIG_PASSPHRASE="${PULUMI_CONFIG_PASSPHRASE:-fixture-passphrase}"
# vendor-lib is linked from outside the project and resolves @pulumi/pulumi through this.
export NODE_PATH="$scratch/shop/node_modules"

{
  (cd "$scratch/pack-lib" && npm pack --silent)
  cd "$scratch/shop"
  npm install --no-audit --no-fund --silent
  pulumi stack init dev
  pulumi up --yes --skip-preview --stack dev
} >&2

echo "$scratch/shop"
