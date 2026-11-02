package player

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
)

// cueStream is the fake stream that keeps the tracks its loads were cued
// with.
type cueStream struct {
	*fakeStream
	tracks []string
}

func (s *cueStream) Load(url string, cue audio.Cue) error {
	s.mu.Lock()
	id := ""
	if cue.Track != nil {
		id = cue.Track.ID
	}
	s.tracks = append(s.tracks, id)
	s.mu.Unlock()
	return s.fakeStream.Load(url, cue)
}

func (s *cueStream) cued() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.tracks)
}

// TestOutputMovesTheStream loads the track that plays again where it was
// as its streams go to a cast device, playing, and back to the computer,
// with the track for the device's screen; until it sounds there, it loads.
func TestOutputMovesTheStream(t *testing.T) {
	h := newHarness(t, Saved{})
	cs := &cueStream{fakeStream: h.stream}
	h.p.stream = cs
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	home := h.p.State().Volume
	h.p.Time(30*time.Second, 200*time.Second)
	h.wait("a at 30s", func(p *Player) bool { return p.s.Position == 30*time.Second })

	h.stream.silence("a") // it sounds once the test says
	h.p.SetOutput(Output{Remote: true, Volume: 0.3})
	h.wait("a loading on the device", func(p *Player) bool { return p.s.Loading && p.s.Remote })
	if got := h.stream.lastLoad(); got != "load url:yt:a 30s true" {
		t.Errorf("moved with %q", got)
	}
	if got := h.p.State(); got.Volume != 0.3 || got.Muted {
		t.Errorf("the device's volume shows as %v, muted %v", got.Volume, got.Muted)
	}
	h.p.Time(30*time.Second+500*time.Millisecond, 200*time.Second)
	h.wait("a sounding on the device", playing("a"))

	h.p.SetOutput(Output{})
	h.wait("a back on the computer", func(p *Player) bool { return !p.s.Remote && p.s.Volume == home })
	if got := h.stream.lastLoad(); got != "load url:yt:a 30.5s true" {
		t.Errorf("moved back with %q", got)
	}
	if got, want := h.stream.last("volume"), fmt.Sprintf("volume %v", home); got != want {
		t.Errorf("the engine's volume back: %q, want %q", got, want)
	}
	if got := cs.cued(); !slices.Equal(got, []string{"yt:a", "yt:a", "yt:a"}) {
		t.Errorf("cued %q", got)
	}
	// The computer is where it plays already.
	n := h.stream.count("load")
	h.p.SetOutput(Output{})
	h.on(func(*Player) {})
	if h.stream.count("load") != n {
		t.Error("staying on the computer loaded the track again")
	}
}

// TestOutputKeepsPaused moves a paused track paused, and one the device
// paused by itself paused too.
func TestOutputKeepsPaused(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.Pause()
	h.wait("paused", func(p *Player) bool { return !p.s.Playing })
	h.p.SetOutput(Output{Remote: true, Volume: 0.5})
	h.wait("moved", func(p *Player) bool { return p.s.Remote })
	// The fake's time moved by a millisecond as it played.
	if got := h.stream.lastLoad(); got != "load url:yt:a 1ms false" {
		t.Errorf("moved with %q", got)
	}

	h.p.Play()
	h.wait("playing on the device", playing("a"))
	// The device's remote pauses it.
	h.p.Playing(false)
	h.wait("paused by the device", func(p *Player) bool { return !p.s.Playing })
	h.p.SetOutput(Output{})
	h.wait("back", func(p *Player) bool { return !p.s.Remote })
	if got := h.stream.lastLoad(); got != "load url:yt:a 1ms false" {
		t.Errorf("moved back with %q", got)
	}
}

// TestOutputVolume sets the device's volume, shows it as the device
// changes it, and keeps none of it: back on the computer, the volume is
// the computer's, as the settings kept it, Flush too.
func TestOutputVolume(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.SetVolume(0.8)
	h.wait("0.8 kept", func(*Player) bool {
		var v []float64
		h.svc.with(func(f *fakeServices) { v = slices.Clone(f.volumes) })
		return len(v) == 1
	})
	h.p.SetOutput(Output{Remote: true, Volume: 0.2})
	h.p.SetVolume(0.6)
	h.wait("the device at 0.6", func(p *Player) bool { return p.s.Volume == 0.6 })
	if got := h.stream.last("volume"); got != "volume 0.6" {
		t.Errorf("set the device's volume with %q", got)
	}
	calls := h.stream.count("volume")
	h.p.OutputVolume(0.25, true)
	h.wait("the device's own change", func(p *Player) bool { return p.s.Volume == 0.25 && p.s.Muted })
	if h.stream.count("volume") != calls {
		t.Error("the device's own volume was sent back to it")
	}
	h.p.ToggleMute()
	h.wait("unmuted", func(p *Player) bool { return !p.s.Muted })
	if got := h.stream.last("volume"); got != "volume 0.25" {
		t.Errorf("unmuted the device with %q", got)
	}
	time.Sleep(4 * saveVolumeAfter)
	h.p.Flush()
	var v []float64
	h.svc.with(func(f *fakeServices) { v = slices.Clone(f.volumes) })
	if len(v) != 1 {
		t.Errorf("kept the volumes %v; the device's aren't the computer's", v)
	}

	h.p.SetOutput(Output{})
	h.wait("the computer's volume", func(p *Player) bool { return p.s.Volume == 0.8 && !p.s.Muted && !p.s.Remote })
	if got := h.stream.last("volume"); got != "volume 0.8" {
		t.Errorf("the engine got %q", got)
	}
	// On the computer, the device's changes are nobody's.
	h.p.OutputVolume(0.1, false)
	h.on(func(p *Player) {
		if p.s.Volume != 0.8 {
			t.Errorf("a device's volume on the computer: %v", p.s.Volume)
		}
	})
}

// TestOutputNoCrossfade plays the next track as the current one ends on a
// cast device, rather than crossfading into it, which the device can't.
func TestOutputNoCrossfade(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) { f.settings.Crossfade = 5 })
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	h.p.SetOutput(Output{Remote: true, Volume: 1})
	h.p.Time(20*time.Second, 200*time.Second)
	h.p.Time(197*time.Second, 200*time.Second)
	h.on(func(*Player) {})
	h.on(func(*Player) {})
	if h.stream.has("crossfade url:yt:b 5s") {
		t.Fatal("crossfaded on the device")
	}
	h.p.Ended()
	h.wait("b playing", playing("b"))
	if got := h.stream.lastLoad(); got != "load url:yt:b 0s true" {
		t.Errorf("b loaded with %q", got)
	}
}

// TestOutputLeavesSpotifyDevices leaves a Spotify track that plays on a
// Spotify device there: the engine has no stream of it to move.
func TestOutputLeavesSpotifyDevices(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) { return Source{Mode: ModeConnect, Track: t}, nil }
	})
	sp := track("s")
	sp.ID, sp.Source = "sp:s", media.SourceSpotify
	h.p.PlayTracks([]media.Track{sp}, 0)
	h.wait("on the device", func(p *Player) bool { return p.connect.active() })
	loads := h.stream.count("load")
	h.p.SetOutput(Output{Remote: true, Volume: 0.4})
	h.wait("moved", func(p *Player) bool { return p.s.Remote })
	if h.stream.count("load") != loads {
		t.Error("a Spotify device's track loaded in the stream")
	}
}
