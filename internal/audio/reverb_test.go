package audio

import (
	"math"
	"slices"
	"testing"
	"time"
)

// verbOf is a reverb at r, its rooms made and its levels glided there.
func verbOf(r Reverb) *reverb {
	f := newReverb()
	f.rooms = newRooms()
	f.setTo(r)
	f.wet.value, f.dry.value = f.wet.target, f.dry.target
	return f
}

// runVerb passes buf through f a quantum at a time, as the engine does.
func runVerb(f *reverb, buf []float32, ch int) []float32 {
	out := slices.Clone(buf)
	for i := 0; i < len(out); i += ch * quantum {
		f.process(out[i:min(i+ch*quantum, len(out))], ch)
	}
	return out
}

// roomNames name the rooms in the tests' messages.
var roomNames = [roomCount]string{"small room", "hall", "plate", "large space"}

// TestReverbNeutralIsSkipped leaves every sample as it was while the
// reverb is off, or on at a wet level of 0, at first and once a room
// glided back out, on two channels and on six.
func TestReverbNeutralIsSkipped(t *testing.T) {
	for _, ch := range []int{2, 6} {
		in := soundNoise(20*quantum, ch, 1)
		if got := runVerb(newReverb(), in, ch); !slices.Equal(got, in) {
			t.Errorf("%d channels: a new reverb changed the samples", ch)
		}
		if got := runVerb(verbOf(Reverb{On: true, Room: RoomHall, Dry: 1}), in, ch); !slices.Equal(got, in) {
			t.Errorf("%d channels: a wet level of 0 changed the samples", ch)
		}
		f := verbOf(Reverb{On: true, Room: RoomLarge, Wet: 0.6, Dry: 0.7})
		runVerb(f, soundNoise(Rate/2, ch, 2), ch)
		f.setTo(Reverb{})
		runVerb(f, soundNoise(Rate, ch, 3), ch) // fifty time constants
		if got := runVerb(f, in, ch); !slices.Equal(got, in) {
			t.Errorf("%d channels: off again, the reverb still changes the samples", ch)
		}
	}
}

// impulse is n frames of ch channels, silent but for 1 on channel c at
// frame at: after the 5 ms a room's input fades in over.
func impulse(n, ch, c, at int) []float32 {
	buf := make([]float32, n*ch)
	buf[at*ch+c] = 1
	return buf
}

// TestReverbImpulse plays an impulse on the left through each room: the
// dry sound at once, then nothing until the pre-delay and the shortest
// line have passed, when the left answers; the right answers a line
// later, as the sound crosses over. Soon after, every sample of the tail
// sounds.
func TestReverbImpulse(t *testing.T) {
	at := rampFrames
	for i, s := range rooms {
		got := runVerb(verbOf(Reverb{On: true, Room: Room(i), Wet: 1, Dry: 0.5}), impulse(Rate, 2, 0, at), 2)
		m := lineLengths(s.size)
		pre := int(math.Round(s.predelay * Rate))
		first := func(c int) int {
			for f := at + 1; f < Rate; f++ {
				if got[2*f+c] != 0 {
					return f - at
				}
			}
			return -1
		}
		if got[2*at] != 0.5 || got[2*at+1] != 0 {
			t.Errorf("%s: the impulse plays %v and %v, want 0.5 and 0", roomNames[i], got[2*at], got[2*at+1])
		}
		if f, want := first(0), pre+m[0]; f != want {
			t.Errorf("%s: the left answers after %d frames, want %d", roomNames[i], f, want)
		}
		if f, want := first(1), pre+m[0]+m[1]; f != want {
			t.Errorf("%s: the right answers after %d frames, want %d", roomNames[i], f, want)
		}
		from := at + pre + 3*m[fdnLines-1]
		for f := from; f < from+Rate/10; f++ {
			if got[2*f] == 0 || got[2*f+1] == 0 {
				t.Fatalf("%s: frame %d of the tail is silent", roomNames[i], f-at)
			}
		}
	}
}

