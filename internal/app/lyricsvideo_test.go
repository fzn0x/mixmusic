package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/align/aligntest"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// timedVideo is the music video of Instant Crush and its song, as
// videoLyrics has them, whose audio the timing reads made from seeds: the
// song a minute long, and the video a scene before it, as another
// mastering has the song, and a scene after.
type timedVideo struct {
	*videoLyrics
	mu sync.Mutex
	// audio is the WAV file of each video ID, and opened counts its opens.
	audio  map[string][]byte
	opened map[string]int
	// fail fails the opens, while set.
	fail error
	told chan string
}

const timedRate = 8000

func newTimedVideo(t *testing.T, scene time.Duration, cache string) *timedVideo {
	t.Helper()
	v := newVideoLyrics(t)
	v.c.a.opts.CacheDir = cache
	song := aligntest.Song(aligntest.Take{Seed: 5, Length: time.Minute, Rate: timedRate})
	video := aligntest.Join(aligntest.Scene(6, scene, timedRate), aligntest.Colour(song, 0.8, 7), aligntest.Scene(8, 5*time.Second, timedRate))
	tv := &timedVideo{videoLyrics: v, opened: map[string]int{}, told: make(chan string, 8), audio: map[string][]byte{
		"khnokW3Mw24": aligntest.WAV(timedRate, song),
		"a5uQMwRMHcs": aligntest.WAV(timedRate, video),
	}}
	v.c.a.lyricsTiming.open = tv.open
	t.Cleanup(v.c.a.OnLyricsTimed(func(id string) { tv.told <- id }))
	return tv
}

func (tv *timedVideo) open(ctx context.Context, videoID string) (io.ReadSeekCloser, error) {
	tv.mu.Lock()
	defer tv.mu.Unlock()
	tv.opened[videoID]++
	if tv.fail != nil {
		return nil, tv.fail
	}
	data, ok := tv.audio[videoID]
	if !ok {
		return nil, errors.New("no such video")
	}
	return readSeekNopCloser{bytes.NewReader(data)}, nil
}

// opens returns how many times the audio of videoID opened.
func (tv *timedVideo) opens(videoID string) int {
	tv.mu.Lock()
	defer tv.mu.Unlock()
	return tv.opened[videoID]
}

type readSeekNopCloser struct{ io.ReadSeeker }

func (readSeekNopCloser) Close() error { return nil }

// waitTold waits until the windows are told the lyrics of trackID changed.
func (tv *timedVideo) waitTold(t *testing.T, trackID string) {
	t.Helper()
	select {
	case id := <-tv.told:
		if id != trackID {
			t.Fatalf("told of %s, not %s", id, trackID)
		}
	case <-time.After(time.Minute):
		t.Fatal("the timing never ended")
	}
}

// waitToldOf waits until the windows are told the lyrics of each of ids
// changed, in any order.
func (tv *timedVideo) waitToldOf(t *testing.T, ids ...string) {
	t.Helper()
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	for range ids {
		select {
		case id := <-tv.told:
			if !want[id] {
				t.Fatalf("told of %s, not of %v", id, ids)
			}
			delete(want, id)
		case <-time.After(time.Minute):
			t.Fatalf("never told of %v", want)
		}
	}
}

// keptTimings returns the timings lyrics-timing.json in cache keeps.
func keptTimings(t *testing.T, cache string) map[string]keptTiming {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cache, "lyrics-timing.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var kept map[string]keptTiming
	if err := json.Unmarshal(data, &kept); err != nil {
		t.Fatal(err)
	}
	return kept
}

// near reports whether the times of the lines of l are those of
// lrclibSynced, 1 s and 4 s, moved by shift, give or take 10 ms.
func near(l lyrics.Lyrics, shift time.Duration) bool {
	if len(l.Lines) != 2 {
		return false
	}
	for i, at := range []time.Duration{time.Second, 4 * time.Second} {
		if d := time.Duration(l.Lines[i].TimeMs)*time.Millisecond - (at + shift); d < -10*time.Millisecond || d > 10*time.Millisecond {
			return false
		}
	}
	return true
}

