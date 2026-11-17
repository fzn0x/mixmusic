package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestVisualizerReducedMotion draws no visualizer while the system asks
// for less motion, and draws it again when it stops asking.
func TestVisualizerReducedMotion(t *testing.T) {
	track := musicVideo()
	s, tt, _, _ := videoShell(t, track)
	click(t, s, tt, "Show the cover")
	tt.Advance(time.Second)
	if _, ok := tt.Find("Visualizer"); !ok {
		t.Fatal("no visualizer over the cover")
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if _, ok := tt.Find("Visualizer"); ok {
		t.Error("the visualizer draws while the system asks for less motion")
	}
	tt.SetPreferences(ui.Preferences{})
	tt.Frame()
	if _, ok := tt.Find("Visualizer"); !ok {
		t.Error("the visualizer didn't come back")
	}
}
