package audio

import "math"

// The stages after the equalizer (#615): stereo width and crossfeed, which
// Sound effects turns off with the equalizer, then mono and balance, which
// serve how the listener hears and stay on. Each glides to a new setting
// as the volume does, so as not to click, and is skipped while it does
// nothing. Width and crossfeed play on stereo alone: on 5.1 they are
// skipped.

// Sound is how the engine shapes what it plays after the equalizer.
type Sound struct {
	// Balance weighs the left side against the right, from -1, the left
	// alone, to 1, the right alone; 0 leaves both. The side it leans to
	// stays as it is, and the other is turned down. On 5.1, the left
	// channels weigh against the right ones, and the centre and the bass
	// channel stay.
	Balance float64
	// Mono plays both sides mixed on each; on 5.1, each left channel mixed
	// with its right one.
	Mono bool
	// Crossfeed mixes a low-passed, slightly delayed copy of each side into
	// the other, for headphones, from 0, off, to 1.
	Crossfeed float64
	// Width scales the side signal: 1 plays it as recorded, 0 in mono, 2
	// twice as wide.
	Width float64
}

const (
	// crossfeedHz is the corner of the copy's low-pass: Bauer's 700 Hz,
	// below which the head shades little of a sound from the side.
	crossfeedHz = 700
	// crossfeedFeed is how far below the side its copy plays in the other
	// ear, at the low notes, at full crossfeed: 4.5 dB.
	crossfeedFeed = 4.5
	// crossfeedDelay is the samples the copy waits, besides the low-pass's
	// own delay, 0.23 ms: about 0.3 ms in all, as a speaker to one side
	// reaches the far ear later.
	crossfeedDelay = 4
	// MaxWidth is the widest Width.
	MaxWidth = 2
)

// crossfeedPole is the share of the distance to the input the copy's
// one-pole low-pass covers each sample.
var crossfeedPole = 1 - math.Exp(-2*math.Pi*crossfeedHz/Rate)

// crossfeedGain is the copy's gain, relative to the side, at full
// crossfeed.
var crossfeedGain = math.Pow(10, -crossfeedFeed/20)

// sound is the stages after the equalizer, on the audio thread.
type sound struct {
	width, crossfeed, mono, balance smoother
	// The crossfeed's memory: each side's low-pass, and the last of them,
	// which the other side hears late, from at on.
	low  [2]float64
	late [2][crossfeedDelay]float64
	at   int
	// fed is whether the crossfeed ran the last quantum: one that starts
	// again starts from silence.
	fed bool
}

func newSound() *sound {
	return &sound{
		width:     newSmoother(1, glide),
		crossfeed: newSmoother(0, glide),
		mono:      newSmoother(0, glide),
		balance:   newSmoother(0, glide),
	}
}

// set has the stages glide to s.
func (f *sound) set(s Sound) {
	f.balance.target = min(max(s.Balance, -1), 1)
	f.mono.target = 0
	if s.Mono {
		f.mono.target = 1
	}
	f.crossfeed.target = min(max(s.Crossfeed, 0), 1)
	f.width.target = min(max(s.Width, 0), MaxWidth)
}

// idle reports whether a stage whose setting is s does nothing: s rests at
// neutral.
func idle(s *smoother, neutral float64) bool {
	return s.settled() && s.value == neutral
}

// process runs a quantum of frames of ch channels interleaved, 2 or 6,
// through the stages: width, crossfeed, mono, then balance.
func (f *sound) process(buf []float32, ch int) {
	if ch == 2 {
		if !idle(&f.width, 1) {
			f.widen(buf)
		}
		if idle(&f.crossfeed, 0) {
			f.fed = false
		} else {
			f.feed(buf)
		}
	}
	if !idle(&f.mono, 0) {
		f.mix(buf, ch)
	}
	if !idle(&f.balance, 0) {
		f.weigh(buf, ch)
	}
}

// widen scales the side signal of stereo frames by the width, and the mid
// signal down where the width is above 1, so that no frame can go past the
// larger of its sides: |m + w·s| over w is at most |m| + |s|.
func (f *sound) widen(buf []float32) {
	for i := 0; i < len(buf); i += 2 {
		w := f.width.next()
		gm, gs := 1.0, w
		if w > 1 {
			gm, gs = 1/w, 1
		}
		l, r := float64(buf[i]), float64(buf[i+1])
		m, s := (l+r)/2*gm, (l-r)/2*gs
		buf[i], buf[i+1] = float32(m+s), float32(m-s)
	}
}

// feed mixes into each side of stereo frames the other's low-passed copy,
// late, and takes its own copy, as late, out of it, by the same share:
// what plays the same on both sides, as a voice in the middle, comes out
// as it went in, while a sound on one side alone reaches the other ear
// softer, duller and later, as from speakers. This is Bauer's crossfeed as
// bs2b makes it, the direct sound lifted above the corner where the copy
// fades, which keeps the sum of both flat.
func (f *sound) feed(buf []float32) {
	if !f.fed {
		f.low = [2]float64{}
		f.late = [2][crossfeedDelay]float64{}
		f.fed = true
	}
	for i := 0; i < len(buf); i += 2 {
		g := f.crossfeed.next() * crossfeedGain
		share := g / (1 + g)
		l, r := float64(buf[i]), float64(buf[i+1])
		f.low[0] += crossfeedPole * (l - f.low[0])
		f.low[1] += crossfeedPole * (r - f.low[1])
		lateL, lateR := f.late[0][f.at], f.late[1][f.at]
		f.late[0][f.at], f.late[1][f.at] = f.low[0], f.low[1]
		f.at = (f.at + 1) % crossfeedDelay
		d := share * (lateR - lateL)
		buf[i], buf[i+1] = float32(l+d), float32(r-d)
	}
	f.low[0], f.low[1] = flush(f.low[0]), flush(f.low[1])
}

// pairs are the left and right channels of each pair of 5.1's, front and
// surround; of stereo's, the first alone.
var pairs = [2][2]int{{0, 1}, {4, 5}}

// mix moves each pair of channels toward their mix, by mono's share.
func (f *sound) mix(buf []float32, ch int) {
	n := 1
	if ch == maxChannels {
		n = 2
	}
	for i := 0; i < len(buf); i += ch {
		m := float32(f.mono.next())
		for _, p := range pairs[:n] {
			l, r := buf[i+p[0]], buf[i+p[1]]
			mid := (l + r) / 2
			buf[i+p[0]], buf[i+p[1]] = l+m*(mid-l), r+m*(mid-r)
		}
	}
}

// weigh turns the left channels down as the balance leans right, and the
// right ones as it leans left.
func (f *sound) weigh(buf []float32, ch int) {
	n := 1
	if ch == maxChannels {
		n = 2
	}
	for i := 0; i < len(buf); i += ch {
		b := f.balance.next()
		gl, gr := float32(min(1, 1-b)), float32(min(1, 1+b))
		for _, p := range pairs[:n] {
			buf[i+p[0]] *= gl
			buf[i+p[1]] *= gr
		}
	}
}

// forget clears the crossfeed's memory, as the device's channels change.
func (f *sound) forget() { f.fed = false }

// SetSound has the stages after the equalizer glide to s: width and
// crossfeed, on stereo alone, then mono and balance.
func (e *Engine) SetSound(s Sound) {
	e.mu.Lock()
	e.sound.set(s)
	e.mu.Unlock()
}