// octave is buf, of stereo frames, through an octave's band about hz.
func octave(buf []float32, hz float64) []float32 {
	b := slices.Clone(buf)
	var lp, hp biquad
	lp.set(lowPass, hz*math.Sqrt2, math.Sqrt2/2, 0)
	hp.set(highPass, hz/math.Sqrt2, math.Sqrt2/2, 0)
	for range 2 {
		lp.process(b, 2)
		hp.process(b, 2)
	}
	return b
}

// t30 is the reverb time of stereo ir, from the fall of its energy from
// -5 to -35 dB, by Schroeder's backward integration, fitted by least
// squares.
func t30(ir []float32) float64 {
	n := len(ir) / 2
	edc := make([]float64, n)
	var acc float64
	for f := n - 1; f >= 0; f-- {
		acc += float64(ir[2*f])*float64(ir[2*f]) + float64(ir[2*f+1])*float64(ir[2*f+1])
		edc[f] = acc
	}
	var sx, sy, sxx, sxy, k float64
	for f := range n {
		db := 10 * math.Log10(edc[f]/edc[0])
		if db <= -5 && db >= -35 {
			x := float64(f) / Rate
			sx, sy, sxx, sxy, k = sx+x, sy+db, sxx+x*x, sxy+x*db, k+1
		}
	}
	slope := (k*sxy - sx*sy) / (k*sxx - sx*sx)
	return -60 / slope
}

// TestReverbDecayTimes measures each room's reverb time from its impulse
// response: at the middle notes, the octave about 500 Hz, within 10% of
// the room's; the high notes, the octave about 4 kHz, fall sooner, by
// about the room's share for them.
func TestReverbDecayTimes(t *testing.T) {
	for i, s := range rooms {
		n := int((1.6*s.time + 0.2) * Rate)
		got := runVerb(verbOf(Reverb{On: true, Room: Room(i), Wet: 1}), impulse(n, 2, 0, rampFrames), 2)
		mid, high := t30(octave(got, 500)), t30(octave(got, 4000))
		if math.Abs(mid/s.time-1) > 0.1 {
			t.Errorf("%s: %.2f s at 500 Hz, want %.2f s", roomNames[i], mid, s.time)
		}
		if share := high / mid; share < s.highs-0.1 || share > s.highs+0.2 {
			t.Errorf("%s: %.2f s at 4 kHz, %.2f of the middle's, want about %.2f", roomNames[i], high, share, s.highs)
		}
	}
	if rooms[RoomSmall].time > 0.7 || rooms[RoomLarge].time < 3.5 {
		t.Error("the rooms don't go from about 0.6 s to 4 s")
	}
}

// power is the mean square of buf's samples of channel c of ch, from
// frame from on.
func power(buf []float32, ch, c, from int) float64 {
	var p float64
	n := 0
	for i := from*ch + c; i < len(buf); i += ch {
		p += float64(buf[i]) * float64(buf[i])
		n++
	}
	return p / float64(n)
}

// correlation is the correlation of channels a and b of ch, from frame
// from on.
func correlation(buf []float32, ch, a, b, from int) float64 {
	var ab float64
	for i := from * ch; i < len(buf); i += ch {
		ab += float64(buf[i+a]) * float64(buf[i+b])
	}
	return ab / float64(len(buf)/ch-from) / math.Sqrt(power(buf, ch, a, from)*power(buf, ch, b, from))
}

// TestReverbLevel plays white noise through each room alone, at a wet
// level of 1, as loud as it came in, within half a decibel, its sides
// unlike each other.
func TestReverbLevel(t *testing.T) {
	for i, s := range rooms {
		in := soundNoise(int((s.time+2)*Rate), 2, 4)
		got := runVerb(verbOf(Reverb{On: true, Room: Room(i), Wet: 1}), in, 2)
		from := int(s.time * Rate)
		for c := range 2 {
			if db := 10 * math.Log10(power(got, 2, c, from)/power(in, 2, c, from)); math.Abs(db) > 0.5 {
				t.Errorf("%s, channel %d: %.2f dB", roomNames[i], c, db)
			}
		}
		if r := correlation(got, 2, 0, 1, from); math.Abs(r) > 0.1 {
			t.Errorf("%s: the sides correlate by %.2f", roomNames[i], r)
		}
	}
}

