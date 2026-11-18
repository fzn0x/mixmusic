package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/locales"
)

// TestGaplessAlbumsRow shows Keep albums gapless in Settings › Playback ›
// Sound, right under Crossfade, while Crossfade is on (#601): on at first,
// and off once its switch is clicked.
func TestGaplessAlbumsRow(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/playback")
	if !tt.HasText("Crossfade") {
		t.Fatalf("no Crossfade: %q", tt.Texts())
	}
	if tt.HasText("Keep albums gapless") {
		t.Error("the row shows while Crossfade is off")
	}
	s.set.Crossfade = 5
	s.save()
	tt.Frame()
	var above float32
	for _, text := range []string{"Crossfade", "Keep albums gapless", locales.CrossfadeGaplessDesc(), "Even loudness"} {
		r, ok := tt.Find(text)
		if !ok || r.H <= 0 || r.Y <= above {
			t.Fatalf("Settings › Playback shows %q out of place, at %v: %q", text, r, tt.Texts())
		}
		above = r.Y
	}
	if !s.svc.Preferences.Get().GaplessAlbums {
		t.Fatal("off at first")
	}
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "Keep albums gapless")
	if !ok || !sw.Checked {
		t.Fatalf("the switch is %+v", sw)
	}
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	until(t, s, tt, "Keep albums gapless off", func() bool { return !s.svc.Preferences.Get().GaplessAlbums })
	show(s, tt, settingsPath("", "gapless"))
	showsOnly(t, tt, "searching gapless", []string{"Keep albums gapless"}, []string{"Even loudness", "Autoplay"})
}

// TestGaplessAlbumsRowInTheLanguage says the row in en-XA, which brackets
// each message of the catalogs.
func TestGaplessAlbumsRowInTheLanguage(t *testing.T) {
	usePseudoLocale(t)
	s := newTestShell(t)
	s.set.Crossfade = 5
	s.save()
	tt := testerOf(t, s, 1280, 4000)
	tt.SetLocale("en-XA")
	show(s, tt, "/settings/playback")
	xa := locales.In("en-XA")
	for _, text := range []string{xa.CrossfadeGapless(), xa.CrossfadeGaplessDesc()} {
		if !tt.HasText(text) {
			t.Errorf("no %q: %q", text, tt.Texts())
		}
	}
	checkPseudo(t, "Settings › Playback", tt.Texts())
}
