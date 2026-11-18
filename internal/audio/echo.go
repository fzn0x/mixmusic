package audio

import "time"

// The echo (#546): each channel repeats after a delay, each repeat fed
// back into the line at the feedback's share, so that it repeats again,
// softer. It plays after the equalizer, before the reverb, so that its
// repeats sound in the room, at a wet and a dry level of its own, which
// glide, as the feedback does; a new delay fades in over the one before,
// read from the same line, so that it neither clicks nor bends the pitch.
// The stage is skipped while its wet level rests at 0 and its dry one at
// 1. The line, two seconds of six channels, is made off the audio thread,
// as the echo is first turned on.

// Echo is how the echo plays: off, or repeats at a delay, a feedback, a
// wet and a dry level.
type Echo struct {
	// On plays the repeats; off, the levels glide to the dry sound alone,
	// and the stage is skipped once they're there.
	On bool
	// Delay is how long after the sound its first repeat plays, and each
	// repeat after the one before, from MinEchoDelay to MaxEchoDelay, by
	// the sample.
	Delay time.Duration
	// Feedback is how loud each repeat plays against the one before, from
	// 0, a single repeat, to MaxEchoFeedback.
	Feedback float64
	// Wet is the repeats' level, and Dry the sound's as it came, each from
	// 0 to 1: MixLevels gives both of a single mix.
	Wet, Dry float64
}

const (
	// MinEchoDelay and MaxEchoDelay are the shortest and the longest delay
	// of the echo.
	MinEchoDelay = 20 * time.Millisecond
	MaxEchoDelay = 2 * time.Second
	// MaxEchoFeedback is the most feedback: the repeats always fade.
	MaxEchoFeedback = 0.9
	// echoFrames is the frames the line holds: the longest delay's.
	echoFrames = int(MaxEchoDelay * Rate / time.Second)
)

// echo is the stage of the echo, on the audio thread.
type echo struct {
	// set is what SetEcho gave.
	set Echo
	// line holds the frames of ch channels the repeats are read from, at
	// is the frame written next, and filled how many frames were written
	// since the line was cleared, up to echoFrames: a frame older than
	// that is silence. SetEcho makes it as the echo is first turned on.
	line       []float32
	ch         int
	at, filled int
	// delay is the delay that plays, in frames, and next, while fading,
	// the one that fades in over it, by fade's share; want is the one
	// asked for, which plays once the fade under way, if any, ends.
	delay, next, want        int
	fading                   bool
	fade, feedback, wet, dry smoother
	// rested is whether the stage was skipped: it starts again from
	// silence. stale is whether the output started again since it last
	// played, cut whether a seek asks it to fade its repeats out over the
	// quantum and forget them.
	rested, stale, cut bool
	// quiet is how many frames the line has taken no sound above
	// quietLevel.
	quiet int
}

func newEcho() *echo {
	return &echo{
		fade: newSmoother(0, glide), feedback: newSmoother(0, glide),
		wet: newSmoother(0, glide), dry: newSmoother(1, glide), rested: true,
	}
}

// newEchoLine makes the echo's line, off the audio thread.
func newEchoLine() []float32 { return make([]float32, echoFrames*maxChannels) }

// echoDelay is the frames of a delay, in its range.
func echoDelay(d time.Duration) int {
	d = min(max(d, MinEchoDelay), MaxEchoDelay)
	return max(1, min(int((d*Rate+time.Second/2)/time.Second), echoFrames))
}

// setTo has the stage glide to x.
func (f *echo) setTo(x Echo) {
	f.set = x
	f.wet.target, f.dry.target = 0, 1
	if x.On {
		f.wet.target, f.dry.target = min(max(x.Wet, 0), 1), min(max(x.Dry, 0), 1)
		f.feedback.target = min(max(x.Feedback, 0), MaxEchoFeedback)
		f.want = echoDelay(x.Delay)
	}
}

// resting reports whether the echo plays nothing: its wet level rests at
// 0.
func (f *echo) resting() bool { return f.line == nil || idle(&f.wet, 0) }

