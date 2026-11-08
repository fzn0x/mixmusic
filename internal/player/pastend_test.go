package player

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestSpotifyPastItsEnd has MixMusic's own device, given a Spotify track
// with a stream after it in the queue, tell it plays on past the track's
// end, its position counting on (#373). The time shown stops at the end,
// a little past it is still the track's, and further past it the track
// ended: the device is paused and the stream after it plays.
func TestSpotifyPastItsEnd(t *testing.T) {
	connectTick, connectAskEvery, connectSettle = time.Hour, time.Hour, 0
	t.Cleanup(func() { connectTick, connectAskEvery, connectSettle = time.Second, 3*time.Second, 5*time.Second })
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = here })
	own := &aheadDevice{}
	h.p.SpotifyHere(own)
	sp1 := spotifyTrack("1")
	h.p.PlayTracks([]media.Track{sp1, track("b", "B")}, 0)
	h.wait("1 playing", func(p *Player) bool { return current(p.s) == "spotify:1" && p.connect.active() && p.s.Playing })
	if !own.has("play spotify:1 0s") {
		t.Fatalf("the device was told %v", own.calls)
	}

	// A second past the end: the length the device knows may be longer.
	tells(h, own, &DeviceState{Playing: true, Progress: 201 * time.Second, Track: &sp1})
	h.wait("the end shown", func(p *Player) bool {
		return current(p.s) == "spotify:1" && p.s.Position == 200*time.Second && p.s.Duration == 200*time.Second
	})

	// Further past it, as the device's position counts on: the track ended.
	tells(h, own, &DeviceState{Playing: true, Progress: 322 * time.Second, Track: &sp1})
	h.wait("b playing", playing("b"))
	if !own.has("pause") {
		t.Errorf("the device was left playing past the end: %v", own.calls)
	}
	if st := h.p.State(); st.Position > st.Duration && st.Duration > 0 {
		t.Errorf("the time shown is %v of %v", st.Position, st.Duration)
	}
}

// TestSpotifyPastItsEndAhead has the device, given the next Spotify tracks
// after the current one, tell it plays the current one on past its end:
// it may still go on to the next by itself, so the player waits a moment,
// then plays the next itself.
func TestSpotifyPastItsEndAhead(t *testing.T) {
	connectTick, connectAskEvery, connectSettle, connectGoOn = time.Hour, time.Hour, 0, 20*time.Millisecond
	t.Cleanup(func() {
		connectTick, connectAskEvery, connectSettle, connectGoOn = time.Second, 3*time.Second, 5*time.Second, 2*time.Second
	})
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = here })
	own := &aheadDevice{}
	h.p.SpotifyHere(own)
	sp1, sp2, sp3 := spotifyTrack("1"), spotifyTrack("2"), spotifyTrack("3")
	h.p.PlayTracks([]media.Track{sp1, sp2, sp3}, 0)
	h.wait("1 playing", func(p *Player) bool { return current(p.s) == "spotify:1" && p.connect.active() && p.s.Playing })
	tells(h, own, &DeviceState{Playing: true, Progress: 210 * time.Second, Track: &sp1})
	h.wait("2 playing", func(p *Player) bool { return current(p.s) == "spotify:2" && p.connect.active() && p.s.Playing })
	if !own.has("play spotify:2 0s then spotify:3") {
		t.Errorf("the device was told %v", own.calls)
	}
}

// TestPastEnd takes a track as ended only well past its end, by the longer
// of the two lengths MixMusic and the device know.
func TestPastEnd(t *testing.T) {
	long := spotifyTrack("1")
	long.DurationMs = 204000
	sp1 := spotifyTrack("1")
	for _, c := range []struct {
		name     string
		state    DeviceState
		dur      time.Duration
		pastEnd  bool
		shownPos time.Duration
	}{
		{"before the end", DeviceState{Progress: 150 * time.Second, Track: &sp1}, 200 * time.Second, false, 150 * time.Second},
		{"just past it", DeviceState{Progress: 201 * time.Second, Track: &sp1}, 200 * time.Second, false, 200 * time.Second},
		{"well past it", DeviceState{Progress: 205 * time.Second, Track: &sp1}, 200 * time.Second, true, 200 * time.Second},
		{"within the device's longer length", DeviceState{Progress: 205 * time.Second, Track: &long}, 200 * time.Second, false, 200 * time.Second},
		{"a length unknown", DeviceState{Progress: 500 * time.Second, Track: &sp1}, 0, false, 500 * time.Second},
	} {
		if got := pastEnd(&c.state, c.dur); got != c.pastEnd {
			t.Errorf("%s: pastEnd = %v", c.name, got)
		}
		if got := within(c.state.Progress, c.dur); got != c.shownPos {
			t.Errorf("%s: within = %v", c.name, got)
		}
	}
}
