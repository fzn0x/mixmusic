package main

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// testBallPlan is the plan of a line of three words, sung from 1 s, with the
// ball going on to a next line at 7 s, in the motion given: the words lie on a
// row whose letters' tops are at y 100, with a ball 5 DIPs wide in radius.
func testBallPlan(motion settings.LyricBallMotion) ballPlan {
	words := []lyrics.Word{word("Slow", 1000, 1500), word("wind", 1500, 2000), word("over", 4000, 4500)}
	return ballPlan{
		line: 2, words: words, spots: []ballPoint{{20, 100}, {120, 100}, {200, 100}},
		next: ballArrival{at: 7000 * time.Millisecond, spot: ballPoint{20, 260}}, hasNext: true,
		motion: ballMotionOf(motion), size: 50, radius: 5,
		field: ui.Rect{X: 0, Y: 0, W: 600, H: 700},
	}
}

func ballNear(a, b float32) bool { return a-b < 0.01 && b-a < 0.01 }

// TestBallPose has the ball come before its first word, sit on the letters of
// a word, hop to the next only in the last of the gap before it, land as it
// starts, squash and leave a ring, fly to the next line, and, where it
// shouldn't move, stay.
func TestBallPose(t *testing.T) {
	pl := testBallPlan(settings.LyricBallHop)
	for _, c := range []struct {
		name         string
		at           time.Duration
		x, y         float32
		scale        float32 // 0 when it isn't there yet, 1 when it is, -1 when it is growing
		squashed     bool
		ring, flying bool
	}{
		{"not there before its first word", 500 * time.Millisecond, 20, 95, 0, false, false, false},
		{"growing as the first word comes", 820 * time.Millisecond, 20, 95, -1, false, false, false},
		{"there as the first word starts, squashed, a ring over it", 1000 * time.Millisecond, 20, 95, 1, true, true, false},
		{"the hop starts, still squashed", 1040 * time.Millisecond, 20, 95, 1, true, true, false},
		{"halfway through the hop, lifted", 1270 * time.Millisecond, 70, 75, 1, false, true, false},
		{"landed as the next word starts", 1500 * time.Millisecond, 120, 95, 1, true, true, false},
		{"waiting in a long gap, no ring left", 3000 * time.Millisecond, 120, 95, 1, false, false, false},
		{"halfway to the word after a gap", 3770 * time.Millisecond, 160, 77.5, 1, false, false, false},
		{"on the last word", 5000 * time.Millisecond, 200, 95, 1, false, false, false},
		{"halfway through the flight to the next line", 6720 * time.Millisecond, 110, 135, 1, false, false, true},
		{"landed on the next line", 7000 * time.Millisecond, 20, 255, 1, true, true, false},
	} {
		po := pl.pose(c.at, false)
		if !po.ok {
			t.Errorf("%s: no pose", c.name)
			continue
		}
		switch {
		case c.scale == 0 && po.scale != 0, c.scale == 1 && po.scale != 1, c.scale == -1 && (po.scale <= 0 || po.scale > 1.2):
			t.Errorf("%s: the ball is grown %v", c.name, po.scale)
		}
		if c.scale == 0 {
			continue
		}
		if !ballNear(po.x, c.x) || !ballNear(po.y, c.y) {
			t.Errorf("%s: the ball is at (%v, %v), not (%v, %v)", c.name, po.x, po.y, c.x, c.y)
		}
		if squashed := po.sy < 0.99; squashed != c.squashed {
			t.Errorf("%s: the ball is squashed: %v, not %v: it is %v tall", c.name, squashed, c.squashed, po.sy)
		}
		if got := po.ringU >= 0; got != c.ring {
			t.Errorf("%s: a ring spreads: %v, not %v", c.name, got, c.ring)
		}
		if po.flying != c.flying {
			t.Errorf("%s: the ball flies: %v, not %v", c.name, po.flying, c.flying)
		}
	}

	// A word that begins the line rings bigger than one in it.
	if po := pl.pose(1100*time.Millisecond, false); !po.ringBig {
		t.Error("the first word of a line leaves a small ring")
	}
	if po := pl.pose(1600*time.Millisecond, false); po.ringBig {
		t.Error("a word in the line leaves a ring as big as its first word's")
	}

	// Where the computer asks for less motion, it sits on the word sung, and
	// is there before the first.
	for _, c := range []struct {
		at   time.Duration
		x, y float32
	}{{500 * time.Millisecond, 20, 95}, {1270 * time.Millisecond, 20, 95}, {1500 * time.Millisecond, 120, 95}, {6720 * time.Millisecond, 200, 95}} {
		po := pl.pose(c.at, true)
		if !ballNear(po.x, c.x) || !ballNear(po.y, c.y) || po.scale != 1 || po.sy != 1 || po.ringU >= 0 || po.flying {
			t.Errorf("still, at %v the ball is %+v, not round at (%v, %v)", c.at, po, c.x, c.y)
		}
	}

	if po := (&ballPlan{}).pose(0, false); po.ok {
		t.Error("a line without words has a ball")
	}
	// Words that the layout found fewer spots for use those it has.
	few := pl
	few.spots = few.spots[:1]
	if po := few.pose(2*time.Second, false); !po.ok {
		t.Error("fewer spots than words leave no ball")
	}
}