// ringing reports whether the line still holds a sound above quietLevel
// that its repeats will play.
func (f *echo) ringing() bool {
	if f.resting() {
		return false
	}
	span := f.delay
	if f.fading {
		span = max(span, f.next)
	}
	return f.quiet < span+quantum
}

// process runs a quantum of frames of ch channels interleaved, 2 or 6,
// through the echo: on 5.1, each channel repeats on its own, as stereo's
// two do.
func (f *echo) process(buf []float32, ch int) {
	if f.resting() {
		f.rested, f.cut, f.quiet = true, false, 0
		if !idle(&f.dry, 1) {
			for i := 0; i < len(buf); i += ch {
				d := float32(f.dry.next())
				for c := range ch {
					buf[i+c] *= d
				}
			}
		}
		return
	}
	if f.rested || f.stale || f.ch != ch {
		if f.rested {
			f.delay, f.fading = f.want, false
			f.feedback.value = f.feedback.target
		}
		f.filled, f.ch = 0, ch
		f.rested, f.stale = false, false
	}
	if !f.fading && f.want != f.delay {
		f.next, f.fading = f.want, true
		f.fade.value, f.fade.target = 0, 1
	}
	n := len(buf) / ch
	var peak float32
	for i := range n {
		fb := float32(f.feedback.next())
		w, d := float32(f.wet.next()), float32(f.dry.next())
		if f.cut {
			w *= float32(n-1-i) / float32(n)
		}
		var m float32
		if f.fading {
			m = float32(f.fade.next())
		}
		k := float32(1)
		if f.filled < rampFrames {
			k = float32(f.filled) / rampFrames
		}
		// The frames the repeats read: delay and next frames ago, or
		// silence where the line wasn't written since it was cleared.
		ra, rb := -1, -1
		if f.filled >= f.delay {
			if ra = f.at - f.delay; ra < 0 {
				ra += echoFrames
			}
		}
		if f.fading && f.filled >= f.next {
			if rb = f.at - f.next; rb < 0 {
				rb += echoFrames
			}
		}
		o := buf[i*ch : i*ch+ch]
		at := f.line[f.at*ch : f.at*ch+ch]
		for c, x := range o {
			var tap float32
			if ra >= 0 {
				tap = f.line[ra*ch+c]
			}
			if f.fading {
				var t float32
				if rb >= 0 {
					t = f.line[rb*ch+c]
				}
				tap += m * (t - tap)
			}
			v := k*x + fb*tap
			if v > -tiny && v < tiny {
				v = 0
			}
			peak = max(peak, v, -v)
			at[c] = v
			o[c] = d*x + w*tap
		}
		if f.at++; f.at == echoFrames {
			f.at = 0
		}
		if f.filled < echoFrames {
			f.filled++
		}
	}
	if f.fading && f.fade.settled() {
		f.delay, f.fading = f.next, false
	}
	if f.cut {
		// What the seek left behind fades out with the quantum, and the
		// line starts from silence.
		f.filled, f.cut, f.quiet = 0, false, 0
		f.delay, f.fading = f.want, false
		return
	}
	if peak*float32(f.wet.value) > quietLevel {
		f.quiet = 0
	} else {
		f.quiet += n
	}
}

// SetEcho has the echo glide to x: its repeats play after the equalizer,
// before the reverb, at x's wet and dry levels, a new delay fading in over
// the one before. Off, or at a wet level of 0, the stage is skipped once
// its levels glided there, and the samples come out as they went in. The
// first time the echo is turned on, it makes its line, here rather than
// on the audio thread.
func (e *Engine) SetEcho(x Echo) {
	e.mu.Lock()
	built := e.echo.line != nil
	e.mu.Unlock()
	var line []float32
	if x.On && !built {
		line = newEchoLine()
	}
	e.mu.Lock()
	if line != nil && e.echo.line == nil {
		e.echo.line = line
	}
	e.echo.setTo(x)
	e.mu.Unlock()
}

// Echo returns what the echo glides to, as SetEcho set it.
func (e *Engine) Echo() Echo {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.echo.set
}
