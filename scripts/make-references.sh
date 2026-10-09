#!/usr/bin/env bash
# Renders each testdata/corpus/*.agr with Grace's gracebat - the reference
# plot-go is compared against - into testdata/ref/NAME.png: gracebat writes
# EPS (it has no PDF device; OneFit's C core does the same, then epstopdf),
# and Ghostscript renders the whole page, 773x600 points, at 144 dpi.
#
# gracebat is used only as a black box (see AGENTS.md). Needs gracebat and gs.
#
#   scripts/make-references.sh [FILE.agr...]
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p testdata/ref
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

files=("$@")
[ ${#files[@]} -gt 0 ] || files=(testdata/corpus/*.agr)

for agr in "${files[@]}"; do
  name=$(basename "$agr" .agr)
  gracebat -hdevice EPS -printfile "$tmp/$name.eps" "$agr" > "$tmp/$name.log" 2>&1 || {
    echo "gracebat failed on $agr:" >&2
    cat "$tmp/$name.log" >&2
    exit 1
  }
  gs -q -dSAFER -dBATCH -dNOPAUSE -sDEVICE=png16m -r144 -g1546x1200 -dFIXEDMEDIA \
    -dTextAlphaBits=4 -dGraphicsAlphaBits=4 \
    -sOutputFile="testdata/ref/$name.png" "$tmp/$name.eps"
  echo "testdata/ref/$name.png"
done
