package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// revealStep is how far reveal scrolls at a time, in DIPs: less than what
// the shortest window of the tests shows of a page, so that nothing is
// scrolled past unseen.
const revealStep = 200

// reveal scrolls the page down, as a touchpad does, until what shows or is
// labeled label is in view, in the upper part of the window, clear of the
// player bar, or the page scrolls no further; it reports whether it shows.
// A test that clicks a row then needs no window tall enough for all that
// is above the row, which grows with each setting added there.
func reveal(tt *ui.Tester, label string) bool {
	if _, ok := tt.Find(label); !ok {
		return false
	}
	b := tt.Image().Bounds()
	w, h := float32(b.Dx()), float32(b.Dy())
	shows := func(r ui.Rect) bool { return r.W > 0 && r.H > 0 && r.Y >= 0 && r.Y+r.H <= h }
	var last ui.Rect
	for range 100 {
		r, _ := tt.Find(label)
		if shows(r) && (r.Y+r.H/2 < 0.6*h || r == last) {
			return true
		}
		last = r
		tt.Scroll(w/2, h/2, 0, revealStep)
	}
	r, _ := tt.Find(label)
	return shows(r)
}

// TestReveal scrolls a page to a row far below the window's foot, and
// clicks it, and finds nothing for what the page doesn't have.
func TestReveal(t *testing.T) {
	s := newTestShell(t)
	tt := tester(t, s)
	show(s, tt, "/settings/playback")
	if err := tt.Click("Cache size"); err == nil {
		t.Fatal("the cache's size shows without scrolling: the test needs a row out of view")
	}
	if !reveal(tt, "Cache size") {
		t.Fatalf("the cache's size doesn't come into view: %q", tt.Texts())
	}
	if err := tt.Click("Cache size"); err != nil {
		t.Errorf("revealed, the cache's size takes no click: %v", err)
	}
	if reveal(tt, "No such row") {
		t.Error("a row the page doesn't have was revealed")
	}
}
