package player

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// recorded is the fake app finding the recordings on YouTube Music of
// Spotify tracks, as their streams are found: those of the tracks whose
// IDs start with "x" are explicit, though Spotify marks none of them so.
// It knows the recordings it found, as the app keeps its matches, and
// those of known from the start.
type recorded struct {
	*fakeServices
	rmu   sync.Mutex
	known map[string]media.Track
}

// recordingOf is the recording on YouTube Music of the Spotify track t.
func recordingOf(t media.Track) media.Track {
	id := strings.TrimPrefix(t.ID, "spotify:")
	return media.Track{ID: "yt:rec-" + id, Source: media.SourceYouTube, Title: t.Title, Explicit: strings.HasPrefix(id, "x")}
}

func (r *recorded) found(t media.Track) media.Track {
	rec := recordingOf(t)
	r.rmu.Lock()
	defer r.rmu.Unlock()
	r.known[t.ID] = rec
	return rec
}

func (r *recorded) Prepare(ctx context.Context, t media.Track) (Source, error) {
	src, err := r.fakeServices.Prepare(ctx, t)
	if err == nil && src.Mode == ModeStream && t.Source == media.SourceSpotify {
		src.Track = r.found(t)
	}
	return src, err
}

func (r *recorded) PrepareStream(ctx context.Context, t media.Track) (Source, error) {
	src, err := r.fakeServices.PrepareStream(ctx, t)
	if err == nil && t.Source == media.SourceSpotify {
		src.Track = r.found(t)
	}
	return src, err
}

func (r *recorded) KnownRecording(t media.Track) (media.Track, bool) {
	r.rmu.Lock()
	defer r.rmu.Unlock()
	rec, ok := r.known[t.ID]
	return rec, ok
}

var _ Recordings = (*recorded)(nil)

// newRecorded is a harness whose app finds the recordings of Spotify
// tracks, and knows those of the tracks known, by their IDs without
// "spotify:", from the start. Its settings play Spotify tracks from
// YouTube Music.
func newRecorded(t *testing.T, known []string, setup ...func(*fakeServices)) (*harness, *recorded) {
	t.Helper()
	r := &recorded{known: map[string]media.Track{}}
	for _, id := range known {
		r.found(spotifyTrack(id))
	}
	setup = append([]func(*fakeServices){func(f *fakeServices) { f.settings.SpotifyPlayback = settings.SpotifyPlaybackMatch }}, setup...)
	h := newHarnessWith(t, Saved{}, func(f *fakeServices) Services {
		r.fakeServices = f
		return r
	}, setup...)
	return h, r
}

func spotifyTracks(ids ...string) []media.Track {
	out := make([]media.Track, len(ids))
	for i, id := range ids {
		out[i] = spotifyTrack(id)
	}
	return out
}

// streaming waits for the Spotify track id to play from its recording.
func streaming(id string) func(*Player) bool {
	return func(p *Player) bool {
		cur, ok := p.s.Current()
		return ok && cur.Track.ID == "spotify:"+id && p.s.Playing && !p.s.Buffering && p.s.Source != nil &&
			p.s.Source.Track.ID == "yt:rec-"+id
	}
}

// streamed is the Spotify tracks the engine loaded the audio of.
func (h *harness) streamed() []string {
	var out []string
	for _, c := range h.stream.with("load ") {
		out = append(out, strings.TrimPrefix(strings.Fields(c)[1], "url:spotify:"))
	}
	return out
}

// spotifySkips is the IDs of the Spotify tracks the queue passes over as
// explicit.
func spotifySkips(s State) []string {
	var out []string
	for _, it := range s.Queue {
		if it.Skip == SkipExplicit {
			out = append(out, strings.TrimPrefix(it.Track.ID, "spotify:"))
		}
	}
	return out
}

// TestHideExplicitRecording passes over the Spotify tracks that play from
// an explicit recording on YouTube Music while the settings hide explicit
// content, though Spotify marks them clean (#583): from the start one
// whose recording the app found before, and one whose recording is found
// as it is to play from then on, without a word. Neither plays.
func TestHideExplicitRecording(t *testing.T) {
	h, _ := newRecorded(t, []string{"x1"}, hidingExplicit)
	h.p.PlayTracks(spotifyTracks("a", "x1", "x2", "b"), 0)
	h.wait("a playing", streaming("a"))
	if got := spotifySkips(h.p.State()); !slices.Equal(got, []string{"x1"}) {
		t.Fatalf("the queue passes over %v", got)
	}
	h.p.Next()
	h.wait("b playing, past x1 and x2", streaming("b"))
	if got := spotifySkips(h.p.State()); !slices.Equal(got, []string{"x1", "x2"}) {
		t.Errorf("the queue passes over %v", got)
	}
	if got := h.streamed(); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("the engine loaded %v", got)
	}
	if n := h.notices(); len(n) != 0 {
		t.Errorf("passing over explicit recordings told %+v", n)
	}
	// Back, it passes over them too.
	h.p.Previous()
	h.wait("a playing again", streaming("a"))
}