// TestBallFlight sets the ball out for the next line in time to take its
// motion's whole time, squeezes the motion where the line's last word leaves
// little room, and has no flight where there is no room, or no next line.
func TestBallFlight(t *testing.T) {
	ms := func(v int) time.Duration { return time.Duration(v) * time.Millisecond }
	for _, c := range []struct {
		name         string
		motion       settings.LyricBallMotion
		nextAt       int
		begin, end   int
		hasFlight    bool
		withNextLine bool
	}{
		{"a hop with room", settings.LyricBallHop, 7000, 6440, 7000, true, true},
		{"a boomerang with room", settings.LyricBallBoomerang, 7000, 5900, 7000, true, true},
		{"a hop squeezed", settings.LyricBallHop, 4400, 4150, 4400, true, true},
		{"no room for a hop", settings.LyricBallHop, 4300, 0, 0, false, true},
		{"no next line", settings.LyricBallHop, 7000, 0, 0, false, false},
	} {
		pl := testBallPlan(c.motion)
		pl.next.at, pl.hasNext = ms(c.nextAt), c.withNextLine
		begin, end, ok := pl.flight()
		if ok != c.hasFlight || ok && (begin != ms(c.begin) || end != ms(c.end)) {
			t.Errorf("%s: the flight is %v to %v, %v; want %v to %v, %v", c.name, begin, end, ok, ms(c.begin), ms(c.end), c.hasFlight)
		}
	}
}

// TestBallWake wakes the line for the ball: when it moves next, at once while
// it does, and never when it moves no more.
func TestBallWake(t *testing.T) {
	ms := func(v int) time.Duration { return time.Duration(v) * time.Millisecond }
	pl := testBallPlan(settings.LyricBallHop)
	for _, c := range []struct {
		name string
		at   int
		want int // -1: never
	}{
		{"before the ball comes", 0, 700},
		{"as the ball comes", 800, 0},
		{"while its first landing rings", 1300, 0},
		{"in the long gap, until the hop to the word after it", 3000, 540},
		{"on the last word, until the flight", 5400, 1040},
		{"in the flight", 6700, 0},
		{"after the landing rings", 7500, -1},
	} {
		want := time.Duration(-1)
		if c.want >= 0 {
			want = ms(c.want)
		}
		if got := pl.wake(ms(c.at)); got != want {
			t.Errorf("%s: the ball wakes in %v, not %v", c.name, got, want)
		}
	}
	pl.hasNext = false
	if got := pl.wake(ms(5400)); got != -1 {
		t.Errorf("with no next line the ball wakes in %v on its last word", got)
	}
}

