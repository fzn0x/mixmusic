package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/volume"
)

// TestVolumeCurveRow chooses the volume's curve in Settings › Playback ›
// Volume, between the equalizer and the lyrics (#383): Natural at first,
// saved as volumeCurve, which leaves the volume's slider where it was, and
// found by a search for "decibel".
func TestVolumeCurveRow(t *testing.T) {
	s := newTestShell(t)
	s.p.SetVolume(0.5)
	s.settle(t, volumeNear(0.5))
	// The window's own height: the page scrolls to the row, wherever the
	// settings above it put it.
	tt := tester(t, s)
	show(s, tt, "/settings/playback")
	if !reveal(tt, "Volume curve") {
		t.Fatalf("the row doesn't come into view: %q", tt.Texts())
	}
	var above float32
	for _, text := range []string{"Spotify notes", "Volume curve", volumeCurveDescription, "Translate lyrics to"} {
		r, ok := tt.Find(text)
		if !ok || r.H <= 0 || r.Y <= above {
			t.Fatalf("Settings › Playback shows %q out of place, at %v: %q", text, r, tt.Texts())
		}
		above = r.Y
	}
	if c := s.svc.Preferences.Get().VolumeCurve; c != volume.Natural {
		t.Fatalf("the volume plays on %q at first", c)
	}
	for _, c := range []volume.Curve{volume.Linear, volume.Natural} {
		click(t, s, tt, string(c))
		if got := s.svc.Preferences.Get().VolumeCurve; got != c {
			t.Errorf("choosing %s saved %q", c, got)
		}
		s.drain()
		if st := s.p.State(); st.Volume != 0.5 || st.Muted {
			t.Errorf("choosing %s moved the volume to %v, muted %v", c, st.Volume, st.Muted)
		}
	}
	for _, q := range []string{"volume curve", "decibel", "linear"} {
		show(s, tt, settingsPath("", q))
		if !tt.HasText("Volume curve") {
			t.Errorf("searching %q doesn't find the row: %q", q, tt.Texts())
		}
	}
}
