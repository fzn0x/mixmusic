package align_test

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/align"
	"github.com/fzn0x/mixmusic/internal/align/aligntest"
)

// offSpeed returns a video of the song of seed played at speed, as
// aligntest's takes are, after a scene of scene: quieter, its highs
// softer, with a hiss.
func offSpeed(seed uint64, speed float64, scene time.Duration) []float32 {
	take := aligntest.Song(aligntest.Take{Seed: seed, Length: 3 * time.Minute, Rate: rate, Speed: speed})
	return aligntest.Join(aligntest.Scene(seed+1, scene, rate), aligntest.Colour(take, 0.8, seed+2))
}

// foundAtSpeed fails t unless the song of seed, found in a video that
// plays it at speed after a scene of scene, has its first and last
// seconds where the video plays them, give or take 10 ms: a note at the
// song's t plays at scene + t / speed of the video.
func foundAtSpeed(t *testing.T, seed uint64, speed float64, scene time.Duration) {
	t.Helper()
	r, err := measure(t, song(seed), offSpeed(seed, speed, scene), rate)
	if err != nil {
		t.Fatalf("shift: %v", err)
	}
	for _, at := range []time.Duration{0, 3*time.Minute - time.Second} {
		got := r.Shift + time.Duration(float64(at)*r.Speed)
		want := scene + time.Duration(float64(at)/speed)
		if d := got - want; d < -10*time.Millisecond || d > 10*time.Millisecond {
			t.Errorf("the song's %v plays at %v, want %v: shift %v, speed %.6f, want %.6f: %s", at, got, want, r.Shift, r.Speed, 1/speed, r.Windows)
		}
	}
	t.Logf("shift %v, speed %.6f, want %.6f: %s", r.Shift, r.Speed, 1/speed, r.Windows)
}

// TestShiftOfATakeALittleSlower finds a song the video plays 0.1% slower,
// whose windows land 108 ms apart, past Agree, and times its last line
// as well as its first.
func TestShiftOfATakeALittleSlower(t *testing.T) {
	foundAtSpeed(t, 91, 0.999, 74690*time.Millisecond)
}

// TestShiftOfATakeALittleFaster finds a song the video plays 0.26%
// faster, whose windows land 280 ms apart.
func TestShiftOfATakeALittleFaster(t *testing.T) {
	foundAtSpeed(t, 101, 1.0026, 2900*time.Millisecond)
}

// TestShiftAtItsOwnSpeed tells a speed of 1 of a video that plays the
// song at its own.
func TestShiftAtItsOwnSpeed(t *testing.T) {
	s := song(111)
	r, err := measure(t, s, aligntest.Join(aligntest.Scene(112, 6061*time.Millisecond, rate), aligntest.Colour(s, 0.8, 113)), rate)
	if err != nil {
		t.Fatalf("shift: %v", err)
	}
	if r.Shift != 6061*time.Millisecond || r.Speed < 1-1e-5 || r.Speed > 1+1e-5 {
		t.Errorf("shift %v, speed %.6f: %s", r.Shift, r.Speed, r.Windows)
	}
}

// TestNoShiftOfATakeJustPastMaxSpeed finds no shift of a take 1.05%
// faster than the song, though its windows match clearly and on a line:
// its speed is past MaxSpeed. A take 0.95% faster is found.
func TestNoShiftOfATakeJustPastMaxSpeed(t *testing.T) {
	noMatch(t, song(121), offSpeed(121, 1.0105, 10*time.Second))
	r, err := measure(t, song(121), offSpeed(121, 1.0095, 10*time.Second), rate)
	if want := 1 / 1.0095; err != nil || r.Speed < want-1e-4 || r.Speed > want+1e-4 || r.Speed <= 1-align.MaxSpeed {
		t.Errorf("a take 0.95%% faster: speed %.6f, want %.6f, %v: %s", r.Speed, want, err, r.Windows)
	}
}

// overScene returns pcm with its stretch from from to to, at rate, a scene
// instead: a video that leaves the song for a scene there, and comes back
// to it in time.
func overScene(pcm []float32, from, to time.Duration, seed uint64) []float32 {
	a := int(int64(from) * rate / int64(time.Second))
	b := int(int64(to) * rate / int64(time.Second))
	return aligntest.Join(pcm[:a], aligntest.Scene(seed, to-from+time.Second, rate)[:b-a], pcm[b:])
}

// TestShiftOfTwoWindows finds the song at its own speed when only two
// windows match clearly, as a line fits any two: the video leaves the song
// for a scene over its third.
func TestShiftOfTwoWindows(t *testing.T) {
	video := overScene(offSpeed(131, 1, 8400*time.Millisecond), 141*time.Second, 165*time.Second, 134)
	r, err := measure(t, song(131), video, rate)
	if err != nil {
		t.Fatalf("shift: %v", err)
	}
	if d := r.Shift - 8400*time.Millisecond; d < -10*time.Millisecond || d > 10*time.Millisecond || r.Speed != 1 {
		t.Errorf("shift %v, speed %v, want 8.4s at 1: %s", r.Shift, r.Speed, r.Windows)
	}
	t.Logf("shift %v, speed %v: %s", r.Shift, r.Speed, r.Windows)
}

// TestNoShiftOfTwoWindowsOffSpeed finds no shift of a take 0.26% faster
// when only two windows match clearly, 140 ms apart: a speed fits any two,
// so it is taken only from all three. With the third, it is found.
func TestNoShiftOfTwoWindowsOffSpeed(t *testing.T) {
	whole := offSpeed(141, 1.0026, 2900*time.Millisecond)
	if _, err := measure(t, song(141), whole, rate); err != nil {
		t.Fatalf("the whole take: %v", err)
	}
	noMatch(t, song(141), overScene(whole, 136*time.Second, 160*time.Second, 144))
}
