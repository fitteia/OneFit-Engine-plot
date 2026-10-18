# How gracebat draws OneFit's plots

Measured from gracebat 5.1.25's EPS output with `cmd/grace-probe` (run it on
a variant of a corpus file and read the primitives), and from the Grace
User's Guide where it says so. This is the renderer's specification; none
of it comes from Grace's source code.

Units: viewport units - the page's shorter side is 1, origin bottom left
(the guide; the EPS is scaled by 600 for a 773x600 page).

## Fixed graphs and xyr

- A Fixed graph keeps equal x and y data scales. The world window's center
  maps to the viewport's center, with the y-world scale used for both axes;
  the saved frame still spans the full viewport. Axis bars and the y-axis
  follow the centered data window.
- An `xyr` row is `(x, y, radius)`. The radius is measured in the shared
  world units and draws an unfilled circle using the set line's color,
  pattern, width and style. The ordinary symbol is drawn at the center using
  its symbol settings. `symbol skip` skips both together. A black-box EPS
  comparison against Grace covers unequal viewport dimensions, visible axes,
  radius variation, center symbols and their separate styles
  (`xyr_test.go`).

## Lines and symbols

- Line width: 0.0015 per unit of `linewidth` (2.0 -> 0.003), but never
  less than 0.0004: `linewidth` 0 to 0.3 all draw 0.0004 wide, and dashes
  scale with that drawn width (measured with `style-frame-negative`).
- Widths and dash lengths are written to the EPS with 4 decimals of the
  viewport (`[0.0263 0.0158]` for linestyle 3 at `linewidth` 3.5). The
  dashes are computed from the unrounded width, then rounded; the rounding
  adds up along a long dashed line, so plot-go draws these rounded values
  in every format (`style-frame-break-top`).
- Dash patterns, in multiples of the line width:

  | linestyle | pattern |
  |---|---|
  | 0 | no line |
  | 1 | solid |
  | 2 | 1 3 |
  | 3 | 5 3 |
  | 4 | 7 3 |
  | 5 | 1 3 5 3 |
  | 6 | 1 3 7 3 |
  | 7 | 1 3 5 3 1 3 |
  | 8 | 5 3 1 3 5 3 |

- Symbols, per unit of `symbol size`, around the point (measured with
  `style-symbols.agr`):

  | symbol | shape |
  |---|---|
  | 0 | none |
  | 1 | circle, radius 0.01 |
  | 2 | square, half side 0.0085 |
  | 3 | diamond, corners at +-0.01 on the axes |
  | 4-7 | triangle up, left, down, right: equilateral, side 0.02, centred on the point (tip 0.0115 from it, base 0.0058) |
  | 8 | plus, arms +-0.01 |
  | 9 | x, arms +-0.0071 (0.01/sqrt 2) |
  | 10 | star: the plus and the x |
  | 11 | the character `symbol char` in `symbol char font`, size 0.028 x `symbol size`, centred on the point by its box as labels are (below) |

  The fill comes first, in `symbol fill color` (fill pattern 0 is none; 1
  solid; 2 to 31 hatched, see Fill patterns), then
  the outline in `symbol color` with the symbol linewidth and linestyle
  (`symbol pattern 0`: no outline). A closed outline's path returns to its
  first point; each arm of the plus, x and star is a path of its own, and
  they are never filled. Each point's symbol is drawn whole before the
  next.
- Line types (`line type`, measured with `style-lines.agr`): 0 none; 1
  straight; 2 left stairs - from each point vertically to the next one's y,
  then across; 3 right stairs - across first, then vertically; 4 segments -
  points joined in twos (0-1, 2-3, ...; a last odd point is left out); 5
  three-segments - in threes (0-1-2, 3-4-5, ...; a last group of two is
  drawn, of one is not). Each segment of types 4 and 5 is a path of its
  own.
- Lines are clipped where they cross the viewport, 0.0001 outside it on
  every side whatever the world (measured with lines nearly along each
  edge, where a margin d moves the crossing by d / slope); fills are cut at
  the viewport itself. Each visible piece is a path of its own; a segment
  with both ends outside starts on the bottom edge, else the right, else
  the top, else the left (`style-set-fill-clip`), one with an end inside
  runs as the data do. Symbols and error bars of points outside the world
  window are not drawn.
- Error bars (`errorbar on`; measured with `style-errorbars`,
  `style-errorbars-log`, `style-errorbars-xy`, `style-errorbars-xy-log`,
  `style-errorbars-ylog-inverted`), drawn after the set's line and before
  its symbols, point by point:
  - the set type gives the errors: x, y, then `dx` (xydx), `dy` (xydy),
    `dx dy` (xydxdy) - each drawn on both sides of the point - or one per
    side: right then left (xydxdx), up then down (xydydy), right, left,
    up, down (xydxdxdydy). The x errors are drawn first, then the y ones;
    each side the right or upper one first.
  - one side: the riser from the point to the error's end, then the cap
    across it - an upward error's cap left to right, a downward one's
    right to left, a rightward one's top to bottom, a leftward one's
    bottom to top (always from the left of the way the error runs, on
    the page). The sign of an error does not matter; 0 draws nothing.
  - `place normal` draws only the right/upper side of an error given once
    for both, `opposite` only the left/lower one; for errors given per
    side, `normal` draws both as `both` does and `opposite` swaps them
    (the right error drawn left, ...).
  - cap half length 0.01 x `errorbar size` (size 0 still strokes a
    zero-length cap); caps use `errorbar linewidth`/`linestyle`, risers
    `riser linewidth`/`linestyle`; style 0 leaves that part out;
    `errorbar pattern 0` draws nothing at all.
  - an end out of the viewport has no cap: the riser runs to the frame;
    with `riser clip on` it runs `riser clip length` instead and ends in
    an arrowhead - from h to the right of the way it runs, 2h back from the
    tip, through the tip, to h to its left; h the cap half length, always
    solid, at the cap's width. An end a log axis cannot place (at or below
    0) is off the page's left or bottom, whichever way the axis runs: on an
    inverted log axis too the riser runs left or down.
  - every part is cut at the frame (an arrowhead past it is cut or gone).
