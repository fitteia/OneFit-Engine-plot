package main

// OneFit's own project settings (OneFit-Engine's Agrs.rakumod, as each
// fit{N}.agr-par is written): a bare plot is drawn as the suite's are.
// Placeholders in braces are filled by autoSettings.

// ofeHead is the page, fonts, colors and defaults.
const ofeHead = `version 50114
page size 773, 600
page scroll 5%
page inout 5%
link page off
map font 0 to "Times-Roman", "Times-Roman"
map font 2 to "Times-Italic", "Times-Italic"
map font 1 to "Times-Bold", "Times-Bold"
map font 3 to "Times-BoldItalic", "Times-BoldItalic"
map font 4 to "Helvetica", "Helvetica"
map font 6 to "Helvetica-Oblique", "Helvetica-Oblique"
map font 5 to "Helvetica-Bold", "Helvetica-Bold"
map font 7 to "Helvetica-BoldOblique", "Helvetica-BoldOblique"
map font 10 to "Courier-Bold", "Courier-Bold"
map font 11 to "Courier-BoldOblique", "Courier-BoldOblique"
map font 8 to "Symbol", "Symbol"
map font 9 to "ZapfDingbats", "ZapfDingbats"
map color 0 to (255, 255, 255), "white"
map color 1 to (0, 0, 0), "black"
map color 2 to (255, 0, 0), "red"
map color 3 to (0, 255, 0), "green"
map color 4 to (0, 0, 255), "blue"
map color 5 to (255, 255, 0), "yellow"
map color 6 to (188, 143, 143), "brown"
map color 7 to (220, 220, 220), "grey"
map color 8 to (148, 0, 211), "violet"
map color 9 to (0, 255, 255), "cyan"
map color 10 to (255, 0, 255), "magenta"
map color 11 to (255, 165, 0), "orange"
map color 12 to (114, 33, 188), "indigo"
map color 13 to (103, 7, 72), "maroon"
map color 14 to (64, 224, 208), "turquoise"
map color 15 to (0, 139, 0), "green4"
reference date 0
date wrap on
date wrap year 1900
default linewidth 1.0
default linestyle 1
default color 1
default pattern 1
default font 4
default char size 1.000000
default symbol size 1
default sformat "%16.8g"
background color 0
page background fill on
timestamp off
timestamp 0.03, 0.03
timestamp color 1
timestamp rot 0
timestamp font 4
timestamp char size 1.000000
timestamp def ""
`

