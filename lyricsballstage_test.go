package main

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// ballSong is made-up lyrics whose lines are sung word by word, with room
// between them for the ball to fly in: the last line is timed by line.
var ballSong = []lyrics.Line{
	{TimeMs: 2000, Text: "Slow wind over the pier", Words: []lyrics.Word{
		word("Slow", 2000, 2800), word("wind", 2800, 3600), word("over", 3600, 4400), word("the", 4400, 4800), word("pier", 4800, 6000)}},
	{TimeMs: 8000, Text: "Lanterns drifting home", Words: []lyrics.Word{
		word("Lanterns", 8000, 9000), word("drifting", 9000, 10200), word("home", 10200, 11000)}},
	{TimeMs: 14000, Text: "Quiet as the harbour light", Words: []lyrics.Word{
		word("Quiet", 14000, 14800), word("as", 14800, 15200), word("the", 15200, 15600), word("harbour", 15600, 16600), word("light", 16600, 18000)}},
	{TimeMs: 20000, Text: "Carry me back to shore"},
}

// ballAt has the song at pos, and the window paint it, with no time gone by.
func ballAt(t *testing.T, s *shell, tt *ui.Tester, pos time.Duration) {
	t.Helper()
	s.p.Time(pos, 200*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == pos })
	tt.Frame()
}

// ballShown counts the bright pixels in the box of a ball in the pose po,
// moved by shift: the white ball's, and what lit text lies under it.
func ballShown(tt *ui.Tester, po ballPose, shift ballPoint, radius float32) int {
	box := ui.Rect{X: po.x + shift.x - radius, Y: po.y + shift.y - radius, W: 2 * radius, H: 2 * radius}
	return brightIn(tt.Image(), box)
}

// TestStageBallFliesToTheNextLine has the ball leave the last word of a line
// in each motion, fly over the lyrics while the next line waits, and land on
// the first word of the next line: where the line sung makes its plan, and
// where the next line, once it is the one sung, finds the ball, which the
// lyrics' own glide, begun by then, doesn't move by a pixel.
func TestStageBallFliesToTheNextLine(t *testing.T) {
	for _, m := range ballMotions[1:] {
		t.Run(string(m.id), func(t *testing.T) {
			s, tt := stageOn(t, ballSong, 4000*time.Millisecond, false)
			s.set.LyricsBallMotion = m.id
			s.drain()
			rig := &s.stage.ball
			// In the middle of the flight, the ball is on its way, in the line's
			// own plan, a good way from the word it left and the one it goes to.
			begin := 8000*time.Millisecond - m.takes
			// (Not at the half: a loop is back where it started then.)
			ballAt(t, s, tt, begin+m.takes*2/5)
			if !rig.has || rig.cur.line != 0 || !rig.cur.hasNext || rig.cur.motion == nil || rig.cur.motion.id != m.id {
				t.Fatalf("the plan of the line sung is %+v", rig.cur)
			}
			b, e, ok := rig.cur.flight()
			if !ok || b != begin || e != 8000*time.Millisecond {
				t.Fatalf("the flight goes from %v to %v, %v", b, e, ok)
			}
			at := rig.clock.pos
			pl, _, po, shift, shown := rig.current(rig.stamp, rig.cur.field)
			if !shown || pl != &rig.cur || !po.flying {
				t.Fatalf("in the middle of the flight the ball is %+v, %v", po, shown)
			}
			last, arrival := rig.cur.spots[len(rig.cur.spots)-1], rig.cur.next.spot
			if d := ballDist(po.x, po.y, last.x, last.y-rig.cur.radius); d < rig.cur.size/4 {
				t.Errorf("in the middle of the flight the ball is still %v from the last word", d)
			}
			if d := ballDist(po.x, po.y, arrival.x, arrival.y-rig.cur.radius); d < rig.cur.size/2 {
				t.Errorf("in the middle of the flight the ball is already %v from where it lands", d)
			}
			if n := ballShown(tt, po, shift, rig.cur.radius); n < int(0.3*4*rig.cur.radius*rig.cur.radius) {
				t.Errorf("the flying ball shows in %d pixels, at (%v, %v)", n, po.x, po.y)
			}

			// The song goes on to the next line, sung before the ball lands on it:
			// the ball is still on its way, over the line that is now sung, and the
			// first word is where the flight meant it to be.
			ballAt(t, s, tt, 8000*time.Millisecond-120*time.Millisecond)
			if rig.cur.line != 1 || !rig.hasPrev || rig.prev.line != 0 {
				t.Fatalf("the rig holds lines %d and %d, with the line before kept: %v", rig.prev.line, rig.cur.line, rig.hasPrev)
			}
			first := rig.cur.spots[0]
			if ballDist(first.x, first.y, arrival.x, arrival.y) > 1.5 {
				t.Errorf("the first word lies at %+v, and the flight lands at %+v", first, arrival)
			}
			at = rig.clock.pos
			if !rig.inbound(at) {
				t.Fatalf("the ball has landed at %v, with the flight to last until 8s", at)
			}
			pl, _, po, shift, shown = rig.current(rig.stamp, rig.cur.field)
			if !shown || pl != &rig.prev || !po.flying {
				t.Errorf("the line sung doesn't carry the flight on: the ball is %+v, %v", po, shown)
			}
			if n := ballShown(tt, po, shift, rig.cur.radius); n < int(0.3*4*rig.cur.radius*rig.cur.radius) {
				t.Errorf("the ball vanishes as the next line is sung: %d pixels at (%v, %v)", n, po.x, po.y)
			}

			// Landed: the line sung has the ball, squashed, on its first word.
			ballAt(t, s, tt, 8000*time.Millisecond+60*time.Millisecond)
			if rig.inbound(rig.clock.pos) {
				t.Error("the ball is still on its way once the first word is sung")
			}
			pl, _, po, shift, shown = rig.current(rig.stamp, rig.cur.field)
			first = rig.cur.spots[0]
			if !shown || pl != &rig.cur || po.flying || po.sy >= 0.99 || !ballNear(po.x, first.x) || !ballNear(po.y, first.y-rig.cur.radius) {
				t.Errorf("landed, the ball is %+v, not squashed on %+v", po, first)
			}
		})
	}
}

