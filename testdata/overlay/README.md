# Overlays in Grace format, without a link

`grace-overlay.agr` is an overlay written by hand in Grace's file format -
not made with xmgrace's Overlay graphs: g1 over g0, the same viewport and x
range, g1's y axis on the right and its x axis without ticks, numbers or
line. `grace-overlay-saved.agr` is
what gracebat saves from it:

    gracebat -nosafe -hdevice EPS -printfile out.eps grace-overlay.agr -saveall grace-overlay-saved.agr

Neither has plot-go's link comment; `figure.Candidates` must find g1 over
g0 sharing x in both, and `figure.Link` must leave the drawing as it is
(`figure/overlay_test.go`, which saves the file again with the installed
gracebat when there is one). A project overlaid with xmgrace's own Overlay
graphs is not here yet; one should be added when available.
