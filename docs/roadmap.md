# Roadmap

What plot-go does not draw yet, and in which order it is to be done
(on hold since 2026-10-13).
[capabilities.md](capabilities.md) is what it draws now. Every gap below is
read, warned about and kept when a project is saved; none of them is on in
the files OneFit itself writes ([inventory.md](inventory.md)), so they
matter for projects made elsewhere and for the Figures editor.

Each step is measured from gracebat like the rest
([grace-behaviour.md](grace-behaviour.md)), released as its own tag, and
then vendored in the GUI and pinned in the debs.

## On hold (the ones that matter for OneFit, when taken up again)

1. **The secondary axis**: alternate axes (`altxaxis`, `altyaxis`), the
   reciprocal scale, tick label formulas and tick label start/stop - the
   "1000/T on top" axis of relaxation plots. Formulas need an evaluator for
   Grace's expression syntax.

## Done

- An axis at zero and axis bars (`type zero true`, `bar on`): v0.2.11.

- The set types with other error bars (`xydx`, `xydxdx`, `xydxdy`,
  `xydydy`, `xydxdxdydy`): v0.2.4.

## Later

- **Drawing objects**: lines with arrows, boxes and ellipses (`@with line`,
  `@with box`, `@with ellipse`). Today they are kept,
  each whole, but not drawn. With them the Figures editor could add annotations.
- **Annotated values** (`avalue`): numbers or labels next to the points.
- **Regions and fixed points**: measure first whether gracebat draws them
  at all; if it does not, only the warning goes.
- **Other set types**: bar, bardy, bardydy, xyhilo, xyr, xysize, xycolor,
  xycolpat, xyvmap, xyboxplot, xyz.
- **Other graph types and stacked graphs**: polar, Smith, pie, chart,
  fixed.
- **Text**: Grace's escapes plot-go leaves out (underline, overline, zoom,
  the other font forms) and fonts outside the standard 14.
- **PNG output**: `-format png` handing the PDF or SVG to an installed
  converter (`pdftoppm`, `gs`, `rsvg-convert`); until then, convert by hand.
- **Packages**: RPM, SUSE and Arch packages of plot-go.
