package main

import "testing"

// TestRoamPage shows the Roam page of the rail: a tile for each tag, by
// era, language and genre.
func TestRoamPage(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 900)
	show(s, tt, "/roam")
	for _, label := range []string{"Roaming 90后…", "Roaming 粤语…", "Roaming 摇滚…"} {
		if _, ok := tt.Find(label); !ok {
			t.Errorf("no tile labelled %q", label)
		}
	}
}