- A set's type given after its data (a parameter file's `s0 type`, after
  the data on the command line) keeps the data's columns in order: those
  past the new type's are dropped, missing ones are 0. Plot-go deliberately
  preserves them instead; see Non-destructive display types below.
- A set the file says nothing more about (measured with `style-defaults`):
  straight line, no symbol, error bars on (place both, size 1, riser clip
  off with length 0.1), every color the set's number through the color map
  but white - set n is color n mod (colors-1) + 1, so 1 to 15 with Grace's
  16, then 1 again; widths and line styles (line, symbol outline, caps,
  risers) from `@default linewidth`/`linestyle`, symbol size from
  `@default symbol size`, the symbol character "A" in `@default font`
  (`@default color` is not used). Without `@default` lines: width 1, style
  1, size 1, font 0. Sets are drawn in number order, whatever order the
  file names them in.

## Symbol skip

- `symbol skip k` (measured with `style-symbol-skip`) draws the symbol of
  every (k+1)-th data row - rows 0, k+1, 2(k+1), ... - counting the rows
  out of view too, so a hidden first point does not shift the others. The
  error bars are left out with their symbols; the line still joins every
  point. A negative skip is rejected (see Values gracebat rejects).

## Set fill, baseline and droplines

Measured with `style-set-fill`, `style-set-baselines`,
`style-set-fill-clip` and `style-set-fill-log`.

- Order: the fill, the line, the droplines, the baseline, then error bars
  and symbols.
- `fill type 1` fills the polygon of the line's points (stair corners of
  line types 2 and 3 included; types 4 and 5 fill through every point);
  `fill type 2` that, then to (largest x, base) and (smallest x, base) -
  by x value, not the first and last points. Other types fill nothing,
  without a message. `fill rule 1` is even-odd (`eofill`), any other
  rule non-zero winding (saved as 0). `fill color`/`fill pattern` as for
  any fill (see Fill patterns); a new set has color 1, pattern 1.
- The base (`baseline type`): 0 y = 0, 1 the set's smallest y, 2 its
  largest, 3 the graph's world bottom, 4 its top, 5 the set's mean y; 6
  and up print "Wrong type of baseline" when drawn and use 0. On a log y
  axis a base at or below 0 is infinitely far below: fills and droplines
  run to the frame, the baseline is not drawn.
- `baseline on` draws a line at the base from the set's smallest x to its
  largest; `dropline on` a line from the base to each point, in data
  order; both in the set's line color, width, style and pattern, cut to
  the frame like lines.
- `line type 0` draws neither fill nor line but keeps droplines and the
  baseline; `line linestyle 0` or `line pattern 0` hides the line, the
  droplines and the baseline but keeps the fill.
- Negative types, rules, colors and patterns are rejected (see Values
  gracebat rejects).

## Inverted axes