// TestBallNextStart wakes the ball that doesn't hop (Reduce Motion) as a word
// starts, and never after the last.
func TestBallNextStart(t *testing.T) {
	ms := func(v int) time.Duration { return time.Duration(v) * time.Millisecond }
	pl := testBallPlan(settings.LyricBallHop)
	for _, c := range []struct{ at, want int }{{0, 1000}, {1000, 500}, {1200, 300}, {1500, 2500}, {3999, 1}, {4000, -1}, {9000, -1}} {
		want := time.Duration(-1)
		if c.want >= 0 {
			want = ms(c.want)
		}
		if got := pl.nextStart(ms(c.at)); got != want {
			t.Errorf("at %d ms the next word starts in %v, not %v", c.at, got, want)
		}
	}
	// The rig has the overlay ask for a frame as a word starts, though the ball
	// doesn't move between: the line asks then too, but the frame that only
	// paints again runs what asked.
	var rig ballRig
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := songClock{pos: ms(1200), seen: now, speed: 1}
	rig.publish(now, &pl, clock, ballLook{ink: stageInk, radius: 5}, true)
	if got := rig.wake(ms(1200), true); got != ms(300) {
		t.Errorf("a ball that doesn't move wakes in %v, not as the next word starts, in 300 ms", got)
	}
	if got := rig.wake(ms(1200), false); got == ms(300) {
		t.Errorf("a ball that hops wakes as the next word starts: %v", got)
	}
}

// TestBallStops has the ball go by time over words that are not in the order
// they are sung: a second voice's words come after the main one's, with times
// that start before them, or with them.
func TestBallStops(t *testing.T) {
	spots := []ballPoint{{10, 0}, {20, 0}, {30, 0}, {40, 0}, {50, 0}}
	for _, c := range []struct {
		name  string
		words []lyrics.Word
		want  []string
	}{
		{"in order", []lyrics.Word{word("a", 1000, 1500), word("b", 1500, 2000), word("c", 2000, 2500)}, []string{"a", "b", "c"}},
		{"a second voice between", []lyrics.Word{word("a", 1000, 1500), word("b", 2000, 2500), word("(oh)", 1500, 2000)}, []string{"a", "(oh)", "b"}},
		{"a second voice first", []lyrics.Word{word("a", 2000, 2500), word("b", 3000, 3500), word("(oh)", 1000, 1500)}, []string{"(oh)", "a", "b"}},
		{"starting together", []lyrics.Word{word("a", 1000, 1500), word("b", 2000, 2500), word("(oh)", 1000, 2000)}, []string{"a", "b"}},
		{"one word", []lyrics.Word{word("a", 1000, 1500)}, []string{"a"}},
		{"none", nil, nil},
	} {
		stops, at := ballStops(c.words, spots)
		var got []string
		for _, w := range stops {
			got = append(got, w.Text)
		}
		if !slices.Equal(got, c.want) || len(at) != len(stops) {
			t.Errorf("%s: the stops are %q at %d spots, not %q", c.name, got, len(at), c.want)
			continue
		}
		for i := 1; i < len(stops); i++ {
			if stops[i].StartMs <= stops[i-1].StartMs {
				t.Errorf("%s: the stops start at %d, then %d", c.name, stops[i-1].StartMs, stops[i].StartMs)
			}
		}
		// Each stop keeps the spot of its word.
		for i, w := range stops {
			for j, o := range c.words {
				if o.Text == w.Text && at[i] != spots[j] {
					t.Errorf("%s: the stop %q lies at %+v, and its word at %+v", c.name, w.Text, at[i], spots[j])
				}
			}
		}
	}
	// Words in order are the stops, not copies of them: the ball costs a line
	// nothing in the usual case.
	words := []lyrics.Word{word("a", 1000, 1500), word("b", 1500, 2000)}
	if stops, _ := ballStops(words, spots); &stops[0] != &words[0] {
		t.Error("words in order are copied")
	}
}

