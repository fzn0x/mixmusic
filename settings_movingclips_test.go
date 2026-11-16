package main

import (
	"context"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestMovingClipsRows shows the clips of moving covers kept, in Settings
// › Playback › Offline cache between the covers kept and the cache's
// size, with Clear moving covers and their own size, once a source of
// clips is set; without one, the rows don't show.
func TestMovingClipsRows(t *testing.T) {
	none := newTestShell(t)
	nt := testerOf(t, none, 1280, 4000)
	show(none, nt, "/settings/playback")
	if nt.HasText("Moving covers kept") || nt.HasText("Size of moving covers") {
		t.Error("the clips' rows show with no source of clips")
	}

	s, tt, page, _ := clipShell(t)
	framesUntil(t, s, tt, page, "mv.load(")
	tt.SetSize(1280, 4000)
	show(s, tt, "/settings/playback")
	var above float32
	for _, text := range []string{"Covers kept", "Moving covers kept", "Size of moving covers", "Cache size"} {
		r, ok := tt.Find(text)
		if !ok || r.Y <= above {
			t.Fatalf("Settings › Playback shows %q out of place, at %v: %q", text, r, tt.Texts())
		}
		above = r.Y
	}
	if !tt.HasText("1 clip in") || !tt.HasText("256 MB at most.") {
		t.Errorf("the row tells %q", tt.Texts())
	}
	sel, ok := tt.AccessByRole(ui.RolePopUpButton, "Size of moving covers")
	if !ok || sel.Value != "256 MB" {
		t.Fatalf("the select is %+v", sel)
	}
	click(t, s, tt, "256 MB")
	click(t, s, tt, "Unlimited")
	until(t, s, tt, "unlimited saved", func() bool {
		return s.svc.Preferences.Get().MovingCoversMB == settings.UnlimitedMovingCovers
	})
	until(t, s, tt, "unlimited told", func() bool { return tt.HasText("Unlimited: every clip that played stays.") })
	click(t, s, tt, "Clear moving covers")
	until(t, s, tt, "the clips cleared", func() bool { return tt.HasText("Moving covers cleared") })
	if u := s.svc.System.MovingCoverUsage(); u.Clips != 0 {
		t.Errorf("cleared, %d clips stay", u.Clips)
	}
	for key := range s.loads {
		if strings.HasPrefix(key, "clip:") {
			t.Errorf("cleared, the sleeve keeps %s", key)
		}
	}
	// The clip is looked for anew.
	if _, err := s.svc.Player.MovingCover(context.Background(), testTracks[0], app.CoverClipOptions{Px: 600}); err != nil {
		t.Errorf("after clearing: %v", err)
	}
}
