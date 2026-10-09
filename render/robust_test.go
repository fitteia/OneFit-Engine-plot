package render

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fitteia/OneFit-Engine-plot/agr"
)

// TestHostileWindowsFinish: windows a file may state but no tick loop can
// step through must neither hang nor crash (Codex's reproductions, among
// others): each test3 variant renders within a deadline, with a warning.
func TestHostileWindowsFinish(t *testing.T) {
	base, err := os.ReadFile("../testdata/corpus/test3-1.agr")
	if err != nil {
		t.Fatal(err)
	}
	world := regexp.MustCompile(`(?m)^@    world .*$`)
	for _, w := range []string{
		"10000000000000000, 0, 10000000000000002, 0.11", // k+1 == k
		"0, 0, Inf, 0.11",
		"0, 0, NaN, 0.11",
		"-Inf, 0, Inf, 0.11",
		"0, 0, 1e308, 0.11",
		"0, 0, 0, 0.11",
		"1e-300, 0, 2e-300, 0.11", // valid, just no tick inside: no warning needed
	} {
		src := world.ReplaceAllString(string(base), "@    world "+w)
		p, err := agr.Parse(strings.NewReader(src))
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan []string, 1)
		go func() {
			_, warnings := Render(p)
			done <- warnings
		}()
		select {
		case warnings := <-done:
			if len(warnings) == 0 && !strings.HasPrefix(w, "1e-300") {
				t.Errorf("world %s: drawn without a warning", w)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("world %s: Render did not finish", w)
		}
	}
}

func TestAutoStepRejectsWhatItCannotStep(t *testing.T) {
	for _, span := range []float64{0, -1, posInf(), nan()} {
		if s := autoStep(span, 6); s != 0 {
			t.Errorf("autoStep(%g) = %g, want 0", span, s)
		}
	}
}

func posInf() float64 { var z float64; return 1 / z }
func nan() float64    { z := posInf(); return z - z }