// TestBallRigKeepsAShortFlight keeps the ball on the last word of a line until
// a flight too short to have begun when the next line is sung (it is sung up
// to lyricsLead before its first word) has set out, and has it fly then: the
// ball isn't taken to the next line before it.
func TestBallRigKeepsAShortFlight(t *testing.T) {
	ms := func(v int) time.Duration { return time.Duration(v) * time.Millisecond }
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	look := ballLook{ink: stageInk, radius: 5}
	before := testBallPlan(settings.LyricBallHop)
	before.next.at = ms(4390) // room: 240 ms, so a hop of 4150 to 4390
	if b, e, ok := before.flight(); !ok || b != ms(4150) || e != ms(4390) {
		t.Fatalf("the flight is %v to %v, %v", b, e, ok)
	}
	var rig ballRig
	clock := songClock{pos: ms(4100), seen: now, speed: 1}
	rig.publish(now, &before, clock, look, false)
	after := testBallPlan(settings.LyricBallHop)
	after.line, after.hasNext = 3, false
	after.words = []lyrics.Word{word("Lanterns", 4390, 4900), word("home", 4900, 5400)}
	after.spots = []ballPoint{{20, 250}, {120, 250}}
	// The next line is sung at 4145, before the flight sets out.
	clock.pos = ms(4145)
	rig.publish(now, &after, clock, look, false)
	field := ui.Rect{W: 600, H: 700}
	for _, c := range []struct {
		at      int
		onPrev  bool
		flying  bool
		x, y    float32
		isRing  bool
		comment string
	}{
		{4145, true, false, 200, 95, true, "on the last word, still"},
		{4250, true, true, 0, 0, true, "set out, the ring of the last word still spreading"},
		{4389, true, true, 0, 0, false, "nearly there"},
		{4400, false, false, 20, 245, false, "landed on the first word of the next line"},
	} {
		rig.clock = songClock{pos: ms(c.at), seen: now, speed: 1}
		pl, _, po, _, ok := rig.current(now, field)
		if !ok || (pl == &rig.prev) != c.onPrev || po.flying != c.flying {
			t.Errorf("at %d ms, %s: the plan is the line before's: %v; flying: %v; ok: %v", c.at, c.comment, pl == &rig.prev, po.flying, ok)
			continue
		}
		if c.x != 0 && (!ballNear(po.x, c.x) || !ballNear(po.y, c.y)) {
			t.Errorf("at %d ms, %s: the ball is at (%v, %v), not (%v, %v)", c.at, c.comment, po.x, po.y, c.x, c.y)
		}
		if c.isRing && po.ringU < 0 {
			t.Errorf("at %d ms, %s: the ring is gone", c.at, c.comment)
		}
	}
}

// TestBallLegs lands every motion on the spot it is made for, with no jump on
// the way, keeping the ball near the lyrics.
func TestBallLegs(t *testing.T) {
	const size, radius = 50, 5
	field := ui.Rect{X: 100, Y: 50, W: 700, H: 600}
	for _, c := range []struct {
		name string
		a, b ballPoint
	}{
		{"to the line below, to the left", ballPoint{600, 300}, ballPoint{140, 390}},
		{"to the line below, to the right", ballPoint{200, 300}, ballPoint{500, 390}},
		{"to a line straight below", ballPoint{300, 300}, ballPoint{300, 380}},
		{"from the field's right edge", ballPoint{780, 300}, ballPoint{130, 400}},
	} {
		for _, m := range ballMotions[1:] {
			path := m.legs(c.a, c.b, ballGeo{size: size, radius: radius, field: field})
			start, _ := path.at(c.a, 0, time.Second)
			end, _ := path.at(c.a, 1, time.Second)
			if !ballNear(start.x, c.a.x) || !ballNear(start.y, c.a.y) {
				t.Errorf("%s, %s: it starts at %+v, not %+v", m.name, c.name, start, c.a)
			}
			if !ballNear(end.x, c.b.x) || !ballNear(end.y, c.b.y) {
				t.Errorf("%s, %s: it ends at %+v, not %+v", m.name, c.name, end, c.b)
			}
			prev := start
			for i := 1; i <= 400; i++ {
				p, _ := path.at(c.a, float32(i)/400, time.Second)
				if math.IsNaN(float64(p.x)) || math.IsNaN(float64(p.y)) {
					t.Fatalf("%s, %s: a NaN at %d/400", m.name, c.name, i)
				}
				if d := math.Hypot(float64(p.x-prev.x), float64(p.y-prev.y)); d > 0.3*size {
					t.Errorf("%s, %s: it jumps %.1f at %d/400", m.name, c.name, d, i)
				}
				if p.x < field.X-4*size || p.x > field.X+field.W+4*size || p.y < field.Y-4*size || p.y > field.Y+field.H+size {
					t.Errorf("%s, %s: it strays to %+v at %d/400", m.name, c.name, p, i)
				}
				prev = p
			}
		}
	}

	// A boomerang goes out past the line's end; a ricochet down to the floor.
	a, b := ballPoint{300, 300}, ballPoint{140, 390}
	geo := ballGeo{size: size, radius: radius, field: field}
	boom, ricochet := ballMotionOf(settings.LyricBallBoomerang).legs(a, b, geo), ballMotionOf(settings.LyricBallRicochet).legs(a, b, geo)
	var farthest, lowest float32
	for i := 0; i <= 200; i++ {
		p, _ := boom.at(a, float32(i)/200, time.Second)
		farthest = max(farthest, p.x)
		q, _ := ricochet.at(a, float32(i)/200, time.Second)
		lowest = max(lowest, q.y)
	}
	if farthest < a.x+size {
		t.Errorf("a boomerang goes out only to %v, from %v", farthest, a.x)
	}
	if lowest < field.Y+field.H-0.2*field.H {
		t.Errorf("a ricochet falls only to %v, of a floor at %v", lowest, field.Y+field.H)
	}
	// A ricochet bounces: it squashes the ball after its first two legs.
	if _, since := ricochet.at(a, 0.3, time.Second); since < 0 {
		t.Error("the ricochet doesn't squash the ball as it bounces off the edge")
	}
}

