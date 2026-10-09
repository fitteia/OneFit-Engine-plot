# What plot-go has to draw

Derived from `inventory.md`: 704 distinct `.agr`/`.agr-par` files written
by OneFit (test suites 1-9, p96 individual, global and hybrid fits, the
GUI's saved plots), 2026-10-09. The inventory is regenerated with
`go run ./cmd/agr-inventory -o docs/inventory.md DIR...`.

OneFit always writes the same project template (the Rust `agrs.rs`
generator in OneFit-Engine-go, and the C core's `xmgr.c` call of
`gracebat`); files differ in their values, not their structure. Nearly
every directive appears in all 704 files.

## Page and graph

- One page, `page size 773, 600` (points), white background.
- One graph, `g0`, `type XY`, with its viewport (`view xmin ... ymax`) and
  data window (`world xmin ... ymax`) stated - OneFit computes the axis
  ranges itself, so Grace's autoscaling is never needed.
- Frame: type 0, black, linestyle 1.

## Axes

- `xaxes scale Logarithmic` (427 files) or `Normal` (277); `yaxes scale
  Normal` (646) or `Logarithmic` (58). Never inverted; alternate axes off.
- Ticks: major spacing stated (`tick major 10`, `1`, `60`, ...); minor
  ticks per major (`9`, `1`, ...); ticks `in`, on both sides, `place
  rounded true`; no grid; no special ticks.
- Tick labels: `format decimal` or `power` (with `prec`), char size 1.5,
  font 4, normal place, auto offset, no angle, stagger or skip.
- Axis labels: char size 1.5, font 4, `layout para`, `place auto`.

## Sets

- `xy` sets: the fitted curves - line type 1 (straight), no symbol,
  linewidth 2.0, colors 1-6, linestyles 1, 3, 4, 5, 6, 7 (solid and
  dashed patterns).
- One `xydy` set per graph: the data - symbol 1 (circle), size 1.0 or
  0.5, no line (type 0). Error bars are off in every file (`errorbar
  off`), so dy is read but not drawn.
- Legend strings per set (empty in every file of the corpus); avalues,
  fill, baseline and droplines are all off.

## Legend and strings

- Legend on, at view coordinates (0.977, 0.8), char size 1.5, font 4,
  but its box is not drawn (`legend box pattern 0`) and every set's legend
  text is empty - in practice no legend appears.
- Text strings (`with string`): one at view coordinates (the run's name,
  font 4, size 1.0, blue) and, in 427 files, labels in world coordinates
  next to the curves (font 0, size 1.5). No rotation, left justified.
- Title, subtitle empty; timestamp off; regions (`r0`...) off.

## Text

- Escapes used: `\s` and `\S` (sub- and superscript), `\N` (back to
  normal), `\-` (smaller), `\x` (Symbol font, for Greek), `\0`, `\4`,
  `\8` (switch to font 0, 4, 8).
- Fonts used: Times-Roman (0), Helvetica (4), Symbol (8) - all three are
  standard PDF fonts, so a PDF can name them without embedding.
- Colors: Grace's standard 16-color map, written in every file.

## Oddities to tolerate

A few files have values Grace itself would reject or ignore:
`xaxis ticklabel format unknown` (2 files), `xaxis ticklabel format 60` (1)
and `xaxis ticklabel prec` with no value (4 parameter files). plot-go warns
and keeps going, as gracebat does; for an unknown format gracebat draws
plain decimals (testdata/ref/test1-13.png), and so does plot-go.

Parameter files (`.agr-par`) write their directives without the leading
`@`; Grace accepts both, and so does plot-go.

## Not needed

Grace's interactive editor, autoscaling, other graph types (bar, polar,
pie, ...), several graphs per page, fits/transforms/scripting, regions,
fills, and fonts beyond the three above.
