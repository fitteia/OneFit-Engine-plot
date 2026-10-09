# OneFit-Engine-plot

`plot-go` will render the Grace (xmgrace) project files OneFit writes -
the `.agr`/`.agr-par` plots of every fit - straight to PDF, SVG and PNG,
without Grace, Ghostscript or `epstopdf`. It is a batch renderer for the
part of the Grace format OneFit uses, not a clone of xmgrace; interactive
plots are the OneFit GUI's job.

Status: step 1 of the plan below - the feature inventory - is done. No
renderer yet.

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

## Clean-room rule

plot-go is distributed under the Artistic License 2.0 (see
[LICENSE](LICENSE) and [NOTICE](NOTICE)). Grace is GPL, so plot-go is
written from the format itself and Grace's documentation (the User's
Guide), never from Grace's source code. `gracebat` is used only as a
black box: its output is the reference the tests compare against.

## Development

```bash
go vet ./...
go test ./...
go run ./cmd/agr-inventory -o docs/inventory.md DIR...
```
