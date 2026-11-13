package main

import "testing"

// TestRoamTab shows Explore's Roam tab: a tile for each tag, by era,
// language and genre.
func TestRoamTab(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 900)
	show(s, tt, "/explore/roam")
	for _, label := range []string{"Roaming 90后…", "Roaming 粤语…", "Roaming 摇滚…"} {
		if _, ok := tt.Find(label); !ok {
			t.Errorf("no tile labelled %q", label)
		}
	}
}
