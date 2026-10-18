# Additional fonts

The standard 14 PostScript fonts work as before. Plot-go can also load
additional TrueType (`.ttf`) and OpenType (`.otf`, including CFF outlines)
fonts installed on the machine that renders the figure.

List discovered PostScript names and full-name aliases:

```sh
plot-go -list-fonts
```

Use one of those names in a Grace font map:

```text
@map font 20 to "URWBookman-Light", "URWBookman-Light"
@with string
@string on
@string loctype view
@string 0.2, 0.5
@string font 20
@string def "Temperature (\#{b0}C)"
```

An inline switch works too: `\f{URWBookman-Light}Temperature\f{}`.
Style variants have separate names, such as `URWBookman-DemiItalic`.
Use the names reported by `-list-fonts`; a font's filename and family name
need not identify its precise style.

## Finding files

`OFE_FONT_PATH` adds font files or directories before the usual system paths:

```sh
OFE_FONT_PATH=/path/to/my-font.ttf plot-go -list-fonts
OFE_FONT_PATH=/path/to/fonts plot-go -o figure.pdf figure.agr
```

Separate multiple entries with the OS path separator (`:` on Linux/macOS,
`;` on Windows). Directories are searched recursively in lexical order;
the first matching name wins. Symbolic-link directories are not traversed.
`OFE_URW_FONTS` is also searched, followed by user font folders, Linux system
folders, macOS Library/Fonts folders, Homebrew's shared fonts and Windows'
Fonts folder. Unknown or malformed fonts do not stop a batch: plot-go warns
and uses Helvetica for both measurement and output.

Discovery and loaded fonts are cached for the process lifetime. Restart a
long-running GUI after installing or replacing a font. In remote mode, fonts
and `OFE_FONT_PATH` belong on the server that renders the figure.

## Consistent output

Additional fonts use their own advances, glyph bounds, pair kerning and
underline metrics. Kerning remains opt-in with `\Fk`, as in standard fonts.
PDF and EPS carry vector Type-3 subsets containing the glyphs used. The font
file is not required on the viewer's machine. PDF includes a Unicode mapping
for selecting/copying text. SVG draws additional fonts as paths even without
`-outline-text`, avoiding substitution by the browser. Such SVG text is not
selectable; standard fonts retain the existing selectable native-text option.

No additional font files are shipped with plot-go. OpenType flags forbidding
embedding, subsetting or outline embedding cause a warned Helvetica fallback.
Use fonts whose terms permit your intended use and distribution of figures.

## Limits

Grace text remains byte-oriented Latin-1; `\#{b0}C` produces degrees Celsius,
and `\x` still selects Symbol for Greek. This does not introduce UTF-8 text,
complex-script shaping or automatic ligatures. Missing characters in an
additional font produce a warning and its visible missing-character glyph.
Font collections (`.ttc`/`.otc`), variable-font instance selection, colour fonts
and Type-1/PFB input are not supported in this first stage. A missing font is
not fetched from the internet.

This support starts in plot-go v0.2.21. Install a Go port and GUI release
that pins/vendors it before using it in Figures previews. Existing Figures controls
can select fonts already present in a project's font map, and text can use
inline font switches; discovering and adding installed fonts from a GUI menu
is a separate integration step.
