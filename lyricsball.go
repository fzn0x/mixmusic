package main

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/lyrics"
)

// The ball of the full-screen lyrics hops from word to word over the line
// sung, as its words are sung, for lyrics timed word by word. It rests on the
// letters of a word until the last ballHop before the next one starts, then
// hops to it, landing as it starts, with a squash and a ring that spreads
// over the word, so that in a long gap between two words it waits rather than
// floats. From the last word of a line it goes on to the first of the next
// in a motion of its own (lyricsballmotion.go), landing on it as it starts.
// While the computer asks for less motion it doesn't hop: it sits on the word
// being sung.
//
// Where the ball is, is a function of the song's time and of the plan of the
// line it is on (ballPlan): where its words lie, and where and when the
// ball lands on the next line. So a seek, a pause and a repaint always find it
// where the song puts it, and the trail behind it costs nothing: it is the
// ball's poses a moment earlier. The ball is not painted by the line it is
// on, but by one overlay over the lyrics (lyricsballrig.go), so that the line's
// blur and fade as it takes the ball over, and the clip of the list, don't
// touch it.

const (
	// ballHop is the longest a hop of the ball from a word to the next takes.
	ballHop = 460 * time.Millisecond
	// ballSettle is how long a landing squashes the ball, and ballRing how
	// long the ring it leaves spreads.
	ballSettle = 190 * time.Millisecond
	ballRing   = 460 * time.Millisecond
	// ballPopLead is how long before the first word of a line, which no flight
	// brought the ball to, the ball appears, and ballPop how long it takes to
	// grow.
	ballPopLead = 300 * time.Millisecond
	ballPop     = 240 * time.Millisecond
	// ballRest is the least the ball stays on the last word of a line before it
	// sets out for the next.
	ballRest = 150 * time.Millisecond
	// ballStale is how old what a line gave the ball's rig may be, before the
	// line is taken for one that no longer shows.
	ballStale = 150 * time.Millisecond
)

// ballLook is how the ball looks: its ink and its radius, in DIPs.
type ballLook struct {
	ink    ui.Color
	radius float32
}

// ballPoint is a point, in DIPs.
type ballPoint struct{ x, y float32 }

// ballArrival is where, and when, the ball lands on the line after the one it
// is on: at the song's time at, on the spot of the first word.
type ballArrival struct {
	at   time.Duration
	spot ballPoint
}

// ballPlan is what a line sung tells the ball of itself: its words, the spots
// they lie at, and where and when the ball goes on to the next line. All
// points are the window's.
type ballPlan struct {
	// line is the line's place in the lyrics.
	line  int
	words []lyrics.Word
	// spots are where the ball touches each word: the middle of its first part,
	// and the top of its letters.
	spots []ballPoint
	// next is where the ball lands on the next line, and hasNext whether it
	// does: it doesn't where that line has no words timed, or there is no room
	// for it to go.
	next    ballArrival
	hasNext bool
	// motion is the motion it goes in, never the one that mixes them.
	motion *ballMotion
	// size is the text's, and radius the ball's.
	size, radius float32
	// field is where the ball may roam: the lyrics' own. The rig sets it.
	field ui.Rect
}

// ballPose is how the ball looks at a time: where it is, how squashed, how
// grown, and the ring a landing spreads.
type ballPose struct {
	// x, y are the middle of the ball when it is round.
	x, y float32
	// sx and sy are how wide and how tall it is, against round.
	sx, sy float32
	// scale is how grown it is: 0 when it isn't there yet.
	scale float32
	// ring is where the ring of a landing is, how far it has spread (0 to 1,
	// or -1 when there is none), and whether it is the ring of a line's first
	// word.
	ring    ballPoint
	ringU   float32
	ringBig bool
	// flying is set while the ball is on its way to the next line.
	flying bool
	ok     bool
}

// hopLen is how long the hop from word k to the next takes.
func (pl *ballPlan) hopLen(k int) time.Duration {
	return min(ballHop, ms(pl.words[k+1].StartMs)-ms(pl.words[k].StartMs))
}

// flight is when the ball sets out for the next line, and when it lands
// there: as the first word of that line starts. It sets out in time to take
// the motion's full time, but not before it has been on its last word for
// ballRest, so that where lines follow each other fast the motion is squeezed.
// ok is false where there is no flight.
func (pl *ballPlan) flight() (begin, end time.Duration, ok bool) {
	n := len(pl.words)
	if !pl.hasNext || n == 0 || pl.motion == nil {
		return 0, 0, false
	}
	end = pl.next.at
	begin = max(end-pl.motion.takes, ms(pl.words[n-1].StartMs)+ballRest)
	return begin, end, end-begin >= pl.motion.least
}

