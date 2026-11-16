package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/sponsorblock"
)

// TestSkipPartsSettings turns the skipping of parts on in Settings ›
// Playback, with Non-music parts, Sponsor and Self-promotion ticked, ticks
// Filler, and credits SponsorBlock: off, the group is its switch alone.
func TestSkipPartsSettings(t *testing.T) {
	s := newTestShell(t)
	// The window's own height: the page scrolls to the group, wherever the
	// groups above it put it.
	tt := tester(t, s)
	show(s, tt, "/settings/playback")
	if !reveal(tt, "Skip the parts SponsorBlock marks") {
		t.Fatalf("the group doesn't come into view: %q", tt.Texts())
	}
	credited := slices.ContainsFunc(tt.Texts(), func(s string) bool {
		return strings.HasPrefix(s, "Uses SponsorBlock data licensed under CC BY-NC-SA 4.0 from https://sponsor.ajay.app/.")
	})
	if !tt.HasText("Skip the parts SponsorBlock marks") || !credited {
		t.Fatalf("no group, or no credit: %q", tt.Texts())
	}
	if tt.HasText("Preview or recap") {
		t.Error("the kinds show while it's off")
	}
	sw, ok := tt.AccessByRole(ui.RoleSwitch, "Skip the parts SponsorBlock marks")
	if !ok || sw.Checked {
		t.Fatalf("no switch, or on at first: %+v", sw)
	}
	tt.ClickAt(sw.Bounds.X+sw.Bounds.W/2, sw.Bounds.Y+sw.Bounds.H/2)
	s.drain()
	tt.Frame()
	if !s.svc.Preferences.Get().SkipParts.On {
		t.Fatal("the switch didn't turn it on")
	}
	tt.Frame()
	for _, kind := range sponsorblock.Kinds {
		a, ok := tt.Access("Skip " + kind.Name())
		if !ok {
			t.Fatalf("no box of %s: %q", kind.Name(), tt.Texts())
		}
		if ticked := a.Checked; ticked != slices.Contains(sponsorblock.DefaultKinds, kind) {
			t.Errorf("%s ticked %v at first", kind.Name(), ticked)
		}
	}
	if !tt.HasText("Tangents and jokes beside the topic. It skips a lot, so it's off at first.") {
		t.Error("Filler doesn't warn that it skips a lot")
	}
	if !reveal(tt, "Filler") {
		t.Fatalf("Filler doesn't come into view: %q", tt.Texts())
	}
	filler, _ := tt.Access("Skip Filler")
	tt.ClickAt(filler.Bounds.X+filler.Bounds.W/2, filler.Bounds.Y+filler.Bounds.H/2)
	s.drain()
	tt.Frame()
	if got := s.svc.Preferences.Get().SkipParts.Kinds; !slices.Contains(got, sponsorblock.Filler) {
		t.Errorf("ticking Filler keeps %v", got)
	}
}