- `xaxes invert on` / `yaxes invert on` (measured with `style-invert`):
  the axis runs from its viewport's far side - data, error bars, ticks,
  tick labels, strings and legends in world coordinates, all mirrored; the
  ticks keep their order (drawn from the world's low end), and so do grid
  lines (drawn from the other axis's low end, now on its far side).

## Grid lines

- `tick major grid on` / `tick minor grid on` (measured with `style-grids`
  and `style-grids-noticks`): a line across the viewport at each major /
  minor tick inside the window, in that tick mark's color, linewidth and
  linestyle; drawn first in the graph, before the sets: x minor, x major,
  y minor, y major. An axis that is off has none; ticks off do not matter.

## Frame

- `frame type` (measured with `style-frame-*`), each one path starting
  after the gap, so its ends have the line's butt caps:

  | type | sides | path |
  |---|---|---|
  | 0 | all four | closed box from the lower left, up first |
  | 1 | left and bottom | upper left, lower left, lower right |
  | 2 | no top | upper left, down, across, up |
  | 3 | no bottom | lower left, up, across, down |
  | 4 | no left | lower left, across, up, back |
  | 5 | no right | lower right, across, up, back |

  Any other type draws no frame, silently. `frame pattern 0` or
  `linestyle 0` draws none either.
- `frame background pattern` not 0 fills the viewport with `frame
  background color`, for every frame type (6 included), first of
  everything in the graph. Pattern 0 fills nothing whatever the color.

## Fill patterns

Measured with `style-patterns-*`. Grace's patterns 0 to 31 are 16 by 16
bitmaps; gracebat's EPS defines them all (`/Pattern0` ... `/Pattern31`)
as uncolored tiling patterns painted with
`16 16 true [-1 0 0 -1 16 16] pat_bits imagemask`, scaled by
`[0.0017 0 0 0.0017 0 0]`:

- a bit is 0.0017 of the viewport (one point on a 600-point page), so a
  tile is 0.0272; tiles start at the page's origin, wherever the area is;
- both axes are turned: row r, bit b (0 the high bit of the row's first
  byte) is the cell (15-b, 15-r) of the tile;
- set bits are painted in the color, the others left as they are.

A fill (frame background, legend box, symbol) with pattern 2 to 31 is
first filled with the page's background color (`@background color`,
whether or not `page background fill` is on), then painted with the
pattern. Pattern 0 draws nothing, 1 is solid, 8 is all zeros (only the
background color shows). A pattern above 31 is "invalid pattern number"
and drawn solid, without the background fill.

Lines take a pattern too, with no background under them: `line pattern`
(the set's line and its legend sample), `errorbar pattern` (risers, caps
and arrowheads), `symbol pattern` (outlines), `frame pattern` and `legend
box pattern`. Pattern 0 draws no line.

plot-go writes the same definitions in its EPS, the same bitmap as an
uncolored tiling pattern in its PDFs, and one SVG `<pattern>` per pattern
and color. Ghostscript moves pattern tiles by up to a device pixel when it
renders them (gracebat's EPS and plot-go's PDF alike), so at 144 dpi a
correct pattern and one shifted by a cell look alike: the tests check
patterns cell by cell at 10 pixels a cell instead
(`TestPatternCellsMatchGracebat`, `TestSVGPatterns`).

## Values gracebat rejects

- A negative color, font, pattern, line style, type, symbol, character,
  count, precision, skip, stagger or justification: gracebat prints
  "Negative value supplied for non-negative" and the field keeps the value
  it had before that line (a default, if nothing set it yet). plot-go does
  the same, with a warning.
- A negative `linewidth` is "Negative linewidth" and becomes 0 (drawn
  0.0004 wide, as above).

## Ticks

- Length 0.02 per unit of tick size (major 1.5 -> 0.03, minor 1.0 -> 0.02),
  pointing in, out, or both; `place both` draws them on both sides.
- Major ticks: multiples of `tick major` inside the world window, ends
  included (a window starting at 1e-05 has no tick at 0).
- Linear minor ticks: `minor ticks` m evenly between majors, step
  major/(m+1).
- Log minor ticks: at 2x, 3x, ... (m+1)x each decade's start, whatever m
  is (m=9 puts the last on the next decade).
- On a log axis `tick major` is the factor between majors. One from 0 to 1
  - a linear spacing such as 0.2 left on an axis made logarithmic - makes
  gracebat say "Invalid major tick spacing, autoticking" and tick every
  n decades: the decades the window spans over `tick default` - 1, rounded
  up, at least 1, majors at the powers of 10^n (0.1 to 10: every decade;
  1e-5 to 1e5: 1e-4, 1e-2, 1, 1e2, 1e4; 1e-5 to 1e6: every third; 1e-30
  to 1e30: every twelfth), minors from `minor ticks` as for that factor.
  The file keeps its value. A negative one draws no ticks.
- Minor ticks come only from intervals (decades) that reach into the
  window, not from one merely touching its edge: a log window starting at
  1e4 gets no 10x minor from 1e3..1e4, one starting at 2e4 gets 1e4's
  minors from 2e4 on.
- Too many ticks: more than about 255 majors, or more than about 255 minors
  across the major intervals the window touches, and gracebat prints "Too
  many ticks ( > MAX_TICKS ), autoticking": the major step becomes the
  smallest of 1, 2, 5 x 10^n giving at most `tick default`+1 intervals
  (default 6 -> 7), and at most one minor tick per interval.

## Special ticks

Measured with `style-ticks-spec-ticks` and `style-ticks-spec-both`.

- `tick spec type ticks` or `both` replaces the spaced ticks with the
  file's own: `tick major i, v` and `tick minor i, v` put tick i at v,
  `ticklabel i, "text"` gives it a label. Only entries 0 to `tick spec`
  N-1 are drawn.
- Drawn in the file's order, not sorted: all the minor ticks, then all the
  majors; grid lines likewise. A tick outside the world window is left out;
  one exactly at its edge is drawn.
- `ticks`: the majors are labelled with their values, as usual. `both`:
  with their own text, `ticklabel prepend` and `append` around it; a major
  without one gets prepend and append alone; minor ticks' labels are never
  drawn. `ticklabel skip` counts the majors as drawn.
- Entries are numbered 0 to 255: a larger number is "Number of ticks
  exceeds maximum" and gracebat then crashes (SIGSYS) without a page;
  plot-go ignores that entry, with a warning. A `tick spec` count above 256
  works as 256.

## Axis sides, offsets, labels

Measured with `style-axes-sides`, `-perp`, `-angle`, `-noticks` and
`-offsets`.

- Each axis has a normal side (bottom/left) and an opposite one
  (top/right); `tick place`, `ticklabel place` and `label place` each say
  which: normal, opposite or both - the opposite side mirrors the normal.
  `offset a, b` moves the axis out from the frame, a on the normal side and
  b on the opposite: its ticks and labels move with it, the frame does not.
- Tick labels: 0.01 beyond the ticks reaching out on their side (or, with
  `ticklabel offset spec`, moved along the axis by its first value and out
  by its second); centred on their tick by their box - at an `angle`, the
  box of their turned glyphs; every (skip+1)th tick only; with `stagger n`
  the label of tick k goes in row k%(n+1), moved out by row x (0.02 x char
  size + 0.01) - a fixed height, whatever the text.
- The axis label goes 0.01 beyond everything drawn on its side: the tick
  labels' ink, or else the ticks' box, their line's half width included (so
  0.0115 from a frame whose ticks point in). `layout para` lays it along the
  axis (the y label turned 90 degrees), `perp` across it (the x label
  turned 90 degrees, its end towards the axis); centred on the axis by its
  ink. `label place spec` with `label place a, b` instead puts its ink
  centre at a along the axis from its middle and b out from the axis.

## Axis at zero, axis bars

Measured with `style-axes-zero`, `-zero-edge`, `-zero-outside`,
`-zero-log`, `style-axes-bar` and `-bar-zero`.