// TestResolveBallMotion gives the motion chosen where it has room, a hop
// where it hasn't, a different motion for each line of the mix, and none
// where there is no room at all.
func TestResolveBallMotion(t *testing.T) {
	ms := func(v int) time.Duration { return time.Duration(v) * time.Millisecond }
	if got := resolveBallMotion(settings.LyricBallBoomerang, 0, ms(2000)); got.id != settings.LyricBallBoomerang {
		t.Errorf("a boomerang with room is a %s", got.id)
	}
	if got := resolveBallMotion(settings.LyricBallBoomerang, 0, ms(400)); got.id != settings.LyricBallHop {
		t.Errorf("a boomerang without room is a %s, not a hop", got.id)
	}
	if got := resolveBallMotion(settings.LyricBallHop, 0, ms(100)); got != nil {
		t.Errorf("there is a %s in 100 ms", got.id)
	}
	for _, id := range settings.LyricBallMotions {
		if got := resolveBallMotion(id, 3, ms(50)); got != nil {
			t.Errorf("%s: there is a %s in 50 ms", id, got.id)
		}
	}
	// The mix goes through the motions in turn, and each line has its own.
	want := []settings.LyricBallMotion{settings.LyricBallHop, settings.LyricBallBoomerang, settings.LyricBallLoop, settings.LyricBallDive, settings.LyricBallRicochet, settings.LyricBallHop}
	for line, id := range want {
		if got := resolveBallMotion(settings.LyricBallMix, line, ms(2000)); got.id != id {
			t.Errorf("line %d of the mix has a %s, not a %s", line, got.id, id)
		}
	}
	// The turn of a line doesn't change with the room of the lines before it, and
	// one that doesn't fit the pause gives it to the next in turn that does.
	for _, c := range []struct {
		line int
		room int
		want settings.LyricBallMotion
	}{
		{2, 2000, settings.LyricBallLoop}, {2, 700, settings.LyricBallLoop}, {2, 650, settings.LyricBallDive},
		{4, 2000, settings.LyricBallRicochet}, {4, 700, settings.LyricBallHop}, {-3, 2000, settings.LyricBallHop},
	} {
		if got := resolveBallMotion(settings.LyricBallMix, c.line, ms(c.room)); got == nil || got.id != c.want {
			t.Errorf("line %d of the mix in %d ms has %v, not a %s", c.line, c.room, got, c.want)
		}
	}
	// In little room, only those that fit are chosen from.
	seen := map[settings.LyricBallMotion]bool{}
	for line := range 12 {
		got := resolveBallMotion(settings.LyricBallMix, line, ms(700))
		if got == nil || got.least > ms(700) || got.legs == nil {
			t.Fatalf("line %d of the mix in 700 ms has %v", line, got)
		}
		seen[got.id] = true
	}
	if seen[settings.LyricBallRicochet] || len(seen) < 3 {
		t.Errorf("the mix in 700 ms goes through %v", seen)
	}
}

