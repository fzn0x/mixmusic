package audio

import "math"

// The reverb (#546): four rooms, each a feedback delay network of
// sixteen lines, after Jot and Chaigne, mixed by a Hadamard matrix, its
// input smeared first by all-pass filters, after Schroeder, and held back
// by the room's pre-delay. Each line's own low-pass sets how fast the
// room's low and high notes decay, so that the reverb time is the room's
// at the low and middle notes and shorter at the high ones. It plays after
// the echo, at a wet and a dry level of its own, which glide; a new room
// fades in over the one before, and the stage is skipped while its wet
// level rests at 0 and its dry one at 1. The lines are made off the audio
// thread, as the reverb is first turned on, so that the thread never
// allocates.

// Room is a room of the reverb.
type Room int

const (
	// RoomSmall is a small room: close walls, and a short, warm decay.
	RoomSmall Room = iota
	// RoomHall is a concert hall, whose decay is full and smooth: the
	// room a sound mood (#547) plays, at levels of its own.
	RoomHall
	// RoomPlate is a plate: dense and bright, as studios add to voices.
	RoomPlate
	// RoomLarge is a large space, as a cathedral, which rings long.
	RoomLarge
	// roomCount is how many rooms there are.
	roomCount
)

// Reverb is how the reverb plays: off, or a room at a wet and a dry
// level.
type Reverb struct {
	// On plays the room; off, the levels glide to the dry sound alone, and
	// the stage is skipped once they're there.
	On   bool
	Room Room
	// Wet is the room's level, from 0 to 1: at 1, it plays white noise as
	// loud as it came in. Dry is the level of the sound as it came, from 0
	// to 1. MixLevels gives both of a single mix; a sound mood sets its
	// own (#547).
	Wet, Dry float64
}

// MixLevels returns the wet and dry levels of a mix from 0, the dry sound
// alone, to 1, the effect's alone: an equal-power crossfade, so that the
// two, mostly unlike each other, keep the loudness between.
func MixLevels(mix float64) (wet, dry float64) {
	switch {
	case mix <= 0:
		return 0, 1
	case mix >= 1:
		return 1, 0
	}
	return math.Sin(mix * math.Pi / 2), math.Cos(mix * math.Pi / 2)
}

// roomShape is how a room sounds.
type roomShape struct {
	// time is the reverb time of the low and middle notes, RT60: the
	// seconds a sound takes to fall 60 dB once it stops.
	time float64
	// highs is the reverb time at dampHz, as a share of time: the walls
	// and the air take up the high notes sooner.
	highs float64
	// predelay is the seconds before the room answers, which keep a voice
	// clear of it.
	predelay float64
	// size scales the delay lines, and the all-pass filters up to 1.
	size float64
	// diffusion is the gain of the first two all-pass filters, which
	// smear the sound into the room; the other two take 0.83 of it.
	diffusion float64
}

// rooms are the rooms' shapes.
var rooms = [roomCount]roomShape{
	RoomSmall: {time: 0.6, highs: 0.5, predelay: 0.005, size: 0.32, diffusion: 0.6},
	RoomHall:  {time: 2.4, highs: 0.5, predelay: 0.025, size: 1, diffusion: 0.7},
	RoomPlate: {time: 1.8, highs: 0.85, predelay: 0.01, size: 0.5, diffusion: 0.75},
	RoomLarge: {time: 4, highs: 0.4, predelay: 0.04, size: 1.5, diffusion: 0.7},
}

