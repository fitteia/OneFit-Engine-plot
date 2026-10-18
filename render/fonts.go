package render

import (
	"sort"

	"github.com/fitteia/OneFit-Engine-plot/internal/fontfile"
)

// AvailableFonts returns names accepted in a project's font map or \f{}.
// Additional names are discovered on the renderer's machine, not in a browser.
func AvailableFonts() []string {
	names := []string{"Times-Roman", "Times-Bold", "Times-Italic", "Times-BoldItalic", "Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Helvetica-BoldOblique", "Courier", "Courier-Bold", "Courier-Oblique", "Courier-BoldOblique", "Symbol", "ZapfDingbats"}
	names = append(names, fontfile.Names()...)
	sort.Strings(names)
	return uniqueNames(names)
}

func uniqueNames(names []string) []string {
	out := names[:0]
	for _, name := range names {
		if len(out) == 0 || out[len(out)-1] != name {
			out = append(out, name)
		}
	}
	return out
}
