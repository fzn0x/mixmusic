package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// The motions of the ball from the last word of a line to the first of the
// next, which Settings › Appearance › Lyrics style chooses among. Each is a
// few legs from where the ball rests to where it lands, which the song's time
// walks along: an arc, as of a ball thrown, a fall, a loop or a curve. A
// motion takes as long as it takes when the line's words leave room for it,
// and is squeezed down to its least when they leave less; where they leave less
// still, the ball hops. The motion that mixes them is a different one for each
// line, in turn, among those with room.

// ballMotion is a motion of the ball between lines.
type ballMotion struct {
	id          settings.LyricBallMotion
	name, about string
	// takes is how long it lasts when it has room, and least the least it can
	// be squeezed to.
	takes, least time.Duration
	// legs make its legs from the middle of the ball on the last word, a, to
	// its middle on the first of the next line, b.
	legs func(a, b ballPoint, g ballGeo) ballPath
}

// ballGeo is what the legs of a motion are drawn to: the size of the text,
// the radius of the ball and the field it roams in.
type ballGeo struct {
	size, radius float32
	field        ui.Rect
}

// ballMotions are the motions, in the order of settings.LyricBallMotions,
// which a test holds them to.
var ballMotions = []ballMotion{
	{id: settings.LyricBallMix, name: "Mix", about: "A different motion for each line, in turn."},
	{id: settings.LyricBallHop, name: "Hop", about: "One arc, as the hops between words are.",
		takes: 560 * time.Millisecond, least: 200 * time.Millisecond, legs: hopLegs},
	{id: settings.LyricBallBoomerang, name: "Boomerang", about: "Thrown out past the end of the line, it curves back to the next.",
		takes: 1100 * time.Millisecond, least: 600 * time.Millisecond, legs: boomerangLegs},
	{id: settings.LyricBallLoop, name: "Loop", about: "A loop over the last word, then a swoop to the next line.",
		takes: 1200 * time.Millisecond, least: 700 * time.Millisecond, legs: loopLegs},
	{id: settings.LyricBallDive, name: "Dive", about: "It rises a little, as to gather itself, and drops to the next line.",
		takes: 850 * time.Millisecond, least: 320 * time.Millisecond, legs: diveLegs},
	{id: settings.LyricBallRicochet, name: "Ricochet", about: "It bounces off the right edge of the lyrics and their floor.",
		takes: 1500 * time.Millisecond, least: 900 * time.Millisecond, legs: ricochetLegs},
}

// ballMotionOf returns the motion id names: the one that mixes them for an
// id that none is.
func ballMotionOf(id settings.LyricBallMotion) *ballMotion {
	for i := range ballMotions {
		if ballMotions[i].id == id {
			return &ballMotions[i]
		}
	}
	return &ballMotions[0]
}

// resolveBallMotion returns the motion the ball goes from line line to the
// next in, when the last word of the line leaves room before the next
// starts: the motion chosen, or, for the one that mixes, the line's turn,
// or the next in turn that fits; a hop where the one chosen doesn't fit; nil
// where not even a hop does.
func resolveBallMotion(id settings.LyricBallMotion, line int, room time.Duration) *ballMotion {
	hop := ballMotionOf(settings.LyricBallHop)
	if id == settings.LyricBallMix {
		// The line's turn is among all the motions, whatever room the lines
		// before it had; where its own doesn't fit the pause, the next that does
		// is taken.
		n := len(ballMotions) - 1
		for j := range n {
			if m := &ballMotions[1+(max(line, 0)+j)%n]; m.least <= room {
				return m
			}
		}
		return nil
	}
	if m := ballMotionOf(id); m.legs != nil && m.least <= room {
		return m
	}
	if hop.least <= room {
		return hop
	}
	return nil
}

// ballLeg is a stretch of a flight: how it goes from where the one before it
// ended, to where it ends.
type ballLeg struct {
	kind legKind
	// to is where it ends; c1 and c2 are the control points of a curve.
	to, c1, c2 ballPoint
	// weight is its share of the flight's time.
	weight float32
	// lift is how high an arc rises over the straight way (a ball thrown), and
	// the radius of a loop.
	lift float32
	// dir is the way a loop turns: -1 sets out toward the left, 1 the right.
	dir float32
	// bounce is set where the ball hits something at the end of the leg, which
	// squashes it.
	bounce bool
}

type legKind uint8

const (
	// legArc is the flight of a thrown ball: straight on in x and in y over
	// time, lifted by a parabola.
	legArc legKind = iota
	// legFall falls, faster and faster.
	legFall
	// legRise rises and slows.
	legRise
	// legSwoop goes sideways first and drops at its end.
	legSwoop
	// legLoop turns once round a circle that its start is the foot of, and
	// ends where it started.
	legLoop
	// legCurve follows a cubic curve, slow at its ends.
	legCurve
)

