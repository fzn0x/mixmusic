package player

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/media"
)

// castStream is a fake cast device as the player's output: the fake stream,
// which keeps the tracks its loads were cued with.
type castStream struct {
	*fakeStream
	tracks []string
}

func newCastStream(h *harness) *castStream { return &castStream{fakeStream: &fakeStream{p: h.p}} }

func (s *castStream) Load(url string, cue audio.Cue) error {
	s.mu.Lock()
	id := ""
	if cue.Track != nil {
		id = cue.Track.ID
	}
	s.tracks = append(s.tracks, id)
	s.mu.Unlock()
	return s.fakeStream.Load(url, cue)
}

func (s *castStream) cued() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.tracks)
}

// TestOutputMovesTheStream loads the track that plays again where it was
// as its streams go to a cast device, playing, with the track for the
// device's screen, and back to the computer; the output it leaves lets go
// of it, and until it sounds on the other, it loads.
func TestOutputMovesTheStream(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a", "b"), 0)
	h.wait("a playing", playing("a"))
	home := h.p.State().Volume
	h.p.Time(30*time.Second, 200*time.Second)
	h.wait("a at 30s", func(p *Player) bool { return p.s.Position == 30*time.Second })

	cast := newCastStream(h)
	cast.silence("a") // it sounds once the test says
	h.p.SetOutput(Output{Stream: cast, Volume: 0.3})
	h.wait("a loading on the device", func(p *Player) bool { return p.s.Loading && p.s.Remote })
	if got := cast.lastLoad(); got != "load url:yt:a 30s true" {
		t.Errorf("moved with %q", got)
	}
	if got := h.stream.copy(); got[len(got)-1] != "unload" {
		t.Errorf("the engine kept the track: %q", got)
	}
	if got := h.p.State(); got.Volume != 0.3 || got.Muted {
		t.Errorf("the device's volume shows as %v, muted %v", got.Volume, got.Muted)
	}
	h.p.Time(30*time.Second+500*time.Millisecond, 200*time.Second)
	h.wait("a sounding on the device", playing("a"))
	if got := cast.cued(); !slices.Equal(got, []string{"yt:a"}) {
		t.Errorf("cued %q", got)
	}

	engineLoads := h.stream.count("load")
	h.p.SetOutput(Output{})
	h.wait("a back on the computer", func(p *Player) bool { return !p.s.Remote && p.s.Volume == home })
	if got := h.stream.lastLoad(); got != "load url:yt:a 30.5s true" || h.stream.count("load") != engineLoads+1 {
		t.Errorf("moved back with %q", got)
	}
	if got, want := h.stream.last("volume"), fmt.Sprintf("volume %v", home); got != want {
		t.Errorf("the engine's volume back: %q, want %q", got, want)
	}
	if !cast.has("unload") {
		t.Errorf("the device kept the track: %q", cast.copy())
	}
	// The computer is where it plays already.
	h.p.SetOutput(Output{})
	h.on(func(*Player) {})
	if h.stream.count("load") != engineLoads+1 {
		t.Error("staying on the computer loaded the track again")
	}
}

// TestOutputKeepsPaused moves a paused track paused, one the device
// paused by itself paused too, and one that plays paused when asked, as
// when the device went away.
func TestOutputKeepsPaused(t *testing.T) {
	h := newHarness(t, Saved{})
	h.p.PlayTracks(tracks("a"), 0)
	h.wait("a playing", playing("a"))
	h.p.Pause()
	h.wait("paused", func(p *Player) bool { return !p.s.Playing })
	cast := newCastStream(h)
	h.p.SetOutput(Output{Stream: cast, Volume: 0.5})
	h.wait("moved", func(p *Player) bool { return p.s.Remote })
	// The fake's time moved by a millisecond as it played.
	if got := cast.lastLoad(); got != "load url:yt:a 1ms false" {
		t.Errorf("moved with %q", got)
	}

	h.p.Play()
	h.wait("playing on the device", playing("a"))
	if !cast.has("play") {
		t.Errorf("played elsewhere: %q", cast.copy())
	}
	// The device's remote pauses it.
	h.p.Playing(false)
	h.wait("paused by the device", func(p *Player) bool { return !p.s.Playing })
	h.p.SetOutput(Output{})
	h.wait("back", func(p *Player) bool { return !p.s.Remote })
	if got := h.stream.lastLoad(); got != "load url:yt:a 1ms false" {
		t.Errorf("moved back with %q", got)
	}

	other := newCastStream(h)
	h.p.SetOutput(Output{Stream: other})
	h.p.Play()
	h.wait("playing on the other device", func(p *Player) bool { return p.s.Playing && p.s.Remote })
	h.p.SetOutput(Output{Paused: true})
	h.wait("back, paused", func(p *Player) bool { return !p.s.Remote && !p.s.Playing })
	if got := h.stream.lastLoad(); got != "load url:yt:a 1ms false" {
		t.Errorf("moved back from the device that went with %q", got)
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
	cast := newCastStream(h)
	h.p.SetOutput(Output{Stream: cast, Volume: 0.2})
	h.p.SetVolume(0.6)
	h.wait("the device at 0.6", func(p *Player) bool { return p.s.Volume == 0.6 })
	if got := cast.last("volume"); got != "volume 0.6" {
		t.Errorf("set the device's volume with %q", got)
	}
	calls := cast.count("volume")
	h.p.OutputVolume(0.25, true)
	h.wait("the device's own change", func(p *Player) bool { return p.s.Volume == 0.25 && p.s.Muted })
	if cast.count("volume") != calls {
		t.Error("the device's own volume was sent back to it")
	}
	h.p.ToggleMute()
	h.wait("unmuted", func(p *Player) bool { return !p.s.Muted })
	if got := cast.last("volume"); got != "volume 0.25" {
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
	// On the computer, a device's changes are nobody's.
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
	cast := newCastStream(h)
	h.p.SetOutput(Output{Stream: cast, Volume: 1})
	h.wait("on the device", playing("a"))
	h.p.Time(20*time.Second, 200*time.Second)
	h.p.Time(197*time.Second, 200*time.Second)
	h.on(func(*Player) {})
	h.on(func(*Player) {})
	if cast.count("crossfade") != 0 || h.stream.count("crossfade") != 0 {
		t.Fatal("crossfaded on the device")
	}
	h.p.Ended()
	h.wait("b playing", playing("b"))
	if got := cast.lastLoad(); got != "load url:yt:b 0s true" {
		t.Errorf("b loaded with %q", got)
	}
}

// TestOutputLeavesSpotifyDevices leaves a Spotify track that plays on a
// Spotify device there: no output has a stream of it to move.
func TestOutputLeavesSpotifyDevices(t *testing.T) {
	h := newHarness(t, Saved{}, func(f *fakeServices) {
		f.prepare = func(t media.Track) (Source, error) { return Source{Mode: ModeConnect, Track: t}, nil }
	})
	sp := track("s")
	sp.ID, sp.Source = "sp:s", media.SourceSpotify
	h.p.PlayTracks([]media.Track{sp}, 0)
	h.wait("on the device", func(p *Player) bool { return p.connect.active() })
	cast := newCastStream(h)
	h.p.SetOutput(Output{Stream: cast, Volume: 0.4})
	h.wait("moved", func(p *Player) bool { return p.s.Remote })
	if cast.count("load") != 0 {
		t.Error("a Spotify device's track loaded on the cast device")
	}
	if !h.p.State().Playing {
		t.Error("the Spotify device's track stopped")
	}
}
