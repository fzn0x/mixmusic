package main

import (
	"image/color"
	"testing"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestLiveCardFacesAreRound finds both faces of the live tile's card round
// at their corners, the artwork's and what plays next: they round
// themselves, as the card clips nothing that turns (#309).
func TestLiveCardFacesAreRound(t *testing.T) {
	s, tt := themed(t, "mix", settings.LayoutBlend)
	near := func(a, b color.RGBA) bool {
		d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
		return d(a.R, b.R) < 24 && d(a.G, b.G) < 24 && d(a.B, b.B) < 24
	}
	check := func(face string, fill color.NRGBA) {
		t.Helper()
		r, ok := tt.Find("Open now playing")
		if !ok {
			t.Fatalf("no live tile: %q", tt.Texts())
		}
		img := tt.Image()
		x, y, size := int(r.X), int(r.Y+(r.H-liveSize(false))/2), int(liveSize(false))
		bg, want := img.RGBAAt(x+size/2, y-4), color.RGBA{fill.R, fill.G, fill.B, 255}
		if got := img.RGBAAt(x+size/2, y+size/2); !near(got, want) {
			t.Fatalf("the %s face isn't where the card is: %v, want %v", face, got, want)
		}
		for _, p := range [][2]int{{x, y}, {x + size - 1, y}, {x, y + size - 1}, {x + size - 1, y + size - 1}} {
			if got := img.RGBAAt(p[0], p[1]); !near(got, bg) {
				t.Errorf("the %s face is square at (%d, %d): %v, the ground %v", face, p[0], p[1], got, bg)
			}
		}
	}
	// The tracks have no artwork: the face is the frame shown until it
	// comes.
	check("artwork", nrgba(s.lk.surface3))
	tt.Advance(liveRest + liveTurn)
	check("up next", nrgba(s.lk.tile(0)))
}
