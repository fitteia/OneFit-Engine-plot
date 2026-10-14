# What plot-go draws

The current contract, kept up to date with the code. plot-go draws the part
of Grace's format below as gracebat does - measured from gracebat's output
(how, in [grace-behaviour.md](grace-behaviour.md)) and checked on every
corpus file within the tolerances stated there ("How closely plot-go
matches"). It is not a complete Grace: what is listed as not drawn is read,
warned about, and kept, but not drawn.

[roadmap.md](roadmap.md) is what is still to be drawn, in order.
[features.md](features.md) is the original inventory (2026-10-09) of what
OneFit's own 704 files use; it is history, not this list.

## Output

- PDF, EPS and SVG (`plot-go`, and gracebat's command line as OneFit's C
  core calls it).
- PNG (`-format png`, `-dpi`, 300 by default; gracebat's `-hdevice PNG`,
  at gracebat's 72 dpi unless `-pexec
  'DEVICE "PNG" DPI n'`): the whole page as gracebat's PNG is, made by an
  installed converter - pdftoppm, else Ghostscript, else ImageMagick
  ([grace-behaviour.md](grace-behaviour.md), "PNG").
- Fonts: the standard 14 PostScript fonts by Grace's font map, measured
  with Adobe's metrics; others warn and use Helvetica.

## Drawn

- Page: size, background color and fill.
- Graphs: any number, `type XY`, each with its world and viewport;
  hidden graphs skipped.
- Axes: normal, logarithmic and reciprocal scales, inverted axes,
  alternate axes (`altxaxis`, `altyaxis`), axis sides and offsets, an axis
  at zero (`type zero true`), axis bars, axis labels (placement, layout, side), major and minor ticks
  (size, color, width, style, direction, sides), special ticks (`tick
  spec`, types ticks and both), grid lines, tick labels in all of Grace's
  32 formats with precision, prepend/append, angle, skip, stagger, side
  and offset, tick label formulas (`ticklabel formula`, Grace's expression
  language) and start/stop.
- Frames: types 0 to 5, background color and pattern.
- Titles, subtitles, the timestamp, and strings in viewport or world
  coordinates, with justification, rotation, size, color, font and Grace's
  text escapes `\s \S \N \+ \- \x \f{}`, `\0` to `\9` and `\\`; others warn
  and are left out.
- Legends: position (viewport or world), box, fill, gaps, length,
  inverted order, every set's line and symbol.
- Sets of type `xy` and those with error bars (`xydx`, `xydy`, `xydxdx`,
  `xydydy`, `xydxdy`, `xydxdxdydy`): line types 0 to 5 (straight, stairs,
  segments), line styles 0 to 8, widths, colors, patterns; symbols 0 to
  11 (character symbols included) with size, outline and fill, symbol
  skip; error bars in x and y (place, size, widths and styles, riser clip
  with arrowheads); fills of types 1 and 2 with their rule, color and pattern;
  baselines (types 0 to 5) and droplines.
- Drawing objects, on the page or in a graph's world: lines with arrow
  heads (at either end or both; line, filled or opaque; length and
  layout), boxes and ellipses (outline style, width and color, fill color
  and pattern). Untouched objects are written back exactly as read. Edited
  and newly added objects are saved from their current settings; unknown
  directives in an edited object are retained. They stay where gracebat
  writes them (after the timestamp, before the strings).
- Grace's 32 fill patterns, on any fill and line.
- Values gracebat rejects (negative colors, styles, ...) are rejected the
  same way.

## Read and kept, but not drawn

A warning names each when it is on. `agr.Write` (the Figures editor's
save, and `-saveall`) writes them back as read, so a project opened and
saved loses none of them.

- Annotated values (`avalue`).
- Regions (`r0` ...).
- Stacked graphs, fixed points, graph types other than XY.
- Other set types (bar, xyhilo, xyr, ...; their data rows are read as far
  as they go); axis scales other than normal, logarithmic and reciprocal.
- Anything else plot-go does not know: an unknown directive warns and is
  kept like these.
