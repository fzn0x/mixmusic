package room

// Keeping in step: how a guest's player follows the host's. The host tells
// where its track is, as heard, at a time of its own clock (a Playhead),
// as it changes and every 2 s; pings give each guest the offset of the
// host's clock from its own (Offset). Every 250 ms, a guest's Follower
// compares where its own track is heard with where the host's is, and
// corrects it: by nothing under 50 ms, by playing up to 2 percent faster
// or slower, at the same pitch, up to 750 ms, and by seeking past that,
// then resting 6 s while the seek settles. A Spotify device has no speed
// but the track's own: those tracks only seek.
//
// Both clocks are monotonic, as time.Since is: a clock set, or a computer
// that changes its time zone, moves neither. docs/architecture/rooms.md
// tells why it is so.

import (
	"math"
	"slices"
	"time"
)

// The bounds of keeping in step.
const (
	// FollowEvery is how often a guest compares.
	FollowEvery = 250 * time.Millisecond
	// InStep is how far a guest may be from the host before it corrects,
	// and Settled how near it comes before it stops correcting.
	InStep  = 50 * time.Millisecond
	Settled = 15 * time.Millisecond
	// SeekPast is how far a guest may be before it seeks rather than play
	// faster or slower, and SeekRest how long it then waits before it
	// compares again.
	SeekPast = 750 * time.Millisecond
	SeekRest = 6 * time.Second
	// MaxAdjust is how much faster or slower a guest plays at most, and
	// adjustGain how much for each second it is off, below that.
	MaxAdjust  = 0.02
	adjustGain = 0.2
)

// A Clock tells the time since a moment of its own, monotonically.
type Clock interface {
	Now() time.Duration
}

// SystemClock is the computer's monotonic clock, from when it was made.
type SystemClock struct{ start time.Time }

// NewSystemClock starts a clock at 0 now.
func NewSystemClock() SystemClock { return SystemClock{start: time.Now()} }

// Now is the time since the clock was made.
func (c SystemClock) Now() time.Duration { return time.Since(c.start) }

// Playhead is where the host's track is: Position, at the host's clock's
// At, moving on at Speed while Playing. Starts tells a start to come: the
// track waits at Position until At, as the host starts all its guests at
// one moment.
type Playhead struct {
	// Entry names the host's entry of the queue.
	Entry    string
	Playing  bool
	Starts   bool
	Position time.Duration
	At       time.Duration
	// Speed is the speed the host plays the track at: 1 is its own.
	Speed float64
}

// waits reports whether the track waits at the host's clock's now: paused,
// or before its start.
func (h Playhead) waits(now time.Duration) bool { return !h.Playing || h.Starts && now < h.At }

// PositionAt is where the host's track is at the host's clock's now: a
// moment before At too, as a guest's estimate of the host's clock may be a
// little behind.
func (h Playhead) PositionAt(now time.Duration) time.Duration {
	if h.waits(now) {
		return h.Position
	}
	speed := h.Speed
	if speed <= 0 {
		speed = 1
	}
	return max(h.Position+time.Duration(float64(now-h.At)*speed), 0)
}

// offsetSamples is how many pings Offset keeps.
const offsetSamples = 8

// Offset estimates how far the host's clock is ahead of a guest's from
// the pings the guest sends: each tells when the guest sent it and got
// the answer, by its own clock, and when the host answered, by the host's.
// The answer that came back soonest is the one the network held least,
// and its estimate the nearest: Offset keeps the last few, and takes it.
type Offset struct {
	samples []offsetSample
}

type offsetSample struct{ offset, rtt time.Duration }

// Add takes in a ping sent at sent and answered at got, by the guest's
// clock, which the host answered at hostAt, by its own.
func (o *Offset) Add(sent, hostAt, got time.Duration) {
	if got < sent {
		return
	}
	s := offsetSample{offset: hostAt - (sent + (got-sent)/2), rtt: got - sent}
	if len(o.samples) == offsetSamples {
		o.samples = slices.Delete(o.samples, 0, 1)
	}
	o.samples = append(o.samples, s)
}

// Get returns the offset of the host's clock, to add to the guest's for
// the host's time, and the round trip it was measured over, once a ping
// came back.
func (o *Offset) Get() (offset, rtt time.Duration, ok bool) {
	if len(o.samples) == 0 {
		return 0, 0, false
	}
	best := o.samples[0]
	for _, s := range o.samples[1:] {
		if s.rtt < best.rtt {
			best = s
		}
	}
	return best.offset, best.rtt, true
}

