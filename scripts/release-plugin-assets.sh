#!/usr/bin/env bash
# Build release archives for a plugin shipped from the finfocus monorepo.
# Usage: release-plugin-assets.sh <module-dir> <package> <binary-name> <version> <out-dir>
set -euo pipefail

if [[ $# -ne 5 ]]; then
  echo "usage: $0 <module-dir> <package> <binary-name> <version> <out-dir>" >&2
  exit 2
fi
module_dir=$1 pkg=$2 binary=$3 version=$4 out_dir=$5

if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  echo "version must be canonical (vX.Y.Z), got: $version" >&2
  exit 2
fi

mkdir -p "$out_dir"
out_dir=$(cd "$out_dir" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

targets=(linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64)
for target in "${targets[@]}"; do
  goos=${target%/*} goarch=${target#*/}
  name="${binary}_${version}_${goos}_${goarch}"
  exe=$binary
  [[ $goos == windows ]] && exe="$binary.exe"
  mkdir -p "$work/$name"
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch \
    go -C "$module_dir" build -trimpath -ldflags "-s -w -X main.version=$version" -o "$work/$name/$exe" "$pkg"
  if [[ $goos == windows ]]; then
    (cd "$work/$name" && zip -q "$out_dir/$name.zip" "$exe")
  else
    tar -C "$work/$name" -czf "$out_dir/$name.tar.gz" "$exe"
  fi
done

(cd "$out_dir" && sha256sum -- *.tar.gz *.zip > checksums.txt)
echo "assets written to $out_dir"
