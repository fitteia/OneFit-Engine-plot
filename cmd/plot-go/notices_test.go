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
