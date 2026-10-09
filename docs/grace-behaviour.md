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

- Symbol 1 (circle): radius 0.01 x `symbol size`, stroked with the symbol
  linewidth; fill pattern 0 is no fill.
- Lines are clipped where they cross the viewport (a curve leaving the top
  ends exactly on the frame); symbols outside the world window are not
  drawn.

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
  gracebat rejects an unknown tick label format as a syntax error and keeps
  its default, `general` (%g-like: "100", not "100.00000").
- Strings (`with string`) start at their point (left justified), in view
  or world coordinates.

gracebat's own text boxes are rounded, so its positions differ from exact
metrics by up to about 0.002; plot-go uses Adobe's metrics, whose ink boxes
differ from Grace's URW fonts by up to about 0.02 em.

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
