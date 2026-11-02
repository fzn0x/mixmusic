package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestAddSettings keeps the parts of a place in their order, whatever the
// order they register in, and refuses two parts at one order, and a place
// in no section.
func TestAddSettings(t *testing.T) {
	var parts []settingsPart
	for _, order := range []int{30, 10, 20, 15} {
		parts = withPart(parts, "playback", order, func(*shell, *ui.Context) {})
	}
	for i, want := range []int{10, 15, 20, 30} {
		if parts[i].order != want {
			t.Fatalf("the parts are at %v", parts)
		}
	}
	panics := func(name string, add func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s doesn't panic", name)
			}
		}()
		add()
	}
	panics("a part at a taken order", func() { withPart(parts, "playback", 20, nil) })
	panics("a part in no section", func() { addSettings("nowhere › Sound", 10, nil) })
	if _, ok := settingsParts["nowhere › Sound"]; ok {
		t.Error("a part in no section registered")
	}
	for place, parts := range settingsParts {
		for i := 1; i < len(parts); i++ {
			if parts[i-1].order >= parts[i].order {
				t.Errorf("the parts of %s are out of order", place)
			}
		}
	}
}

// TestSettingsOrder shows the groups and rows that register from their
// features' files in the order they always had.
func TestSettingsOrder(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 4400)
	for path, texts := range map[string][]string{
		"/settings/appearance": {"layout", "Layout", "theme", "Follow the computer", "mix options", "colour and type",
			"Colours from a file", "Follow the artwork", "Accent colour", "Zoom", "Text size", "Density", "Icons only in the sidebar"},
		"/settings/playback": {"spotify tracks", "How Spotify tracks play", "sound", "Quality", "Crossfade", "Even loudness",
			"Volume levelling", "Play 5.1 surround when a track has it", "Autoplay", "Skip songs made with AI",
			"Skip songs you dislike", "speed", "music videos", "equalizer", "lyrics", "offline cache", "downloads", "tools"},
	} {
		show(s, tt, path)
		var above float32
		for _, text := range texts {
			r, ok := tt.Find(text)
			if !ok {
				t.Errorf("%s doesn't show %q", path, text)
				continue
			}
			if r.Y <= above {
				t.Errorf("%s shows %q above what comes before it", path, text)
			}
			above = r.Y
		}
	}
}
