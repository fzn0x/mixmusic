package native

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
)

// shows fails unless the element labelled label shows whole in a window
// h tall, as a row of tall: Find tells the part of it the window shows.
func shows(t *testing.T, tt *ui.Tester, h int, label string, tall float32) {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok || r.H < tall-0.5 || r.Y < 0 || r.Y+r.H > float32(h)+0.5 {
		t.Errorf("%q doesn't show whole in a window %d tall: %v (found %v)", label, h, r, ok)
	}
}

// TestPaletteInShortWindow fits the palette's list to a window too short
// for it, whose last command its end shows.
func TestPaletteInShortWindow(t *testing.T) {
	const w, h = 924, 425
	s := newTestShell(t)
	tt := testerOf(t, s, w, h)
	s.paletteOpen = true
	tt.Frame()
	tt.Advance(time.Second)
	cmds := s.commands()
	first, last := cmds[0].title, cmds[len(cmds)-1].title
	r, ok := tt.Find(first)
	if !ok || r.H == 0 {
		t.Fatalf("the palette doesn't show its first command: %q", tt.Texts())
	}
	tt.Scroll(r.X+r.W/2, r.Y+r.H/2, 0, 1e4)
	tt.Frame()
	shows(t, tt, h, last, 36)
}

// TestPaletteFollowsArrows brings the row the arrows choose into view.
func TestPaletteFollowsArrows(t *testing.T) {
	const w, h = 924, 425
	s := newTestShell(t)
	tt := testerOf(t, s, w, h)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	tt.Type("theme")
	tt.Frame()
	var themes []string
	for _, cmd := range s.commands() {
		if cmd.group == "themes" {
			themes = append(themes, cmd.title)
		}
	}
	if len(themes) < 8 {
		t.Fatalf("only %d themes to choose", len(themes))
	}
	// The search for what was typed comes first, then the themes.
	for _, title := range themes {
		tt.Key(0, ui.KeyDown)
		tt.Frame()
		shows(t, tt, h, title, 36)
	}
	for range themes {
		tt.Key(0, ui.KeyUp)
		tt.Frame()
	}
	shows(t, tt, h, "Search for “theme”", 36)
	// The row chosen doesn't keep the list from scrolling away from it.
	r, _ := tt.Find("Search for “theme”")
	tt.Scroll(r.X+r.W/2, r.Y+r.H/2, 0, 1e4)
	tt.Frame()
	tt.Frame()
	shows(t, tt, h, themes[len(themes)-1], 36)
}

// TestOfferFirstFocus opens the update offer with the focus on later, not
// on skip this version, which a Space pressed for the music, or typed in
// the search box the offer takes the focus from, would press.
func TestOfferFirstFocus(t *testing.T) {
	offer := &app.Release{Version: "9.9.9", Newer: true, Page: "https://example.invalid/notes",
		Download: "https://example.invalid/m.deb", File: "mixmusic-9.9.9-linux-amd64.deb"}
	s := newTestShell(t)
	tt := tester(t, s)
	s.offer = offer
	tt.Frame()
	if !tt.Focused("later") || tt.Focused("skip this version") {
		t.Error("the offer opens with the focus elsewhere than on later")
	}

	s = newTestShell(t)
	tt = tester(t, s)
	tt.Key(ui.Cmd, ui.KeyF)
	tt.Frame()
	tt.Type("lua")
	tt.Frame()
	s.offer = offer
	tt.Frame()
	if !tt.Focused("later") {
		t.Error("the offer, opening over the search box, leaves the focus elsewhere than on later")
	}
}
