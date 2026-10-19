# What plot-go has to draw

History: the inventory plot-go started from (2026-10-09), what OneFit's
own files used then. What plot-go draws now - more than this, and what it
does not - is in [capabilities.md](capabilities.md).

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
- Missing, nonpositive or nonfinite page sizes warn and use 792 by 612
  points consistently in the drawing and PDF, EPS and SVG output.
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
  fill, baseline and droplines are all off. Point annotations are now
  supported beyond this inventory; see [capabilities.md](capabilities.md).

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

The current renderer also supports underline/overline, zoom, baseline and
position shifts, newlines, hexadecimal characters and legacy upper-half
character controls. These are additions for imported Grace projects, not
features found in the original OneFit inventory; see
[capabilities.md](capabilities.md) and the `style-text-*` reference fixtures.

## Oddities to tolerate

A few files have values Grace itself would reject or ignore:
`xaxis ticklabel format unknown` (2 files), `xaxis ticklabel format 60` (1)
and `xaxis ticklabel prec` with no value (4 parameter files). plot-go warns
and keeps going, as gracebat does; for an unknown format gracebat draws
numbers in its default `general` format ("100"; testdata/ref/test1-13.png),
and so does plot-go.

Parameter files (`.agr-par`) write their directives without the leading
`@`; Grace accepts both, and so does plot-go.

## Not needed

Grace's interactive editor, autoscaling, other graph types (bar, polar,
pie, ...), several graphs per page, fits/transforms/scripting, regions,
fills, and fonts beyond the three above.

## Text controls added beyond the original inventory

The renderer recognizes the documented Grace text controls in all label
contexts: scripts, fonts, size, decorations, shifts, matrices, rotation/slant,
inline colours, pair kerning, position marks and both direction controls.
Ligature toggles are accepted with the supported Core-14 Latin-1 fonts, which
have no encodable ligature substitutions. The overline intentionally clears
glyph ink instead of following Grace's baseline-level stroke. See
[capabilities.md](capabilities.md) for syntax and font limits, and
[grace-behaviour.md](grace-behaviour.md) for the measured layout rules.

Latin font metrics use the same Latin-1 encoding as exported text and glyph
outlines, including hexadecimal characters such as `\#{b0}C` (degrees
Celsius). This keeps character spacing, alignment and bounds consistent
between PDF and the outlined SVG preview. Symbol and ZapfDingbats retain
their separate font encodings. The `style-text-latin1` fixture checks common
special characters, accented letters, font changes, rotation and justification
against output generated by the Grace executable.

Additional TTF/OpenType fonts are discovered on the rendering machine and
resolved by PostScript or full name. They use their own metrics and outlines;
PDF/EPS embed the glyphs used, and SVG uses paths. This extends the original
inventory without changing its standard-font output. See [fonts.md](fonts.md)
for examples, embedding restrictions and the remaining Latin-1 limitation.

### Drawing object layers

Plot-go adds three drawing-object layers. The default layer follows Grace: an
object draws over the data curves and axes, then under text and legends.
**Behind data** draws the object after the graph background/grid but before its
curves and axes; page-attached objects draw before all graphs. This lets curves
remain visible through an opaque shape fill while later text remains visible
over it. **On top** draws the object after all graphs, legends and text. All
layers retain the object’s data/page attachment. Behind-data and On-top choices
are saved as `# plot-go: next object behind-data` and
`# plot-go: next object on-top` immediately before the object block. Unmarked
objects keep Grace ordering. Grace ignores these comments and may drop them on
save; its renderer does not implement these extensions.


### Ordering drawing objects

`Project.MoveShapeAcrossLayers` moves a line, arrow, box or ellipse one place
forward or backward among shapes with the same attachment (page, or one graph's
data). At each layer edge it crosses between behind-data, Grace's normal layer,
and foreground. Shapes of different kinds can overlap in the chosen order.
Hidden shapes keep their place. Object indices and coordinates stay unchanged.
Until reordered, shapes retain Grace's box/ellipse/line order. Optional positive
ranks are saved as `# plot-go: next object order N` before each object block;
new or copied shapes enter after ranked shapes, in the default kind order.
Grace ignores these comments and may discard them on save. Ordering does not
cross graph/page or layer boundaries. Use `OnTop` to cover a legend, or
`BehindData` to put an annotation under curves.

