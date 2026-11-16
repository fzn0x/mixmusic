package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestMovingCoverRow turns moving covers on from Settings › Playback,
// below the picture of music videos, and off from the palette's command;
// a search for "gif" finds the row.
func TestMovingCoverRow(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/playback")
	videos, ok := tt.Find("Show the video of music videos")
	if !ok {
		t.Fatalf("no music videos' row: %q", tt.Texts())
	}
	row, ok := tt.Find("Show moving covers")
	if !ok || row.Y <= videos.Y {
		t.Fatalf("the row is at %v, the music videos' at %v", row, videos)
	}
	if s.svc.Preferences.Get().MovingCovers {
		t.Fatal("on at first")
	}
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "Show moving covers")
	if !ok || sw.Checked {
		t.Fatalf("the switch is %+v", sw)
	}
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	until(t, s, tt, "moving covers saved", func() bool { return s.svc.Preferences.Get().MovingCovers })
	var found bool
	for _, cmd := range s.commands() {
		if cmd.id == "view.movingCovers" {
			found = true
			if cmd.title != "Show or hide moving covers" {
				t.Errorf("the command is %q", cmd.title)
			}
			cmd.run()
		}
	}
	if !found {
		t.Fatal("no command")
	}
	if s.svc.Preferences.Get().MovingCovers {
		t.Error("the command left the setting on")
	}
	show(s, tt, settingsPath("", "gif"))
	showsOnly(t, tt, "searching gif", []string{"Show moving covers"}, []string{"Crossfade", "Show the video of music videos"})
}