// ballDist is how far (x0, y0) is from (x1, y1).
func ballDist(x0, y0, x1, y1 float32) float32 {
	return float32(math.Hypot(float64(x1-x0), float64(y1-y0)))
}

// TestStageBoomerangLeavesTheLyrics has the boomerang fly to the right of the
// last word, over where the lyrics lie, and show there: the ball is not held
// by the list's edges.
func TestStageBoomerangLeavesTheLyrics(t *testing.T) {
	s, tt := stageOn(t, ballSong, 4000*time.Millisecond, false)
	s.set.LyricsBallMotion = settings.LyricBallBoomerang
	s.drain()
	m := ballMotionOf(settings.LyricBallBoomerang)
	rig := &s.stage.ball
	ballAt(t, s, tt, 8000*time.Millisecond-m.takes/2)
	pl := &rig.cur
	last := pl.spots[len(pl.spots)-1]
	// The farthest it gets, and when.
	var far ballPose
	var when time.Duration
	begin, end, _ := pl.flight()
	for at := begin; at < end; at += 5 * time.Millisecond {
		if po := pl.pose(at, false); po.x > far.x {
			far, when = po, at
		}
	}
	if far.x < last.x+pl.size {
		t.Fatalf("the boomerang gets only to x %v, of a word at %v in a font %v high", far.x, last.x, pl.size)
	}
	ballAt(t, s, tt, when)
	_, _, po, shift, shown := rig.current(rig.stamp, rig.cur.field)
	if !shown {
		t.Fatal("the ball isn't there at the farthest it flies")
	}
	if n := ballShown(tt, po, shift, pl.radius); n < int(0.3*4*pl.radius*pl.radius) {
		t.Errorf("the ball shows in %d pixels at (%v, %v), past the end of the line at x %v", n, po.x, po.y, last.x)
	}
}

