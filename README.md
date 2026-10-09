# OneFit-Engine-plot

`plot-go` will render the Grace (xmgrace) project files OneFit writes -
the `.agr`/`.agr-par` plots of every fit - straight to PDF, SVG and PNG,
without Grace, Ghostscript or `epstopdf`. It is a batch renderer for the
part of the Grace format OneFit uses, not a clone of xmgrace; interactive
plots are the OneFit GUI's job.

Status: steps 1 to 5 are done. `plot-go FILE.agr` writes a PDF (or EPS,
SVG) without Grace; the renderer matches gracebat primitive by primitive,
and the output matches gracebat's page pixel for pixel, allowing for text
placed up to 0.7 pt apart (see docs/grace-behaviour.md). plot-go also takes
gracebat's own command line - the one OneFit's C core gives Grace, recorded
from a real fit in testdata/gracebat-call - and writes the same EPS and an
equivalent saved project. Next: making OneFit use it (step 6).

```bash
go install ./cmd/plot-go
plot-go fit-curves-1.agr                  # -> fit-curves-1.pdf
plot-go -o plot.svg fit-curves-1.agr      # format from the extension
plot-go -format eps fit-curves-1.agr      # what OneFit's C core asks for

# gracebat's command line, as OneFit's C core calls Grace:
plot-go -settype xydy gnu0.da_ -nxy fit-curves-1 -param fit1.agr-par \
  -hdevice EPS -hardcopy -printfile fit-curves-1.eps -saveall fit-curves-1.agr
```

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

## Clean-room rule

plot-go is distributed under the Artistic License 2.0 (see
[LICENSE](LICENSE) and [NOTICE](NOTICE)). Grace is GPL, so plot-go is
written from the format itself and Grace's documentation (the User's
Guide), never from Grace's source code. `gracebat` is used only as a
black box: its output is the reference the tests compare against.

## Test data

- `testdata/corpus/`: nine real OneFit plots chosen to cover the format's
  variety - normal and log axes, decimal and power tick labels, 3 to 7
  sets, curve labels, every text escape, small symbols, and one malformed
  tick label format. They come from the OneFit-Engine test suites and the
  p96-10 sample that is already public in OneFit-Engine-assistant.
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
