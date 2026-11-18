package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// slideTo moves the slider name to its end key's end, as the keyboard does.
func slideTo(t *testing.T, s *shell, tt *ui.Tester, name string, key ui.Key) {
	t.Helper()
	reveal(tt, name)
	a, ok := tt.AccessByRole(ui.RoleSlider, name)
	if !ok {
		t.Fatalf("no slider %q: %q", name, tt.Texts())
	}
	tt.ClickAt(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2)
	tt.Key(0, key)
	s.drain()
	tt.Frame()
}

// TestReverbRows turns the reverb on, chooses a room, which tells how it
// sounds, and sets the mix, each kept in settings.json; while Sound
// effects is off, a note says the reverb waits.
func TestReverbRows(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/settings/playback")
	for _, want := range []string{"Reverb", "Small room", "Hall", "Plate", "Large space",
		"A vast space, as a cathedral, that rings for about 4 seconds.",
		"25% of the room against the music's own sound: at 100%, the room alone."} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %q", want, tt.Texts())
		}
	}
	reveal(tt, "Play the music in a room, after the equalizer.")
	clickSwitch(t, s, tt, "Reverb")
	until(t, s, tt, "the reverb on", func() bool { return eqSaved(s).Reverb.On })
	clickSwitch(t, s, tt, "Reverb")
	until(t, s, tt, "the reverb off", func() bool { return !eqSaved(s).Reverb.On })
	// Choosing a room turns the reverb on.
	click(t, s, tt, "Plate")
	until(t, s, tt, "the plate", func() bool { r := eqSaved(s).Reverb; return r.On && r.Room == settings.RoomPlate })
	slideTo(t, s, tt, "Reverb mix", ui.KeyEnd)
	until(t, s, tt, "the mix up", func() bool { return eqSaved(s).Reverb.Mix == 100 })
	if !tt.HasText("100% of the room against the music's own sound: at 100%, the room alone.") {
		t.Errorf("the mix tells %q", tt.Texts())
	}
	if hasTextStarting(tt.Texts(), "Sound effects are off: the reverb") {
		t.Error("the note of Sound effects shows while they're on")
	}
	s.set.Equalizer.Effects = false
	s.save()
	tt.Frame()
	if !hasTextStarting(tt.Texts(), "Sound effects are off: the reverb") {
		t.Errorf("no note of Sound effects: %q", tt.Texts())
	}
}

// TestEchoRows chooses an echo's preset, which turns it on and sets its
// sliders, and moves a slider, which makes it Custom; each row tells what
// it does.
func TestEchoRows(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/settings/playback")
	// The theme's segments show their words in lowercase.
	for _, want := range []string{"Echo", "slapback", "quarter note", "long tail", "One quick repeat, 120 ms later.",
		"120 ms from the music to its first repeat, and from each repeat to the next.", "0%: a single repeat."} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %q", want, tt.Texts())
		}
	}
	click(t, s, tt, "long tail")
	until(t, s, tt, "the long tail", func() bool {
		x := eqSaved(s).Echo
		return x.On && x == settings.Echo{On: true, Preset: "tail", Time: 750, Feedback: 60, Mix: 30}
	})
	for _, want := range []string{"Repeats every 750 ms, each feeding the next, for a long trail.",
		"60%: each repeat plays this loud against the one before."} {
		if !tt.HasText(want) {
			t.Errorf("no %q: %q", want, tt.Texts())
		}
	}
	slideTo(t, s, tt, "Time", ui.KeyEnd)
	until(t, s, tt, "the time up", func() bool { x := eqSaved(s).Echo; return x.Time == 2000 && x.Preset == settings.EchoCustom })
	if !tt.HasText("Custom: a time, feedback and mix of your own.") {
		t.Errorf("Custom isn't told: %q", tt.Texts())
	}
	slideTo(t, s, tt, "Feedback", ui.KeyEnd)
	until(t, s, tt, "the feedback up", func() bool { return eqSaved(s).Echo.Feedback == settings.MaxEchoFeedback })
	slideTo(t, s, tt, "Echo mix", ui.KeyHome)
	until(t, s, tt, "the mix down", func() bool { return eqSaved(s).Echo.Mix == 0 })
	click(t, s, tt, "quarter note")
	until(t, s, tt, "the quarter note", func() bool { return eqSaved(s).Echo.Preset == "quarter" })
	reveal(tt, "Repeat the music a moment later, each repeat softer, after the equalizer.")
	clickSwitch(t, s, tt, "Echo")
	until(t, s, tt, "the echo off", func() bool { return !eqSaved(s).Echo.On })
}