// ofeGraph is graph 0: its window, view, axes, legend and frame.
const ofeGraph = `r0 off
link r0 to g0
r0 type above
r0 linestyle 1
r0 linewidth 1.0
r0 color 1
r0 line 0, 0, 0, 0
r1 off
link r1 to g0
r1 type above
r1 linestyle 1
r1 linewidth 1.0
r1 color 1
r1 line 0, 0, 0, 0
r2 off
link r2 to g0
r2 type above
r2 linestyle 1
r2 linewidth 1.0
r2 color 1
r2 line 0, 0, 0, 0
r3 off
link r3 to g0
r3 type above
r3 linestyle 1
r3 linewidth 1.0
r3 color 1
r3 line 0, 0, 0, 0
r4 off
link r4 to g0
r4 type above
r4 linestyle 1
r4 linewidth 1.0
r4 color 1
r4 line 0, 0, 0, 0
g0 on
g0 hidden false
g0 type XY
g0 stacked false
g0 bar hgap 0.000000
g0 fixedpoint off
g0 fixedpoint type 0
g0 fixedpoint xy 0.000000, 0.000000
g0 fixedpoint format general general
g0 fixedpoint prec 6, 6
with g0
    world xmin {xmin}
    world xmax {xmax}
    world ymin {ymin}
    world ymax {ymax}
    stack world 0, 0, 0, 0
    znorm 1
    view xmin 0.183247
    view xmax 1.038397
    view ymin 0.149994
    view ymax 0.849964
    title ""
    title font 4
    title size 1.500000
    title color 1
    subtitle ""
    subtitle font 4
    subtitle size 1.000000
    subtitle color 1
    xaxes scale {xscale}
    yaxes scale {yscale}
    xaxes invert off
    yaxes invert off
    xaxis  on
    xaxis  type zero false
    xaxis  offset 0.000000 , 0.000000
    xaxis  bar off
    xaxis  bar color 1
    xaxis  bar linestyle 1
    xaxis  bar linewidth 2.0
    xaxis  label ""
    xaxis  label layout para
    xaxis  label place auto
    xaxis  label char size 1.500000
    xaxis  label font 4
    xaxis  label color 1
    xaxis  label place normal
    xaxis  tick on
    xaxis  tick major {xstep}
    xaxis  tick minor ticks {xminor}
    xaxis  tick default 6
    xaxis  tick place rounded true
    xaxis  tick in
    xaxis  tick major size 1.500000
    xaxis  tick major color 1
    xaxis  tick major linewidth 2
    xaxis  tick major linestyle 1
    xaxis  tick major grid off
    xaxis  tick minor color 1
    xaxis  tick minor linewidth 2
    xaxis  tick minor linestyle 1
    xaxis  tick minor grid off
    xaxis  tick minor size 1.000000
    xaxis  ticklabel on
    xaxis  ticklabel format {xformat}
    xaxis  ticklabel prec {xprec}
    xaxis  ticklabel formula ""
    xaxis  ticklabel append ""
    xaxis  ticklabel prepend ""
    xaxis  ticklabel angle 0
    xaxis  ticklabel skip 0
    xaxis  ticklabel stagger 0
    xaxis  ticklabel place normal
    xaxis  ticklabel offset auto
    xaxis  ticklabel offset 0.000000 , 0.010000
    xaxis  ticklabel start type auto
    xaxis  ticklabel start 0.000000
    xaxis  ticklabel stop type auto
    xaxis  ticklabel stop 0.000000
    xaxis  ticklabel char size 1.500000
    xaxis  ticklabel font 4
    xaxis  ticklabel color 1
    xaxis  tick place both
    xaxis  tick spec type none
    yaxis  on
    yaxis  type zero false
    yaxis  offset 0.000000 , 0.000000
    yaxis  bar off
    yaxis  bar color 1
    yaxis  bar linestyle 1
    yaxis  bar linewidth 1.0
    yaxis  label ""
    yaxis  label layout para
    yaxis  label place auto
    yaxis  label char size 1.500000
    yaxis  label font 4
    yaxis  label color 1
    yaxis  label place normal
    yaxis  tick on
    yaxis  tick major {ystep}
    yaxis  tick minor ticks {yminor}
    yaxis  tick default 6
    yaxis  tick place rounded true
    yaxis  tick in
    yaxis  tick major size 1.500000
    yaxis  tick major color 1
    yaxis  tick major linewidth 2
    yaxis  tick major linestyle 1
    yaxis  tick major grid off
    yaxis  tick minor color 1
    yaxis  tick minor linewidth 2
    yaxis  tick minor linestyle 1
    yaxis  tick minor grid off
    yaxis  tick minor size 1.000000
    yaxis  ticklabel on
    yaxis  ticklabel format {yformat}
    yaxis  ticklabel prec {yprec}
    yaxis  ticklabel formula ""
    yaxis  ticklabel append ""
    yaxis  ticklabel prepend ""
    yaxis  ticklabel angle 0
    yaxis  ticklabel skip 0
    yaxis  ticklabel stagger 0
    yaxis  ticklabel place normal
    yaxis  ticklabel offset auto
    yaxis  ticklabel offset 0.000000 , 0.010000
    yaxis  ticklabel start type auto
    yaxis  ticklabel start 0.000000
    yaxis  ticklabel stop type auto
    yaxis  ticklabel stop 0.000000
    yaxis  ticklabel char size 1.500000
    yaxis  ticklabel font 4
    yaxis  ticklabel color 1
    yaxis  tick place both
    yaxis  tick spec type none
    altxaxis  off
    altyaxis  off
    legend on
    legend loctype view
    legend 0.977315227809, 0.799966426
    legend box color 1
    legend box pattern 0
    legend box linewidth 1.0
    legend box linestyle 1
    legend box fill color 0
    legend box fill pattern 1
    legend font 4
    legend char size 1.500000
    legend color 1
    legend length 4
    legend vgap 1
    legend hgap 1
    legend invert false
    frame type 0
    frame linestyle 1
    frame linewidth 2.0
    frame color 1
    frame pattern 1
    frame background color 0
    frame background pattern 0
`

// ofeSet is a set's style: OneFit's data (symbols) or curve (line).
const ofeSet = `    s{k} hidden false
    s{k} type {type}
    s{k} symbol {symbol}
    s{k} symbol size 1
    s{k} symbol color 1
    s{k} symbol pattern 1
    s{k} symbol fill color 1
    s{k} symbol fill pattern 0
    s{k} symbol linewidth 2.0
    s{k} symbol linestyle 1
    s{k} symbol char 65
    s{k} symbol char font 4
    s{k} symbol skip 0
    s{k} line type {line}
    s{k} line linestyle 1
    s{k} line linewidth 2.0
    s{k} line color 1
    s{k} line pattern 1
    s{k} baseline type 0
    s{k} baseline off
    s{k} dropline off
    s{k} fill type 0
    s{k} fill rule 0
    s{k} fill color 1
    s{k} fill pattern 1
    s{k} avalue off
    s{k} avalue type 2
    s{k} avalue char size 1.000000
    s{k} avalue font 4
    s{k} avalue color 1
    s{k} avalue rot 0
    s{k} avalue format general
    s{k} avalue prec 3
    s{k} avalue prepend ""
    s{k} avalue append ""
    s{k} avalue offset 0.000000 , 0.000000
    s{k} errorbar {errorbar}
    s{k} errorbar place both
    s{k} errorbar color 1
    s{k} errorbar pattern 1
    s{k} errorbar size 1.000000
    s{k} errorbar linewidth 1.0
    s{k} errorbar linestyle 1
    s{k} errorbar riser linewidth 1.0
    s{k} errorbar riser linestyle 1
    s{k} errorbar riser clip off
    s{k} errorbar riser clip length 0.100000
    s{k} comment "sm1-gph.agr"
    s{k} legend  ""
`
