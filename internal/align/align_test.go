package align_test

import (
	"errors"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/align"
	"github.com/fzn0x/mixmusic/internal/align/aligntest"
)

const rate = 8000

// lengthOf returns how long pcm lasts at rate.
func lengthOf(pcm []float32, rate int) time.Duration {
	return time.Duration(len(pcm)) * time.Second / time.Duration(rate)
}

// envelopeOf returns the envelope of the stretch s of pcm at rate.
func envelopeOf(pcm []float32, rate int, s align.Stretch) align.Envelope {
	a := int(int64(s.From) * int64(rate) / int64(time.Second))
	b := min(len(pcm), int(int64(s.To)*int64(rate)/int64(time.Second)))
	l := align.NewLevels(rate, time.Duration(a)*time.Second/time.Duration(rate))
	l.Write(pcm[a:b])
	return l.Envelope()
}

// measure finds where song starts in video, reading of each the stretches
// the plan asks for, as the app does.
func measure(t testing.TB, song, video []float32, rate int) (align.Result, error) {
	t.Helper()
	p, ok := align.NewPlan(lengthOf(song, rate), lengthOf(video, rate))
	if !ok {
		t.Fatalf("no plan of a song of %v and a video of %v", lengthOf(song, rate), lengthOf(video, rate))
	}
	windows := make([]align.Envelope, len(p.Windows))
	for i, w := range p.Windows {
		windows[i] = envelopeOf(song, rate, w)
	}
	return p.Shift(windows, envelopeOf(video, rate, p.Video))
}

// song is a song of 3 minutes, made from seed.
func song(seed uint64) []float32 {
	return aligntest.Song(aligntest.Take{Seed: seed, Length: 3 * time.Minute, Rate: rate})
}

// found fails t unless the song starts at want in the video, give or take
// 10 ms.
func found(t *testing.T, s, video []float32, rate int, want time.Duration) {
	t.Helper()
	foundNear(t, s, video, rate, want, 10*time.Millisecond)
}

// foundNear fails t unless the song starts at want in the video, give or
// take slack.
func foundNear(t *testing.T, s, video []float32, rate int, want, slack time.Duration) {
	t.Helper()
	r, err := measure(t, s, video, rate)
	if err != nil {
		t.Fatalf("shift: %v", err)
	}
	if d := r.Shift - want; d < -slack || d > slack {
		t.Errorf("shift = %v, want %v: %s", r.Shift, want, r.Windows)
	} else {
		t.Logf("shift = %v, %v off: %s", r.Shift, d.Abs(), r.Windows)
	}
}

// TestShiftOfAVideoThatOpensWithAScene finds the song 29.137 s into a
// video that opens with a scene and ends with another, its audio
// mastered apart: quieter, its highs softer, with a hiss.
func TestShiftOfAVideoThatOpensWithAScene(t *testing.T) {
	s := song(1)
	video := aligntest.Join(aligntest.Scene(2, 29137*time.Millisecond, rate), aligntest.Colour(s, 0.7, 3), aligntest.Scene(4, 12*time.Second, rate))
	found(t, s, video, rate, 29137*time.Millisecond)
}

// TestShiftOfAVideoWithoutTheSongsStart finds a song whose first 7.5 s
// the video leaves out: before the video's start.
func TestShiftOfAVideoWithoutTheSongsStart(t *testing.T) {
	s := song(11)
	video := aligntest.Join(aligntest.Colour(aligntest.Seconds(s, rate, 7500*time.Millisecond), 1.2, 5), aligntest.Scene(6, 15*time.Second, rate))
	found(t, s, video, rate, -7500*time.Millisecond)
}

// TestShiftOfAVideoLongerByItsOutro finds the song at the video's start,
// for a video longer only by what follows the song.
func TestShiftOfAVideoLongerByItsOutro(t *testing.T) {
	s := song(21)
	video := aligntest.Join(aligntest.Colour(s, 0.9, 7), aligntest.Scene(8, 40*time.Second, rate))
	found(t, s, video, rate, 0)
}

// TestShiftOfALongScene finds a song after a scene of two and a half
// minutes: the search reaches past the difference of the lengths.
func TestShiftOfALongScene(t *testing.T) {
	s := song(31)
	video := aligntest.Join(aligntest.Scene(9, 150*time.Second+3*time.Millisecond, rate), s)
	found(t, s, video, rate, 150*time.Second+3*time.Millisecond)
}

// TestShiftOfATakeOffTheBeat finds the shift of a take whose notes each
// fall up to 30 ms apart from the song's, at its tempo: the lyrics keep
// time with it as well as with the song.
func TestShiftOfATakeOffTheBeat(t *testing.T) {
	off := aligntest.Song(aligntest.Take{Seed: 1, Length: 3 * time.Minute, Rate: rate, Jitter: 30 * time.Millisecond})
	foundNear(t, song(1), aligntest.Join(aligntest.Scene(12, 10*time.Second, rate), off), rate, 10*time.Second, 30*time.Millisecond)
}