// pose returns how the ball looks at the song's time at. Where still is set,
// the ball doesn't hop or fly, squash or spread rings: it sits on the word
// being sung, and on the first of the next line as it starts.
func (pl *ballPlan) pose(at time.Duration, still bool) ballPose {
	n := min(len(pl.words), len(pl.spots))
	if n == 0 {
		return ballPose{}
	}
	r := pl.radius
	// The ball rests on a spot with its bottom on it.
	rest := func(k int) ballPoint { return ballPoint{pl.spots[k].x, pl.spots[k].y - r} }
	start := func(k int) time.Duration { return ms(pl.words[k].StartMs) }
	po := ballPose{ok: true, sx: 1, sy: 1, scale: 1, ringU: -1}
	// The word it sits on is the last begun.
	k := -1
	for k+1 < n && start(k+1) <= at {
		k++
	}
	if k < 0 {
		c := rest(0)
		po.x, po.y = c.x, c.y
		if !still {
			po.scale = ballGrow(at - (start(0) - ballPopLead))
		}
		return po
	}
	c := rest(k)
	po.x, po.y = c.x, c.y
	if still {
		return po
	}
	po.landed(at-start(k), pl.spots[k], k == 0, true)
	if k+1 < n {
		next, hop := start(k+1), pl.hopLen(k)
		if hop > 0 && at >= next-hop {
			u := ballUnit(float32(at-(next-hop)) / float32(hop))
			to := rest(k + 1)
			// The arc rises with how far the next word lies.
			reach := min(max(0.2*ballAbs(to.x-c.x), 0.35*pl.size), 1.1*pl.size)
			po.x, po.y = lerp(c.x, to.x, u), lerp(c.y, to.y, u)-reach*4*u*(1-u)
			po.stretch(u)
		}
		return po
	}
	begin, end, ok := pl.flight()
	if !ok || at < begin {
		return po
	}
	b := ballPoint{pl.next.spot.x, pl.next.spot.y - r}
	// The ring of the last word's landing goes on spreading as the ball leaves.
	ring, ringU, ringBig := po.ring, po.ringU, po.ringBig
	po = ballPose{ok: true, sx: 1, sy: 1, scale: 1, ringU: -1}
	if at >= end {
		// Landed, where the next line's first word is.
		po.x, po.y = b.x, b.y
		po.landed(at-end, pl.next.spot, true, true)
		return po
	}
	po.ring, po.ringU, po.ringBig = ring, ringU, ringBig
	dur := end - begin
	u := float32(at-begin) / float32(dur)
	g := ballGeo{size: pl.size, radius: r, field: pl.field}
	path := pl.motion.legs(c, b, g)
	p, since := path.at(c, u, dur)
	po.x, po.y, po.flying = p.x, p.y, true
	po.stretch(u)
	if since >= 0 {
		// It bounced off an edge a moment ago.
		po.landed(since, p, false, false)
	}
	return po
}

// landed squashes the pose as the ball landed since ago: hard at first, then
// back to round, with a ring over the spot of the word if ring is set; more
// for the first word of a line, big.
func (po *ballPose) landed(since time.Duration, spot ballPoint, big, ring bool) {
	if since < 0 {
		return
	}
	if since < ballSettle {
		t := float32(since) / float32(ballSettle)
		amount := float32(0.26)
		if big {
			amount = 0.36
		}
		s := amount * (1 - t) * (1 - t)
		po.sy *= 1 - s
		po.sx *= 1 + 0.7*s
	}
	if ring && since < ballRing {
		po.ring, po.ringU, po.ringBig = spot, float32(since)/float32(ballRing), big
	}
}

// stretch lengthens the pose along its fall as it flies, u of the way.
func (po *ballPose) stretch(u float32) {
	s := float32(math.Sin(math.Pi * float64(u)))
	po.sy *= 1 + 0.14*s
	po.sx *= 1 - 0.07*s
}

// ballGrow is how grown the ball is, since ago of its coming: it grows past
// its size, and settles.
func ballGrow(since time.Duration) float32 {
	if since <= 0 {
		return 0
	}
	if since >= ballPop {
		return 1
	}
	t := float32(since)/float32(ballPop) - 1
	const c = 1.70158
	return 1 + (c+1)*t*t*t + c*t*t
}

// wake is how long of the song until the ball moves: 0 while it does, which
// takes in a landing's squash and ring, and -1 when it doesn't again. It
// moves when a word is about to start, as the ball comes to it, for the first
// word or by a hop, and as it flies to the next line.
func (pl *ballPlan) wake(at time.Duration) time.Duration {
	n := min(len(pl.words), len(pl.spots))
	if n == 0 {
		return -1
	}
	next := time.Duration(-1)
	// moves reports whether the ball moves in the stretch of the song from
	// from to to, and notes when the next one starts.
	moves := func(from, to time.Duration) bool {
		if at >= from && at < to {
			return true
		}
		if from > at && (next < 0 || from-at < next) {
			next = from - at
		}
		return false
	}
	first := ms(pl.words[0].StartMs)
	if moves(first-ballPopLead, first+ballRing) {
		return 0
	}
	for k := 0; k+1 < n; k++ {
		start := ms(pl.words[k+1].StartMs)
		if moves(start-max(pl.hopLen(k), 0), start+ballRing) {
			return 0
		}
	}
	if begin, end, ok := pl.flight(); ok && moves(begin, end+ballRing) {
		return 0
	}
	return next
}