const (
	// fdnLines is how many delay lines a room has: enough that even the
	// large space's tail is dense.
	fdnLines = 16
	// shortestLine and longestLine are the delays of a room of size 1,
	// in samples: the lines spread geometrically between them, each the
	// prime at or after its place, so that no two echo in step.
	shortestLine = 700
	longestLine  = 2900
	// dampHz is where a room's reverb time is its highs' share.
	dampHz = 4000
	// rampFrames is how long a room's input, or the echo's, fades in as
	// it starts from silence, so that a sound already playing doesn't
	// start it with a step: 5 ms.
	rampFrames = Rate / 200
	// tiny is the least magnitude a sample kept in a delay line, or a
	// filter's memory, has: anything quieter is 0, far below hearing, so
	// that a tail fading out never reaches the subnormal numbers that slow
	// the processor down.
	tiny = 1e-20
	// quietLevel is the level under which a tail is silent, -100 dBFS: an
	// output that plays one no louder doesn't stay on for it.
	quietLevel = 1e-5
)

// diffusers are the delays of the all-pass filters of each side's input,
// in samples, at size 1, as Dattorro's, at 48 kHz, with the right's apart.
var diffusers = [2][4]int{{229, 173, 613, 449}, {241, 163, 593, 467}}

// fdnSigns are the signs, for each pair of lines, of the input into them,
// and of the fronts' and the surrounds' outputs of them. None is a row of
// the Hadamard matrix, whose rows would read back only two of the lines a
// pass before, and the surrounds' are orthogonal to the fronts', so that
// the two pairs play unlike.
var fdnSigns = [fdnLines / 2][3]float32{
	{1, 1, 1}, {1, -1, 1}, {-1, 1, -1}, {1, 1, -1},
	{-1, -1, -1}, {1, 1, 1}, {-1, 1, 1}, {-1, -1, 1},
}

// allpass is Schroeder's all-pass filter: w = x + g·w[n-D], y = w[n-D] -
// g·w.
type allpass struct {
	buf []float32
	at  int
	g   float32
}

func (a *allpass) step(x float32) float32 {
	d := a.buf[a.at]
	w := x + a.g*d
	if w > -tiny && w < tiny {
		w = 0
	}
	a.buf[a.at] = w
	if a.at++; a.at == len(a.buf) {
		a.at = 0
	}
	return d - a.g*w
}

// room is a room's network, and its memory.
type room struct {
	// pre holds the pre-delay's stereo frames, from preAt the oldest.
	pre   []float32
	preAt int
	diff  [2][4]allpass
	lines [fdnLines][]float32
	at    [fdnLines]int
	// b and a are each line's low-pass, y = b·x + a·y[n-1], whose gain is
	// the decay of a pass along its line, at 0 Hz and at dampHz as the
	// room's times ask; b takes the Hadamard matrix's scale, 1/4, too.
	b, a [fdnLines]float32
	low  [fdnLines]float32
	// gain makes the room as loud as what comes in, for white noise.
	gain float32
	// fed is how many frames came in since the room was cleared, up to
	// rampFrames: its input fades in over them.
	fed int
	// span is how long the room holds a sound, in frames: after that long
	// of silence in and out, it is silent.
	span int
}

// isPrime reports whether n, above 1, is prime.
func isPrime(n int) bool {
	for d := 2; d*d <= n; d++ {
		if n%d == 0 {
			return false
		}
	}
	return n > 1
}

// primeFrom is the least prime at or after n.
func primeFrom(n int) int {
	for !isPrime(n) {
		n++
	}
	return n
}

// lineLengths are the delays of a room of size, in samples.
func lineLengths(size float64) [fdnLines]int {
	var m [fdnLines]int
	for i := range m {
		at := shortestLine * math.Pow(longestLine/float64(shortestLine), float64(i)/(fdnLines-1))
		m[i] = primeFrom(int(math.Round(size * at)))
		if i > 0 && m[i] <= m[i-1] {
			m[i] = primeFrom(m[i-1] + 1)
		}
	}
	return m
}

