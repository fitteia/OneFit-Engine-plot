package main

import (
	"os"
	"strings"
	"testing"

	"github.com/fitteia/OneFit-Engine-plot/internal/afm"
)

// The copies embedded in the binary must be the repository's.
func TestNoticesMatchTheRepository(t *testing.T) {
	for name, embedded := range map[string]string{"NOTICE": notice, "LICENSE": license} {
		b, err := os.ReadFile("../../" + name)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != embedded {
			t.Errorf("cmd/plot-go/%s differs from the repository's %s: copy it", name, name)
		}
	}
	if !strings.Contains(afm.AdobeNotice(), "may be used, copied, and distributed for any purpose") {
		t.Error("Adobe's notice is not embedded")
	}
}

// Every module plot-go requires is compiled into it, so -notices must carry
// its license: one added to go.mod without its notice fails here.
func TestNoticesCoverEveryModule(t *testing.T) {
	b, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	all := notices()
	inRequire := false
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		switch {
		case len(f) >= 2 && f[0] == "require" && f[1] == "(":
			inRequire = true
		case len(f) >= 1 && f[0] == ")":
			inRequire = false
		case len(f) >= 3 && f[0] == "require":
			f = f[1:]
			fallthrough
		case inRequire && len(f) >= 2:
			// a license section of its own, not merely a mention
			if !strings.Contains(all, "---- "+f[0]+" (") || !strings.Contains(all[strings.Index(all, "---- "+f[0]+" ("):], "license ----") {
				t.Errorf("plot-go -notices has no license section for %s, which go.mod requires", f[0])
			}
		}
	}
}
