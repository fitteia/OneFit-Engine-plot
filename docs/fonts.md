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

TrueType collections (`.ttc`) are searched too, each font in them by its
own names. A font plot-go cannot draw - one its font reader cannot parse, or
a bitmap-only font such as color emoji - is not listed.

The names found are kept in a cache file, `plot-go/fonts.json` in the
user's cache folder (`~/Library/Caches` on macOS, `$XDG_CACHE_HOME` or
`~/.cache` on Linux): with each font file's size and modification time.
A run lists the font folders and reads only the files that are new or
changed since, so only the first run (and the first after installing
fonts) reads them all. Deleting the file is harmless: it is made again. A
long-running GUI looks at the folders again after 30 seconds, so a font
installed meanwhile shows up without a restart. Replaced additional font files
refresh their parsed faces, metrics and outlines on the same discovery cycle.
Reopen the Figures font chooser to refresh its list and samples. Changes are
detected by file size and modification time. In remote mode, fonts and
`OFE_FONT_PATH` belong on the server that renders the figure.

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

The standard fonts (Times, Helvetica, Courier, Symbol, ZapfDingbats and the
rest of the base 35) are embedded in a PDF when the URW base-35 fonts are
installed: the glyphs used, subset from the URW OpenType file's CFF outlines
into a Type1C font program (`FontFile3`) with the font's own encoding and a
Unicode mapping - the same kind of font Ghostscript embeds from an EPS, and
drawn the same. Without the URW fonts, or for a character the URW font has
no glyph for, the font is named instead and the viewer supplies it. EPS
output names the standard fonts, as PostScript does.

## Limits

Grace text remains byte-oriented Latin-1; `\#{b0}C` produces degrees Celsius,
and `\x` still selects Symbol for Greek. This does not introduce UTF-8 text,
complex-script shaping or automatic ligatures. Missing characters in an
additional font produce a warning and its visible missing-character glyph.
OpenType collections (`.otc`), variable-font instance selection, colour fonts
and Type-1/PFB input are not supported in this first stage. A missing font is
not fetched from the internet.

This support starts in plot-go v0.2.21. Install a Go port and GUI release
that pins/vendors it before using it in Figures previews. Figures Font buttons
provide a searchable chooser for installed fonts, including named TTC faces,
with samples drawn on the rendering server. Imported font maps and inline
font switches continue to work.
