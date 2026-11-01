package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

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
	for _, family := range []string{face, faceSemilight, faceRound, faceMaterial, faceCrate} {
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