// nextStart is how long of the song until the next word starts, -1 when none
// does: where the ball doesn't hop (still), it moves as a word starts.
func (pl *ballPlan) nextStart(at time.Duration) time.Duration {
	next := time.Duration(-1)
	for _, w := range pl.words {
		if s := ms(w.StartMs); s > at && (next < 0 || s-at < next) {
			next = s - at
		}
	}
	return next
}

// ballStops are the stops the ball makes over a line, with the spots they
// lie at, in the order the song sings them, each starting later than the one
// before: lyrics with a second voice list its words after those of the main
// one, with times that may come before, or with, the words before them. The
// ball goes by time, from one stop to the next; a word that starts together
// with an earlier one shares its stop. Where the words are in order already,
// as most are, they are the stops.
func ballStops(words []lyrics.Word, spots []ballPoint) ([]lyrics.Word, []ballPoint) {
	n := min(len(words), len(spots))
	ordered := true
	for i := 1; i < n; i++ {
		if words[i].StartMs <= words[i-1].StartMs {
			ordered = false
			break
		}
	}
	if ordered {
		return words[:n], spots[:n]
	}
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	slices.SortStableFunc(idx, func(a, b int) int { return cmp.Compare(words[a].StartMs, words[b].StartMs) })
	stops, at := make([]lyrics.Word, 0, n), make([]ballPoint, 0, n)
	for _, i := range idx {
		if len(stops) > 0 && words[i].StartMs <= stops[len(stops)-1].StartMs {
			continue
		}
		stops, at = append(stops, words[i]), append(at, spots[i])
	}
	return stops, at
}

// spots returns where the ball touches each word of the layout, in buf: the
// middle of the first row it is on, at the top of its letters, which stand
// over the row's baseline as high as the tallest of them. A word the layout
// couldn't find lies where the one before it does.
func (g *sungLayout) spots(buf []ballPoint) []ballPoint {
	buf = buf[:0]
	for range g.width {
		buf = append(buf, ballPoint{x: -1, y: -1})
	}
	size := g.key.font.Size
	for _, row := range g.rows {
		for _, cell := range row.cells {
			if buf[cell.word].x < 0 {
				buf[cell.word] = ballPoint{x: (cell.x0 + cell.x1) / 2, y: row.baseline - g.rise[cell.word]*size}
			}
		}
	}
	for i := range buf {
		if buf[i].x < 0 && i > 0 {
			buf[i] = buf[i-1]
		}
	}
	return buf
}

// How high letters stand over the baseline, against the size of the font:
// the capitals and the tall letters, the x-height of the others, and the
// ideographs, which fill their square. Faces differ by a little, which a
// ball that touches the letters only squashes a little differently for.
const (
	riseTall = 0.72
	riseX    = 0.52
	riseWide = 0.84
)

// tallLetters are the lowercase letters, beyond ASCII's, that stand as high as
// a capital: those with a mark over them, and the ascenders, of the Latin,
// Greek, Cyrillic and Arabic scripts. The other lowercase letters,
// as the Cyrillic "сердце" or the Greek "και", stand as high as an x.
const tallLetters = "àáâãäåāăćĉċčďéèêëēĕėěĝğġĥíìîïĩīĭįĵĺľłńňñòóôõöōŏőŕřśŝšťùúûüũūŭůűýÿźżžßðþħ" +
	"άέήίόύώΐΰϊϋβδζθλξφψ" +
	"бйёђћфљњїќў" +
	"اأإآلكطظ"

// glyphRise is how high the tallest letter of a word stands over its
// baseline, against the size of the font.
func glyphRise(word string) float32 {
	rise := float32(riseX)
	for _, r := range word {
		switch {
		case r >= 0x2E80, r >= 0x0900 && r <= 0x0A7F, r >= 0x0E00 && r <= 0x0EFF:
			// Ideographs fill their square; the letters of Devanagari, Bengali and
			// Gurmukhi hang from a line at their top, and Thai stands tall.
			return riseWide
		case r >= 0x0590 && r <= 0x05FF:
			// Hebrew letters stand about as high as a capital.
			rise = riseTall
		case r < 0x80:
			if unicode.IsUpper(r) || unicode.IsDigit(r) || strings.ContainsRune("bdfhijklt'\"!?", r) {
				rise = riseTall
			}
		case unicode.IsUpper(r), strings.ContainsRune(tallLetters, r):
			rise = riseTall
		}
	}
	return rise
}

func ballUnit(v float32) float32 { return min(max(v, 0), 1) }

func ballAbs(v float32) float32 { return float32(math.Abs(float64(v))) }