// newRoom makes the room of shape s, off the audio thread.
func newRoom(s roomShape) *room {
	r := &room{}
	pre := int(math.Round(s.predelay * Rate))
	r.pre = make([]float32, 2*pre)
	r.span = pre
	scale := min(s.size, 1)
	for side := range r.diff {
		for j, d := range diffusers[side] {
			n := primeFrom(int(math.Round(scale * float64(d))))
			g := s.diffusion
			if j >= 2 {
				g *= 0.83
			}
			r.diff[side][j] = allpass{buf: make([]float32, n), g: float32(g)}
			if side == 0 {
				r.span += n
			}
		}
	}
	m := lineLengths(s.size)
	r.span += m[fdnLines-1]
	// Each line's low-pass, of its own length: a pass along it falls by
	// 60 dB in the reverb time, at 0 Hz and at dampHz.
	w := 2 * math.Pi * dampHz / Rate
	var sum [256]float64 // each frequency's loss of energy a pass, over the lines
	for i, n := range m {
		r.lines[i] = make([]float32, n)
		g0 := math.Pow(10, -3*float64(n)/(s.time*Rate))
		gd := math.Pow(10, -3*float64(n)/(s.time*s.highs*Rate))
		// |b/(1 - a·e^-jw)| = gd with b = g0·(1-a): the root of the
		// quadratic in a between 0 and 1.
		qa, qb := g0*g0-gd*gd, g0*g0-gd*gd*math.Cos(w)
		a := 0.0
		if qa > 0 {
			a = (qb - math.Sqrt(qb*qb-qa*qa)) / qa
		}
		b := g0 * (1 - a)
		r.a[i], r.b[i] = float32(a), float32(b/4)
		for k := range sum {
			z := math.Pi * (float64(k) + 0.5) / float64(len(sum))
			h2 := b * b / (1 - 2*a*math.Cos(z) + a*a)
			sum[k] += 1 - h2
		}
	}
	// A side's output reads half the lines, each at the energy that comes
	// in over what a pass loses, on average over the lines: white noise in
	// plays 8/(1 - mean |h|²) as loud, each frequency on its own.
	var power float64
	for _, loss := range sum {
		power += fdnLines / 2 / (loss / fdnLines)
	}
	r.gain = float32(1 / math.Sqrt(power/float64(len(sum))))
	return r
}

// clear forgets what the room holds, as a seek or a new start does: it
// starts from silence, its input fading in.
func (r *room) clear() {
	clear(r.pre)
	r.preAt = 0
	for side := range r.diff {
		for j := range r.diff[side] {
			clear(r.diff[side][j].buf)
			r.diff[side][j].at = 0
		}
	}
	for i := range r.lines {
		clear(r.lines[i])
		r.at[i] = 0
	}
	r.low = [fdnLines]float32{}
	r.fed = 0
}

