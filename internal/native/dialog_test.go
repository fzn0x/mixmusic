package native

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
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
