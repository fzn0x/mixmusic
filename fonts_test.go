package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSelawikLight sets Selawik's weight 300, the page titles', in Selawik
// Light, not in Semilight, which says it weighs 300 too and took it in
// about half the launches (#241): Light is 5.4% narrower than Regular,
// Semilight 2.7%.
func TestSelawikLight(t *testing.T) {
	registerFonts()
	const line = "The quick brown fox jumps over the lazy dog 0123456789"
	width := func(weight int) float32 {
		w, _ := ui.ShapeText(line, ui.Font{Family: face, Size: 40, Weight: weight}, 0).Size()
		return w
	}
	if r := width(300) / width(400); r > 0.96 {
		t.Errorf("Selawik's weight 300 is %.3f of its 400's width, Semilight's; Light's is 0.946", r)
	}
}

// TestFacesRegistered checks that the faces MixMusic ships register under
// the families the themes name, and that their licenses show in Settings
// › About.
func TestFacesRegistered(t *testing.T) {
	registerFonts()
	have := map[string]bool{}
	for _, f := range ui.FontFamilies() {
		if f.Registered {
			have[f.Name] = true
		}
	}
	for _, family := range []string{face, faceSemilight, faceRound, faceMaterial, faceArchivo} {
		if !have[family] {
			t.Errorf("%s isn't registered: %v", family, have)
		}
		if got := fontFamily("'" + family + "', sans-serif"); family != face && got != family {
			t.Errorf("a theme naming %s gets %q", family, got)
		}
	}
	notices := fontNotices()
	for _, want := range []string{"Roboto Flex Project Authors", "Archivo Project Authors"} {
		found := false
		for _, n := range notices {
			found = found || strings.Contains(n.Text, want)
		}
		if !found {
			t.Errorf("no license of %s", want)
		}
	}
}
