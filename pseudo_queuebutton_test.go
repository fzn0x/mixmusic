package main

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/locales"
)

// newTexts are the texts of the window that weren't among before: what a
// hover or a click brought.
func newTexts(tt *ui.Tester, before []string) []string {
	var out []string
	for _, txt := range tt.Texts() {
		if !slices.Contains(before, txt) {
			out = append(out, txt)
		}
	}
	return out
}

// TestPseudoLocaleQueueButton shows the queue's button of a row under the
// pointer and its toast, then the buttons of a song's tile, in en-XA: what
// they bring to the window is translated.
func TestPseudoLocaleQueueButton(t *testing.T) {
	usePseudoLocale(t)
	var names []string
	for _, tr := range testTracks {
		names = append(names, tr.Title, tr.Artists[0].Name, tr.Album.Name)
	}
	strobe := testTracks[2]
	s, tt := queueAlbum(t, false)
	tt.SetLocale("en-XA")
	tt.Frame()
	before := tt.Texts()
	hoverRow(t, s, tt, strobe.Title)
	added := newTexts(tt, before)
	if !slices.Contains(added, locales.QueueButtonAdd(strobe.Title)) {
		t.Fatalf("no queue button; the hover brought %q", added)
	}
	checkPseudo(t, "A row under the pointer", added, names...)
	before = tt.Texts()
	click(t, s, tt, locales.QueueButtonAdd(strobe.Title))
	if !toasted(t, s, tt, locales.QueueButtonPlaying(strobe.Title)) {
		t.Errorf("no toast of the song playing: %q", tt.Texts())
	}
	checkPseudo(t, "The toast of the queue's button", newTexts(tt, before), names...)

	feed := []media.Shelf{{Title: "Quick picks", Items: itemsOfTracks(testTracks[:3]...)}}
	s, tt = homeShell(t, feed)
	tt.SetLocale("en-XA")
	tt.Frame()
	before = tt.Texts()
	r, ok := tt.Find(strobe.Title)
	if !ok {
		t.Fatalf("no tile of %q; the window shows %q", strobe.Title, tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	s.drain()
	tt.Frame()
	tt.Advance(time.Second)
	added = newTexts(tt, before)
	for _, label := range []string{locales.QueueButtonAdd(strobe.Title), locales.QueueButtonPlayNext(strobe.Title)} {
		if !slices.Contains(added, label) {
			t.Errorf("no %q; the hover brought %q", label, added)
		}
	}
	checkPseudo(t, "A song's tile under the pointer", added, names...)
}
