#!/usr/bin/env bash
# Builds plot-go for every platform OneFit runs on into dist/:
# plot-go_OS_ARCH (.exe on Windows) and SHA256SUMS - names without the
# version, so ".../releases/latest/download/plot-go_linux_arm64" always
# fetches the newest (the version is in the binary: plot-go -version).
# plot-go is pure Go (no cgo), so one machine builds them all. The release workflow runs
# this on a version tag; it also runs by hand for a local test.
#
#   VERSION=0.1.0 scripts/build-release.sh
set -euo pipefail

cd "$(dirname "$0")/.."
version="${VERSION:-$(git describe --tags --always --dirty | sed 's/^v//')}"
rm -rf dist
mkdir -p dist
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
  os="${target%/*}" arch="${target#*/}"
  out="dist/plot-go_${os}_${arch}"
  [ "$os" = windows ] && out="$out.exe"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$out" ./cmd/plot-go
  echo "$out"
done
# the notices go with the binaries (the binaries also carry them: -notices)
cp LICENSE NOTICE internal/afm/adobe-core14/MustRead.html dist/
(cd dist && sha256sum plot-go_* > SHA256SUMS)
