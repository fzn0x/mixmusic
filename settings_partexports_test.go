package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestPartExportsSetting shows Leave marked parts out of exports in Settings
// › Playback › Skip parts of videos while parts are skipped, off at first,
// telling that M4A copies keep them, and turns it on.
func TestPartExportsSetting(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	show(s, tt, "/settings/playback")
	if tt.HasText("Leave marked parts out of exports") {
		t.Error("the row shows while parts aren't skipped")
	}
	if !reveal(tt, "Skip the parts SponsorBlock marks") {
		t.Fatalf("the group doesn't come into view: %q", tt.Texts())
	}
	on, _ := tt.AccessByRole(ui.RoleSwitch, "Skip the parts SponsorBlock marks")
	tt.ClickAt(on.Bounds.X+on.Bounds.W/2, on.Bounds.Y+on.Bounds.H/2)
	s.drain()
	tt.Frame()
	if !reveal(tt, "Leave marked parts out of exports") {
		t.Fatalf("the row doesn't come into view: %q", tt.Texts())
	}
	if !tt.HasText("Copies Export… writes leave out the parts skipped above. Copies in M4A keep them: MixMusic can't cut those yet.") {
		t.Errorf("the row doesn't tell that M4A copies keep them: %q", tt.Texts())
	}
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "Leave marked parts out of exports")
	if !ok || sw.Checked {
		t.Fatalf("no switch, or on at first: %+v", sw)
	}
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	s.drain()
	tt.Frame()
	if !s.svc.Preferences.Get().SkipParts.Exports {
		t.Fatal("the switch didn't turn it on")
	}
}