// TestBallMotionsTable keeps the motions as the settings list them, with a
// name and what each does, and the times of each in order.
func TestBallMotionsTable(t *testing.T) {
	if len(ballMotions) != len(settings.LyricBallMotions) {
		t.Fatalf("%d motions, and the settings list %d", len(ballMotions), len(settings.LyricBallMotions))
	}
	names := map[string]bool{}
	for i, m := range ballMotions {
		if m.id != settings.LyricBallMotions[i] {
			t.Errorf("motion %d is %q, and the settings list %q", i, m.id, settings.LyricBallMotions[i])
		}
		if m.name == "" || m.about == "" || names[m.name] {
			t.Errorf("motion %q has the name %q and says %q", m.id, m.name, m.about)
		}
		names[m.name] = true
		if (m.legs == nil) != (m.id == settings.LyricBallMix) {
			t.Errorf("motion %q has legs: %v", m.id, m.legs != nil)
		}
		if m.id != settings.LyricBallMix && (m.least <= 0 || m.least > m.takes) {
			t.Errorf("motion %q takes %v, at least %v", m.id, m.takes, m.least)
		}
	}
}

// TestGlyphRise has the ball touch the tops of the letters the word has: its
// capitals and tall letters, or, where it has none, the x-height.
func TestGlyphRise(t *testing.T) {
	for _, c := range []struct {
		word string
		want float32
	}{
		{"over", riseX}, {"now", riseX}, {"a", riseX}, {"sun", riseX}, {"gypsy", riseX},
		{"Over", riseTall}, {"wind", riseTall}, {"the", riseTall}, {"1", riseTall}, {"café", riseTall}, {"don't", riseTall},
		{"日本", riseWide},
		// Beyond ASCII, a letter is as tall as a capital, or has a mark over it, or
		// is an ascender; the others stand as high as an x.
		{"öu", riseTall}, {"ñu", riseTall}, {"ça", riseX}, {"preço", riseX}, {"Привет", riseTall}, {"сердце", riseX}, {"шла", riseX},
		{"йод", riseTall}, {"και", riseX}, {"άνα", riseTall}, {"σε", riseX}, {"שלום", riseTall}, {"מם", riseTall}, {"नमस्ते", riseWide}, {"สวัสดี", riseWide}, {"نم", riseX}, {"ال", riseTall},
	} {
		if got := glyphRise(c.word); got != c.want {
			t.Errorf("%q rises %v, not %v", c.word, got, c.want)
		}
	}
}

// TestBallSpots finds where the ball touches each word of a laid-out line: in
// order along a row, at the top of its letters over the row's baseline, and,
// where the line wraps, the later words on lower rows.
func TestBallSpots(t *testing.T) {
	newTestShell(t) // registers the faces
	text := "Slow wind over the pier"
	words := []lyrics.Word{word("Slow", 0, 1), word("wind", 1, 2), word("over", 2, 3), word("the", 3, 4), word("pier", 4, 5)}
	var dir ui.Direction
	font := ui.Font{Family: face, Size: 40, Weight: 400}

	wide := shapeSung(sungKey{text, font, 2000, dir}, words)
	spots := wide.spots(nil)
	if len(spots) != len(words) {
		t.Fatalf("%d spots for %d words", len(spots), len(words))
	}
	base := wide.rows[0].baseline
	for i, sp := range spots {
		want := base - glyphRise(words[i].Text)*font.Size
		if sp.x < 0 || !ballNear(sp.y, want) || i > 0 && sp.x <= spots[i-1].x {
			t.Errorf("in a wide line the word %d lies at %+v, after %+v; its letters' top is at y %v", i, sp, spots[max(i-1, 0)], want)
		}
	}
	if spots[2].y <= spots[1].y {
		t.Errorf("a word of low letters touches as high as one of tall: %v and %v", spots[2].y, spots[1].y)
	}
	if base <= 0 || base > 2*font.Size {
		t.Errorf("the baseline is at %v, for a font 40 high", base)
	}

	narrow := shapeSung(sungKey{text, font, 150, dir}, words).spots(nil)
	rows := map[float32]bool{}
	for i, sp := range narrow {
		rows[float32(math.Round(float64(sp.y/10)))] = true
		if sp.x < 0 || i > 0 && sp.y < narrow[i-1].y-0.8*font.Size {
			t.Errorf("in a narrow line the word %d lies at %+v, after %+v", i, sp, narrow[i-1])
		}
		if i > 0 && sp.y == narrow[i-1].y && sp.x <= narrow[i-1].x {
			t.Errorf("in a narrow line the word %d lies at %+v, not after %+v on its row", i, sp, narrow[i-1])
		}
	}
	if len(rows) < 3 {
		t.Errorf("a narrow line stays on %d heights", len(rows))
	}
}