// TestStageBallMotions follows the setting: the motion chosen between every two
// lines, and, for the mix, a different one for each.
func TestStageBallMotions(t *testing.T) {
	s, tt := stageOn(t, ballSong, 5000*time.Millisecond, false)
	rig := &s.stage.ball
	for _, c := range []struct {
		set  settings.LyricBallMotion
		pos  time.Duration
		line int
		want settings.LyricBallMotion
	}{
		{settings.LyricBallLoop, 5000 * time.Millisecond, 0, settings.LyricBallLoop},
		{settings.LyricBallMix, 5000 * time.Millisecond, 0, settings.LyricBallHop},
		{settings.LyricBallMix, 12000 * time.Millisecond, 1, settings.LyricBallBoomerang},
	} {
		s.set.LyricsBallMotion = c.set
		s.drain()
		ballAt(t, s, tt, c.pos)
		got := settings.LyricBallMotion("none")
		if rig.cur.motion != nil {
			got = rig.cur.motion.id
		}
		if rig.cur.line != c.line || !rig.cur.hasNext || got != c.want {
			t.Errorf("%s at %v: line %d goes on in %q, with a next line: %v; want line %d in %s", c.set, c.pos, rig.cur.line, got, rig.cur.hasNext, c.line, c.want)
		}
	}
}

// TestStageBallNeedsTheNextLineTimedByWords sets the ball out for the next
// line only where its words have times: the ball of the last line sung word by
// word stays on its last word, and a line timed by line has none.
func TestStageBallNeedsTheNextLineTimedByWords(t *testing.T) {
	s, tt := stageOn(t, ballSong, 16000*time.Millisecond, false)
	rig := &s.stage.ball
	if rig.cur.line != 2 || rig.cur.hasNext {
		t.Errorf("the last line sung word by word has a flight to a line without words: line %d, %v", rig.cur.line, rig.cur.hasNext)
	}
	was := rig.stamp
	ballAt(t, s, tt, 21000*time.Millisecond)
	tt.Advance(300 * time.Millisecond)
	if !rig.stamp.Equal(was) {
		t.Error("a line timed by line gave the ball a plan")
	}
}

// TestLyricBallMotionSettings offers the motions between lines in Settings
// while the ball is on, says what the chosen one does, and keeps the one
// chosen.
func TestLyricBallMotionSettings(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 3000)
	show(s, tt, "/settings/appearance")
	if !tt.HasText("Ball between lines") || !tt.HasText(ballMotionOf(settings.LyricBallMix).about) {
		t.Fatalf("Settings doesn't offer the motion between lines: %q", tt.Texts())
	}
	s.set.LyricsBallMotion = settings.LyricBallBoomerang
	s.drain()
	tt.Frame()
	if !tt.HasText(ballMotionOf(settings.LyricBallBoomerang).about) {
		t.Errorf("Settings doesn't say what the boomerang does: %q", tt.Texts())
	}
	click(t, s, tt, "Motion between lines")
	click(t, s, tt, "Loop")
	if got := s.set.LyricsBallMotion; got != settings.LyricBallLoop {
		t.Errorf("choosing Loop leaves the motion %q", got)
	}
	if got := newShell(s.a, s.p).set.LyricsBallMotion; got != settings.LyricBallLoop {
		t.Errorf("the motion kept is %q", got)
	}
	// With the ball off there is no motion to choose.
	s.set.LyricsBall = false
	s.drain()
	tt.Frame()
	if tt.HasText("Ball between lines") {
		t.Errorf("Settings offers a motion for a ball that is off: %q", tt.Texts())
	}
	if !slices.Contains(settings.LyricBallMotions, s.set.LyricsBallMotion) {
		t.Errorf("the motion %q isn't one of them", s.set.LyricsBallMotion)
	}
}

// TestStageBallRender draws the ball's flights to MIXMUSIC_RENDER, a sheet of
// four moments of each motion, for a look.
func TestStageBallRender(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	s, tt := stageOn(t, ballSong, 4000*time.Millisecond, false)
	// The lines fade and blur to where their distance puts them, which takes a
	// moment: let it pass before the frames are taken, still in the first line.
	ballAt(t, s, tt, 6000*time.Millisecond)
	tt.Advance(time.Second)
	for _, m := range ballMotions[1:] {
		s.set.LyricsBallMotion = m.id
		s.drain()
		sheet := image.NewRGBA(image.Rect(0, 0, 4*342, 430))
		for i, u := range []float64{0.12, 0.4, 0.68, 0.96} {
			ballAt(t, s, tt, 8000*time.Millisecond-m.takes+time.Duration(u*float64(m.takes)))
			img := tt.Image()
			for y := 0; y < 430; y++ {
				for x := 0; x < 342; x++ {
					sheet.Set(i*342+x, y, img.At(464+2*x, 150+2*y))
				}
			}
		}
		f, err := os.Create(filepath.Join(dir, "ball-"+string(m.id)+".png"))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, sheet)
		f.Close()
	}
}
