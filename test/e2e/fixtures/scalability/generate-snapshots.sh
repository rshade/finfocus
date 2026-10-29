#!/usr/bin/env bash
# Regenerates the checked-in scalability plan snapshots.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")"
mkdir -p snapshots
for count in 10 50 100 500 1000; do
  go run ./cmd/generate -count="$count" -output="snapshots/plan-$count.json"
done
echo "Generated all snapshots."