// TestLyricsTimedToAVideo shows a music video's lyrics with the song's
// times while it finds where the song starts in the video, saying so,
// then moved 12.5 s later, as the video opens with a scene of 12.5 s,
// once the windows are told. The shift is kept on disk: the next run
// times them at once, reading no audio.
func TestLyricsTimedToAVideo(t *testing.T) {
	cache := t.TempDir()
	tv := newTimedVideo(t, 12500*time.Millisecond, cache)
	l, err := tv.c.Lyrics(t.Context(), instantCrushVideo)
	if err != nil || !l.Synced || l.Note != timingNote || !near(l, 0) {
		t.Fatalf("while timing: %+v, %v", l, err)
	}
	tv.waitTold(t, instantCrushVideo.ID)
	l, err = tv.c.Lyrics(t.Context(), instantCrushVideo)
	if err != nil || !l.Synced || l.Note != "Timed to this video (+12.5 s)" || !near(l, 12500*time.Millisecond) || l.Source != "LRCLIB" {
		t.Fatalf("timed: %+v, %v", l, err)
	}
	if k := keptTimings(t, cache)["a5uQMwRMHcs khnokW3Mw24"]; !k.Matched || k.ShiftMs < 12490 || k.ShiftMs > 12510 {
		t.Errorf("kept %+v", k)
	}
	if tv.opens("khnokW3Mw24") != 1 || tv.opens("a5uQMwRMHcs") != 1 {
		t.Errorf("opened the audio %v", tv.opened)
	}

	again := newTimedVideo(t, 12500*time.Millisecond, cache)
	if l, err := again.c.Lyrics(t.Context(), instantCrushVideo); err != nil || !near(l, 12500*time.Millisecond) || l.Note == timingNote {
		t.Errorf("the next run: %+v, %v", l, err)
	}
	if len(again.opened) != 0 {
		t.Errorf("the next run opened the audio %v", again.opened)
	}
}

// TestLyricsTimedToALongerVideo shows the lyrics of a video longer than
// its song without their times while it finds the song, then timed.
func TestLyricsTimedToALongerVideo(t *testing.T) {
	tv := newTimedVideo(t, 29100*time.Millisecond, t.TempDir())
	video := instantCrushVideo
	video.DurationMs = (5*60 + 38 + 29) * 1000
	l, err := tv.c.Lyrics(t.Context(), video)
	if err != nil || l.Synced || l.Note != timingNote {
		t.Fatalf("while timing: %+v, %v", l, err)
	}
	tv.waitTold(t, video.ID)
	l, err = tv.c.Lyrics(t.Context(), video)
	if err != nil || !l.Synced || l.Note != "Timed to this video (+29.1 s)" || !near(l, 29100*time.Millisecond) {
		t.Fatalf("timed: %+v, %v", l, err)
	}
}

