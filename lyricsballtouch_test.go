package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestStageBallTouchesTheLettersInEveryStyle checks, in every style's face,
// that the spot the ball lands on is the top of the word's letters: the
// words of the line sung are looked at once they are all lit and the ball is
// on a later word, so nothing covers them, and the top of their bright
// pixels is where the ball's bottom was to be.
func TestStageBallTouchesTheLettersInEveryStyle(t *testing.T) {
	for _, id := range settings.LyricStyles {
		t.Run(string(id), func(t *testing.T) {
			s, tt := stageOn(t, stageSong, 14000*time.Millisecond, false)
			s.set.LyricsStyle = id
			s.drain()
			tt.Frame()
			tt.Advance(time.Second)
			// Still in the same line, with "Slow", "wind" and "over" sung and lit.
			ballAt(t, s, tt, 17000*time.Millisecond)
			r, ok := tt.Find("Slow wind over the pier")
			if !ok {
				t.Fatalf("the line sung doesn't show: %q", tt.Texts())
			}
			rig := &s.stage.ball
			if !rig.has || rig.cur.line != 3 || len(rig.cur.spots) != 5 {
				t.Fatalf("the line sung gave the ball's rig %+v", rig.cur)
			}
			size := rig.cur.size
			st := lyricStyleOf(id)
			words := stageSong[3].Words
			font := ui.Font{Family: st.family, Size: size, Weight: st.weight, LineHeight: st.leading}
			var dir ui.Direction
			g := shapeSung(sungKey{stageSong[3].Text, font, r.W, dir}, words)
			img := tt.Image()
			for k := range 3 {
				var cell *sungCell
				for _, row := range g.rows {
					for i := range row.cells {
						if row.cells[i].word == k && cell == nil {
							cell = &row.cells[i]
						}
					}
				}
				if cell == nil {
					t.Fatalf("the layout has no place for the word %q", words[k].Text)
				}
				spot := rig.cur.spots[k]
				// The top of the lit pixels of the word, within a little of the spot.
				box := ui.Rect{X: r.X + cell.x0, Y: spot.y - 0.4*size, W: cell.x1 - cell.x0, H: 0.8 * size}
				top := float32(-1)
				for y := int(box.Y); y < int(box.Y+box.H) && top < 0; y++ {
					if brightIn(img, ui.Rect{X: box.X, Y: float32(y), W: box.W, H: 1}) > 0 {
						top = float32(y)
					}
				}
				off := top - spot.y
				t.Logf("%s %q: letters' top at %v, the ball's bottom at %v: %+.1f (%.3f of the font)", id, words[k].Text, top, spot.y, off, off/size)
				if top < 0 || stageAbs(off) > 0.09*size {
					t.Errorf("%s %q: the ball touches at y %v and the letters' tops are at %v, in a font %v high", id, words[k].Text, spot.y, top, size)
				}
			}
		})
	}
}

// TestStageBallInAZoomedWindow keeps the ball on the letters when the app is
// zoomed: the spots come from the frame's bounds, the ball is painted in the
// overlay's own, and the two must agree.
func TestStageBallInAZoomedWindow(t *testing.T) {
	s, tt := stageOn(t, stageSong, 14000*time.Millisecond, false)
	s.set.Zoom = 1.25
	s.drain()
	tt.Frame()
	tt.Advance(time.Second)
	ballAt(t, s, tt, 14300*time.Millisecond)
	tt.Advance(10 * time.Millisecond)
	rig := &s.stage.ball
	if !rig.has || len(rig.cur.spots) != 5 {
		t.Fatalf("the line sung gave the ball's rig %+v", rig.cur)
	}
	spot, radius, z := rig.cur.spots[0], rig.cur.radius, float32(s.zoom)
	if z < 1.2 {
		t.Fatalf("the window shows the zoom %v, not 125%%", z)
	}
	// The rig's places are the content's, as the painter draws them; the
	// window shows them zoomed.
	ball := ui.Rect{X: (spot.x - radius - 1) * z, Y: (spot.y - 2*radius - 1) * z, W: (2*radius + 2) * z, H: (2*radius + 1) * z}
	if n := brightIn(tt.Image(), ball); n < int(0.3*ball.W*ball.H) {
		t.Errorf("the ball shows in %d pixels of %v, at the zoomed spot %+v", n, ball.W*ball.H, ball)
	}
}
