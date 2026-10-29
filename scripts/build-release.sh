#!/usr/bin/env bash
# Builds plot-go for every platform OneFit runs on into dist/:
#   plot-go_OS_ARCH (.exe on Windows)    the bare binaries - names without
#                                        the version, so ".../releases/latest/
#                                        download/plot-go_linux_arm64" always
#                                        fetches the newest (OFE-Raku's INSTALL
#                                        downloads them by these names)
#   LICENSE, THIRD-PARTY-LICENSES.txt, MustRead.html
#                                        the licences that go with them
#   plot-go-VERSION-OS-ARCH.zip          one binary and those three files
#   SHA256SUMS                           of the binaries and the zips
# The version is in the binary (plot-go -version) and the binaries also
# carry the notices (-notices). plot-go is pure Go (no cgo), so one machine
# builds them all. The release workflow runs this on a version tag; it also
# runs by hand for a local test.
#
#   VERSION=0.1.0 scripts/build-release.sh
set -euo pipefail

cd "$(dirname "$0")/.."
version="${VERSION:-$(git describe --tags --always --dirty | sed 's/^v//')}"
rm -rf dist
mkdir -p dist
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  os="${target%/*}" arch="${target#*/}"
  out="dist/plot-go_${os}_${arch}"
  [ "$os" = windows ] && out="$out.exe"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$out" ./cmd/plot-go
  echo "$out"
done
# The licences: LICENSE (Artistic 2.0), MustRead.html on its own (Adobe's
# AFM metrics may not go anywhere without that file) and the NOTICE with
# every embedded font's and library's licence in THIRD-PARTY-LICENSES.txt.
cp LICENSE internal/afm/adobe-core14/MustRead.html dist/
section() { # TITLE FILE: one text under a heading
  printf '\n\n%s\n%s\n%s\n\n' "========================================================================" "$1" "========================================================================"
  cat "$2"
}
{
  echo "plot-go $version: notices and third-party licences"
  echo
  echo "plot-go is distributed under the Artistic License 2.0 (LICENSE). It"
  echo "embeds the components below, each under its own licence. Adobe's terms"
  echo "for the AFM font metrics are in MustRead.html, which must accompany them."
  section "plot-go: NOTICE" NOTICE
  section "Liberation fonts: SIL Open Font License 1.1" internal/glyph/OFL.txt
  section "github.com/go-fonts/liberation: LICENSE" internal/glyph/go-fonts-LICENSE
  section "golang.org/x/image: LICENSE" internal/glyph/x-image-LICENSE
  section "golang.org/x/image: PATENTS" internal/glyph/x-image-PATENTS
  section "golang.org/x/text: LICENSE" internal/glyph/x-text-LICENSE
  section "golang.org/x/text: PATENTS" internal/glyph/x-text-PATENTS
} > dist/THIRD-PARTY-LICENSES.txt
# One zip per platform, its files in a folder of the zip's own name; the
# times are the commit's, so the same commit gives the same zip.
stamp="$(git log -1 --format=%cd --date=format:%Y%m%d%H%M.%S)"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
for bin in dist/plot-go_*; do
  platform="${bin#dist/plot-go_}" ext=""
  case "$platform" in *.exe) platform="${platform%.exe}" ext=.exe ;; esac
  name="plot-go-$version-${platform/_/-}"
  mkdir "$stage/$name"
  cp "$bin" "$stage/$name/plot-go$ext"
  cp dist/LICENSE dist/THIRD-PARTY-LICENSES.txt dist/MustRead.html "$stage/$name/"
  touch -t "$stamp" "$stage/$name" "$stage/$name"/*
  (cd "$stage" && TZ=UTC zip -q -X -r "$OLDPWD/dist/$name.zip" "$name")
done
(cd dist && sha256sum plot-go_* plot-go-*.zip > SHA256SUMS)