// TestHideExplicitRecordingChosen plays no Spotify track the user chooses
// whose recording is explicit, and tells why: one known before doesn't
// change the queue, and one found as it was to play moves it on.
func TestHideExplicitRecordingChosen(t *testing.T) {
	h, _ := newRecorded(t, []string{"x1"}, hidingExplicit)
	h.p.PlayTracks(spotifyTracks("a", "b"), 0)
	h.wait("a playing", streaming("a"))
	before := queueIDs(h.p.State())

	h.p.PlayTracks(spotifyTracks("c", "x1", "d"), 1)
	h.on(func(*Player) {})
	if got := queueIDs(h.p.State()); !slices.Equal(got, before) || !streaming("a")(h.p) {
		t.Errorf("choosing x1 made the queue %v, at %s", got, current(h.p.State()))
	}
	if !h.toldExplicit() {
		t.Errorf("nothing told why: %+v", h.notices())
	}

	h.p.PlayTracks(spotifyTracks("c", "x2", "d"), 1)
	h.wait("d playing, past x2", streaming("d"))
	if got := spotifySkips(h.p.State()); !slices.Equal(got, []string{"x2"}) {
		t.Errorf("the queue passes over %v", got)
	}
	n := h.notices()
	if len(n) != 2 || !strings.Contains(n[1].Message, "Spotify x2") || !strings.Contains(n[1].Message, "your settings hide explicit content") {
		t.Errorf("told %+v", n)
	}
	if got := h.streamed(); slices.ContainsFunc(got, func(id string) bool { return strings.HasPrefix(id, "x") }) {
		t.Errorf("the engine loaded %v", got)
	}

	// Chosen again in the queue, x2 doesn't play: its recording is known.
	h.p.PlayAt(1)
	h.on(func(*Player) {})
	if !streaming("d")(h.p) || len(h.notices()) != 3 {
		t.Errorf("choosing x2 again moved to %s, told %+v", current(h.p.State()), h.notices())
	}
}

// TestHideExplicitRecordingSetting plays an explicit recording while the
// settings don't hide explicit content, and moves on from it once they do.
func TestHideExplicitRecordingSetting(t *testing.T) {
	h, _ := newRecorded(t, nil)
	h.p.PlayTracks(spotifyTracks("x1", "a"), 0)
	h.wait("x1 playing from its recording", streaming("x1"))
	h.svc.with(hidingExplicit)
	h.p.SettingsChanged(h.svc.Settings())
	h.wait("a playing, moved on from x1", streaming("a"))
	if got := spotifySkips(h.p.State()); !slices.Equal(got, []string{"x1"}) {
		t.Errorf("the queue passes over %v", got)
	}
}

// TestHideExplicitRecordingOnDevice plays a Spotify track on a Spotify
// device by Spotify's mark, whatever its recording on YouTube Music, but
// not from that recording when no device can play it.
func TestHideExplicitRecordingOnDevice(t *testing.T) {
	h, _ := newRecorded(t, []string{"x1"}, hidingExplicit, func(f *fakeServices) {
		f.settings.SpotifyPlayback = settings.SpotifyPlaybackConnect
		f.prepare = onDevice
	})
	h.p.PlayTracks(spotifyTracks("x1"), 0)
	h.wait("the device playing x1", func(p *Player) bool { return p.connect.active() && h.device.has("play spotify:x1 0s") })
	if got := spotifySkips(h.p.State()); len(got) != 0 {
		t.Errorf("the queue passes over %v", got)
	}

	h, _ = newRecorded(t, nil, hidingExplicit, func(f *fakeServices) {
		f.settings.SpotifyPlayback = settings.SpotifyPlaybackConnect
		f.prepare = onDevice
	})
	h.device.playErr = errors.New("no Spotify device is active.")
	h.p.PlayTracks(spotifyTracks("x2", "a"), 0)
	h.wait("a, x2's fallback refused", func(p *Player) bool { return h.stream.lastLoad() == "load url:spotify:a 0s true" })
	if got := h.streamed(); !slices.Equal(got, []string{"a"}) {
		t.Errorf("the engine loaded %v", got)
	}
	if !h.toldExplicit() {
		t.Errorf("nothing told why x2 didn't play: %+v", h.notices())
	}
}

// TestHideExplicitRecordingLater passes over a Spotify track whose
// recording the app found explicit after it was queued, as a download
// finds it, without preparing it; and the next track, whose recording is
// found explicit as it's readied ahead, is marked then, and the one after
// it preloaded instead.
func TestHideExplicitRecordingLater(t *testing.T) {
	h, r := newRecorded(t, nil, hidingExplicit)
	h.p.PlayTracks(spotifyTracks("a", "x1", "b", "x2", "c"), 0)
	h.wait("a playing", streaming("a"))
	r.found(spotifyTrack("x1"))
	h.p.Next()
	h.wait("b playing, past x1", streaming("b"))
	var prepared []string
	h.svc.with(func(f *fakeServices) { prepared = slices.Clone(f.prepared) })
	if slices.Contains(prepared, "spotify:x1") {
		t.Errorf("x1 was prepared: %v", prepared)
	}
	h.p.Time(prefetchAfter+time.Second, 200*time.Second)
	h.wait("x2 found explicit", func(p *Player) bool { return slices.Equal(spotifySkips(p.s), []string{"x1", "x2"}) })
	h.p.Time(prefetchAfter+2*time.Second, 200*time.Second)
	h.wait("c preloaded in place of x2", func(*Player) bool { return h.stream.has("preload url:spotify:c") })
	if h.stream.has("preload url:spotify:x2") {
		t.Error("x2 was preloaded")
	}
	h.p.Ended()
	h.wait("c playing, past x2", streaming("c"))
}
