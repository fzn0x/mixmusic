package audio

// A deck plays faster or slower at its track's own pitch by waveform-
// similarity overlap-add (WSOLA), the method of W. Verhelst and M.
// Roelands, "An overlap-add technique based on waveform similarity (WSOLA)
// for high quality time-scale modification of speech", ICASSP 1993. It
// cuts the input into windows a hop apart in the output, each taken from
// about where the speed puts it in the input, within a search range: from
// where it best continues the window before, so that their overlap adds up
// in phase. At 1× a deck plays its ring as it is.

import "math"

// MinSpeed and MaxSpeed bound the speeds a deck plays at.
const (
	MinSpeed = 0.5
	MaxSpeed = 3
)

const (
	// stretchWindow is the length of the windows, 20 ms, and stretchHop
	// how far apart they are in the output, half of it: Hann windows half
	// their length apart add up to 1.
	stretchWindow = 960
	stretchHop    = stretchWindow / 2
	// stretchSearch is how far from where the speed puts a window it may
	// start, 15 ms either way.
	stretchSearch = 720
	// stretchStride is the step of the coarse search, in the windows'
	// starts and in the samples it compares; a search one frame at a
	// time around the best of it follows.
	stretchStride = 4
	// stretchHeld is the input a stretcher holds at most: from where the
	// window before goes on, or the search range around the next window's
	// start, to a window past both. It is most at MaxSpeed, where the
	// window before may go on from far before the range.
	stretchHeld = 2*stretchSearch + stretchHop*(MaxSpeed-1) + stretchWindow
	// stretchFrames is room for it.
	stretchFrames = 4096
)

// stretchHann is a periodic Hann window: w[i] + w[i+stretchHop] is 1.
var stretchHann = func() (w [stretchWindow]float32) {
	for i := range w {
		w[i] = float32(0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/stretchWindow))
	}
	return w
}()

// clampSpeed returns v within MinSpeed and MaxSpeed, and 1 for a speed
// that isn't a number.
func clampSpeed(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 1
	}
	return min(max(v, MinSpeed), MaxSpeed)
}

// stretchMode is what a stretcher plays.
type stretchMode int8

const (
	// bypassed plays the ring as it is, at 1×.
	bypassed stretchMode = iota
	// stretching plays the windows.
	stretching
	// unwinding plays the input held as it is, on the way back to 1×: it
	// continues the last window as the next one would at 1×.
	unwinding
)

// stretcher time-stretches what a deck reads from its ring. It belongs to
// the audio thread, and allocates nothing.
type stretcher struct {
	// src is the ring it reads: another one, as after a seek, starts it
	// over.
	src  *ring
	mode stretchMode
	// in holds have frames of the input, interleaved, from frame lo of
	// the ring; mono holds them with their channels mixed, which the
	// search compares.
	in   [2 * stretchFrames]float32
	mono [stretchFrames]float32
	lo   int64
	have int
	// end is the frame of the ring the track ends at, once the ring ended,
	// else -1: past it, the input is silence.
	end int64
	// prev is where the window before starts, nominal where the speed puts
	// the next, and floor the earliest it may start: where the stretch
	// started, as what came before played and is gone.
	prev    int64
	nominal float64
	floor   int64
	// fresh is set until the first window: the one before is the input as
	// it played, whose falling half tail takes then.
	fresh bool
	// out is the hop the last window made, of which outAt frames played,
	// and tail the falling half of that window, which the next window's
	// rising half adds to.
	out, tail [2 * stretchHop]float32
	outAt     int
	// from and to are the frames of the input out plays from and to.
	from, to int64
	// done is set once it played the end of the track.
	done bool
}

// reset starts the stretcher over on r, at 1×.
func (s *stretcher) reset(r *ring) {
	s.src, s.mode, s.have, s.end, s.done = r, bypassed, 0, -1, false
	s.outAt = stretchHop
}

