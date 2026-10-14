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

## Done

- PNG output (`-format png`, gracebat's `-hdevice PNG`) through an
  installed converter: v0.2.13.
- The secondary axis: alternate axes (`altxaxis`, `altyaxis`), the
  reciprocal scale, tick label formulas and start/stop: v0.2.12. The
  Figures editor's control for it is in the GUI.
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
- **Packages**: RPM, SUSE and Arch packages of plot-go.
