package main

import (
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// The line sung doesn't paint the ball: it tells it, as it paints itself,
// where its words lie and where the next line's first word is, to a rig, and
// one overlay over the lyrics paints the ball from the rig. A line that took
// the ball over would blur and fade it while it landed, as it does itself,
// and the list of lines would clip it where it flew past them.
//
// The rig keeps the plan of the line before, too, for as long as its flight
// to the next line lasts: the next line is sung 250 ms ahead of its time (see
// lyricsLead), which is before the ball lands on it, and it would otherwise
// cut the flight short.

// ballFeed is what a line sung gives to the rig, besides its words.
type ballFeed struct {
	rig  *ballRig
	look ballLook
	// line is the line's place in the lyrics, and motion the motion the user
	// chose between lines.
	line   int
	motion settings.LyricBallMotion
	// hasNext is set where the line after has its words timed: the text it
	// shows, and its first word, which the ball goes on to.
	hasNext  bool
	nextText string
	nextWord lyrics.Word
	// col is the column the line stands in, and top how much room it keeps above
	// its text: the next line's first row lies where the column ends, and the
	// room of its own column begins. The column's box is read from the frame
	// before (Element.Bounds), in the window's DIPs: what a line paints is in
	// the coordinates of its own layer for as long as its opacity and blur
	// move, as they do when it becomes the line sung, which is when the ball
	// lands on it.
	col ui.Element
	top float32
}

// ballRig holds the plan of the line sung, and that of the line before it, for
// the overlay to paint the ball from.
type ballRig struct {
	cur, prev ballPlan
	// has is set once a line gave its plan, and hasPrev while prev's flight
	// goes on.
	has, hasPrev bool
	// stamp is when cur was given, which the overlay reads to see that the line
	// is still painting.
	stamp time.Time
	clock songClock
	look  ballLook
	still bool
	// wakeIn is how long of the song until the ball moves, as the line told
	// when it gave its plan: 0 while it does. The overlay asks for frames by it
	// as the line does, since a frame that only paints again runs the drawings
	// that asked for it, and no other.
	wakeIn time.Duration
}

// publish gives the rig the plan of a line, as it is painted at now.
func (r *ballRig) publish(now time.Time, pl *ballPlan, clock songClock, look ballLook, still bool) {
	if r.has && r.cur.line != pl.line {
		// The song went on to another line: the one before keeps its flight.
		r.prev, r.cur = r.cur, r.prev
		r.hasPrev = true
	}
	spots := append(r.cur.spots[:0], pl.spots...)
	r.cur = *pl
	r.cur.spots = spots
	r.has, r.stamp, r.clock, r.look, r.still = true, now, clock, look, still
}

// inbound reports whether the ball still belongs to the line before this one
// at the song's time at: it has a flight to this line that hasn't landed,
// whether it has set out or not. This line is sung up to lyricsLead before its
// first word, so before a short flight has begun the ball is still on the last
// word of the line before, and mustn't be taken from it.
func (r *ballRig) inbound(at time.Duration) bool {
	if !r.hasPrev || r.prev.line+1 != r.cur.line || len(r.cur.spots) == 0 {
		return false
	}
	_, end, ok := r.prev.flight()
	return ok && at < end
}

// wake is how long of the song until the ball moves, as ballPlan.wake says,
// with the flight in from the line before. Where the ball doesn't hop or fly
// (still), it moves as a word starts, and the line, which asks to be painted
// again then, has the overlay ask too.
func (r *ballRig) wake(at time.Duration, still bool) time.Duration {
	switch {
	case !r.has:
		return -1
	case still:
		return r.cur.nextStart(at)
	case r.inbound(at):
		return r.prev.wake(at)
	}
	return r.cur.wake(at)
}

// current returns the plan the ball follows at now, the song's time then, and
// the ball's pose by it, moved by shift: the plan of the line sung, or, while
// the ball is still on its way from the line before, that line's. ok is false
// where there is no ball to show: no line gave a plan lately, or the ball
// isn't there yet.
func (r *ballRig) current(now time.Time, field ui.Rect) (pl *ballPlan, at time.Duration, po ballPose, shift ballPoint, ok bool) {
	if !r.has || now.Sub(r.stamp) > ballStale {
		return nil, 0, ballPose{}, ballPoint{}, false
	}
	at = r.clock.at(now)
	pl = &r.cur
	if r.hasPrev {
		if r.inbound(at) {
			// The flight goes on in the geometry it was made in, moved by how far
			// the lyrics glided since: the line it lands on is where it is now.
			pl = &r.prev
			shift = ballPoint{r.cur.spots[0].x - r.prev.next.spot.x, r.cur.spots[0].y - r.prev.next.spot.y}
		} else {
			r.hasPrev = false
		}
	}
	pl.field = field
	po = pl.pose(at, r.still)
	return pl, at, po, shift, po.ok && po.scale > 0
}

// paint paints the ball over field, the rect of the overlay, which is where
// the ball may roam.
func (r *ballRig) paint(p *ui.Painter, field ui.Rect) {
	pl, at, po, shift, ok := r.current(p.Now(), field)
	if pl == nil {
		// No line is giving the ball a plan: it is out of sight, and asks for
		// no frames.
		return
	}
	// The ball moves as the line paints it again, and no more: a frame that
	// only paints again runs the drawings that asked for it. A ball that
	// isn't there yet asks too, for the moment it comes.
	switch w := r.wakeIn; {
	case !r.clock.moving || w < 0:
	case w == 0:
		p.AnimationFrameRate(60)
	default:
		p.After(r.clock.wall(w))
	}
	if !ok {
		return
	}
	ink, rad := r.look.ink, r.look.radius
	// A trail follows the ball in flight: where it was a moment before.
	if begin, _, ok := pl.flight(); po.flying && ok {
		for j := 1; j <= 6; j++ {
			t := at - time.Duration(j)*30*time.Millisecond
			if t < begin {
				break
			}
			q := pl.pose(t, false)
			f := 1 - float32(j)/7
			rr := rad * (0.35 + 0.5*f)
			p.Fill(ui.Rect{X: q.x + shift.x - rr, Y: q.y + shift.y - rr, W: 2 * rr, H: 2 * rr}, ink.Alpha(0.3*f), rr)
		}
	}
	if po.ringU >= 0 {
		spread := po.ringU
		radius, strength := rad*(1+1.8*spread), float32(0.32)
		if po.ringBig {
			radius, strength = rad*(1+3*spread), 0.5
		}
		fade := (1 - spread) * (1 - spread)
		// The ring lies flat on the word.
		box := ui.Rect{X: po.ring.x + shift.x - radius, Y: po.ring.y + shift.y - radius*0.3, W: 2 * radius, H: 0.6 * radius}
		p.Stroke(box, ink.Alpha(strength*fade), box.H/2, 1.5)
	}
	rad *= po.scale
	w, h := 2*rad*po.sx, 2*rad*po.sy
	// A squashed ball stays on what it lands on: its bottom stays.
	box := ui.Rect{X: po.x + shift.x - w/2, Y: po.y + shift.y + rad - h, W: w, H: h}
	p.Shadow(box, min(w, h)/2, 0, 0, 2*rad, 0, ink.Alpha(0.55))
	p.Fill(box, ink, min(w, h)/2)
}

// ballOverlay lays the overlay over its parent, which paints the ball of the
// rig. It takes no pointer.
func ballOverlay(c *ui.Context, rig *ballRig) {
	ui.Box(c.Key("ball-overlay")).Absolute().Fill().PassThrough().Draw(func(p *ui.Painter, r ui.Rect) { rig.paint(p, r) })
}

// publishBall tells the rig where the words of the line sung lie, with the text
// origin at (x, y), and returns how long of the song until the ball moves,
// as ballPlan.wake says. The next line's first word is laid out here too, as
// that line will lay it out when it is the one sung, and put under the line's
// column.
func (l *sungLine) publishBall(p *ui.Painter, g *sungLayout, x, y float32, font ui.Font, dir ui.Direction, words []lyrics.Word, clock songClock, f *ballFeed, still bool) time.Duration {
	l.spots = g.spots(l.spots)
	for i := range l.spots {
		l.spots[i].x += x
		l.spots[i].y += y
	}
	stops, spots := ballStops(words, l.spots)
	pl := ballPlan{line: f.line, words: stops, spots: spots, size: font.Size, radius: f.look.radius}
	col := f.col.Bounds()
	if f.hasNext && len(stops) > 0 && f.nextWord.Text != "" && col.W > 0 {
		key := sungKey{f.nextText, font, g.key.width, dir}
		if l.nextShaped == nil || l.nextShaped.key != key {
			l.nextShaped = shapeSung(key, []lyrics.Word{f.nextWord})
		}
		l.nextSpots = l.nextShaped.spots(l.nextSpots)
		if len(l.nextSpots) > 0 && l.nextSpots[0].x >= 0 {
			first, at := l.nextSpots[0], ms(f.nextWord.StartMs)
			room := at - (ms(stops[len(stops)-1].StartMs) + ballRest)
			if m := resolveBallMotion(f.motion, f.line, room); m != nil {
				pl.motion, pl.hasNext = m, true
				pl.next = ballArrival{at: at, spot: ballPoint{x + first.x, col.Y + col.H + f.top + first.y}}
			}
		}
	}
	now := p.Now()
	f.rig.publish(now, &pl, clock, f.look, still)
	wake := f.rig.wake(clock.at(now), still)
	f.rig.wakeIn = wake
	return wake
}