## Overlays

The `figure` library overlays graphs: `Overlay(fig, base, over, "x")` puts
graph `over` on graph `base`, sharing the x axis (or "y"). The overlaid graph
takes the base's viewport; its frame and frame background are off, its copy
of the shared axis is off (the base draws it once) and its free axis moves to
the opposite side - right for a y axis, top for an x axis - with an axis line
styled like the base's frame. Further overlays on that side are moved out by
the axis offset, each beyond what the one before draws (numbers and label)
plus `AxisGap`, as drawn when `ArrangeOverlays` runs (again after edits
that change what an axis draws). `OverlayAll` overlays every graph on the
first.

- `SyncLinks` copies the shared axis - range, scale, inversion, ticks,
  numbers, formula, colour and label - from the graph edited to the others;
  each keeps where its copy is drawn and whether it is. The label is the
  group's; only the copy that draws it keeps the text. After a label edit,
  `ShareLabel` makes the edited copy's text the group's, even none, on any
  copy; after other edits `SyncLinks` keeps the label when the copy edited
  has none.
- `OverlayShift` moves an overlay off its base at right angles to the
  shared axis (down for x, left for y), so it draws its own linked copy.
  Only the outermost copy has the numbers and the label; the others draw
  their line and ticks.
- `AxisColor` colours an axis whole; on a shared axis, in every graph.
- When a graph is overlaid (`Overlay`, `OverlayAll`), its legend, if it
  covers another legend of its group, goes below the lowest of them, left
  edges aligned, `LegendGap` apart; legends already apart stay where they
  are. Only then: later edits never move a legend (`ArrangeOverlays`
  places axes only).
- `Candidates` lists graphs that look overlaid without a link - at the
  same place as an earlier graph, with its range and scale on an axis, as
  an overlay saved by Grace would be - for an editor to offer; `Link` adds
  the link and syncs the shared axis, leaving the drawing as it was. Tested
  on a hand-written Grace-format overlay and gracebat's save of it
  (`testdata/overlay`), not yet on one made with xmgrace's Overlay graphs.
- `Unlink` makes a graph its own again; `RemoveGraph` of a base makes its
  first overlay the base. `Layout` places each group as one panel, with room
  for the free axes on the right or on top (`Reach`).

The link is saved as `# plot-go: gN overlays gM shared x|y` before the
graph. Grace ignores it and drops it on save; the drawing does not depend on
it. A link to a missing graph, to the graph itself or to an overlay warns
and is dropped.

## Figures from numeric data files

The `figure` library imports numeric text tables or OneFit `# DATA` blocks.
Column indexes are positive and 1-based. Blank comma/semicolon cells keep
their positions; selecting a missing value skips that row with a warning.
`ReadData` represents these missing cells as NaN internally, and no NaN is
copied into plotted sets. Selecting other, present columns remains valid.
`UpdateFiles` caches parsed blocks per source for one refresh, so many sets
from one file do not cause repeated full-file parses.


### Raster exports

The command-line tool exports PNG and losslessly compressed TIFF (`.tiff` or
`.tif`) using the shared `raster` package. Resolution is explicit with `-dpi`
(default 300), with DPI metadata. Both are cropped to the drawing, as the PDF,
EPS and SVG are (`-fullpage`: the whole page, as gracebat's PNG); the crop box
is extended to whole pixels so pixel-snapped lines stay sharp. Grace-compatible
`-hdevice PNG`/`TIFF` calls use 72 DPI unless `DEVICE "name" DPI n` overrides it.
Poppler is preferred, with Ghostscript and ImageMagick fallbacks. Windows
Ghostscript console names and standard installation folders are supported.
No JPEG export is provided. DPI is limited to 1–2400 and a page to 100 million
pixels, to bound converter memory use; conversion respects cancellation and a
two-minute timeout.
