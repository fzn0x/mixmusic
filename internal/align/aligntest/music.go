// Package aligntest makes audio that sounds much as music does, to test
// what measures recordings against each other without recordings of
// anyone's songs: drums on a beat, a bass and a tune of random notes, all
// from a seed. A take of a song may play it faster, or each note a little
// off its time, as a live take does.
package aligntest

import (
	"encoding/binary"
	"math"
	"math/rand/v2"
	"time"
)

// Take is how a song made from a seed is played.
type Take struct {
	// Seed makes the song: the same seed, the same notes.
	Seed uint64
	// Length is how long the song is, at its own speed.
	Length time.Duration
	// Rate is the sample rate of the audio made.
	Rate int
	// Speed plays it faster, above 1, or slower; 0 is 1.
	Speed float64
	// Jitter moves each note by up to that much either way, as a live take
	// plays it, apart from the beat.
	Jitter time.Duration
	// Drift moves the notes slowly ahead of the beat and back, by up to
	// that much, a cycle a minute, as a band plays live.
	Drift time.Duration
	// Loop plays its first two bars over and over, as a song of one loop.
	Loop bool
}

// note is a sound of a song: when it starts, how long it lasts, what it is
// and how loud.
type note struct {
	at, length time.Duration
	kind       int
	pitch      float64
	gain       float64
}

const (
	kick = iota
	snare
	hat
	bass
	tune
)

// score returns the notes of the song of seed, length long at its own
// speed.
func score(seed uint64, length time.Duration, loop bool) []note {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	beat := time.Duration(float64(time.Minute) / (96 + r.Float64()*40))
	scale := []float64{0, 2, 4, 7, 9, 12, 14, 16, 19}
	root := 45 + r.IntN(8) // a MIDI note, A2 or above
	var notes []note
	bars := int(length/(4*beat)) + 1
	if loop {
		bars = 2
	}
	for bar := range bars {
		at := time.Duration(bar) * 4 * beat
		loud := 0.6 + 0.4*r.Float64()
		for b := range 4 {
			t := at + time.Duration(b)*beat
			if b%2 == 0 || r.IntN(5) == 0 {
				notes = append(notes, note{at: t, length: 150 * time.Millisecond, kind: kick, gain: loud})
			}
			if b%2 == 1 {
				notes = append(notes, note{at: t, length: 120 * time.Millisecond, kind: snare, gain: loud * 0.7})
			}
			for h := range 2 {
				if r.IntN(6) > 0 {
					notes = append(notes, note{at: t + time.Duration(h)*beat/2, length: 40 * time.Millisecond, kind: hat, gain: loud * (0.2 + 0.3*r.Float64())})
				}
			}
			notes = append(notes, note{at: t, length: beat * 9 / 10, kind: bass, pitch: midi(root - 12 + int(scale[r.IntN(4)])), gain: loud * 0.5})
		}
		for t := at; t < at+4*beat; {
			step := beat / 2 * time.Duration(1+r.IntN(3))
			if r.IntN(5) > 0 {
				notes = append(notes, note{at: t, length: step, kind: tune, pitch: midi(root + 12 + int(scale[r.IntN(len(scale))])), gain: loud * (0.25 + 0.2*r.Float64())})
			}
			t += step
		}
	}
	if loop {
		two, first := 2*4*beat, notes
		for at := two; at < length; at += two {
			for _, n := range first {
				n.at += at
				notes = append(notes, n)
			}
		}
	}
	return notes
}

func midi(n int) float64 { return 440 * math.Pow(2, float64(n-69)/12) }

// Song returns the audio of a take, mono.
func Song(take Take) []float32 {
	speed := take.Speed
	if speed == 0 {
		speed = 1
	}
	length := time.Duration(float64(take.Length) / speed)
	out := make([]float32, int(int64(length)*int64(take.Rate)/int64(time.Second)))
	jitter := rand.New(rand.NewPCG(take.Seed+1, 7))
	noise := rand.New(rand.NewPCG(take.Seed+2, 11))
	for _, n := range score(take.Seed, take.Length, take.Loop) {
		at := time.Duration(float64(n.at) / speed)
		if take.Drift > 0 {
			at += time.Duration(float64(take.Drift) * math.Sin(2*math.Pi*n.at.Minutes()))
		}
		if take.Jitter > 0 {
			at += time.Duration((jitter.Float64()*2 - 1) * float64(take.Jitter))
		}
		play(out, take.Rate, at, n, noise)
	}
	return out
}

