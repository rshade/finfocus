#!/usr/bin/env bash
set -euo pipefail

spike_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_dir=$(git -C "$spike_dir" rev-parse --show-toplevel)
spike_tmp=$(mktemp -d /tmp/finfocus-clef-spike.XXXXXX)
trap 'rm -rf -- "$spike_tmp"' EXIT

# Isolate the experiment's extra dependency from the production module.
cp -a "$repo_dir/plugins/scorer/." "$spike_tmp/"
cp "$spike_dir/spike_test.go" "$spike_tmp/internal/backends/jev/clef_spike_test.go"
mkdir -p "$spike_tmp/internal/backends/jev/testdata"
cp "$spike_dir/testdata/response.json" "$spike_tmp/internal/backends/jev/testdata/clef-response.json"
cd "$spike_tmp"
export GOWORK=off
go get github.com/rshade/go-decide/clefclient@v0.1.0
gofmt -w internal/backends/jev/clef_spike_test.go
go test -tags clefspike -v -count=1 -run '^TestClefSpike$' ./internal/backends/jev