- `type zero true` draws the axis where the other axis is 0 instead of at
  the frame: its ticks and tick labels there, `offset` moving each side
  away from that line. When 0 is outside the other axis's window - or that
  axis is logarithmic - nothing of the axis is drawn, its title and bar
  included; 0 exactly at the window's edge is drawn there.
- At zero, `tick in` and `out` swap: "in" points away from the graph's
  middle (down from an x axis), "out" towards it; `both` is drawn as at the
  frame. Tick labels go 0.01 beyond the ticks reaching out on their side,
  as at the frame.
- The axis title stays by the frame: 0.01 beyond a line 0.0015 out from the
  frame - whatever the tick and frame line widths - or beyond the tick
  labels when they reach further out (with 0 at the window's edge).
- `bar on` draws a line along the whole axis (`bar color`, `linestyle`,
  `linewidth`) on each side the ticks are placed on (`tick place`), the
  ticks themselves on or off; at the axis, so `offset` and zero move it
  (at zero both sides draw it on the same line). An axis that is off has
  no bar. Each axis draws its bar first, then its ticks.

## Alternate axes

Measured with `style-axes-alt` and black-box runs of gracebat.

- `altxaxis` and `altyaxis` are axes of their own in the graph's world and
  scale (`xaxes`/`yaxes`), set with the same directives as `xaxis`/`yaxis`
  and drawn by the same rules - an alternate axis with the x axis's
  settings draws exactly what the x axis does. They are drawn after the y
  axis (x, then y), before the frame.
- One switched on with nothing else is Grace's default axis - the axis
  gracebat saves for a graph it makes itself: bar on, ticks every 0.2
  (size 1, minor 0.5, in, both sides), tick labels and title in font 0 at
  size 1 on the normal side, general format, precision 5.
- Grace does not save them: `-saveall` writes `altxaxis off` even for one
  that is on and drawn, and none of its settings. A file with one comes
  from plot-go's writer (the Figures editor) or is written by hand; plot-go
  writes them whole.

## Tick label formulas

Measured with `style-axes-formula` and grace-probe runs over some 80
formulas.

- `ticklabel formula` makes each tick's label the formula of `$t`, the
  tick's value, in the axis's format: `1000/$t` labels the ticks at 1 to 5
  1000, 500, 333.33, ... Ticks stay where they are.
- The language: numbers (`1`, `2.5`, `.5`, `2.`, `1e-3`), `pi`, `$t` (any
  case); `||`/`or`, `&&`/`and`, `== != < > <= >=` (giving 0 or 1), `+ -`,
  `* / %`, unary `-`, `!`, `not`, and `^` - tighter than unary minus
  (`-$t^2` is -(t^2)) and grouping from the right; the functions `abs sqr
  sqrt exp ln log10 log2 floor ceil rint` (to even), `sin cos tan asin
  acos atan sinh cosh tanh asinh acosh atanh`, `mod(a, b)`. Not: `log`,
  `int`, `min`, `max`, `pow`, `e`, `**`, unary `+`.
- A formula gracebat cannot read, or one dividing by zero (`/` or `%` by
  0 at any tick), is "Error in tick transformation formula": that axis has
  no tick labels. `mod(a, 0)`, `sqrt` of a negative, and the like give NaN.
- Not a number prints `nan`, infinity `inf` and `-inf` - in power format
  10 to the `nan`/`inf` (`-10` for -inf), in scientific `nan` times 10 to
  0 (NaN) or 2147483647 (either infinity).

## Tick label start and stop

- `ticklabel start type spec` with `ticklabel start a`, and `stop type
  spec` with `stop b`: only the ticks whose own values are from a to b
  (both included, before any formula) are labelled; every tick is still
  drawn.

## Reciprocal scale

Measured with `style-axes-reciprocal` and `-reciprocal-zero`.

- `xaxes scale Reciprocal` places values linearly in -1/v (v still grows to
  the right, or up): ticks at multiples of `tick major`, minor ticks evenly
  between them in v, as on a normal axis. Inverted, it is mirrored.
- A window holding 0 is "X-axis contains 0" (or Y), and nothing of the
  graph is drawn.

## Text

- Font size: 0.028 per unit of char size (1.5 -> 0.042).
- Fonts by number from the file's font map; `\x` is the Symbol font.
- Escapes (the guide gives the factors rounded; measured exactly):
  `\-` scales by 2^(-1/4); `\s` shifts the baseline by -0.4 and `\S` by
  +0.6 of the current size, then scale by 2^(-1/2); `\N` restores size and
  baseline; `\0`..`\9` switch to that font number.
- Common text controls (measured with `style-text-controls`,
  `style-text-decorations`, `style-text-decoration-rotation` and
  `style-text-context`):
  - `\z{x}` multiplies the current size; `\Z{x}` sets a size relative to
    the original; empty braces restore the original size. `\N` also
    restores the original size and the current baseline.
  - `\v{x}` shifts by x times the current size; empty braces return to
    the current baseline. `\V{x}` moves that baseline and the pen;
    empty braces undo the baseline change. `\h{x}` shifts horizontally
    by x times the current size. `\n` returns to the original horizontal
    position and moves the baseline down one current font size.
  - `\#{...}` inserts hexadecimal bytes; `\c` and `\C` enable and
    disable adding 128 to literal character codes below 128.
  - `\u`/`\U` and `\o`/`\O` start/stop underline and overline. Underlines
    keep Grace's measured, font-specific offset and thickness. Grace's EPS
    puts its `\o` stroke just above the baseline, which crosses many glyphs;
    plot-go deliberately places the overline above the run's tallest glyph,
    with a gap of 0.04 em. This is an intentional visual correction to
    Grace, not exact parity for overlines. Both decorations scale and rotate
    with the run, and their stroked boxes participate in justification.
  - Grace rounds run advances to its 300 dpi text grid. For a 600 pt
    viewport this is 0.0004 per run in each page direction, including
    rotated strings. Decoration paths expose this rounding; their run
    origins and decorated bounds therefore follow it. Paths decorating
    axis labels use the existing text-position tolerance, since their
    placement also includes raster-measured tick boxes; stroke widths and
    styles retain the path tolerances. Existing undecorated strings retain
    their continuous layout within the existing text tolerance.
- Additional text controls measured from independent black-box EPS probes:
  - Matrix arguments are written `xx xy yx yy`; new transforms multiply on
    the left. `\t{}` resets the whole font matrix, including zoom; `\T{...}`
    resets before applying its argument. `\r{}` applies a rotation in degrees,
    `\l{}` applies a horizontal shear; `\q`/`\Q` add/subtract a shear of 0.25.
    Pen advances, shifts, glyph bounds and decorations follow the matrix.
  - Absolute zoom and `\N` normalize the height perpendicular to the
    transformed baseline. In measured Grace output `\N` retains rotation,
    shear and aspect ratio; it restores normal height and the current baseline.
    The guide's abbreviated equivalence to a matrix reset is not the measured
    behavior. Baseline justification uses both components of the final advance.
  - `\R{number}` and `\R{name}` change text/decorations colour; `\R{}` restores
    the label's original colour. This state resets for each label.
  - `\Fk` enables pair kerning; `\FK` disables it (the default). Grace uses
    the URW AFM pairs, which can differ from OpenType GPOS pairs. plot-go reads
    installed URW AFMs beside the font or in its Type-1 sibling directory,
    falling back to the embedded Adobe AFM pairs. Explicit glyph positions are
    exported consistently in EPS, PDF and SVG.
  - `\Fl`/`\FL` delimit ligature-enabled/disabled runs. The supported
    Core-14/URW Latin-1 fonts provide no encodable ligature substitutions;
    measured Grace leaves `fi`, `fl`, `ffi` and `ffl` unchanged in these fonts.
  - `\m{n}` records the current advance position (n=0..31); `\M{n}` returns
    there and restores the current baseline, without restoring font/colour/
    transform or replacing the baseline. An unset mark returns to the origin.
  - `\dr` reverses bytes in subsequent substrings, `\dl` restores their
    original order. `\dR` advances to the left before drawing each run;
    `\dL` restores normal advancing. Ordering and advancing are independent.
- Strings use the fonts' advance widths; kerning is disabled unless requested.
- Text encoding (gracebat's EPS prolog defines it): Latin-1, except that 39
  is quoteright, 45 the hyphen (not minus) and 96 the grave accent, as in
  Adobe Standard. plot-go's EPS, PDF and SVG use the same.
- Every label is aligned by the box of its whole typeset string: ink,
  except that the box starts at the pen origin rather than the first ink
  (a leading "1" has a wide left margin, and gracebat ignores it):
  - x tick labels: centred on the tick; ink top 0.01 below the axis (the
    file's `ticklabel offset 0, 0.01`), plus the tick length when ticks
    point out.
  - y tick labels: ink right edge 0.01 left of the axis; centred
    vertically on the tick.
  - axis labels: centred on the viewport side; 0.01 beyond the outer ink
    edge of all that axis's tick labels; the y label rotated 90 degrees.
- `power` tick labels are "10" with the exponent as a superscript.
  gracebat rejects an unknown tick label format name as a syntax error and
  keeps its default, `general` (%g-like: "100", not "100.00000").
- Tick label formats (measured with `style-formats-numbers`, `-dates`,
  `-degrees`; p is the precision):
  - decimal: p decimals, a value rounding to zero without its sign;
    exponential: C's %.pe; general: C's %.pg (p at least 1);
  - power: 10 with the decimal logarithm, at p decimals, as superscript
    ("10^2.00" at p 2; negative values "-10^..."; 0 as decimal);
  - scientific: the mantissa at p decimals - as rounded, not renormalised
    (999 at 1 is "10.0") - then the Symbol font's multiply sign and 10 with
    the exponent as superscript; 0 as decimal;
  - engineering: the mantissa at p decimals, a space and an SI prefix
    every 3 decades, y to Y (micro the Symbol font's m), beyond them the
    mantissa grows; none for 1 to 999 (the space stays);
  - computing: C's %.pg, from 1024 on divided by 1024 per prefix K, M, G,
    T, P, E, Z, Y, no space;
  - dates (days from `@reference date`, Julian day 0 by default: noon, 1
    January 4713 BC): ddmmyy, mmddyy, yymmdd, mmdd, monthday, daymonth and
    dayofyear round to the nearest day (noon rounds up); mmyy, months,
    monthsy, monthl to the nearest month's start; dayofweeks, dayofweekl,
    and the times (hms, mmddhms, mmddyyhms, yymmddhms, seconds rounded) use
    the exact moment. The Julian calendar before 15 October 1582, the
    Gregorian from then; years before 1 AD as negative BC years (Julian day
    0 is in -4713). "dayofweek" is a syntax error: the name is dayofweeks;
  - degrees (lon: E/W, lat: N/S): degreeslon "12.50E", degreesmmlon
    "12 30.00' E", degreesmmsslon "12 30' 0.00\" E", mmsslon without the
    degrees; the last part at p decimals; 0 alone ("0", "0 0'", "0 0' 0\"").
- A format may also be given by number, in that order: 0 decimal, 1
  exponential, 2 general, 3 power, 4 scientific, 5 engineering, 6
  computing, 7 ddmmyy ... 23 yymmddhms, 24 degreeslon ... 31 mmsslat. A
  number Grace does not know (OneFit wrote 60) draws as decimal - and is saved as "unknown", which gracebat then rejects when it
  reads its own project back, so that project no longer draws what it
  printed. plot-go saves the format by name.
- Defaults for what a file leaves out: format general, precision 5 (a
  "ticklabel prec" line without a value is ignored, leaving 5).
- Strings (`with string`), in view or world coordinates, are placed by
  `string just` (measured with `style-justify`, every value at 0, 30, 90
  and 315 degrees): the low two bits horizontal - 0 left, 1 right, 2
  centre; 3 draws nothing - the next two vertical - 0 the baseline, 1
  bottom, 2 top, 3 middle. On the baseline the string moves along its own
  direction by its advance width; aligned vertically, it moves by the box,
  on the page, of its turned glyph outlines and its pen origin (left puts
  that box's left edge on the point, and so on).
- A new string starts as a copy of the previous one's settings - place,
  color, rotation, font, justification, char size, coordinates - but off
  and without text; the first at 0, 0 in view coordinates, in the
  `@default` color, font and char size (`style-strings-inherit`,
  `style-strings-defaults`). A negative `string rot` is rejected
  ("Negative value supplied for non-negative") and the rotation kept.

Text is measured with the URW base-35 fonts' glyphs, as gracebat does (its
fonts are those files): plot-go reads each character's ink box from the
installed URW OpenType font (Debian's fonts-urw-base35, macOS's
~/Library/Fonts from the font-urw-base35 cask, or $OFE_URW_FONTS) and
falls back to Adobe's metrics, whose ink
differs by up to 0.02 em, where the URW fonts are not installed; advance
widths are Adobe's either way (the URW fonts are metric compatible).
gracebat takes its boxes from its 300 dpi glyph rasters, rounded and hinted
(an edge may be a pixel, 0.0004, off the outline: Helvetica's "f" at char
size 1.5 is 0.01 em taller), so positions differ from the exact outlines by
up to about 0.0006.

## Timestamp

- `timestamp on` (measured with `style-timestamp`): gracebat prints the
  time it draws at, not the file's `timestamp def` - C's date format,
  "Sun Oct 11 23:25:03 2026" (local time) - from the timestamp's place
  (view coordinates; the pen origin, as strings), rotated by `rot`, in its
  font, char size (0.028 per unit) and color, drawn last of all. plot-go
  does the same; $SOURCE_DATE_EPOCH (seconds, shown in UTC) fixes the time
  for reproducible output (the pixel tests set it to the reference's).

## Titles

- A graph's `title` and `subtitle` (measured with `style-titles` and
  `style-titles-big`), each in its own font, size (0.028 per unit) and
  color: centred on the viewport by its box, as labels are; the title's
  ink 0.06 above the viewport's top, the subtitle's 0.02 above it - fixed,
  whatever their sizes, so a large subtitle may overlap the title. Drawn
  after the legend, title first.

## Legend

Measured with `style-legend`, `style-legend-symbols`, `style-legend-gaps`
and `style-legend-world`.

- An entry per visible set with a legend text, in set order (`legend
  invert`: reversed); hidden sets and empty texts have none, and a legend
  without entries draws nothing. Error bars are not part of an entry.
- Each entry: a sample - the set's line (if it has a line type) across
  `legend length` x 0.01, with its symbol at both ends; or, without a line,
  the symbol at the middle of that length - and the text, in `legend font`,
  `char size`, `color`, starting at length + hgap + 0.01 x the largest
  `symbol size` of the graph's visible sets (whether they draw symbols or
  not). The sample's line and symbols sit at the middle of the text's ink
  box. Entries are stacked by their texts' ink boxes, `vgap` x 0.01 apart.
- The entries are then moved so that the box of everything they draw (the
  line's half width and the symbols' outlines included) has its top left
  corner at the legend's place (`legend x, y`, in view or world
  coordinates) less hgap x 0.01 and vgap x 0.01; the legend box is that box
  widened by those gaps: filled first (`box fill pattern` not 0, `box fill
  color`), then outlined (`box pattern` not 0, `box color`, `linewidth`,
  `linestyle`). Drawing order: the box, then per entry its text, line,
  symbols. The legend comes after the graph's strings in world coordinates.
- A large symbol in a middle entry does not move its neighbours (only the
  box of everything drawn counts), so it may overlap them, as in Grace.


## Drawing objects

Lines, boxes and ellipses (`@with line` ... `@line def`), measured from
gracebat's EPS (corpus style-objects, style-objects-order,
style-objects-carry):

- Order: in each place (a graph's world, after its frame; the page,
  after every graph) the boxes, then the ellipses, then the lines - each
  kind in file order, whatever order the kinds come in. Not clipped to the
  graph.
- A box is the path corner 1, (x1, y2), corner 2, (x2, y1), as given
  (reversed corners stay reversed); an ellipse is the full ellipse in the
  box of its corners. Each is filled (fill color and pattern; pattern 0
  none) then outlined (color, width, style; style 0 none).
- A line is drawn, then its arrow heads, the start's first (arrow 1 the
  start, 2 the end, 3 both). With L 0.01 times "arrow length" (viewport
  units, whatever the line's width) and "arrow layout" a, b: the head's
  back corners are L back from the tip and L*a/2 either side of the line,
  its notch L*(1-b) back from the tip (b 1: no notch). Type 0 is the two
  sides as one stroke; 1 is filled with the line's color, 2 with the
  page's background color, both then outlined. Heads are solid, in the
  line's width and color, also when the line's style is 0. A line of no
  length gets no head; an arrow type other than 0 to 2 none either
  (gracebat: "Internal error in draw_arrowhead()").
- What a block leaves out is as the previous object of its kind had it,
  "on" included; corners are 0, 0, 0, 0 and the graph -1 (drawn in graph
  0's world). The first of a kind starts off, with the "@default" line
  settings, fill color 1, pattern 0, no arrow. An object that is off is
  not saved by gracebat.
- In a graph's world, a value its axis cannot place (log at or below 0,
  reciprocal at 0) is at the page's left or bottom edge.

## Drawing order

Page background; then per graph: each set (fill, line, droplines,
baseline, error bars, symbols), the x axis
(bar, minor ticks, major ticks, tick labels, label), the y axis likewise,
the alternate x and y axes likewise, the
frame, the drawing objects in that graph's world, the strings placed in
it, the legend and the titles; then the drawing objects and the strings
placed on the page (view coordinates), and the timestamp. Each tick is drawn on the
normal side, then the opposite one.

## How closely plot-go matches

Close, within stated tolerances - not identical. Two kinds of test, on
every corpus file (`testdata/corpus`, gracebat's output in `testdata/ref`):

- Primitives (`render`'s TestMatchesGracebat, `internal/drawcmp`): the
  same paths, arcs, texts, fonts and colors, in the same order. Points
  within 0.0002 viewport units (gracebat's EPS has 4 decimals; legends
  0.001, placed by text boxes gracebat measures on 300 dpi rasters),
  text positions within 0.002 (the largest seen is 0.0006), font matrices
  and widths within 0.0002, colors within 0.002, arc angles within 0.001
  degree. A clipped fill's outline may start at another vertex or repeat
  one (same area). Each compared property has a test that changes it alone
  and must be caught.
- Pixels (`backend`): plot-go's EPS, PDF and SVG, rendered at 144 dpi,
  against gracebat's EPS rendered the same way. A pixel differs when a
  channel is more than 96 of 255 off and no pixel within 2 (text may sit a
  fraction of a point apart) matches; at most 0.2% of the inked pixels may
  differ (0.4% and 0.5% for two legend files, see pixelLimits). Fill
  patterns are checked cell by cell at 10 pixels a cell instead, since
  Ghostscript moves pattern tiles by up to a pixel.

What this covers is the corpus: the features listed in this file, as
OneFit and the Figures editor use them - not all of Grace (see
docs/features.md for what plot-go does not draw).

## PNG

Measured with `gracebat -hdevice PNG` (and `-pexec 'DEVICE "PNG" DPI n'`):

- The image is the whole page, not cropped: the page's size in points
  times dpi/72 pixels (773x600 points: 773x600 pixels at the default 72
  dpi, 1546x1200 at 144), with the resolution recorded in the file.
- Lines are not antialiased, and each is a whole number of pixels thick:
  the width in pixels rounded, one at least (at 300 dpi Grace widths 0.5,
  1, 2, 3, 6 are 2, 4, 8, 11, 23 pixels).
- Every area is filled even-odd, also those the vector devices fill
  non-zero (a star drawn in one stroke keeps its middle empty) - the
  User's Guide says the raster devices have only that rule.

plot-go's own `-format png` is at 300 dpi unless `-dpi` says otherwise
(72 is too coarse for a publication); gracebat's command line
(`-hdevice PNG`) gets gracebat's 72, or the `-pexec` DPI. It makes the
PNG from its own full-page PDF with an installed converter: pdftoppm (given the whole-pixel widths, which it then draws as
gracebat does), else Ghostscript or ImageMagick (the widths as they are,
since Ghostscript inks every pixel a line touches). Every fill is even-odd.
Unlike gracebat's, the image is antialiased, and a converter places each
line to the pixel its own way, so a line may sit a pixel from gracebat's.
Over the corpus at 72 dpi every image has gracebat's size, and at most
3.6% of the pixels differ by more than 30% (edges and pattern tile
phases); at 144 dpi pdftoppm's image matches the gracebat references
(testdata/ref) to 0.5% of its ink, Ghostscript's to 5%.

## The command line OneFit uses

Recorded from a real fit (OneFit-Engine test 3 on a OneFit server, with a
logging stand-in for `grace` on the PATH); it is testdata/gracebat-call:

    grace -version
    grace -settype xydy gnu0.da_ -nxy fit-curves-1 -param fit1.agr-par \
      -hdevice EPS -hardcopy -printfile fit-curves-1.eps -saveall fit-curves-1.agr

then `epstopdf fit-curves-1.eps`. Arguments are taken in order: the data
file becomes set 0 (xydy, from -settype), each y column of the curves file
another xy set, and the parameter file's settings then apply to them (in
an order with the curves first, the parameter file's "s0 type xydy" would
make the first curve the data set). A saved project (-saveall) is Grace's
whole state: settings, then each set's data as "@target G0.Sn", "@type T",
rows in "%16.8g", "&". World and view are saved as one line each ("world
xmin, ymin, xmax, ymax"), whatever form the parameter file used.

## End to end

With plot-go standing in for `grace` on the PATH (for one command only),
all nine OneFit-Engine test suites were fitted on a OneFit server twice -
with Grace and with plot-go: the same 110 plots, PDFs and zip archives
come out, and every plot matches gracebat's pixel for pixel (worst 0.014%
of inked pixels, allowing for text up to 0.7 pt apart). Two cases found
this way are now tests: a window edge just past a tick (no tick at 0 for
a window starting at 1e-06), and the numeric tick label format above
(testdata/gracebat-call-format60).

## Point annotations

Measured on 2026-10-17 using gracebat 5.1.25 as a black box. The
`style-avalue-*.agr` corpus fixtures and their EPS/PNG references were
created for these measurements; existing reference files were unchanged.
The fixtures were saved by Grace to spell out its defaults. Inactive
axis placement fields are explicit so the parser/writer round-trip is
stable even though Grace omits these fields when an axis is off.

- `avalue on` labels the set after its symbols and error bars. Hidden
  sets have no labels. The default type is 2 (Y), format general,
  precision 3, size 1, default font/color, rotation 0 and offset 0, 0.
- Types: 0 has no value; 1 is X; 2 is Y; 3 is X followed by comma-space
  and Y; 4 is the optional quoted string after a data row's numbers;
  5 uses the third numeric column, including an error-bar column. When
  the requested value is absent, only prepend/append remain. Type 0
  also draws prepend/append if nonempty. Types beyond 5 draw nothing
  (Grace reports an invalid annotation type); plot-go warns too.
- Prepend and append surround the whole label, including an X/Y pair.
  Formatting uses the same formats as tick labels; Grace text escapes
  are processed after formatting. A quoted label is not another numeric
  column and remains attached to its row when a set is imported or refreshed.
- Placement is horizontally centred and bottom aligned by the page's
  bounding box of the rotated text, including the pen origin, like string
  justification 6. With the point at 0.32, 0.38, Times-Roman size 1 and
  text `[3]`, Grace places the pen at 0.3050, 0.3844; at 90 degrees it
  places it at 0.3272, 0.3800. The existing text tolerance applies.
- Offsets add viewport X/Y distances after justification. They neither
  scale with the graph's world nor rotate with the text. Offset 0.1, -0.2
  moves that pen to 0.4050, 0.1844.
- Symbol skip applies even with no symbol. Rows outside the world still
  count for skip. Visible boundary points keep their labels, which may
  extend outside the frame; outside points have no label. Logarithmic
  and inverted axes move the anchor; they do not change the printed values.
- Symbol size does not move the annotation. Character size, font, color
  and arbitrary rotation are independent of the symbols.

The automated suite compares every new fixture's drawing against its
Grace EPS, checks that Grace draws plot-go's saved copy the same way,
and checks model/save/reopen and imported-label independence. Fixtures
cover types 0 through 5, missing third columns, superscripts, color/font/
size/exponential precision, 37/90-degree rotation, offsets, skip, symbols,
world edges and inverted logarithmic axes. This does not establish
compliance for the other set or graph types listed as unsupported.

## Per-point symbol sizes (`xysize`)

Black-box EPS comparisons show that an `xysize` row's third value replaces
`symbol size` for that point. It is not multiplied by the set size. Its
magnitude is used, so negative values draw the same size as positive values;
a zero value has zero geometric extent (an outline can still leave a dot).
The usual symbol shape, fill, outline, skip and world-window inclusion apply,
including character symbols. The legend sample still uses the set's ordinary
`symbol size`. Circles, filled squares, character symbols, zero/negative sizes,
row skipping and legend placement are covered by the renderer comparisons.

## XY points with Z annotations (`xyz`)

The Grace User's Guide defines `xyz` as the same XY plot with a third numeric
value available for point annotation. A black-box EPS comparison confirms the
third column is shown when annotated values are enabled with type 5 (the third
numeric column), and does not change the point geometry. plot-go uses the same
line, symbol, clipping, skip and annotation behavior as `xy`.

## Hi/Lo/Open/Close candles (`xyhilo`)

Grace reads five values per row: x, high, low, open and close. Black-box
EPS comparisons show a vertical high-to-low stroke, an open tick from the
data x position toward the left, and a close tick toward the right. The
symbol size sets the half-length of both ticks in viewport units; symbol
color, line width, line style and pattern style all three strokes, even when
the ordinary symbol and set line are off. Symbol skip counts source rows,
including rows outside the viewport. Candles are clipped to the graph view.

## Per-point fill colors (`xycolor`)

An `xycolor` row's third value is rounded to the nearest integer color-map
index for that point's symbol fill. The symbol outline keeps the set's regular
symbol color. The selected fill pattern still determines whether and how the
interior is painted; a pattern of zero leaves the symbol unfilled. Black-box
EPS comparisons cover fractional indices, two distinct fill colors and the
ordinary outline style.

## Vector maps (`xyvmap`)

Each row gives an origin in data coordinates `(x, y)` and a vector offset in
normalized graph-view coordinates `(dx, dy)`. The segment runs from the
transformed origin to that point plus the offsets, clipped to the graph frame.
Vector strokes remain visible when the ordinary set line and error bars are
off. They use the error-bar color, pattern and riser width/style; symbol skip
selects source rows. Fully visible vectors have open arrowheads sized by the
error-bar size; Grace omits the head when clipping the vector at the frame.
`gracebat` comparisons cover short and clipped vectors, skipping, and the
riser stroke settings.

## Box plots (`xyboxplot`)

The Grace User's Guide gives six columns: x, median, upper and lower box
limits, then upper and lower whisker endpoints. A `gracebat` EPS comparison
confirms the box uses the symbol fill color/pattern; its outline and median
use symbol color, pattern, width and style. Symbol size is the half-width of
the box and whisker caps, and symbol skip selects rows. Whisker stems use the
error-bar riser color, pattern, width and style; caps use the error-bar line
width and style. Turning error bars off hides both whiskers and caps while
leaving the box and median. The set is supported only in XY graphs.

### Non-destructive display types (intentional difference)
Plot-go retains all numeric columns when a set type changes, including columns
unused by the selected display type. Unlike Grace's resizing, it never drops
trailing values or fills missing columns. Rows with fewer than x/y are rejected;
rows missing columns required by their display type remain stored but are not
drawn. Numeric data is written with round-trip float64 precision. Opening such
a project in Grace may resize its data; this preservation applies to plot-go.