// render plays stereo frames in through the room into out, four of its
// sides a frame: left and right, then the left and right surround, which
// it plays only for surround. It returns the loudest of what came in and
// what came out.
func (r *room) render(in, out []float32, surround bool) float32 {
	var peak float32
	var h [fdnLines]float32
	for f := 0; f < len(in)/2; f++ {
		x, y := in[2*f], in[2*f+1]
		peak = max(peak, x, -x, y, -y)
		if r.fed < rampFrames {
			k := float32(r.fed) / rampFrames
			x, y = x*k, y*k
			r.fed++
		}
		if len(r.pre) > 0 {
			p := r.pre[r.preAt : r.preAt+2]
			x, p[0] = p[0], x
			y, p[1] = p[1], y
			if r.preAt += 2; r.preAt == len(r.pre) {
				r.preAt = 0
			}
		}
		for j := range r.diff[0] {
			x = r.diff[0][j].step(x)
			y = r.diff[1][j].step(y)
		}
		// The lines' outputs: the left's of the even ones, the right's
		// of the odd ones, each all added, and the surrounds' with every
		// other sign, unlike the fronts'.
		var l, rr, ls, rs float32
		for i := 0; i < fdnLines; i += 2 {
			o0, o1 := r.lines[i][r.at[i]], r.lines[i+1][r.at[i+1]]
			s := &fdnSigns[i/2]
			l += s[1] * o0
			rr += s[1] * o1
			ls += s[2] * o0
			rs += s[2] * o1
			lo0 := r.b[i]*o0 + r.a[i]*r.low[i]
			lo1 := r.b[i+1]*o1 + r.a[i+1]*r.low[i+1]
			if lo0 > -tiny && lo0 < tiny {
				lo0 = 0
			}
			if lo1 > -tiny && lo1 < tiny {
				lo1 = 0
			}
			r.low[i], r.low[i+1] = lo0, lo1
			h[i], h[i+1] = lo0, lo1
		}
		// The Hadamard matrix, unscaled, by butterflies.
		for s := 1; s < fdnLines; s <<= 1 {
			for i := 0; i < fdnLines; i += 2 * s {
				for j := i; j < i+s; j++ {
					h[j], h[j+s] = h[j]+h[j+s], h[j]-h[j+s]
				}
			}
		}
		// The left comes into the even lines, the right into the odd ones.
		for i := 0; i < fdnLines; i += 2 {
			s := &fdnSigns[i/2]
			v0, v1 := h[i]+s[0]*x, h[i+1]+s[0]*y
			if v0 > -tiny && v0 < tiny {
				v0 = 0
			}
			if v1 > -tiny && v1 < tiny {
				v1 = 0
			}
			r.lines[i][r.at[i]], r.lines[i+1][r.at[i+1]] = v0, v1
			if r.at[i]++; r.at[i] == len(r.lines[i]) {
				r.at[i] = 0
			}
			if r.at[i+1]++; r.at[i+1] == len(r.lines[i+1]) {
				r.at[i+1] = 0
			}
		}
		l, rr = l*r.gain, rr*r.gain
		peak = max(peak, l, -l, rr, -rr)
		o := out[4*f : 4*f+4]
		o[0], o[1] = l, rr
		if surround {
			o[2], o[3] = ls*r.gain, rs*r.gain
		}
	}
	return peak
}

// reverb is the stage of the reverb, on the audio thread.
type reverb struct {
	// set is what SetReverb gave.
	set Reverb
	// rooms are made by SetReverb as the reverb is first turned on.
	rooms *[roomCount]*room
	// play is the room that plays, and next, while fading, the one that
	// fades in over it, by fade's share; want is the room asked for,
	// which plays once the fade under way, if any, ends.
	play, next *room
	want       Room
	fading     bool
	fade       smoother
	wet, dry   smoother
	// rested is whether the stage was skipped: it starts again from
	// silence. stale is whether the output started again since it last
	// played, cut whether a seek asks it to fade its tail out over the
	// quantum and forget it.
	rested, stale, cut bool
	// quiet is how many frames the stage has held no sound above
	// quietLevel.
	quiet int
	// The quantum's input, mixed down to stereo, and what each room
	// plays of it.
	in         [2 * quantum]float32
	wetA, wetB [4 * quantum]float32
}

func newReverb() *reverb {
	return &reverb{wet: newSmoother(0, glide), dry: newSmoother(1, glide), fade: newSmoother(0, glide), rested: true}
}

// newRooms makes the rooms, off the audio thread.
func newRooms() *[roomCount]*room {
	var rs [roomCount]*room
	for i, s := range rooms {
		rs[i] = newRoom(s)
	}
	return &rs
}

// setTo has the stage glide to r.
func (f *reverb) setTo(r Reverb) {
	f.set = r
	f.wet.target, f.dry.target = 0, 1
	if r.On {
		f.wet.target, f.dry.target = min(max(r.Wet, 0), 1), min(max(r.Dry, 0), 1)
		f.want = min(max(r.Room, 0), roomCount-1)
	}
}

// resting reports whether the room plays nothing: its wet level rests at
// 0.
func (f *reverb) resting() bool { return f.rooms == nil || idle(&f.wet, 0) }

// ringing reports whether the room still plays a sound above quietLevel,
// as a tail after the music stopped.
func (f *reverb) ringing() bool {
	if f.resting() || f.play == nil {
		return false
	}
	span := f.play.span
	if f.fading {
		span = max(span, f.next.span)
	}
	return f.quiet < span
}