// TestShiftAt48kHz finds the shift of audio at the engine's rate as at
// 8 kHz.
func TestShiftAt48kHz(t *testing.T) {
	const rate = 48000
	s := aligntest.Song(aligntest.Take{Seed: 41, Length: time.Minute, Rate: rate})
	video := aligntest.Join(aligntest.Scene(10, 12345*time.Millisecond, rate), aligntest.Colour(s, 0.8, 11))
	found(t, s, video, rate, 12345*time.Millisecond)
}

// noMatch fails t unless the song isn't found in the video.
func noMatch(t *testing.T, s, video []float32) {
	t.Helper()
	r, err := measure(t, s, video, rate)
	if !errors.Is(err, align.ErrNoMatch) {
		t.Fatalf("shift = %v, %v; want no clear match: %s", r.Shift, err, r.Windows)
	}
	t.Log(err)
}

// TestNoShiftOfALiveTake finds no shift of a take that drifts ahead of
// the song's beat and back, as a band plays live.
func TestNoShiftOfALiveTake(t *testing.T) {
	live := aligntest.Song(aligntest.Take{Seed: 1, Length: 3 * time.Minute, Rate: rate, Jitter: 30 * time.Millisecond, Drift: 500 * time.Millisecond})
	noMatch(t, song(1), aligntest.Join(aligntest.Scene(12, 10*time.Second, rate), live))
}

// TestNoShiftOfAFasterTake finds no shift of a take 1.5% faster than the
// song.
func TestNoShiftOfAFasterTake(t *testing.T) {
	faster := aligntest.Song(aligntest.Take{Seed: 1, Length: 3 * time.Minute, Rate: rate, Speed: 1.015})
	noMatch(t, song(1), aligntest.Join(aligntest.Scene(13, 10*time.Second, rate), faster))
}

// TestNoShiftOfACutVerse finds no shift of a video that cuts 20 s out of
// the song's middle: its windows disagree.
func TestNoShiftOfACutVerse(t *testing.T) {
	s := song(51)
	noMatch(t, s, aligntest.Join(aligntest.Scene(14, 20*time.Second, rate), aligntest.Cut(s, rate, 80*time.Second, 100*time.Second)))
}

// TestNoShiftOfAnotherSong finds no shift of a video of another song.
func TestNoShiftOfAnotherSong(t *testing.T) {
	noMatch(t, song(61), aligntest.Join(aligntest.Scene(15, 10*time.Second, rate), song(62)))
}

// TestNoShiftOfALoop finds no shift of a song that plays one loop over
// and over: each window matches every loop alike.
func TestNoShiftOfALoop(t *testing.T) {
	loop := aligntest.Song(aligntest.Take{Seed: 71, Length: 3 * time.Minute, Rate: rate, Loop: true})
	noMatch(t, loop, aligntest.Join(aligntest.Scene(16, 20*time.Second, rate), loop))
}

// TestPlans compares nothing of a song too short for windows of
// MinOverlap, or of a video too long, and searches the reach either way
// of the lengths' difference.
func TestPlans(t *testing.T) {
	if _, ok := align.NewPlan(15*time.Second, 30*time.Second); ok {
		t.Error("a plan of a 15-second song")
	}
	if _, ok := align.NewPlan(3*time.Minute, 25*time.Minute); ok {
		t.Error("a plan of a video of 25 minutes")
	}
	p, ok := align.NewPlan(3*time.Minute, 3*time.Minute+29*time.Second)
	if !ok || p.Min != -align.Reach || p.Max != 29*time.Second+align.Reach || len(p.Windows) != 3 {
		t.Fatalf("plan = %+v, %v", p, ok)
	}
	for _, w := range p.Windows {
		if w.To-w.From < align.MinOverlap {
			t.Errorf("a window of %v", w.To-w.From)
		}
	}
	if p.Video.From != 0 || p.Video.To != 3*time.Minute+29*time.Second {
		t.Errorf("the video's stretch read is %v", p.Video)
	}
}

// BenchmarkShift measures the work of finding a song of 3:30 in a video
// of 4:10, from its audio at 48 kHz, as decoded.
func BenchmarkShift(b *testing.B) {
	const rate = 48000
	s := aligntest.Song(aligntest.Take{Seed: 81, Length: 210 * time.Second, Rate: rate})
	video := aligntest.Join(aligntest.Scene(17, 40*time.Second, rate), s, aligntest.Scene(18, 20*time.Second, rate))
	for b.Loop() {
		if _, err := measure(b, s, video, rate); err != nil {
			b.Fatal(err)
		}
	}
}