// Local is what a guest's player does with the host's track now.
type Local struct {
	// Has is set while it holds the host's entry, loaded: playing or
	// paused, not loading.
	Has     bool
	Playing bool
	// Position is where its track is heard now.
	Position time.Duration
	// OnDevice is set while a Spotify device plays it, which has no speed
	// but the track's own: it only seeks.
	OnDevice bool
}

// Correction is what a Follower has the guest's player do. Its zero value
// does nothing.
type Correction struct {
	// Seek has the player seek to To.
	Seek bool
	To   time.Duration
	// Play and Pause play and pause it.
	Play, Pause bool
	// Speed, when not 0, is the speed to play the track at: the host's,
	// a little faster or slower.
	Speed float64
}

// playGrace is how long after asking the player to play the follower
// waits for it to, before it takes it for one that doesn't.
const playGrace = time.Second

// Follower keeps a guest's player in step with the host's: Step, every
// FollowEvery, and at the moment a start is due (Due).
type Follower struct {
	// correcting is set while it plays faster or slower, until it is
	// Settled; restUntil is when, by the guest's clock, a seek settled,
	// and asked when it last asked the player to play.
	correcting bool
	restUntil  time.Duration
	asked      time.Duration
	hasAsked   bool
	// lead is how far ahead of the host a seek goes, as the last one
	// landed behind by it: a seek takes a moment to sound. seeked is set
	// until the first comparison after a seek's rest.
	lead   time.Duration
	seeked bool
}

// maxLead bounds a seek's lead.
const maxLead = 500 * time.Millisecond

// Off returns how far the guest's track is from the host's at the guest's
// clock's now: ahead when positive.
func Off(now, offset time.Duration, host Playhead, local Local) time.Duration {
	return local.Position - host.PositionAt(now+offset)
}

// Due returns how long, from the guest's clock's now, until the host's
// start, if it is ahead: the guest steps then.
func Due(now, offset time.Duration, host Playhead) (time.Duration, bool) {
	if !host.Playing || !host.Starts {
		return 0, false
	}
	wait := host.At - (now + offset)
	return wait, wait > 0
}

// Step compares the guest's track with the host's at the guest's clock's
// now, offset from the host's, and returns what to correct.
func (f *Follower) Step(now, offset time.Duration, host Playhead, local Local) Correction {
	var c Correction
	if !local.Has {
		f.correcting, f.hasAsked = false, false
		return c
	}
	hostSpeed := host.Speed
	if hostSpeed <= 0 {
		hostSpeed = 1
	}
	hostNow := now + offset
	want := host.PositionAt(hostNow)
	off := local.Position - want
	abs := max(off, -off)
	// What it asked to play may not play yet: it waits for it, a while.
	waiting := !local.Playing && f.hasAsked && now-f.asked < playGrace
	if f.seeked && now >= f.restUntil && local.Playing && abs <= SeekPast {
		// How far behind the last seek landed is the next one's lead.
		f.lead, f.seeked = min(max(f.lead-off, 0), maxLead), false
	}
	adjust := 0.0
	switch {
	case host.waits(hostNow):
		// The host pauses, or its start is still to come: the guest waits
		// where the host's track is.
		f.correcting, f.hasAsked = false, false
		c.Pause = local.Playing
		c.Seek, c.To = abs > InStep, want
	case waiting:
	case now < f.restUntil:
		c.Play = !local.Playing
	case abs > SeekPast, !local.Playing && abs > InStep:
		// Too far to catch up: it seeks to where the host is, a moment
		// ahead, and rests while the seek settles.
		c.Seek, c.To, c.Play = true, want+f.lead, !local.Playing
		f.restUntil, f.correcting, f.seeked = now+SeekRest, false, true
	case !local.Playing:
		c.Play = true
	case local.OnDevice:
	default:
		if abs > InStep {
			f.correcting = true
		} else if abs < Settled {
			f.correcting = false
		}
		if f.correcting {
			adjust = -math.Max(-MaxAdjust, math.Min(MaxAdjust, off.Seconds()*adjustGain))
		}
	}
	if c.Play {
		f.asked, f.hasAsked = now, true
	}
	if !c.Seek {
		c.To = 0
	}
	if !local.OnDevice {
		// Said again each time: the player sets its own as it loads.
		c.Speed = hostSpeed * (1 + adjust)
	}
	return c
}

// Reset forgets what the follower did, for another track, but how long a
// seek takes to sound.
func (f *Follower) Reset() { *f = Follower{lead: f.lead} }
