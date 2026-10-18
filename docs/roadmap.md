# Roadmap

What plot-go does not draw yet, and in which order it is to be done.
[capabilities.md](capabilities.md) is what it draws now. Every gap below is
read, warned about and kept when a project is saved; none of them is on in
the files OneFit itself writes ([inventory.md](inventory.md)), so they
matter for projects made elsewhere and for the Figures editor.

Each step is measured from gracebat like the rest
([grace-behaviour.md](grace-behaviour.md)), released as its own tag, and
then vendored in the GUI and pinned in the debs.

## Done

- **Variable-size symbols** (`xysize`): implemented, pending release. Per-point
  size column with standard symbol styles, character symbols and legend sample
  sizing measured against Grace. Figures pairs this with a plot-type choice and
  the data editor/calculator. Plot-type changes preserve all stored columns;
  users explicitly add or edit the size column in the data editor.

- **XY bar family**: v0.2.24. `bar`, `bardy` and `bardydy` render with
  baseline selection, width, row skipping, fills, outlines, legends and
  symmetric/asymmetric y errors. Figures exposes width, baseline and fill
  controls alongside its existing data editor and column calculator.
  The fitting Plots pane is unchanged.

- **Hi/Lo/Open/Close candles** (`xyhilo`): v0.2.26.
  The renderer and Figures editor support five-column candle data, Grace's
  symbol-based stroke styling, size and row skipping. The fitting Plots pane
  is unchanged.

- **Per-point fill colors** (`xycolor`): v0.2.27.
  The third data column selects each symbol's fill color from the project
  color map; Figures exposes the type and explains fill/outline behavior.
  The fitting Plots pane is unchanged.

- **Vector maps** (`xyvmap`): implemented, pending release. The four columns
  are `(x, y, dx, dy)`; vector offsets are in normalized graph-view
  coordinates. Figures exposes the type, arrowhead size and Grace vector
  stroke controls. The fitting Plots pane is unchanged.

- **Foreground annotations**: v0.2.22. Optional per-object On top setting
  draws shapes above legends and text while retaining coordinate attachment.
  Saved as a plot-go comment extension; unmarked Grace projects are unchanged.

- **Common text controls**: v0.2.17. Underline and overline, relative/absolute zoom,
  vertical and horizontal shifts, baseline changes, newlines, hexadecimal
  characters and legacy upper-half character controls. v0.2.18 moved the
  overline above glyph ink, intentionally correcting Grace's baseline-level
  placement.
- **Remaining documented text controls**: v0.2.18. Matrix composition/reset, inline
  rotation/slant/oblique, colours, AFM pair kerning, position marks and text
  directions. Ligature toggles are recognized; the supported Core-14 Latin-1
  fonts have no encodable ligature substitutions, matching measured Grace.
  These controls, including the corrected overline placement, are included in
  v0.2.21, which the GUI and OFE-Go pin and vendor. v0.2.20 also fixes
  Latin-1 advance widths in the outlined SVG preview.

- **Annotated values** (`avalue`): v0.2.16. Point values and quoted labels,
  including style, rotation, offsets, symbol skip and clipping. The GUI
  Figures editor has per-set Point labels controls and preserves labels
  during data editing and calculator operations.
- **Editable drawing objects**: v0.2.15. Changed and newly added arrows,
  boxes and ellipses are saved from their current settings.

- **Regions and fixed points**: measured against `gracebat` and the Grace
  User's Guide. Enabled regions select data for analyses and transformations;
  fixed points set the interactive locator origin. Neither changes hardcopy
  output, so plot-go preserves these directives without warning rather than
  implementing them as drawing features.

- **Remaining set types surveyed**: Grace draws `xyboxplot` and
  `xyz` in XY graphs.
  `xycolpat` draws in pie graphs and `xyr` in fixed graphs; Grace rejects both
  in XY graphs. plot-go does not yet render any of these types.

- PNG output (`-format png`, gracebat's `-hdevice PNG`) through an
  installed converter, and drawing objects (lines with arrows, boxes,
  ellipses): v0.2.13. The Figures editor can now be given annotations.
- The secondary axis: alternate axes (`altxaxis`, `altyaxis`), the
  reciprocal scale, tick label formulas and start/stop: v0.2.12. The
  Figures editor's control for it is in the GUI.
- An axis at zero and axis bars (`type zero true`, `bar on`): v0.2.11.

- The set types with other error bars (`xydx`, `xydxdx`, `xydxdy`,
  `xydydy`, `xydxdxdydy`): v0.2.4.

## Later

- **Other set types**: xyr, xycolpat, xyboxplot, xyz.
- **Other graph types and stacked graphs**: polar, Smith, pie, chart,
  fixed.
- **Axis scales** beyond normal, logarithmic and reciprocal.
- **Further font support**: font collections, variable/colour fonts, Unicode
  text beyond the current Latin-1 encoding and font-specific shaping/ligatures.
- **Packages**: RPM, SUSE and Arch packages of plot-go.

## Additional fonts: v0.2.21

- Additional installed TTF/OpenType fonts by PostScript/full name, with their
  own glyph advances, ink bounds, kerning and underline metrics.
- Discoverable font names (`-list-fonts`) and additional font paths
  (`OFE_FONT_PATH`). Standard font output remains unchanged.
- Self-contained PDF/EPS vector glyph subsets and outlined SVG, including
  selectable PDF text. Embedding-restricted fonts use a warned fallback.
- The GUI and OFE-Go pin and vendor this release; OFE-Raku pins its installer.
  Figures font discovery controls are a separate step; the GUI can select fonts
  present in an imported project's font map, or use `\f{font-name}` in text.