// process runs a quantum of frames of ch channels interleaved, 2 or 6,
// through the room: on 5.1, it hears the five main channels mixed down to
// two, as the engine's default downmix mixes them, and plays its fronts
// and its surrounds on theirs; the centre and the bass channel take the
// dry level alone.
func (f *reverb) process(buf []float32, ch int) {
	if f.resting() {
		f.rested, f.cut, f.quiet = true, false, 0
		if !idle(&f.dry, 1) {
			f.level(buf, ch)
		}
		return
	}
	if f.rested || f.stale {
		if f.rested {
			f.play, f.next, f.fading = f.rooms[f.want], nil, false
		}
		f.play.clear()
		if f.next != nil {
			f.next.clear()
		}
		f.rested, f.stale = false, false
	}
	if !f.fading && f.rooms[f.want] != f.play {
		f.next, f.fading = f.rooms[f.want], true
		f.next.clear()
		f.fade.value, f.fade.target = 0, 1
	}
	n := len(buf) / ch
	in := f.in[:2*n]
	if ch == 2 {
		copy(in, buf)
	} else {
		m := &defaultDownmix
		for i := range n {
			var l, r float32
			for c, v := range buf[i*ch : i*ch+ch] {
				l += m[0][c] * v
				r += m[1][c] * v
			}
			in[2*i], in[2*i+1] = l, r
		}
	}
	surround := ch == maxChannels
	peak := f.play.render(in, f.wetA[:4*n], surround)
	if f.fading {
		peak = max(peak, f.next.render(in, f.wetB[:4*n], surround))
	}
	for i := range n {
		w, d := float32(f.wet.next()), float32(f.dry.next())
		if f.cut {
			w *= float32(n-1-i) / float32(n)
		}
		v := f.wetA[4*i : 4*i+4]
		if f.fading {
			m := float32(f.fade.next())
			u := f.wetB[4*i : 4*i+4]
			for j := range v {
				v[j] += m * (u[j] - v[j])
			}
		}
		o := buf[i*ch : i*ch+ch]
		o[0] = d*o[0] + w*v[0]
		o[1] = d*o[1] + w*v[1]
		if surround {
			o[2] *= d
			o[3] *= d
			o[4] = d*o[4] + w*v[2]
			o[5] = d*o[5] + w*v[3]
		}
	}
	if f.fading && f.fade.settled() {
		f.play, f.next, f.fading = f.next, nil, false
	}
	if f.cut {
		// What the seek left behind fades out with the quantum, and the
		// room asked for starts from silence.
		f.play, f.next, f.fading, f.cut = f.rooms[f.want], nil, false, false
		f.play.clear()
		f.quiet = 0
		return
	}
	if peak*float32(f.wet.value) > quietLevel {
		f.quiet = 0
	} else {
		f.quiet += n
	}
}

// level plays a quantum at the dry level alone, while the room plays
// nothing.
func (f *reverb) level(buf []float32, ch int) {
	for i := 0; i < len(buf); i += ch {
		d := float32(f.dry.next())
		for c := range ch {
			buf[i+c] *= d
		}
	}
}

// SetReverb has the reverb glide to r: its room plays after the echo, at
// r's wet and dry levels, a new room fading in over the one before. Off,
// or at a wet level of 0, the stage is skipped once its levels glided
// there, and the samples come out as they went in. The first time the
// reverb is turned on, it makes its rooms, here rather than on the audio
// thread.
func (e *Engine) SetReverb(r Reverb) {
	e.mu.Lock()
	built := e.verb.rooms != nil
	e.mu.Unlock()
	var rs *[roomCount]*room
	if r.On && !built {
		rs = newRooms()
	}
	e.mu.Lock()
	if rs != nil && e.verb.rooms == nil {
		e.verb.rooms = rs
	}
	e.verb.setTo(r)
	e.mu.Unlock()
}

// Reverb returns what the reverb glides to, as SetReverb set it.
func (e *Engine) Reverb() Reverb {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.verb.set
}
