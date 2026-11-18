package app

import (
	"math"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/audio"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestReverbOf gives the engine the settings' room at the levels of its
// mix, while the reverb and Sound effects are on, and no reverb else.
func TestReverbOf(t *testing.T) {
	q := settings.Default().Equalizer
	if r := reverbOf(q); r != (audio.Reverb{}) {
		t.Errorf("the defaults play a reverb: %+v", r)
	}
	q.Reverb = settings.Reverb{On: true, Room: settings.RoomLarge, Mix: 50}
	r := reverbOf(q)
	if !r.On || r.Room != audio.RoomLarge || math.Abs(r.Wet-math.Sqrt(0.5)) > 1e-12 || math.Abs(r.Dry-math.Sqrt(0.5)) > 1e-12 {
		t.Errorf("the large space at 50%%: %+v", r)
	}
	for id, want := range map[string]audio.Room{
		settings.RoomSmall: audio.RoomSmall, settings.RoomHall: audio.RoomHall,
		settings.RoomPlate: audio.RoomPlate, settings.RoomLarge: audio.RoomLarge,
	} {
		q.Reverb.Room = id
		if r := reverbOf(q); r.Room != want {
			t.Errorf("%s plays room %d, want %d", id, r.Room, want)
		}
	}
	if len(rooms) != len(settings.Rooms) {
		t.Errorf("%d rooms of %d", len(rooms), len(settings.Rooms))
	}
	q.Effects = false
	if r := reverbOf(q); r != (audio.Reverb{}) {
		t.Errorf("Sound effects off: %+v", r)
	}
	q.Effects, q.Reverb.Mix = true, 0
	if r := reverbOf(q); r.Wet != 0 || r.Dry != 1 {
		t.Errorf("a mix of 0: %+v", r)
	}
}

// TestEchoOf gives the engine the settings' time, feedback and the levels
// of the mix, while the echo and Sound effects are on, and no echo else.
func TestEchoOf(t *testing.T) {
	q := settings.Default().Equalizer
	if x := echoOf(q); x != (audio.Echo{}) {
		t.Errorf("the defaults play an echo: %+v", x)
	}
	q.Echo = settings.Echo{On: true, Preset: settings.EchoCustom, Time: 330, Feedback: 45, Mix: 100}
	if x := echoOf(q); x != (audio.Echo{On: true, Delay: 330 * time.Millisecond, Feedback: 0.45, Wet: 1, Dry: 0}) {
		t.Errorf("Sound effects on: %+v", x)
	}
	q.Effects = false
	if x := echoOf(q); x != (audio.Echo{}) {
		t.Errorf("Sound effects off: %+v", x)
	}
}

// TestApplyReverbAndEcho gives MixMusic's own engine the reverb and the
// echo as the settings change.
func TestApplyReverbAndEcho(t *testing.T) {
	a := newQuietApp(t, nil)
	e := audio.NewEngine(a.openNative, startSilent, engineEvents{a})
	t.Cleanup(e.Close)
	a.nativeOnce.Do(func() { a.native = e })
	a.applyEngine(a.settings.Get().Clean())
	if e.Reverb() != (audio.Reverb{}) || e.Echo() != (audio.Echo{}) {
		t.Errorf("at first: %+v and %+v", e.Reverb(), e.Echo())
	}
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) {
		s.Equalizer.Reverb = settings.Reverb{On: true, Room: settings.RoomPlate, Mix: 100}
		s.Equalizer.Echo.On = true
	}); err != nil {
		t.Fatal(err)
	}
	if r := e.Reverb(); r != (audio.Reverb{On: true, Room: audio.RoomPlate, Wet: 1, Dry: 0}) {
		t.Errorf("the plate at 100%%: %+v", r)
	}
	if x := e.Echo(); !x.On || x.Delay != 120*time.Millisecond || x.Feedback != 0 {
		t.Errorf("the slapback: %+v", x)
	}
	if _, err := (&Preferences{a}).Update(func(s *settings.Settings) { s.Equalizer.Effects = false }); err != nil {
		t.Fatal(err)
	}
	if e.Reverb() != (audio.Reverb{}) || e.Echo() != (audio.Echo{}) {
		t.Errorf("Sound effects off: %+v and %+v", e.Reverb(), e.Echo())
	}
}