// TestBallRigKeepsTheFlight gives the ball to the line sung next only when
// the flight from the line before is done, and not before: the next line is
// sung before the ball lands on it. It moves the flight by how far the lyrics
// glided, to land where the line now is, and shows no ball of a line that
// stopped painting.
func TestBallRigKeepsTheFlight(t *testing.T) {
	ms := func(v int) time.Duration { return time.Duration(v) * time.Millisecond }
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	look := ballLook{ink: stageInk, radius: 5}
	var rig ballRig
	clock := songClock{pos: ms(6800), seen: now, speed: 1, moving: false}
	before := testBallPlan(settings.LyricBallHop)
	rig.publish(now, &before, clock, look, false)

	// The song goes on to line 3, whose first word lies a little higher than the
	// flight meant: the lyrics glided.
	after := testBallPlan(settings.LyricBallHop)
	after.line = 3
	after.words = []lyrics.Word{word("Lanterns", 7000, 7500), word("home", 7500, 8000)}
	after.spots = []ballPoint{{20, 250}, {120, 250}}
	after.hasNext = false
	rig.publish(now, &after, clock, look, false)
	if !rig.hasPrev || rig.prev.line != 2 || rig.cur.line != 3 {
		t.Fatalf("the rig holds lines %d and %d, with the line before kept: %v", rig.prev.line, rig.cur.line, rig.hasPrev)
	}

	field := ui.Rect{W: 600, H: 700}
	for _, c := range []struct {
		at      int
		inbound bool
	}{{6000, true}, {6440, true}, {6900, true}, {7000, false}, {7400, false}} {
		clock.pos = ms(c.at)
		rig.clock = clock
		if got := rig.inbound(ms(c.at)); got != c.inbound {
			t.Errorf("at %d ms the ball is on its way in from the line before: %v, not %v", c.at, got, c.inbound)
		}
	}

	// At the middle of the flight the ball is where the line before flew it,
	// less the 10 DIPs by which the next line's first word lies higher.
	rig.has, rig.hasPrev = true, true
	rig.clock = songClock{pos: ms(6720), seen: now, speed: 1}
	pl, _, po, shift, ok := rig.current(now, field)
	if !ok || pl != &rig.prev || !po.flying {
		t.Fatalf("the ball doesn't follow the flight: %v, %+v", ok, po)
	}
	if !ballNear(shift.x, 0) || !ballNear(shift.y, -10) {
		t.Errorf("the flight is moved by %+v, not (0, -10)", shift)
	}
	if !rig.hasPrev {
		t.Error("the flight was let go of while it goes on")
	}

	// Once it has landed, the line sung has the ball, and the flight is let go.
	rig.clock = songClock{pos: ms(7100), seen: now, speed: 1}
	pl, _, po, _, ok = rig.current(now, field)
	if !ok || pl != &rig.cur || rig.hasPrev || po.flying {
		t.Errorf("after the landing the ball isn't the line sung's: %v, %+v", ok, po)
	}

	// A line that doesn't paint for a moment shows no ball.
	if _, _, _, _, ok := rig.current(now.Add(ballStale+time.Millisecond), field); ok {
		t.Error("a line that stopped painting still has a ball")
	}
	if _, _, _, _, ok := (&ballRig{}).current(now, field); ok {
		t.Error("a rig no line gave a plan has a ball")
	}

	// A line that isn't the next one's keeps nothing: a seek.
	rig = ballRig{}
	far := testBallPlan(settings.LyricBallHop)
	rig.publish(now, &far, clock, look, false)
	far.line = 9
	rig.publish(now, &far, clock, look, false)
	rig.clock = songClock{pos: ms(6720), seen: now, speed: 1}
	if rig.inbound(ms(6720)) {
		t.Error("a line far from the one before has its flight")
	}
}