// start stretches from the frame the stretcher plays next, which the ring
// or the input held has.
func (s *stretcher) start(r *ring, speed float64) {
	if s.mode == bypassed {
		s.lo, s.have = r.read.Load(), 0
	}
	q := s.lo
	s.mode = stretching
	s.prev = q - stretchHop
	s.nominal = float64(s.prev) + stretchHop*speed
	s.floor, s.fresh = q, true
	s.outAt, s.from, s.to = stretchHop, q, q
}

// read moves up to len(out)/2 frames of r, played at speed, into out, and
// returns how many: fewer while the ring waits for the decoder, and at the
// end of the track.
func (s *stretcher) read(r *ring, out []float32, speed float64) int {
	if r != s.src {
		s.reset(r)
	}
	want, got := len(out)/2, 0
	for got < want {
		switch s.mode {
		case bypassed:
			if speed != 1 {
				s.start(r, speed)
				continue
			}
			return got + r.readTo(out[2*got:2*want])
		case unwinding:
			if speed != 1 {
				s.start(r, speed)
				continue
			}
			n := s.have
			if s.end >= 0 {
				n = int(min(int64(n), max(s.end-s.lo, 0)))
			}
			if n == 0 {
				// What follows is in the ring, or the silence past the end.
				s.mode, s.have = bypassed, 0
				continue
			}
			k := min(n, want-got)
			copy(out[2*got:2*(got+k)], s.in[:2*k])
			s.drop(s.lo + int64(k))
			got += k
		case stretching:
			if s.outAt < stretchHop {
				k := min(stretchHop-s.outAt, want-got)
				copy(out[2*got:2*(got+k)], s.out[2*s.outAt:])
				s.outAt += k
				got += k
				continue
			}
			if s.end >= 0 && s.prev+stretchHop >= s.end {
				s.done = true
				return got
			}
			if speed == 1 {
				// At 1× the next window would start where the last one
				// goes on, whose falling half and the next's rising half
				// add up to the input as it is: it plays from there.
				s.drop(s.prev + stretchHop)
				s.mode = unwinding
				continue
			}
			if !s.step(r, speed) {
				return got
			}
		}
	}
	return got
}

// step makes the next hop of the output, at speed, once the ring has the
// input it needs.
func (s *stretcher) step(r *ring, speed float64) bool {
	center := int64(math.Round(s.nominal))
	lo := max(center-stretchSearch, s.floor, s.lo)
	hi := max(center+stretchSearch, lo)
	// The window before goes on from tpl: the next one is the stretch of
	// the input most like it.
	tpl := s.prev + stretchHop
	if !s.fill(r, max(tpl, hi)+stretchWindow) {
		return false
	}
	if s.fresh {
		s.fresh = false
		in := s.in[2*(tpl-s.lo):]
		for f := range stretchHop {
			w := stretchHann[stretchHop+f]
			s.tail[2*f], s.tail[2*f+1] = w*in[2*f], w*in[2*f+1]
		}
	}
	x := s.best(tpl, lo, hi, center)
	in := s.in[2*(x-s.lo) : 2*(x-s.lo+stretchWindow)]
	for f := range stretchHop {
		w := stretchHann[f]
		s.out[2*f] = s.tail[2*f] + w*in[2*f]
		s.out[2*f+1] = s.tail[2*f+1] + w*in[2*f+1]
	}
	for f := range stretchHop {
		w := stretchHann[stretchHop+f]
		s.tail[2*f] = w * in[2*(stretchHop+f)]
		s.tail[2*f+1] = w * in[2*(stretchHop+f)+1]
	}
	s.from, s.to = tpl, x+stretchHop
	s.prev = x
	s.nominal += stretchHop * speed
	s.outAt = 0
	// What the next window may start from, and what it continues.
	s.drop(min(x+stretchHop, int64(math.Round(s.nominal))-stretchSearch))
	return true
}

