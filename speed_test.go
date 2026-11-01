package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestSpeedSettings sets the speed of songs with its slider, by steps, and
// puts back their own with normal.
func TestSpeedSettings(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 3200)
	show(s, tt, "/settings/playback")
	for _, want := range []string{"speed", "Songs", "Their own speed.", "Speed changes what MixMusic plays itself: YouTube Music, and Spotify tracks played from it."} {
		if !tt.HasText(want) {
			t.Errorf("the settings don't say %q", want)
		}
	}
	if tt.HasText("normal") {
		t.Error("normal is offered at the songs' own speed")
	}
	r, ok := tt.Find("Speed of songs")
	if !ok {
		t.Fatalf("no slider of the speed: %q", tt.Texts())
	}
	for _, c := range []struct {
		x    float32
		want float64
		says string
	}{{r.X + r.W/2, 1.75, "1.75×, at their own pitch."}, {r.X + r.W - 1, 3, "3×, at their own pitch."}, {r.X + 1, 0.5, "0.5×, at their own pitch."}} {
		tt.ClickAt(c.x, r.Y+r.H/2)
		s.drain()
		tt.Frame()
		if got := s.svc.Preferences.Get().SongSpeed; got != c.want {
			t.Errorf("the slider keeps the speed %v, want %v", got, c.want)
		}
		if !tt.HasText(c.says) {
			t.Errorf("at %v×, the row doesn't say %q: %q", c.want, c.says, tt.Texts())
		}
	}
	click(t, s, tt, "normal")
	if got := s.svc.Preferences.Get().SongSpeed; got != 1 {
		t.Errorf("normal keeps the speed %v", got)
	}
	if !tt.HasText("Their own speed.") || tt.HasText("normal") {
		t.Errorf("at 1×, the row shows %q", tt.Texts())
	}
}

// TestPlayerBarSpeed shows the speed after the artists while MixMusic
// plays the track at another than its own.
func TestPlayerBarSpeed(t *testing.T) {
	s := newTestShell(t)
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := tester(t, s)
	if !tt.HasText("M83") || tt.HasText("M83 · 1×") {
		t.Errorf("at 1×, the player bar shows %q", tt.Texts())
	}
	st := settings.Default()
	st.SongSpeed = 1.5
	s.p.SettingsChanged(st)
	s.settle(t, func(st player.State) bool { return st.Speed == 1.5 })
	tt.Frame()
	if !tt.HasText("M83 · 1.5×") {
		t.Errorf("at 1.5×, the player bar shows %q", tt.Texts())
	}
}

// TestSpeedCommands moves the speed of songs by quarters from the command
// palette, and tells it.
func TestSpeedCommands(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	run := func(title string, want float64, says string) {
		t.Helper()
		tt.Key(ui.Cmd, ui.KeyK)
		tt.Frame()
		tt.Type(title)
		tt.Frame()
		click(t, s, tt, title)
		if got := s.svc.Preferences.Get().SongSpeed; got != want {
			t.Errorf("%s keeps the speed %v, want %v", title, got, want)
		}
		if !tt.HasText(says) {
			t.Errorf("%s doesn't tell %q: %q", title, says, tt.Texts())
		}
		// The toast goes before the next command's.
		tt.Advance(time.Minute)
		tt.Frame()
		if tt.HasText(says) {
			t.Fatalf("the toast %q stays", says)
		}
	}
	run("Speed up", 1.25, "Songs play at 1.25×")
	run("Speed up", 1.5, "Songs play at 1.5×")
	run("Slow down", 1.25, "Songs play at 1.25×")
	run("Normal speed", 1, "Songs play at their own speed")
	run("Slow down", 0.75, "Songs play at 0.75×")
	// From a speed between quarters, to the next quarter.
	s.set.SongSpeed = 1.1
	run("Speed up", 1.25, "Songs play at 1.25×")
	s.set.SongSpeed = 1.1
	run("Slow down", 1, "Songs play at their own speed")
	// Not past the ends.
	s.set.SongSpeed = 3
	run("Speed up", 3, "Songs play at 3×")
	s.set.SongSpeed = 0.5
	run("Slow down", 0.5, "Songs play at 0.5×")
	// None has keys of its own.
	for _, cmd := range s.commands() {
		if (cmd.id == "player.faster" || cmd.id == "player.slower" || cmd.id == "player.normalSpeed") && cmd.shortcut != "" {
			t.Errorf("%s has the keys %s", cmd.id, cmd.shortcut)
		}
	}
}

// TestSpeedLabel writes speeds as the settings keep them.
func TestSpeedLabel(t *testing.T) {
	for v, want := range map[float64]string{1: "1×", 1.25: "1.25×", 0.5: "0.5×", 2: "2×", 1.1: "1.1×", settings.CleanSpeed(2.35): "2.35×"} {
		if got := speedLabel(v); got != want {
			t.Errorf("%v reads %q, want %q", v, got, want)
		}
	}
}

// TestSpeedToastOnDevice tells that a Spotify device plays its track at
// its own speed.
func TestSpeedToastOnDevice(t *testing.T) {
	s := newTestShell(t)
	s.st.Source = &player.Source{Mode: player.ModeConnect}
	s.setSongSpeed(1.5)
	if n := s.notes[len(s.notes)-1]; n.Message != "Songs play at 1.5× — Spotify plays this one at its own speed" {
		t.Errorf("told %q", n.Message)
	}
}

// TestLyricsLead lights the lyrics up as far ahead as the track moves on
// between two positions.
func TestLyricsLead(t *testing.T) {
	for speed, want := range map[float64]time.Duration{0: 250 * time.Millisecond, 1: 250 * time.Millisecond, 2: 500 * time.Millisecond, 0.5: 125 * time.Millisecond} {
		if got := lyricsLead(speed); got != want {
			t.Errorf("at %v×, a lead of %v, want %v", speed, got, want)
		}
	}
}
