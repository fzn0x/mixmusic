package player

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// quickDevice has the player ask a device what it plays at once, and
// often.
func quickDevice(t *testing.T) {
	connectTick, connectAskEvery, connectSettle = 5*time.Millisecond, 10*time.Millisecond, 0
	t.Cleanup(func() { connectTick, connectAskEvery, connectSettle = time.Second, 3*time.Second, 5*time.Second })
}

// sounding tells whether d tells it plays something.
func sounding(d *fakeDevice) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state != nil && d.state.Playing
}

// TestOwnDeviceLostIsSilenced has MixMusic's own device tell it plays the
// Spotify track under another name, so the player loses it there, and then
// a song the user picks from YouTube Music plays: the device is hushed,
// rather than left to sound beside it.
func TestOwnDeviceLostIsSilenced(t *testing.T) {
	quickDevice(t)
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = here })
	own := &aheadDevice{}
	h.p.SpotifyHere(own)
	h.p.PlayTracks([]media.Track{spotifyTrack("1")}, 0)
	h.wait("1 on the device", func(p *Player) bool { return p.connect.active() && p.s.Playing })
	own.set(&DeviceState{Playing: true, Progress: time.Minute, Track: &media.Track{
		ID: "spotify:x", Title: "Spotify 1 - Remastered", Artists: []media.Ref{{Name: "Sp"}}, DurationMs: 200000}})
	h.wait("1 lost", func(p *Player) bool { return p.connect.lost })
	h.p.PlayTracks([]media.Track{track("a", "A")}, 0)
	h.wait("a playing", playing("a"))
	h.wait("the device hushed", func(*Player) bool { return own.has("hush") && !sounding(&own.fakeDevice) })
}

// takesThenFails is MixMusic's own device taking the track it's told to
// play, though the call fails: as when Spotify took the play, but its
// answer didn't come back in time.
type takesThenFails struct{ aheadDevice }

func (d *takesThenFails) Play(ctx context.Context, t media.Track, at time.Duration) error {
	return d.PlayAhead(ctx, t, at, nil)
}

func (d *takesThenFails) PlayAhead(ctx context.Context, t media.Track, at time.Duration, ahead []media.Track) error {
	_ = d.aheadDevice.PlayAhead(ctx, t, at, ahead)
	return errors.New("Spotify didn't answer in time")
}

// TestOwnDeviceFailedPlayIsSilenced has MixMusic's own device take a track
// whose play failed: the player plays it from YouTube Music instead, and
// the device is hushed, rather than playing it a second time beside it.
func TestOwnDeviceFailedPlayIsSilenced(t *testing.T) {
	quickDevice(t)
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = here })
	own := &takesThenFails{}
	h.p.SpotifyHere(own)
	h.p.PlayTracks([]media.Track{spotifyTrack("1")}, 0)
	h.wait("1 from YouTube Music", func(*Player) bool { return h.stream.lastLoad() == "load url:spotify:1 0s true" })
	h.wait("the device hushed", func(*Player) bool { return own.has("hush") && !sounding(&own.fakeDevice) })
}

// TestOwnDeviceGoneElsewhereIsSilenced has MixMusic's own device go on, at
// the end of the Spotify track, to one the queue doesn't hold, as Spotify's
// own autoplay does: the stream after it in the queue plays, and the
// device is hushed.
func TestOwnDeviceGoneElsewhereIsSilenced(t *testing.T) {
	quickDevice(t)
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = here })
	own := &aheadDevice{}
	h.p.SpotifyHere(own)
	sp1 := spotifyTrack("1")
	h.p.PlayTracks([]media.Track{sp1, track("b", "B")}, 0)
	h.wait("1 on the device", func(p *Player) bool { return p.connect.active() && p.s.Playing })
	own.set(&DeviceState{Playing: true, Progress: 199 * time.Second, Track: &sp1})
	h.wait("1 ending", func(p *Player) bool { return p.connect.ending })
	own.set(&DeviceState{Playing: true, Progress: time.Second, Track: &media.Track{
		ID: "spotify:autoplay", Title: "Something Spotify chose", DurationMs: 180000}})
	h.wait("b playing", playing("b"))
	h.wait("the device hushed", func(*Player) bool { return own.has("hush") && !sounding(&own.fakeDevice) })
}

// TestConnectDeviceLostPlaysOn has someone play something else on a
// Spotify Connect device mid-track, and the user then pick a song from
// YouTube Music: what they play there isn't paused for it.
func TestConnectDeviceLostPlaysOn(t *testing.T) {
	quickDevice(t)
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = onDevice })
	h.p.PlayTracks([]media.Track{spotifyTrack("1")}, 0)
	h.wait("1 on the device", func(p *Player) bool { return p.connect.active() && p.s.Playing })
	h.device.set(&DeviceState{Playing: true, Progress: time.Minute, Track: &media.Track{ID: "spotify:theirs"}})
	h.wait("1 lost", func(p *Player) bool { return p.connect.lost })
	h.p.PlayTracks([]media.Track{track("a", "A")}, 0)
	h.wait("a playing", playing("a"))
	if h.device.has("pause") || !sounding(h.device) {
		t.Errorf("the device was paused for a song played elsewhere: %v", h.device.calls)
	}
}

// TestOwnDeviceUnusedIsLeftAlone plays only YouTube Music: MixMusic's own
// device, which played nothing, is told nothing.
func TestOwnDeviceUnusedIsLeftAlone(t *testing.T) {
	quickDevice(t)
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.prepare = here })
	own := &aheadDevice{}
	h.p.SpotifyHere(own)
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.Next()
	h.wait("b playing", playing("b"))
	h.p.Pause()
	h.wait("paused", func(p *Player) bool { return !p.s.Playing })
	if calls := own.with(""); len(calls) > 0 {
		t.Errorf("the unused device was told %v", calls)
	}
}
