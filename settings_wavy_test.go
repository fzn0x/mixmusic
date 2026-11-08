package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestWavyProgressRow turns the wavy progress bar on and off from Settings
// › Appearance › Colour and type, below Icons only in the sidebar: off at
// first, saved as wavyProgress, and found by a search for "squiggly".
func TestWavyProgressRow(t *testing.T) {
	s := newTestShell(t)
	// The window's own height: Tab brings the switch into view, wherever
	// the settings above it put it.
	tt := tester(t, s)
	show(s, tt, "/settings/appearance")
	if s.svc.Preferences.Get().WavyProgress {
		t.Fatal("on at first")
	}
	focused := func() bool { return tt.Focused("Wavy progress bar") && !tt.Focused(wavyProgressDescription) }
	for i := 0; i < 300 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !focused() {
		t.Fatalf("Tab doesn't reach the switch: %q", tt.Texts())
	}
	tt.Advance(time.Second)
	var above float32
	for _, text := range []string{"Icons only in the sidebar", "Wavy progress bar", wavyProgressDescription} {
		r, ok := tt.Find(text)
		if !ok || r.H <= 0 || r.Y <= above {
			t.Fatalf("Settings › Appearance shows %q out of place, at %v: %q", text, r, tt.Texts())
		}
		above = r.Y
	}
	for _, want := range []bool{true, false} {
		tt.Key(0, ui.KeySpace)
		s.drain()
		tt.Frame()
		if got := s.svc.Preferences.Get().WavyProgress; got != want {
			t.Errorf("the switch saved wavyProgress %v, not %v", got, want)
		}
	}
	for _, q := range []string{"wavy", "squiggly", "seek bar"} {
		show(s, tt, settingsPath("", q))
		if !tt.HasText("Wavy progress bar") {
			t.Errorf("searching %q doesn't find the row: %q", q, tt.Texts())
		}
	}
}
