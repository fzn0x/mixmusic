package main

import (
	"fmt"
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// themed is a shell in the built-in theme id and the layout design given,
// playing, and its window.
func themed(t *testing.T, id string, layout settings.Layout) (*shell, *ui.Tester) {
	t.Helper()
	s := newTestShell(t)
	st := s.svc.Preferences.Get()
	st.FollowSystem, st.Theme, st.Layout, st.Panel = false, id, layout, "none"
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

// TestLayoutDesignsLayOut checks that each layout design lays the window
// out as it says, and Tiles as before, in themes of their own: the rows'
// height, the covers' size, the player bar docked or floating, and the
// pages' titles and pivots.
func TestLayoutDesignsLayOut(t *testing.T) {
	for _, c := range []struct {
		layout              settings.Layout
		theme, title, pivot string
		row, tile           float32
		floating            bool
	}{
		{settings.LayoutTiles, "mix", "home", "everything", 52, 172, false},
		{settings.LayoutExpressive, "material", "Home", "Everything", 64, 176 + 2*12, true},
		{settings.LayoutCrate, "walnut", "Home", "Everything", 48, 196 + 2*9, false},
		{settings.LayoutBlend, "mix", "Home", "Everything", 56, 188 + 2*10.5, true},
		{settings.LayoutCrate, "sakura", "Home", "Everything", 48, 196 + 2*9, false},
	} {
		t.Run(string(c.layout)+"-"+c.theme, func(t *testing.T) {
			s, tt := themed(t, c.theme, c.layout)
			s.loads["home:netease"] = &loaded[[]media.Shelf]{done: true, value: shelves(1, 4)}
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

// TestHomeTilesLayOut checks that the user's own tiles on Home follow the
// layout design: flat fields of the tile colours in Tiles, and cards on
// the surface, as the covers', in the others.
func TestHomeTilesLayOut(t *testing.T) {
	for _, layout := range settings.Layouts {
		t.Run(string(layout), func(t *testing.T) {
			s, tt := themed(t, "mix", layout)
			show(s, tt, "/")
			r, ok := tt.Find("0 tracks")
			if !ok {
				t.Fatalf("no tile of liked songs: %q", tt.Texts())
			}
			// Left of the caption, in the tile's padding.
			fill := s.lk.surface
			if layout == settings.LayoutTiles {
				fill = s.lk.tile(1)
			}
			want := col(over(nrgba(fill), nrgba(s.lk.bg)))
			if got := at(tt.Image(), int(r.X)-6, int(r.Y+r.H/2)); got != want {
				t.Errorf("the tile of liked songs is %v, want %v", got, want)
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

// TestExpressiveIndicator checks that Expressive's rail shows the current
// page on an indicator that grows from the middle on a spring, at once
// while the computer asks for less motion, and that the window settles
// once it has grown.
func TestExpressiveIndicator(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		t.Run(fmt.Sprint("reduce motion ", reduce), func(t *testing.T) {
			s, tt := themed(t, "material", settings.LayoutExpressive)
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

// TestSpringsSettle presses, hovers and leaves the controls of the layout
// designs that move on springs, in light and dark themes: once still, the
// window asks for no more frames.
func TestSpringsSettle(t *testing.T) {
	for _, c := range []struct {
		layout settings.Layout
		theme  string
	}{
		{settings.LayoutExpressive, "material"}, {settings.LayoutExpressive, "material-dark"},
		{settings.LayoutCrate, "walnut"}, {settings.LayoutCrate, "walnut-light"},
		{settings.LayoutBlend, "mix"}, {settings.LayoutBlend, "daylight"},
	} {
		t.Run(string(c.layout)+"-"+c.theme, func(t *testing.T) {
			s, tt := themed(t, c.theme, c.layout)
			s.loads["home:netease"] = &loaded[[]media.Shelf]{done: true, value: shelves(1, 4)}
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
			// Each press waits for the player to do it, on its own
			// goroutine: on a busy runner, a track still loading showed no
			// Pause, and one that came after the window was still asked for
			// a frame. Pause waits for the track to have loaded, as a track
			// loading plays on.
			press := func(label string, done func(player.State) bool) {
				t.Helper()
				click(t, s, tt, label)
				s.settle(t, done)
				tt.Frame()
			}
			at := s.p.State().Index
			press("Next", func(st player.State) bool { return st.Index == at+1 && st.Playing && !st.Loading })
			press("Previous", func(st player.State) bool { return st.Index == at && st.Playing && !st.Loading })
			press("Pause", func(st player.State) bool { return !st.Playing })
			tt.Advance(3 * time.Second)
			settled(t, tt)
		})
	}
}

// TestFloatingPanelRoom checks that the main pane's width, which the
// pages' titles and shelves fit, leaves room for the margin of a floating
// side panel.
func TestFloatingPanelRoom(t *testing.T) {
	for _, c := range []struct {
		layout settings.Layout
		margin float32
	}{{settings.LayoutTiles, 0}, {settings.LayoutCrate, 0}, {settings.LayoutExpressive, floatMargin}, {settings.LayoutBlend, floatMargin}} {
		s, tt := themed(t, "mix", c.layout)
		s.panel = "queue"
		var got, want float32
		tt = ui.NewTester(func(ctx *ui.Context) {
			s.view(ctx)
			got = s.paneWidth(ctx)
			want = 1280 - s.railWidth() - s.panelWidth(ctx) - c.margin
		}, 1280, 820)
		tt.Frame()
		if got != want {
			t.Errorf("%s: the pane is %v wide, want %v", c.layout, got, want)
		}
	}
}

// TestLiveTileRoom checks that the rail's live tile shows whole above the
// player bar, or not at all, in every layout design, the rail folded or
// not, under macOS's window buttons or not, in windows down to 650 tall:
// Expressive's rail, of taller entries, ran on under its floating bar in
// macOS's default window of 1280×787 (#258). The tile shows there in
// every design, as in Windows' 1280×757 inside, and the entries keep
// their height in a window of 1536×870, where nothing ran on, beside a
// long list of mixes.
func TestLiveTileRoom(t *testing.T) {
	for _, layout := range settings.Layouts {
		t.Run(string(layout), func(t *testing.T) {
			s, _ := themed(t, "mix", layout)
			for i := range 20 {
				if _, err := s.svc.Collection.CreatePlaylist(fmt.Sprint("Mix ", i), "", nil); err != nil {
					t.Fatal(err)
				}
			}
			tt := testerOf(t, s, 1536, 870)
			natural := float32(42)
			if s.lk.tonal {
				natural = s.lk.tonalEntry()
			}
			for _, folded := range []bool{false, true} {
				st := s.svc.Preferences.Get()
				st.SidebarCollapsed = folded
				if _, err := s.svc.Preferences.Set(st); err != nil {
					t.Fatal(err)
				}
				s.refresh()
				for _, bar := range []ui.TitleBar{{}, macTitleBar} {
					tt.SetTitleBar(bar)
					for _, size := range [][2]int{{1536, 870}, {1280, 787}, {1280, 757}, {1280, 720}, {1280, 700}, {1280, 650}} {
						w, h := size[0], size[1]
						tt.SetSize(w, h)
						show(s, tt, "/")
						what := fmt.Sprintf("%dx%d, folded %v, macOS %v", w, h, folded, bar.Left > 0)
						top := float32(h) - s.lk.barRoom()
						// The first of that label is the tile, or the player
						// bar's artwork where there is none.
						r, _ := tt.Find("Open now playing")
						tile := r.Y < top || r.H == 0
						switch {
						case tile && (r.H != liveSize(folded) || r.Y+r.H > top):
							t.Errorf("%s: the live tile shows at %+v, the player bar from %v", what, r, top)
						case !tile && (h >= 787 || h == 757 && bar.Left == 0):
							t.Errorf("%s: no live tile", what)
						}
						entry, _ := tt.Find(s.lk.words("Settings"))
						if h == 870 && entry.H != natural {
							t.Errorf("%s: the entries are %v tall, want %v", what, entry.H, natural)
						}
						if mixes, ok := tt.Find(s.lk.words("Your mixes")); !folded && (!ok || mixes.Y < entry.Y+entry.H) {
							t.Errorf("%s: the mixes at %+v run into the entries, to %+v", what, mixes, entry)
						}
					}
				}
			}
		})
	}
}
