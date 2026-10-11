# How gracebat draws OneFit's plots

Measured from gracebat 5.1.25's EPS output with `cmd/grace-probe` (run it on
a variant of a corpus file and read the primitives), and from the Grace
User's Guide where it says so. This is the renderer's specification; none
of it comes from Grace's source code.

Units: viewport units - the page's shorter side is 1, origin bottom left
(the guide; the EPS is scaled by 600 for a 773x600 page).

## Lines and symbols

- Line width: 0.0015 per unit of `linewidth` (2.0 -> 0.003).
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
  solid - the hatched patterns 2 and up are drawn solid by plot-go), then
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
- Lines are clipped where they cross the viewport (a curve leaving the top
  ends on the frame - gracebat's clip box is about 0.00015 larger than the
  viewport, plot-go clips at the viewport); each visible piece is a path of
  its own. Symbols and error bars of points outside the world window are
  not drawn.
- Error bars (`errorbar on`, xydy sets; measured with `style-errorbars`
  and `style-errorbars-log`), drawn after the set's line and before its
  symbols, point by point:
  - the riser from the point to y+dy, the cap across its end (left to
    right), the riser down to y-dy, its cap (right to left); `place normal`
    draws only the upper half, `opposite` only the lower one. The sign of
    dy does not matter; dy 0 draws nothing.
  - cap half length 0.01 x `errorbar size` (size 0 still strokes a
    zero-length cap); caps use `errorbar linewidth`/`linestyle`, risers
    `riser linewidth`/`linestyle`; style 0 leaves that part out;
    `errorbar pattern 0` draws nothing at all.
  - an end out of the viewport (or that a log axis cannot place: at or
    below 0) has no cap: the riser runs to the frame; with `riser clip on`
    it runs `riser clip length` instead and ends in an arrowhead - from
    (x+h, tip-2h) through the tip to (x-h, tip-2h) going up, the other way
    down, h the cap half length, always solid, at the cap's width.
  - every part is cut at the frame (an arrowhead past it is cut or gone).
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

## Grid lines

- `tick major grid on` / `tick minor grid on` (measured with `style-grids`
  and `style-grids-noticks`): a line across the viewport at each major /
  minor tick inside the window, in that tick mark's color, linewidth and
  linestyle; drawn first in the graph, before the sets: x minor, x major,
  y minor, y major. An axis that is off has none; ticks off do not matter.

## Ticks

- Length 0.02 per unit of tick size (major 1.5 -> 0.03, minor 1.0 -> 0.02),
  pointing in, out, or both; `place both` draws them on both sides.
- Major ticks: multiples of `tick major` inside the world window, ends
  included (a window starting at 1e-05 has no tick at 0).
- Linear minor ticks: `minor ticks` m evenly between majors, step
  major/(m+1).
- Log minor ticks: at 2x, 3x, ... (m+1)x each decade's start, whatever m
  is (m=9 puts the last on the next decade).
- Minor ticks come only from intervals (decades) that reach into the
  window, not from one merely touching its edge: a log window starting at
  1e4 gets no 10x minor from 1e3..1e4, one starting at 2e4 gets 1e4's
  minors from 2e4 on.
- Too many ticks: more than about 255 majors, or more than about 255 minors
  across the major intervals the window touches, and gracebat prints "Too
  many ticks ( > MAX_TICKS ), autoticking": the major step becomes the
  smallest of 1, 2, 5 x 10^n giving at most `tick default`+1 intervals
  (default 6 -> 7), and at most one minor tick per interval.

## Text

- Font size: 0.028 per unit of char size (1.5 -> 0.042).
- Fonts by number from the file's font map; `\x` is the Symbol font.
- Escapes (the guide gives the factors rounded; measured exactly):
  `\-` scales by 2^(-1/4); `\s` shifts the baseline by -0.4 and `\S` by
  +0.6 of the current size, then scale by 2^(-1/2); `\N` restores size and
  baseline; `\0`..`\9` switch to that font number.
- Strings are laid out run by run with the fonts' advance widths (no
  kerning).
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
- A format may also be given by number: 0 decimal, 1 exponential, 2
  general, 3 power, 4 scientific, 5 engineering, 6 computing, 7 and up dates
  and times. A number Grace does not know (OneFit wrote 60) draws as
  decimal - and is saved as "unknown", which gracebat then rejects when it
  reads its own project back, so that project no longer draws what it
  printed. plot-go saves the format by name.
- Defaults for what a file leaves out: format general, precision 5 (a
  "ticklabel prec" line without a value is ignored, leaving 5).
- Strings (`with string`) start at their point (left justified), in view
  or world coordinates.

Text is measured with the URW base-35 fonts' glyphs, as gracebat does (its
fonts are those files): plot-go reads each character's ink box from the
installed URW OpenType font and falls back to Adobe's metrics, whose ink
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


## Drawing order

Page background; then per graph: each set (line, then symbols), the x axis
(minor ticks, major ticks, tick labels, label), the y axis likewise, the
frame, and the strings placed in that graph's world coordinates; then the
strings placed on the page (view coordinates). Each tick is drawn on the
normal side, then the opposite one.

## How closely plot-go matches

`render`'s test draws every corpus file and compares it with gracebat's
EPS primitive by primitive: all paths, arcs, texts, fonts, sizes and colors
agree, texts within 0.0012 and points within 0.0001 (the EPS's rounding).

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