// TestReverbWetAndDry keeps the levels apart: the dry sound at its own,
// the room at its own, as a sound mood sets them; a mix gives both, by
// an equal-power crossfade.
func TestReverbWetAndDry(t *testing.T) {
	in := soundNoise(Rate/2, 2, 5)
	room := runVerb(verbOf(Reverb{On: true, Room: RoomHall, Wet: 1}), in, 2)
	got := runVerb(verbOf(Reverb{On: true, Room: RoomHall, Wet: 0.5, Dry: 0.8}), in, 2)
	for i := range got {
		if want := 0.8*float64(in[i]) + 0.5*float64(room[i]); math.Abs(float64(got[i])-want) > 1e-5 {
			t.Fatalf("sample %d: %v, want %v", i, got[i], want)
		}
	}
	for _, c := range []struct{ mix, wet, dry float64 }{{0, 0, 1}, {1, 1, 0}, {-1, 0, 1}, {2, 1, 0}} {
		if w, d := MixLevels(c.mix); w != c.wet || d != c.dry {
			t.Errorf("a mix of %v: %v and %v, want %v and %v", c.mix, w, d, c.wet, c.dry)
		}
	}
	for _, m := range []float64{0.1, 0.25, 0.5, 0.9} {
		w, d := MixLevels(m)
		if math.Abs(w*w+d*d-1) > 1e-12 || w <= 0 || d <= 0 {
			t.Errorf("a mix of %v: %v and %v, not of equal power", m, w, d)
		}
	}
	if w, d := MixLevels(0.5); math.Abs(w-d) > 1e-15 {
		t.Errorf("a mix of 0.5: %v and %v", w, d)
	}
}

// toneSteps is the largest step from one sample to the next of each of
// buf's stretches of n frames, on any channel of ch: into each frame of
// the stretch, from the one before.
func toneSteps(buf []float32, ch, n int) []float64 {
	var steps []float64
	for from := 0; from < len(buf); from += n * ch {
		largest := 0.0
		for i := max(from, ch); i < min(from+n*ch, len(buf)); i++ {
			largest = max(largest, math.Abs(float64(buf[i]-buf[i-ch])))
		}
		steps = append(steps, largest)
	}
	return steps
}

// TestReverbGlides turns the reverb on, changes its mix and its room, and
// turns it off, as a tone plays: none of the changes steps the sound by
// more than half again the most it steps playing steadily, before or
// after, so none clicks.
func TestReverbGlides(t *testing.T) {
	const second = Rate
	tone := soundTone(13*second, 2, 220, [maxChannels]float64{0.5, 0.3}, 0)
	f := verbOf(Reverb{})
	changes := []struct {
		at int
		r  Reverb
	}{
		{1, Reverb{On: true, Room: RoomHall, Wet: 0.5, Dry: 0.8}},
		{4, Reverb{On: true, Room: RoomHall, Wet: 0.9, Dry: 0.4}},
		{7, Reverb{On: true, Room: RoomPlate, Wet: 0.9, Dry: 0.4}},
		{10, Reverb{}},
	}
	got := slices.Clone(tone)
	next := 0
	for i := 0; i < len(got); i += 2 * quantum {
		if next < len(changes) && i >= 2*changes[next].at*second {
			f.setTo(changes[next].r)
			next++
		}
		f.process(got[i:min(i+2*quantum, len(got))], 2)
	}
	// The steps of each tenth of a second.
	steps := toneSteps(got, 2, second/10)
	for _, c := range changes {
		at := 10 * c.at
		// Steady: the second before the change, and the last half second
		// before the next, by when the room has filled.
		before, after := slices.Max(steps[at-10:at]), slices.Max(steps[at+25:at+30])
		if before == 0 || after == 0 {
			t.Fatalf("to %+v: no tone", c.r)
		}
		if during := slices.Max(steps[at : at+5]); during > 1.5*max(before, after) {
			t.Errorf("to %+v: a step of %.4f, against %.4f before and %.4f after", c.r, during, before, after)
		}
	}
}

