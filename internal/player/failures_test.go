package player

import (
	"errors"
	"iter"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// failedOnce waits for the player to keep n failures, and returns them,
// the latest first.
func (h *harness) failedOnce(n int) []Failure {
	h.t.Helper()
	h.wait("the failures kept", func(p *Player) bool { return len(p.Failures()) >= n })
	return h.p.Failures()
}

// TestFailureOfAStream keeps a stream that broke off, was tried again as
// many times as there are waits, and failed: its steps, the tries in one,
// with why the stream tells, the time, and the ID its notice names (#351).
func TestFailureOfAStream(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.failure = "YouTube refused it" })
	before := time.Now()
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Time(42*time.Second, 200*time.Second)
	h.stream.silence("a")
	useRetries(t, h, "a", 0)
	h.p.Error("the stream failed")
	h.wait("b playing", playing("b"))
	f := h.failedOnce(1)
	if len(f) != 1 || f[0].Track.ID != "yt:a" || f[0].At.Before(before) || f[0].Stopped || f[0].Recording.ID != "" {
		t.Fatalf("failures %+v", f)
	}
	want := []Step{{"the stream", "YouTube refused it", 1}, {"the stream, tried again", "YouTube refused it", len(retryWaits)}}
	if !slices.Equal(f[0].Steps, want) {
		t.Errorf("steps %+v, want %+v", f[0].Steps, want)
	}
	n := h.notices()
	if len(n) != 1 || n[0].Failure != f[0].ID || f[0].ID == 0 {
		t.Errorf("notices %+v, failure %d", n, f[0].ID)
	}
	if got, ok := h.p.FailureOf(f[0].ID); !ok || got.Track.ID != "yt:a" {
		t.Errorf("FailureOf(%d) = %+v, %v", f[0].ID, got, ok)
	}
	if _, ok := h.p.FailureOf(f[0].ID + 1); ok {
		t.Error("a failure of no notice")
	}
}

// TestFailuresInARow keeps each track whose stream wasn't found, with the
// step that failed it, and the last as the one the player stopped after.
func TestFailuresInARow(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(media.Track) (Source, error) { return Source{}, errors.New("offline.") }
	})
	h.p.PlayTracks(tracks("a", "b", "c", "d"), 0)
	f := h.failedOnce(maxFailures)
	var got []string
	for _, k := range f {
		got = append(got, k.Track.ID)
		if !slices.Equal(k.Steps, []Step{{"finding its stream", "offline.", 1}}) {
			t.Errorf("%s: steps %+v", k.Track.ID, k.Steps)
		}
	}
	if !slices.Equal(got, []string{"yt:c", "yt:b", "yt:a"}) || !f[0].Stopped || f[1].Stopped || f[0].ID <= f[1].ID {
		t.Errorf("failures %+v", f)
	}
	for i, n := range h.notices() {
		if want := f[len(f)-1-i].ID; n.Failure != want {
			t.Errorf("notice %d names failure %d, not %d", i, n.Failure, want)
		}
	}
}

// TestFailureLogged writes a track that couldn't play to the log, with
// why, for a report of the problem: the notice of it goes.
func TestFailureLogged(t *testing.T) {
	var logs strings.Builder
	var mu sync.Mutex
	log.SetOutput(writerFunc(func(b []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return logs.Write(b)
	}))
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) {
			if t.ID == "yt:a" {
				return Source{}, errors.New("YouTube plays this track only to signed-in viewers")
			}
			return streamOf(t), nil
		}
	})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("b playing", playing("b"))
	mu.Lock()
	got := logs.String()
	mu.Unlock()
	if want := "mixmusic: couldn't play yt:a: YouTube plays this track only to signed-in viewers"; !strings.Contains(got, want) {
		t.Errorf("the log lacks %q:\n%s", want, got)
	}
}

// steppedErr is an error that tells its steps, as the resolver's does
// when yt-dlp failed after YouTube's player API.
type steppedErr struct{}

func (steppedErr) Error() string { return "this track is private" }

func (steppedErr) Steps() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		_ = yield("YouTube's player API", "LOGIN_REQUIRED") && yield("yt-dlp", "this track is private")
	}
}

// TestFailureSteps keeps the steps an error tells, of a Spotify track
// whose Spotify Connect device couldn't play it, nor YouTube Music.
func TestFailureSteps(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.settings.SpotifyPlayback = settings.SpotifyPlaybackConnect
		f.prepare = onDevice
		f.streamErr = steppedErr{}
	})
	h.device.playErr = errors.New("no Spotify device is active.")
	h.p.PlayTracks([]media.Track{spotifyTrack("1")}, 0)
	f := h.failedOnce(1)
	want := []Step{
		{"Spotify Connect", "no Spotify device is active.", 1},
		{"YouTube's player API", "LOGIN_REQUIRED", 1},
		{"yt-dlp", "this track is private", 1},
	}
	if !slices.Equal(f[0].Steps, want) {
		t.Errorf("steps %+v, want %+v", f[0].Steps, want)
	}
}

// TestFailureOfYTDLP names yt-dlp the step of an error it printed, and
// the file that of a track on this computer.
func TestFailureOfYTDLP(t *testing.T) {
	file := media.Track{ID: "file:L3NvbmcubXAz", Source: media.SourceFile, Title: "Song"}
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) {
			if t.Source == media.SourceFile {
				return Source{}, errors.New("the file is gone")
			}
			return Source{}, errors.New("yt-dlp: ERROR: [youtube] a: Video unavailable")
		}
	})
	h.p.PlayTracks([]media.Track{track("a"), file}, 0)
	f := h.failedOnce(2)
	if want := []Step{{"yt-dlp", "ERROR: [youtube] a: Video unavailable", 1}}; !slices.Equal(f[1].Steps, want) {
		t.Errorf("yt-dlp's steps %+v", f[1].Steps)
	}
	if want := []Step{{"the file", "the file is gone", 1}}; !slices.Equal(f[0].Steps, want) {
		t.Errorf("the file's steps %+v", f[0].Steps)
	}
}

// TestFailuresKept keeps the latest keptFailures failures, and tells
// which way a stream that played from a recording was to play.
func TestFailuresKept(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	var first int
	h.on(func(p *Player) {
		src := *p.s.Source
		src.Track = track("rec")
		p.s.Source = &src
		for i := range keptFailures + 2 {
			id := p.keepFailure("it broke", false)
			if i == 0 {
				first = id
			}
		}
	})
	f := h.p.Failures()
	if len(f) != keptFailures || f[len(f)-1].ID != first+2 || f[0].Recording.ID != "yt:rec" {
		t.Errorf("kept %d, the oldest %d, the recording %q", len(f), f[len(f)-1].ID, f[0].Recording.ID)
	}
	if _, ok := h.p.FailureOf(first); ok {
		t.Error("the oldest is kept")
	}
	if want := []Step{{"the stream", "it broke", 1}}; !slices.Equal(f[0].Steps, want) {
		t.Errorf("steps %+v", f[0].Steps)
	}
}