// TestLyricsOfAVideoOfAnotherTake keeps the song's times, as before, for
// a video whose audio is another song's: no clear match, kept so that it
// isn't measured again.
func TestLyricsOfAVideoOfAnotherTake(t *testing.T) {
	cache := t.TempDir()
	tv := newTimedVideo(t, time.Second, cache)
	tv.audio["a5uQMwRMHcs"] = aligntest.WAV(timedRate, aligntest.Song(aligntest.Take{Seed: 9, Length: 70 * time.Second, Rate: timedRate}))
	if _, err := tv.c.Lyrics(t.Context(), instantCrushVideo); err != nil {
		t.Fatal(err)
	}
	tv.waitTold(t, instantCrushVideo.ID)
	l, err := tv.c.Lyrics(t.Context(), instantCrushVideo)
	if err != nil || !l.Synced || l.Note != "" || !near(l, 0) {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	if k, ok := keptTimings(t, cache)["a5uQMwRMHcs khnokW3Mw24"]; !ok || k.Matched {
		t.Errorf("kept %+v, %v", k, ok)
	}
	if _, state := tv.c.a.videoTiming(instantCrushVideo, instantCrushSong); state != timingNone || tv.opens("a5uQMwRMHcs") != 1 {
		t.Errorf("measured again: %v, %v", state, tv.opened)
	}
}

// TestLyricsNotTimedWhileOff keeps the song's times, reading no audio,
// with Time lyrics to music videos off.
func TestLyricsNotTimedWhileOff(t *testing.T) {
	tv := newTimedVideo(t, 12500*time.Millisecond, t.TempDir())
	tv.c.a.settings = testSettings(t, func(s *settings.Settings) { s.LyricsTimeVideos = false })
	l, err := tv.c.Lyrics(t.Context(), instantCrushVideo)
	if err != nil || !l.Synced || l.Note != "" || !near(l, 0) {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	if len(tv.opened) != 0 {
		t.Errorf("opened the audio %v", tv.opened)
	}
}

// TestLyricsTimingThatFails shows the song's times once the audio fails
// to open, keeps nothing, and doesn't try again for a while.
func TestLyricsTimingThatFails(t *testing.T) {
	cache := t.TempDir()
	tv := newTimedVideo(t, 12500*time.Millisecond, cache)
	tv.fail = errors.New("YouTube refused to send its audio")
	if l, err := tv.c.Lyrics(t.Context(), instantCrushVideo); err != nil || l.Note != timingNote {
		t.Fatalf("while timing: %+v, %v", l, err)
	}
	tv.waitTold(t, instantCrushVideo.ID)
	l, err := tv.c.Lyrics(t.Context(), instantCrushVideo)
	if err != nil || !l.Synced || l.Note != "" || !near(l, 0) {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	if kept := keptTimings(t, cache); len(kept) != 0 {
		t.Errorf("kept %v", kept)
	}
	if _, state := tv.c.a.videoTiming(instantCrushVideo, instantCrushSong); state != timingNone || tv.opens("khnokW3Mw24") != 1 {
		t.Errorf("tried again at once: %v, %v", state, tv.opened)
	}
}

// TestLyricsTimingOneAtATime measures one video at a time: one asked for
// while another, which no longer plays, is measured, takes its place, and
// the other keeps nothing.
func TestLyricsTimingOneAtATime(t *testing.T) {
	tv := newTimedVideo(t, 12500*time.Millisecond, t.TempDir())
	held := make(chan struct{})
	open := tv.open
	tv.c.a.lyricsTiming.open = func(ctx context.Context, videoID string) (io.ReadSeekCloser, error) {
		if videoID == "bbbbbbbbbbb" {
			select {
			case <-held:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return open(ctx, videoID)
	}
	first := media.Track{ID: "yt:aaaaaaaaaaa", Source: media.SourceYouTube, Video: true}
	firstSong := media.Track{ID: "yt:bbbbbbbbbbb", Source: media.SourceYouTube}
	if _, state := tv.c.a.videoTiming(first, firstSong); state != timingUnderWay {
		t.Fatalf("the first: %v", state)
	}
	if _, state := tv.c.a.videoTiming(instantCrushVideo, instantCrushSong); state != timingUnderWay {
		t.Fatalf("the second: %v", state)
	}
	tv.waitToldOf(t, first.ID, instantCrushVideo.ID)
	if shift, state := tv.c.a.videoTiming(instantCrushVideo, instantCrushSong); state != timingFound || shift < 12490*time.Millisecond || shift > 12510*time.Millisecond {
		t.Errorf("the second: %v, %v", shift, state)
	}
	if _, state := tv.c.a.videoTiming(first, firstSong); state != timingUnderWay {
		t.Errorf("the first, given way, isn't measured when asked again: %v", state)
	}
	close(held)
	tv.waitTold(t, first.ID)
}

// TestLyricsTimingStopsAsTheAppCloses stops the measurement under way,
// which keeps nothing, and starts no other once the app closed.
func TestLyricsTimingStopsAsTheAppCloses(t *testing.T) {
	cache := t.TempDir()
	tv := newTimedVideo(t, 12500*time.Millisecond, cache)
	opened := make(chan struct{})
	tv.c.a.lyricsTiming.open = func(ctx context.Context, videoID string) (io.ReadSeekCloser, error) {
		close(opened)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if _, state := tv.c.a.videoTiming(instantCrushVideo, instantCrushSong); state != timingUnderWay {
		t.Fatalf("state %v", state)
	}
	<-opened
	tv.c.a.stopLyricsTiming()
	if kept := keptTimings(t, cache); len(kept) != 0 {
		t.Errorf("kept %v", kept)
	}
	if _, state := tv.c.a.videoTiming(instantCrushVideo, instantCrushSong); state != timingNone {
		t.Errorf("once closed: %v", state)
	}
}

// TestTimedNote says how far into the video the song starts, to a tenth
// of a second.
func TestTimedNote(t *testing.T) {
	for shift, want := range map[time.Duration]string{
		29137 * time.Millisecond: "Timed to this video (+29.1 s)",
		-7500 * time.Millisecond: "Timed to this video (-7.5 s)",
		-20 * time.Millisecond:   "Timed to this video (+0.0 s)",
	} {
		if got := timedNote(shift); got != want {
			t.Errorf("timedNote(%v) = %q, want %q", shift, got, want)
		}
	}
}