// play adds the sound of n to out, at at.
func play(out []float32, rate int, at time.Duration, n note, noise *rand.Rand) {
	start := int(int64(at) * int64(rate) / int64(time.Second))
	count := int(int64(n.length) * int64(rate) / int64(time.Second))
	var lp float64 // a one-pole low-pass of the noise, for the snare
	for i := range count {
		j := start + i
		if j < 0 || j >= len(out) {
			continue
		}
		t := float64(i) / float64(rate)
		left := 1 - float64(i)/float64(count)
		var v float64
		switch n.kind {
		case kick:
			v = math.Sin(2*math.Pi*(50*t+30*(1-math.Exp(-t*30))/30)) * math.Exp(-t*18)
		case snare:
			lp += 0.5 * (noise.Float64()*2 - 1 - lp)
			v = (0.7*lp + 0.3*math.Sin(2*math.Pi*190*t)) * math.Exp(-t*25)
		case hat:
			v = (noise.Float64()*2 - 1) * math.Exp(-t*90)
		case bass, tune:
			attack := min(1, t*200)
			for h, w := range []float64{1, 0.5, 0.25} {
				v += w * math.Sin(2*math.Pi*n.pitch*float64(h+1)*t)
			}
			v *= attack * math.Exp(-t*3) * left
		}
		out[j] += float32(v * n.gain * 0.4)
	}
}

// Scene returns audio that isn't music, length long at rate: a rumble, and
// bursts of noise of random lengths, as voices and steps in a scene.
func Scene(seed uint64, length time.Duration, rate int) []float32 {
	r := rand.New(rand.NewPCG(seed, 3))
	out := make([]float32, int(int64(length)*int64(rate)/int64(time.Second)))
	var brown, burst float64
	left := 0
	for i := range out {
		brown = 0.995*brown + 0.02*(r.Float64()*2-1)
		if left == 0 {
			left = rate/20 + r.IntN(rate/2)
			burst = 0
			if r.IntN(2) == 0 {
				burst = 0.05 + 0.15*r.Float64()
			}
		}
		left--
		out[i] = float32(brown + burst*(r.Float64()*2-1))
	}
	return out
}

// Join returns the audio of parts one after another.
func Join(parts ...[]float32) []float32 {
	var out []float32
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Colour returns pcm as another mastering may have it: louder or quieter
// by gain, its highs softened, with a hiss under it.
func Colour(pcm []float32, gain float64, seed uint64) []float32 {
	r := rand.New(rand.NewPCG(seed, 5))
	out := make([]float32, len(pcm))
	var lp float64
	for i, v := range pcm {
		lp += 0.6 * (float64(v) - lp)
		out[i] = float32(gain*lp + 0.003*(r.Float64()*2-1))
	}
	return out
}

// Cut returns pcm without its stretch from from to to, at rate.
func Cut(pcm []float32, rate int, from, to time.Duration) []float32 {
	a := int(int64(from) * int64(rate) / int64(time.Second))
	b := int(int64(to) * int64(rate) / int64(time.Second))
	return Join(pcm[:a], pcm[b:])
}

// Seconds returns pcm from from on, at rate.
func Seconds(pcm []float32, rate int, from time.Duration) []float32 {
	return pcm[int(int64(from)*int64(rate)/int64(time.Second)):]
}

// WAV returns a WAV file of mono pcm at rate, of 16-bit samples.
func WAV(rate int, pcm []float32) []byte {
	data := make([]byte, 44+2*len(pcm))
	copy(data, "RIFF")
	binary.LittleEndian.PutUint32(data[4:], uint32(36+2*len(pcm)))
	copy(data[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(data[16:], 16)
	binary.LittleEndian.PutUint16(data[20:], 1)
	binary.LittleEndian.PutUint16(data[22:], 1)
	binary.LittleEndian.PutUint32(data[24:], uint32(rate))
	binary.LittleEndian.PutUint32(data[28:], uint32(rate*2))
	binary.LittleEndian.PutUint16(data[32:], 2)
	binary.LittleEndian.PutUint16(data[34:], 16)
	copy(data[36:], "data")
	binary.LittleEndian.PutUint32(data[40:], uint32(2*len(pcm)))
	for i, v := range pcm {
		binary.LittleEndian.PutUint16(data[44+2*i:], uint16(int16(max(-1, min(1, v))*32767)))
	}
	return data
}