// best returns the start, from lo to hi, of the window of the input most
// like the one at tpl: coarsely, by stretchStride, then frame by frame
// around the best. The likeness is their correlation over the energy's
// root of the start's window, on the channels mixed. Silence has no
// likeness: its window starts at center, where the speed puts it.
func (s *stretcher) best(tpl, lo, hi, center int64) int64 {
	m := s.mono[:s.have]
	t := m[tpl-s.lo:][:stretchWindow]
	var te float32
	for i := 0; i < stretchWindow; i += stretchStride {
		te += t[i] * t[i]
	}
	best := min(max(center, lo), hi)
	if te < 1e-9 {
		return best
	}
	score := float32(math.Inf(-1))
	// The energy of each start's samples, which the next start's shares
	// but for one at each end.
	j := int(lo - s.lo)
	var e float64
	for i := 0; i < stretchWindow; i += stretchStride {
		e += float64(m[j+i]) * float64(m[j+i])
	}
	for x := lo; x <= hi; x += stretchStride {
		j := int(x - s.lo)
		c := m[j:][:stretchWindow]
		var dot float32
		for i := 0; i < stretchWindow; i += stretchStride {
			dot += t[i] * c[i]
		}
		if e > 1e-12 {
			if v := dot / float32(math.Sqrt(e)); v > score {
				best, score = x, v
			}
		}
		if x+stretchStride <= hi {
			e += float64(m[j+stretchWindow])*float64(m[j+stretchWindow]) - float64(m[j])*float64(m[j])
			e = max(e, 0)
		}
	}
	coarse := best
	score = float32(math.Inf(-1))
	for x := max(coarse-stretchStride+1, lo); x <= min(coarse+stretchStride-1, hi); x++ {
		c := m[x-s.lo:][:stretchWindow]
		var dot, e float32
		for i := range stretchWindow {
			dot += t[i] * c[i]
			e += c[i] * c[i]
		}
		if e > 1e-12 {
			if v := dot / float32(math.Sqrt(float64(e))); v > score {
				best, score = x, v
			}
		}
	}
	return best
}

// fill reads the ring until the input reaches frame need, and reports
// whether it did: past the end of the track, the input is silence.
func (s *stretcher) fill(r *ring, need int64) bool {
	for s.lo+int64(s.have) < need {
		at := s.have
		k := int(min(need-s.lo-int64(at), int64(stretchFrames-at)))
		if k <= 0 {
			return false
		}
		if s.end >= 0 {
			clear(s.in[2*at : 2*(at+k)])
			clear(s.mono[at : at+k])
			s.have += k
			continue
		}
		// The ring ended if it had, with all it holds, before it was read.
		ended := r.done.Load()
		n := r.readTo(s.in[2*at : 2*(at+k)])
		for f := at; f < at+n; f++ {
			s.mono[f] = 0.5 * (s.in[2*f] + s.in[2*f+1])
		}
		s.have += n
		if n < k {
			if !ended {
				return false
			}
			s.end = s.lo + int64(s.have)
		}
	}
	return true
}

// drop lets go of the input before frame keep.
func (s *stretcher) drop(keep int64) {
	k := int(min(max(keep-s.lo, 0), int64(s.have)))
	if k == 0 {
		return
	}
	copy(s.in[:], s.in[2*k:2*s.have])
	copy(s.mono[:], s.mono[k:s.have])
	s.have -= k
	s.lo += int64(k)
}

// position returns the frame of r, from its start, that plays next.
func (s *stretcher) position(r *ring) int64 {
	if r != s.src || s.mode == bypassed {
		return r.read.Load()
	}
	p := s.lo
	if s.mode == stretching {
		// Each hop fades from the input at from to that at to.
		p = s.from + (s.to-s.from)*int64(s.outAt)/stretchHop
	}
	// The last windows play into the silence past the end, which isn't
	// the track's.
	return min(p, r.write.Load())
}

// pending reports whether the stretcher has more of r to play than the
// ring holds.
func (s *stretcher) pending(r *ring) bool {
	if r != s.src {
		return false
	}
	switch s.mode {
	case stretching:
		return !s.done
	case unwinding:
		return s.have > 0 && (s.end < 0 || s.lo < s.end)
	}
	return false
}
