# Roadmap

What plot-go does not draw yet, and in which order it is to be done
(the two items that mattered for OneFit are done; the rest below is for
later).
[capabilities.md](capabilities.md) is what it draws now. Every gap below is
read, warned about and kept when a project is saved; none of them is on in
the files OneFit itself writes ([inventory.md](inventory.md)), so they
matter for projects made elsewhere and for the Figures editor.

Each step is measured from gracebat like the rest
([grace-behaviour.md](grace-behaviour.md)), released as its own tag, and
then vendored in the GUI and pinned in the debs.

## Implemented, awaiting release

- **Common text controls**: underline and overline, relative/absolute zoom,
  vertical and horizontal shifts, baseline changes, newlines, hexadecimal
  characters and legacy upper-half character controls. Measured against
  Grace across the 14 standard fonts and rotated/justified labels. These
  changes are not yet included in the GUI or OFE-Go dependency pins.

## Done

- **Annotated values** (`avalue`): v0.2.16. Point values and quoted labels,
  including style, rotation, offsets, symbol skip and clipping. The GUI
  Figures editor has per-set Point labels controls and preserves labels
  during data editing and calculator operations.
- **Editable drawing objects**: v0.2.15. Changed and newly added arrows,
  boxes and ellipses are saved from their current settings.

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

- **Regions and fixed points**: measure first whether gracebat draws them
  at all; if it does not, only the warning goes.
- **Other set types**: bar, bardy, bardydy, xyhilo, xyr, xysize, xycolor,
  xycolpat, xyvmap, xyboxplot, xyz.
- **Other graph types and stacked graphs**: polar, Smith, pie, chart,
  fixed.
- **Axis scales** beyond normal, logarithmic and reciprocal.
- **Remaining text controls**: transformation matrices, rotation/slant and
  oblique forms, inline colours, kerning, ligatures, marked positions and
  direction controls; fonts outside the standard 14.
- **Packages**: RPM, SUSE and Arch packages of plot-go.
