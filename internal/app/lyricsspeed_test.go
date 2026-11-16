package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/align/aligntest"
	"github.com/fzn0x/mixmusic/lyrics"
)

// linesAt reports whether the lines of l, of lrclibSynced at 1 s and 4 s, are
// at shift + their time × speed, to the millisecond.
func linesAt(l lyrics.Lyrics, shift time.Duration, speed float64) bool {
	return len(l.Lines) == 2 && l.Lines[0].TimeMs == int(shift.Milliseconds())+int(1000*speed+0.5) && l.Lines[1].TimeMs == int(shift.Milliseconds())+int(4000*speed+0.5)
}

// TestLyricsTimedToAVideoOffSpeed times the lyrics of a video that plays
// its song of 3 minutes 0.26% faster, after a scene of 12.5 s, whose
// windows land 280 ms apart: the lines move to the shift plus their times
// at the video's speed, the note names it, and the speed is kept.
func TestLyricsTimedToAVideoOffSpeed(t *testing.T) {
	cache := t.TempDir()
	tv := newTimedVideo(t, 12500*time.Millisecond, cache)
	const speed = 1.0026 // as aligntest has it: align's is 1 / speed
	song := aligntest.Song(aligntest.Take{Seed: 15, Length: 3 * time.Minute, Rate: timedRate})
	faster := aligntest.Song(aligntest.Take{Seed: 15, Length: 3 * time.Minute, Rate: timedRate, Speed: speed})
	tv.audio["khnokW3Mw24"] = aligntest.WAV(timedRate, song)
	tv.audio["a5uQMwRMHcs"] = aligntest.WAV(timedRate, aligntest.Join(aligntest.Scene(16, 12500*time.Millisecond, timedRate), aligntest.Colour(faster, 0.8, 17), aligntest.Scene(18, 5*time.Second, timedRate)))
	if _, err := tv.c.Lyrics(t.Context(), instantCrushVideo); err != nil {
		t.Fatal(err)
	}
	tv.waitTold(t, instantCrushVideo.ID)
	k := keptTimings(t, cache)["a5uQMwRMHcs khnokW3Mw24"]
	if want := 1 / speed; !k.Matched || k.ShiftMs < 12490 || k.ShiftMs > 12510 || k.Speed < want-1e-4 || k.Speed > want+1e-4 {
		t.Fatalf("kept %+v, want a speed of %.6f", k, want)
	}
	l, err := tv.c.Lyrics(t.Context(), instantCrushVideo)
	if err != nil || !l.Synced || l.Note != "Timed to this video (+12.5 s, 0.26% faster)" || !linesAt(l, k.shift(), k.Speed) {
		t.Fatalf("timed: %+v, %v", l, err)
	}
	// The song's 2:59 lands where the video plays it, give or take 10 ms.
	lateAt := float64(179 * time.Second)
	late := k.shift() + time.Duration(lateAt*k.Speed)
	if d := late - 12500*time.Millisecond - time.Duration(lateAt/speed); d < -10*time.Millisecond || d > 10*time.Millisecond {
		t.Errorf("the song's 2:59 plays at %v, %v off", late, d)
	}
}

// keepTimings writes the timings kept to lyrics-timing.json in cache.
func keepTimings(t *testing.T, cache string, kept map[string]keptTiming) {
	t.Helper()
	data, err := json.Marshal(kept)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "lyrics-timing.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestLyricsTimedAsKept times the lyrics as lyrics-timing.json keeps
// them, reading no audio: a record #565 kept, without a speed, at the
// song's own, and one with a speed at that speed.
func TestLyricsTimedAsKept(t *testing.T) {
	for speed, note := range map[float64]string{
		0:      "Timed to this video (+12.5 s)",
		1.0026: "Timed to this video (+12.5 s, 0.26% slower)",
	} {
		cache := t.TempDir()
		keepTimings(t, cache, map[string]keptTiming{"a5uQMwRMHcs khnokW3Mw24": {At: time.Now().Unix(), Matched: true, ShiftMs: 12500, Speed: speed}})
		tv := newTimedVideo(t, time.Second, cache)
		want := speed
		if want == 0 {
			want = 1
		}
		l, err := tv.c.Lyrics(t.Context(), instantCrushVideo)
		if err != nil || !l.Synced || l.Note != note || !linesAt(l, 12500*time.Millisecond, want) {
			t.Errorf("kept at a speed of %v: %+v, %v", speed, l, err)
		}
		if len(tv.opened) != 0 {
			t.Errorf("kept at a speed of %v: opened the audio %v", speed, tv.opened)
		}
	}
}

// TestTimedNoteAtASpeed names how much slower or faster the video plays
// the song, to a hundredth of a percent, unless that is none.
func TestTimedNoteAtASpeed(t *testing.T) {
	for _, c := range []struct {
		shift time.Duration
		speed float64
		want  string
	}{
		{2900 * time.Millisecond, 1.0026, "Timed to this video (+2.9 s, 0.26% slower)"},
		{-1100 * time.Millisecond, 0.997413, "Timed to this video (-1.1 s, 0.26% faster)"},
		{74690 * time.Millisecond, 1.001017, "Timed to this video (+74.7 s, 0.1% slower)"},
		{-20 * time.Millisecond, 0.99, "Timed to this video (+0.0 s, 1% faster)"},
		{6061 * time.Millisecond, 1.000004, "Timed to this video (+6.1 s)"},
		{6061 * time.Millisecond, 0.99996, "Timed to this video (+6.1 s)"},
	} {
		if got := timedNote(c.shift, c.speed); got != c.want {
			t.Errorf("timedNote(%v, %v) = %q, want %q", c.shift, c.speed, got, c.want)
		}
	}
}
