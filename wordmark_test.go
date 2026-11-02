package main

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestFitSize checks that a text keeps its size where it fits, shrinks by
// halves of a DIP where it doesn't, and stops at two thirds of its size.
func TestFitSize(t *testing.T) {
	for _, c := range []struct {
		size, w, room, want float32
	}{
		{24, 100, 118, 24},
		{24, 118, 118, 24},
		{24, 130, 118, 21.5},
		{24, 0, 118, 24},
		{24, 400, 118, 16},
	} {
		if got := fitSize(c.size, c.w, c.room); got != c.want {
			t.Errorf("fitSize(%v, %v, %v) = %v, want %v", c.size, c.w, c.room, got, c.want)
		}
	}
}

// TestWordmarkFits checks that MixMusic's name in the unfolded rail fits
// whole in every built-in theme, layout design and density, as Sakura's
// wide serif didn't, and keeps its size where it fitted.
func TestWordmarkFits(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 800)
	for _, th := range s.inv.Themes {
		if !th.Builtin {
			continue
		}
		for _, layout := range settings.Layouts {
			for _, density := range []settings.Density{settings.DensityComfortable, settings.DensityCompact} {
				st := s.svc.Preferences.Get()
				st.FollowSystem, st.Theme, st.Layout, st.Density, st.SidebarCollapsed = false, th.ID, layout, density, false
				if _, err := s.svc.Preferences.Set(st); err != nil {
					t.Fatal(err)
				}
				s.refresh()
				tt.Frame()
				if !tt.HasText("mixmusic") {
					t.Fatalf("%s, %s: no name in the rail: %q", th.ID, layout, tt.Texts())
				}
				f := s.wordmark
				if f.font != s.lk.display || f.room <= 0 || f.width <= 0 {
					t.Fatalf("%s, %s, %s: the name wasn't measured for this theme: %+v", th.ID, layout, density, f)
				}
				if f.size != 24 {
					t.Logf("%s, %s, %s: %v wide at 24 in %v, shows at %v", th.ID, layout, density, f.width, f.room, f.size)
				}
				switch {
				case f.width <= f.room && f.size != 24:
					t.Errorf("%s, %s, %s: the name fits at 24 in %v, but shows at %v", th.ID, layout, density, f.room, f.size)
				case f.width > f.room && f.width*f.size/24 > f.room && f.size > 16:
					t.Errorf("%s, %s, %s: the name is %v wide at %v, in %v", th.ID, layout, density, f.width*f.size/24, f.size, f.room)
				}
			}
		}
	}
}