// TestReverbFiveOne has the room hear the five main channels of 5.1 and
// play on the fronts and the surrounds, each pair unlike the other; the
// centre and the bass channel take the dry level alone.
func TestReverbFiveOne(t *testing.T) {
	const n = 2 * Rate
	at := rampFrames
	got := runVerb(verbOf(Reverb{On: true, Room: RoomHall, Wet: 1, Dry: 0.5}), impulse(n, 6, 2, at), 6)
	for f := range n {
		centre, lfe := got[6*f+2], got[6*f+3]
		want := float32(0)
		if f == at {
			want = 0.5
		}
		if centre != want || lfe != 0 {
			t.Fatalf("frame %d: the centre plays %v and the bass channel %v", f, centre, lfe)
		}
	}
	for _, c := range []int{0, 1, 4, 5} {
		if power(got, 6, c, at+1) == 0 {
			t.Errorf("channel %d hears nothing of the centre's room", c)
		}
	}
	in := soundNoise(3*Rate, 6, 6)
	for i := 3; i < len(in); i += 6 {
		in[i] = 0 // the room doesn't hear the bass channel
	}
	got = runVerb(verbOf(Reverb{On: true, Room: RoomHall, Wet: 1}), in, 6)
	for _, p := range [][2]int{{0, 4}, {1, 5}, {0, 1}, {4, 5}} {
		if r := correlation(got, 6, p[0], p[1], Rate); math.Abs(r) > 0.15 {
			t.Errorf("channels %d and %d correlate by %.2f", p[0], p[1], r)
		}
	}
	if db := 10 * math.Log10(power(got, 6, 4, Rate)/power(got, 6, 0, Rate)); math.Abs(db) > 1 {
		t.Errorf("the surrounds play %.2f dB against the fronts", db)
	}
}

// TestReverbCut fades the tail out over the quantum a seek asks it to,
// and leaves nothing of it after; an output that started again starts
// from silence too.
func TestReverbCut(t *testing.T) {
	silence := make([]float32, 2*quantum)
	for _, how := range []string{"cut", "stale"} {
		f := verbOf(Reverb{On: true, Room: RoomLarge, Wet: 1, Dry: 1})
		runVerb(f, soundNoise(Rate, 2, 7), 2)
		if got := runVerb(f, silence, 2); slices.Max(got) == 0 {
			t.Fatal("no tail after the noise")
		}
		if how == "cut" {
			f.cut = true
			got := runVerb(f, silence, 2)
			if got[len(got)-2] != 0 || got[len(got)-1] != 0 {
				t.Errorf("the quantum of the seek ends at %v and %v", got[len(got)-2], got[len(got)-1])
			}
			// It fades over the quantum: its last quarter is far softer
			// than its first.
			q := len(got) / 4
			if first, last := power(got[:q], 1, 0, 0), power(got[3*q:], 1, 0, 0); last > first/10 {
				t.Errorf("the tail doesn't fade over the seek's quantum: %v, then %v", first, last)
			}
		} else {
			f.stale = true
		}
		if got := runVerb(f, make([]float32, 2*Rate), 2); slices.Max(got) != 0 || slices.Min(got) != 0 {
			t.Errorf("%s: the tail plays on", how)
		}
	}
}

// subnormal reports whether v is a subnormal float32, slow to compute.
func subnormal(v float32) bool {
	return v != 0 && math.Abs(float64(v)) < math.SmallestNonzeroFloat32*(1<<23)
}

// roomState is every number a room keeps.
func roomState(r *room) []float32 {
	s := slices.Clone(r.pre)
	for side := range r.diff {
		for j := range r.diff[side] {
			s = append(s, r.diff[side][j].buf...)
		}
	}
	for i := range r.lines {
		s = append(s, r.lines[i]...)
	}
	return append(s, r.low[:]...)
}

