package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestNotchRow shows the switch of the player around the notch in Settings
// › On the desktop on Macs with a notch, below Discord's rows, where a
// search for "notch" or "dynamic island" finds it, and saves it as
// notchPlayer. Elsewhere it doesn't show, searched for too.
func TestNotchRow(t *testing.T) {
	const label = "Player around the notch"
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 2400)
	show(s, tt, "/settings/desktop")
	if tt.HasText(label) {
		t.Error("without a notch, the row shows")
	}
	show(s, tt, settingsPath("", "dynamic island"))
	if tt.HasText(label) {
		t.Error("without a notch, a search finds the row")
	}

	was := hasNotch
	hasNotch = func() bool { return true }
	t.Cleanup(func() { hasNotch = was })
	show(s, tt, "/settings/desktop")
	var above float32
	for _, text := range []string{"Show what plays on Discord", label, "Start at login"} {
		r, ok := tt.Find(text)
		if !ok || r.Y <= above {
			t.Fatalf("Settings › Desktop shows %q out of place: %q", text, tt.Texts())
		}
		above = r.Y
	}
	if !s.svc.Preferences.Get().NotchPlayer {
		t.Fatal("off at first")
	}
	focused := func() bool { return tt.Focused(label) && !tt.Focused(notchDescription()) }
	for i := 0; i < 200 && !focused(); i++ {
		tt.Key(0, ui.KeyTab)
	}
	if !focused() {
		t.Fatalf("Tab doesn't reach the switch: %q", tt.Texts())
	}
	tt.Key(0, ui.KeySpace)
	s.drain()
	tt.Frame()
	if s.svc.Preferences.Get().NotchPlayer {
		t.Error("the switch didn't turn the player around the notch off")
	}
	for _, q := range []string{"notch", "dynamic island", "Dynamic Island"} {
		show(s, tt, settingsPath("", q))
		if !tt.HasText(label) {
			t.Errorf("searching %q doesn't find the row: %q", q, tt.Texts())
		}
	}
}
