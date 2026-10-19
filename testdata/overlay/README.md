# Overlays saved by Grace

`grace-overlay.agr` is a hand-written overlay, as Grace's Graph -> Overlay
makes one: g1 over g0, the same viewport and x range, g1's y axis on the
right and its x axis without ticks or numbers. `grace-overlay-saved.agr` is
what gracebat saves from it:

    gracebat -nosafe -hdevice EPS -printfile out.eps grace-overlay.agr -saveall grace-overlay-saved.agr

Neither has plot-go's link comment; `figure.Candidates` must find g1 over
g0 sharing x in both (`figure/overlay_test.go`, which saves it again with
the installed gracebat when there is one).