// at returns where the leg is, u of the way, from the point from.
func (l ballLeg) at(from ballPoint, u float32) ballPoint {
	switch l.kind {
	case legFall:
		return ballPoint{lerp(from.x, l.to.x, u), lerp(from.y, l.to.y, u*u)}
	case legRise:
		return ballPoint{lerp(from.x, l.to.x, u), lerp(from.y, l.to.y, 1-(1-u)*(1-u))}
	case legSwoop:
		return ballPoint{lerp(from.x, l.to.x, u*u*(3-2*u)), lerp(from.y, l.to.y, u*u)}
	case legLoop:
		phi := 2 * math.Pi * float64(u)
		return ballPoint{from.x + l.dir*l.lift*float32(math.Sin(phi)), from.y - l.lift*float32(1-math.Cos(phi))}
	case legCurve:
		t := float32(0.5 - 0.5*math.Cos(math.Pi*float64(u)))
		m := 1 - t
		a, b, c, d := m*m*m, 3*m*m*t, 3*m*t*t, t*t*t
		return ballPoint{a*from.x + b*l.c1.x + c*l.c2.x + d*l.to.x, a*from.y + b*l.c1.y + c*l.c2.y + d*l.to.y}
	}
	return ballPoint{lerp(from.x, l.to.x, u), lerp(from.y, l.to.y, u) - 4*l.lift*u*(1-u)}
}

// end returns where the leg ends, from the point from.
func (l ballLeg) end(from ballPoint) ballPoint {
	if l.kind == legLoop {
		return from
	}
	return l.to
}

// ballPath is the legs of a flight, in order.
type ballPath struct {
	legs [4]ballLeg
	n    int
}

// ballLegs makes a path of legs: no more than four.
func ballLegs(legs ...ballLeg) ballPath {
	var p ballPath
	p.n = copy(p.legs[:], legs)
	return p
}

// at returns where the ball is, u of the way along a flight that takes dur,
// from the point from, and how long ago it bounced off the end of the leg
// before, -1 where it didn't.
func (bp *ballPath) at(from ballPoint, u float32, dur time.Duration) (ballPoint, time.Duration) {
	var total float32
	for _, l := range bp.legs[:bp.n] {
		total += l.weight
	}
	done, at := float32(0), from
	for i, l := range bp.legs[:bp.n] {
		share := l.weight / total
		if u < done+share || i == bp.n-1 {
			lu := ballUnit((u - done) / share)
			since := time.Duration(-1)
			if i > 0 && bp.legs[i-1].bounce {
				since = time.Duration(float64(lu) * float64(share) * float64(dur))
			}
			return l.at(at, lu), since
		}
		done += share
		at = l.end(at)
	}
	return from, -1
}

// hopLegs go in one arc, which rises with how far the next line lies.
func hopLegs(a, b ballPoint, g ballGeo) ballPath {
	lift := min(max(0.2*ballAbs(b.x-a.x), 0.8*g.size), 1.8*g.size)
	return ballLegs(ballLeg{kind: legArc, to: b, weight: 1, lift: lift})
}

// boomerangLegs throw the ball out to the right, past the end of the line
// and as far as the lyrics reach, and curve it back, from above, to the first
// word of the next line.
func boomerangLegs(a, b ballPoint, g ballGeo) ballPath {
	s := g.size
	out := max(g.field.X+g.field.W-g.radius-0.3*s, a.x+1.6*s)
	return ballLegs(ballLeg{kind: legCurve, to: b, weight: 1,
		c1: ballPoint{out, a.y - 1.5*s}, c2: ballPoint{out - 0.3*s, b.y - 2.2*s}})
}

// loopLegs turn the ball once round a circle over the last word, setting
// out to the left, then arc it down to the next line.
func loopLegs(a, b ballPoint, g ballGeo) ballPath {
	s := g.size
	lift := min(max(0.18*ballAbs(b.x-a.x), 0.9*s), 1.7*s)
	return ballLegs(
		ballLeg{kind: legLoop, weight: 0.5, lift: 0.6 * s, dir: -1},
		ballLeg{kind: legArc, to: b, weight: 0.5, lift: lift},
	)
}

// diveLegs rise a little, as to gather the ball, and drop it to the next
// line in a swoop.
func diveLegs(a, b ballPoint, g ballGeo) ballPath {
	up := ballPoint{a.x, a.y - 0.5*g.size}
	return ballLegs(
		ballLeg{kind: legRise, to: up, weight: 0.3},
		ballLeg{kind: legSwoop, to: b, weight: 0.7},
	)
}

// ricochetLegs have the ball hit the right edge of the lyrics, fall to
// their floor, and bounce up to the first word of the next line.
func ricochetLegs(a, b ballPoint, g ballGeo) ballPath {
	s, f := g.size, g.field
	right := max(f.X+f.W-g.radius-0.2*s, a.x+1.2*s)
	floor := max(f.Y+f.H-g.radius-0.08*f.H, b.y+3*s)
	wall := ballPoint{right, a.y + 0.28*(floor-a.y)}
	ground := ballPoint{(right + b.x) / 2, floor}
	return ballLegs(
		ballLeg{kind: legArc, to: wall, weight: 0.24, lift: 0.5 * s, bounce: true},
		ballLeg{kind: legFall, to: ground, weight: 0.26, bounce: true},
		ballLeg{kind: legArc, to: b, weight: 0.5, lift: 0.3*(floor-b.y) + 0.6*s},
	)
}