// TestReverbNoSubnormals plays each room's tail down to silence: none of
// what it keeps ever becomes a subnormal number, which would slow the
// processor down on the silence after music, and in the end it keeps only
// zeros.
func TestReverbNoSubnormals(t *testing.T) {
	for i, s := range rooms {
		f := verbOf(Reverb{On: true, Room: Room(i), Wet: 1})
		buf := impulse(rampFrames+quantum, 2, 0, rampFrames)
		runVerb(f, buf, 2)
		// Without the flush, the tail reaches float32's subnormals, 760 dB
		// down, in 760/60 reverb times.
		silence := make([]float32, Rate)
		for range int(13*s.time) + 2 {
			runVerb(f, silence, 2)
			if slices.ContainsFunc(roomState(f.play), subnormal) {
				t.Fatalf("%s: a subnormal number in the tail", roomNames[i])
			}
		}
		if slices.ContainsFunc(roomState(f.play), func(v float32) bool { return v != 0 }) {
			t.Errorf("%s: the tail never went to silence", roomNames[i])
		}
		if f.ringing() {
			t.Errorf("%s: the reverb rings in silence", roomNames[i])
		}
	}
}

// TestReverbRinging tells the engine the room still sounds while its
// tail does, and not once it fell under -100 dBFS.
func TestReverbRinging(t *testing.T) {
	f := verbOf(Reverb{On: true, Room: RoomHall, Wet: 0.5, Dry: 1})
	runVerb(f, soundNoise(Rate/2, 2, 8), 2)
	silence := make([]float32, 2*quantum)
	rang := 0
	for f.ringing() && rang < 20*Rate {
		got := runVerb(f, silence, 2)
		if peak := max(slices.Max(got), -slices.Min(got)); peak > 1e-3 && !f.ringing() {
			t.Fatalf("a tail of %v doesn't ring", peak)
		}
		rang += quantum
	}
	// From about -15 dB to -100 dB, at 60 dB in 2.4 s.
	if s := float64(rang) / Rate; s < 2.5 || s > 5 {
		t.Errorf("the tail rang %.2f s", s)
	}
	if got := runVerb(f, silence, 2); max(slices.Max(got), -slices.Min(got)) > quietLevel {
		t.Errorf("a tail of %v once it stopped ringing", max(slices.Max(got), -slices.Min(got)))
	}
	if verbOf(Reverb{}).ringing() {
		t.Error("a reverb that's off rings")
	}
}

// TestEngineReverbTail keeps the output on while the reverb's tail sounds
// after a pause, and lets it go once the tail is silent; a seek fades the
// tail out, and so does an output that starts again.
func TestEngineReverbTail(t *testing.T) {
	e, dev, _ := newTestEngine(t)
	e.SetReverb(Reverb{On: true, Room: RoomHall, Wet: 0.7, Dry: 0.7})
	if got := e.Reverb(); got != (Reverb{On: true, Room: RoomHall, Wet: 0.7, Dry: 0.7}) {
		t.Errorf("Reverb() is %+v", got)
	}
	if err := e.Load(cuesFront, Cue{Autoplay: true}); err != nil {
		t.Fatal(err)
	}
	buffered(t, e, 0, ringFrames/2)
	dev.play(t, Rate/2)
	e.Seek(time.Second)
	e.mu.Lock()
	cut := e.verb.cut && e.echo.cut
	e.mu.Unlock()
	if !cut {
		t.Error("the seek didn't fade the tails out")
	}
	buffered(t, e, 0, ringFrames/2)
	dev.play(t, Rate/2)
	e.Pause()
	// A second and a fifth after the pause, the tail still sounds, and the
	// output stays.
	got := dev.play(t, Rate+Rate/5)
	tail := got[len(got)-2*quantum:]
	if max(slices.Max(tail), -slices.Min(tail)) < 1e-4 {
		t.Fatal("no tail after the pause")
	}
	e.report()
	if dev.isStopped() {
		t.Fatal("the output stopped while the tail sounds")
	}
	// Ten seconds on, the tail is long silent: the output goes.
	dev.play(t, 10*Rate)
	e.report()
	if !dev.isStopped() {
		t.Fatal("the output still plays after the tail")
	}
	if err := e.Play(); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	stale := e.verb.stale && e.echo.stale
	e.mu.Unlock()
	if !stale {
		t.Error("the output started again with the tails it had")
	}
}
