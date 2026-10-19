# OneFit-Engine-plot

`plot-go` renders the Grace (xmgrace) project files OneFit writes - the
`.agr`/`.agr-par` plots of every fit - straight to PDF, EPS and SVG,
without Grace, Ghostscript or `epstopdf`. It is a batch renderer for the
part of the Grace format OneFit uses, not a clone of xmgrace; interactive
plots are the OneFit GUI's job.

Status: steps 1 to 6 of the plan below are done; step 7 is the OneFit
GUI's Figures editor, in use and growing. What plot-go draws, and
what it reads and keeps without drawing, is in
[docs/capabilities.md](docs/capabilities.md): a measured subset of Grace,
not all of it; what is still to come, in [docs/roadmap.md](docs/roadmap.md). On every corpus file plot-go matches gracebat within stated
tolerances - primitive by primitive, and on the rendered page - not
exactly; the tolerances are in
[docs/grace-behaviour.md](docs/grace-behaviour.md) ("How closely plot-go
matches"). plot-go also takes gracebat's own command line - the one
OneFit's C core gives Grace, recorded from real fits in
testdata/gracebat-call* - and writes the EPS and a saved project. OneFit's
C core uses it when installed, and the OneFit GUI draws its Figures with
it.

```bash
go install ./cmd/plot-go
plot-go fit-curves-1.agr                  # -> fit-curves-1.pdf
plot-go -o plot.svg fit-curves-1.agr      # format from the extension
plot-go -format eps fit-curves-1.agr      # what OneFit's C core asks for
plot-go -o plot.png fit-curves-1.agr      # cropped to the drawing, at 300 dpi
plot-go -dpi 600 -o plot.png fit-curves-1.agr   # line art at 600 dpi
plot-go -fullpage -o plot.png fit-curves-1.agr  # the whole page, as gracebat's PNG
plot-go -dpi 600 -o plot.tiff fit-curves-1.agr  # lossless TIFF (.tif also accepted)

# gracebat's command line, as OneFit's C core calls Grace:
plot-go -settype xydy gnu0.da_ -nxy fit-curves-1 -param fit1.agr-par \
  -hdevice EPS -hardcopy -printfile fit-curves-1.eps -saveall fit-curves-1.agr
```

PNG and TIFF are cropped to the drawing, as the PDF, EPS and SVG are, so they
drop straight into a document; `-fullpage` keeps the whole page instead (what
gracebat's PNG is). This holds for gracebat's command line (`-hdevice PNG`) too.
`-dpi` selects their resolution
(default 300); it does not affect PDF/EPS/SVG. TIFF uses lossless compression
and both formats record resolution metadata. Exports accept 1–2400 DPI and
at most 100 million pixels per page; choose a lower DPI for a large page.

Raster export needs one installed converter: Poppler (`pdftoppm`) is tried
first, then Ghostscript, then ImageMagick. On Windows, Ghostscript's console
programs `gswin64c.exe` and `gswin32c.exe` are found on PATH or in the standard
Program Files `gs/gs*/bin` folders, newest version first. Add a nonstandard
installation's `bin` directory to PATH. Windows' unrelated `convert.exe` is
never used. ImageMagick may also need Ghostscript and a policy allowing PDF.
Vector exports do not require any of these converters.

## Plan

1. Inventory: list every directive and text escape OneFit's files use
   (`cmd/agr-inventory`, results in [`docs/features.md`](docs/features.md)
   and [`docs/inventory.md`](docs/inventory.md)).
2. Parser and model for that part of the format.
3. Renderer: page/viewport/world coordinates, normal and log axes, ticks
   and tick labels, xy and xydy sets, symbols, line styles, error bars,
   legend, strings, Grace text escapes.
4. Reference tests: render a curated set of real files with plot-go and
   with `gracebat`, rasterise both, compare within a tolerance, in CI.
5. A command line compatible with the `gracebat` call OneFit's C core
   makes (`-nxy`, `-settype xydy`, `-param`, `-printfile`, `-device`,
   `-saveall`), writing PDF directly.
6. The C core uses plot-go when installed and falls back to `gracebat`;
   `onefite doctor` reports which.
7. A live, editable preview in the OneFit GUI: the GUI shows plot-go's
   SVG - the same drawing as the PDF - and edits (ranges, labels, styles,
   texts) go into the `.agr` and re-render. Both are Artistic 2.0, so the
   GUI can use plot-go as a library.

## Install

Prebuilt binaries for Linux (amd64, arm64), macOS (Intel, Apple Silicon)
and Windows are attached to each [release](https://github.com/fitteia/OneFit-Engine-plot/releases)
(`plot-go_OS_ARCH`, with `SHA256SUMS`; `releases/latest/download/plot-go_linux_amd64`
is always the newest); put one on the PATH as `plot-go`. Or, with Go: `go install github.com/fitteia/OneFit-Engine-plot/cmd/plot-go@latest`.

Building from source requires Go 1.25 or newer; use the latest security patch
of your Go release. Release builds use the current stable Go toolchain.
The image/font and text libraries are pinned to `golang.org/x/image v0.45.0`
and `golang.org/x/text v0.41.0`, including fixes for malformed TIFF and SFNT
font inputs.

OneFit's C core (OneFit-Engine-C) uses plot-go for its plots when it finds
it on the PATH, and Grace otherwise; `OFE_PLOTTER=grace` forces Grace.

plot-go measures and draws text with the URW base-35 fonts, as Ghostscript
and gracebat do, when they are installed: `fonts-urw-base35` on Debian and
Ubuntu; on macOS the onefite-go Homebrew formula installs them (in
Homebrew's `share/fonts/urw-base35`), as does `brew install --cask
font-urw-base35` (into `~/Library/Fonts`); or any folder named in
`OFE_URW_FONTS`. Without them it
uses the Liberation fonts it carries: same widths, but legends and justified
texts can sit 1.5-2 points off gracebat's.

## Clean-room rule

plot-go is distributed under the Artistic License 2.0 (see
[LICENSE](LICENSE) and [NOTICE](NOTICE)). Grace is GPL, so plot-go is
written from the format itself and Grace's documentation (the User's
Guide), never from Grace's source code. `gracebat` is used only as a
black box: its output is the reference the tests compare against.

Common text controls include underline and overline (`\u…\U`, `\o…\O`);
plot-go places overlines above the glyphs, correcting Grace's baseline-level
placement. Other controls include relative/absolute size changes
(`\z{1.5}`, `\Z{1.5}`), horizontal/vertical
shifts (`\h{}`, `\v{}`), baseline changes (`\V{}`), newlines (`\n`) and
hexadecimal characters (`\#{b5}`). They work wherever a Grace string is
drawn, including titles, axis labels, legends and point annotations.
Inline matrices (`\t{}`, `\T{}`), rotation/slant (`\r{}`, `\l{}`, `\q`,
`\Q`), colours (`\R{}`), pair kerning (`\Fk`, `\FK`), position marks
(`\m{}`, `\M{}`), and independent character/advance directions
(`\dl`, `\dr`, `\dL`, `\dR`) are also supported. Ligature toggles
(`\Fl`, `\FL`) are accepted; the supported Core-14 Latin-1 fonts do not
provide encodable ligature substitutions, matching the measured Grace output.
Additional installed TTF/OpenType fonts can be used by PostScript or full name.
Run `plot-go -list-fonts` to list discovered names. Set `OFE_FONT_PATH` to a
font file or a path-list of font directories to use fonts outside the usual
system folders. Their glyphs are embedded in PDF/EPS and outlined in SVG, so
viewers do not need the font installed. See [fonts.md](docs/fonts.md) for
examples, search order, embedding restrictions and encoding limits.

The [capabilities](docs/capabilities.md) list and
[roadmap](docs/roadmap.md) describe the supported controls and remaining font limitations.

## Test data

- `testdata/corpus/`: nine real OneFit plots chosen to cover the format's
  variety - normal and log axes, decimal and power tick labels, 3 to 7
  sets, curve labels, every text escape, small symbols, and one malformed
  tick label format; they come from the OneFit-Engine test suites and the
  p96-10 sample that is already public in OneFit-Engine-assistant. And
  `style-*.agr`, one per feature measured since (frames, patterns, legends,
  set fills, ...), each named in docs/grace-behaviour.md.
- `testdata/ref/`: gracebat's rendering of each, made by
  `scripts/make-references.sh` (gracebat -> EPS -> Ghostscript, the whole
  page at 144 dpi). Regenerating gives byte-identical files.

## Development

```bash
go vet ./...
go test ./...
go run ./cmd/agr-inventory -o docs/inventory.md DIR...
PLOT_GO_CORPUS=DIR:DIR go test ./agr/   # also parse a whole local corpus
scripts/make-references.sh               # needs gracebat and gs
```

### Point annotations

The renderer supports Grace's `avalue` point labels: X, Y, X/Y, quoted
per-point text or the third numeric column. For example, `s0 avalue on`
and `s0 avalue type 2` label each point with Y; type 4 uses a trailing
quoted label in its data row (`1 2 "peak"`). Font, color, size, rotation,
format, precision, prefix/suffix and viewport offsets round-trip through
saving. See [the measured behaviour](docs/grace-behaviour.md#point-annotations)
and [roadmap](docs/roadmap.md) for release and Figures integration status.
