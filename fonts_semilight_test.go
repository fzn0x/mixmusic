package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestSelawikSemilight sets the nav labels' family in Selawik Semilight,
// on every system: 2.7% narrower than Regular, where Light, which Windows
// and Linux took before MyGo 0.8, is 5.4% narrower.
func TestSelawikSemilight(t *testing.T) {
	registerFonts()
	const line = "The quick brown fox jumps over the lazy dog 0123456789"
	width := func(family string, weight int) float32 {
		w, _ := ui.ShapeText(line, ui.Font{Family: family, Size: 40, Weight: weight}, 0).Size()
		return w
	}
	if r := width(faceSemilight, 0) / width(face, 400); r < 0.96 || r > 0.99 {
		t.Errorf("Selawik Semilight is %.3f of Selawik's width; Semilight's is 0.973, Light's 0.946", r)
	}
}
