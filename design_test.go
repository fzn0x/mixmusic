package main

import (
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// themed is a shell in the built-in theme id, playing, and its window.
func themed(t *testing.T, id string) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShell(t)
	st := s.svc.Preferences.Get()
	st.FollowSystem, st.Theme, st.Panel = false, id, "none"
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	s.panel = ""
	// Tall enough for the shelves under the home's tiles.
	tt := testerOf(t, s, 1280, 1400)
	s.p.PlayTracks(testTracks, 1)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt.Frame()
	return s, tt
}

// at is the colour of img at (x, y), in DIPs of a window of scale 1.
func at(img *image.RGBA, x, y int) ui.Color {
	c := img.RGBAAt(x, y)
	return ui.Color{R: c.R, G: c.G, B: c.B, A: c.A}
}

// TestDesignLanguageLayouts checks that the Material and Crate themes lay
// the window out as their layout tokens say, and Mix as before: the rows'
// height, the covers' size, the player bar docked or floating, and the
// pages' titles and pivots.
func TestDesignLanguageLayouts(t *testing.T) {
	for _, c := range []struct {
		theme, title, pivot string
		row, tile           float32
		floating            bool
	}{
		{"mix", "home", "everything", 52, 172, false},
		{"material", "Home", "Everything", 64, 176 + 2*12, true},
		{"crate", "Home", "Everything", 48, 196 + 2*9, false},
	} {
		t.Run(c.theme, func(t *testing.T) {
			s, tt := themed(t, c.theme)
			s.loads["home"] = &loaded[[]media.Shelf]{done: true, value: shelves(1, 4)}
			show(s, tt, "/")
			if !tt.HasText(c.title) || !tt.HasText(c.pivot) {
				t.Errorf("the home page doesn't say %q and %q: %q", c.title, c.pivot, tt.Texts())
			}
			if r, ok := tt.Find("Album 0"); !ok || r.W != c.tile {
				t.Errorf("a cover's tile is %v wide, want %v", r.W, c.tile)
			}
			// A floating bar shows the ground under it, at the window's
			// foot; a docked one its surface.
			img := tt.Image()
			foot := at(img, 4, img.Bounds().Dy()-3)
			if floats := foot == s.lk.bg; floats != c.floating {
				t.Errorf("the player bar floats: %v; the foot of the window is %v, the ground %v", floats, foot, s.lk.bg)
			}
			// The song liked last comes first.
			for _, tr := range testTracks[2:5] {
				if err := s.svc.Collection.SetLiked(t.Context(), tr, true); err != nil {
					t.Fatal(err)
				}
			}
			show(s, tt, "/library/liked")
			a, okA := tt.Find(testTracks[4].Title)
			b, okB := tt.Find(testTracks[3].Title)
			if !okA || !okB || b.Y-a.Y != c.row {
				t.Errorf("rows %v apart, want %v", b.Y-a.Y, c.row)
			}
		})
	}
}

// indicatorWidth is how wide the indicator of the rail's entry of label
// shows, on a line across the entry over its icon and label.
func indicatorWidth(t *testing.T, s *shell, tt *ui.Tester, label string) int {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok || r.X > s.railWidth() {
		t.Fatalf("no %q in the rail: %v, %q", label, r, tt.Texts())
	}
	img := tt.Image()
	y, n := int(r.Y+6), 0
	for x := range int(s.railWidth()) {
		if at(img, x, y) != s.lk.bg {
			n++
		}
	}
	return n
}

// TestExpressiveIndicator checks that the Material themes' rail shows the
// current page on an indicator that grows from the middle on a spring,
// at once while the computer asks for less motion, and that the window
// settles once it has grown.
func TestExpressiveIndicator(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		t.Run(fmt.Sprint("reduce motion ", reduce), func(t *testing.T) {
			s, tt := themed(t, "material")
			tt.SetPreferences(ui.Preferences{ReduceMotion: reduce})
			show(s, tt, "/")
			tt.Advance(time.Second)
			full := indicatorWidth(t, s, tt, "Home")
			if full < 100 {
				t.Fatalf("the current page's indicator is %d wide", full)
			}
			click(t, s, tt, "Stats")
			tt.Advance(40 * time.Millisecond)
			grown := indicatorWidth(t, s, tt, "Stats")
			switch {
			case reduce && grown != full:
				t.Errorf("with less motion, the indicator is %d wide at once, want %d", grown, full)
			case !reduce && (grown == 0 || grown >= full):
				t.Errorf("40 ms in, the indicator is %d wide, of %d", grown, full)
			}
			tt.Advance(2 * time.Second)
			if got := indicatorWidth(t, s, tt, "Stats"); got != full {
				t.Errorf("once grown, the indicator is %d wide, want %d", got, full)
			}
			if indicatorWidth(t, s, tt, "Home") != 0 {
				t.Error("the page left keeps its indicator")
			}
			settled(t, tt)
		})
	}
}

// TestSpringsSettle presses, hovers and leaves the controls of the themes
// that move on springs: once still, the window asks for no more frames.
func TestSpringsSettle(t *testing.T) {
	for _, id := range []string{"material", "material-dark", "crate", "crate-light"} {
		t.Run(id, func(t *testing.T) {
			s, tt := themed(t, id)
			s.loads["home"] = &loaded[[]media.Shelf]{done: true, value: shelves(1, 4)}
			show(s, tt, "/")
			r, ok := tt.Find("Album 1")
			if !ok {
				t.Fatalf("no tile: %q", tt.Texts())
			}
			tt.Move(r.X+r.W/2, r.Y+r.H/3)
			tt.Advance(50 * time.Millisecond)
			if !tt.HasText("Album 1") {
				t.Fatal("the tile went")
			}
			tt.Move(5, 5)
			for _, label := range []string{"Next", "Previous", "Pause"} {
				click(t, s, tt, label)
			}
			tt.Advance(3 * time.Second)
			settled(t, tt)
		})
	}
}
