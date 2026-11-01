package main

import (
	"runtime"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/player"
)

// refreshKeys are the keys of Refresh this page: F5, but ⇧⌘R on macOS,
// where F5 takes Fn on most keyboards. Mod+R stays Repeat's.
var refreshKeys = refreshKeysOn(runtime.GOOS)

// refreshKeysOn are refreshKeys on goos.
func refreshKeysOn(goos string) string {
	if goos == "darwin" {
		return "Mod+Shift+R"
	}
	return "F5"
}

// refresher is a loaded of any type, as a refresh sees it.
type refresher interface {
	refresh() bool
	isRefreshing() bool
}

// refresh has fetch ask again for what a refresh refreshes, unless the
// first answer or a refresh's is still to come: it tells whether fetch
// will.
func (l *loaded[T]) refresh() bool {
	if !l.done || l.refreshing {
		return false
	}
	l.again, l.refreshing = true, true
	return true
}

func (l *loaded[T]) isRefreshing() bool { return l.refreshing }

// refreshPage asks again for what the page shown and the side panel
// fetched in the frame before, which they go on showing until the answers
// come, and renders again the views of plugins the window shows. While a
// refresh is out, it does nothing.
func (s *shell) refreshPage() {
	if s.refreshing() {
		return
	}
	asked := false
	for key := range s.shown {
		if l, ok := s.loads[key].(refresher); ok && l.refresh() {
			asked = true
		}
	}
	for v := range s.mountedViews {
		v.Remount()
		asked = true
	}
	if asked {
		s.refreshAt, s.refreshTold = s.router.Location(), false
	}
}

// refreshing tells whether a refresh asks again for what the page shows.
func (s *shell) refreshing() bool {
	for key := range s.shown {
		if s.refreshingKey(key) {
			return true
		}
	}
	return false
}

// refreshingKey tells whether a refresh asks again for key.
func (s *shell) refreshingKey(key string) bool {
	l, ok := s.loads[key].(refresher)
	return ok && l.isRefreshing()
}

// refreshFailed tells why what the page showed couldn't be refreshed, by
// the first answer of a refresh that failed: one toast a refresh.
func (s *shell) refreshFailed(err error) {
	if s.refreshTold {
		return
	}
	s.refreshTold = true
	s.notes = append(s.notes, player.Notice{Message: "Couldn't refresh: " + err.Error(), Error: true})
}

// nothingToRefresh tells whether the page shows only what is always
// current: it fetched nothing, and shows no plugin's view.
func (s *shell) nothingToRefresh() bool { return len(s.shown) == 0 && len(s.mountedViews) == 0 }

// keepShown keeps what the page shown and the side panel fetched in the
// frame, which a refresh asks again for, and tells screen readers that a
// refresh ended: not one whose toast told it failed, which they hear, nor
// one of a page the user left meanwhile. The refresh button, built before
// the page, shows what this frame found in the next, which it asks for.
func (s *shell) keepShown(c *ui.Context) {
	s.shown, s.showing = s.showing, s.shown
	clear(s.showing)
	if s.nothingToRefresh() != s.refreshOff || s.refreshing() != s.refreshBusy {
		c.Invalidate()
	}
	if s.refreshAt == "" || s.refreshing() {
		return
	}
	if s.refreshAt == s.router.Location() && !s.refreshTold {
		c.Announce("Page refreshed")
	}
	s.refreshAt = ""
}

// refreshButton refreshes the page, after the history arrows. It turns to
// a spinner while the answers come, which leaves the clicks be, and is
// dimmed on a page that shows only what is always current, as the liked
// songs: there, it keeps its place.
func (s *shell) refreshButton(c *ui.Context) {
	k := &s.lk
	busy, off := s.refreshing(), s.nothingToRefresh()
	s.refreshBusy, s.refreshOff = busy, off
	label, tip := "Refresh this page", "Refresh this page"
	if keys := shortcutLabel(s.shortcutOf(command{id: "view.refresh", shortcut: refreshKeys})); keys != "" {
		tip += " (" + keys + ")"
	}
	b := s.buttonBase(c.Key("refresh")).Size(32, 32).Radius(k.radiusSm).Label(label).Tooltip(tip).Cursor(ui.CursorPointer).
		Disabled(off)
	ink := k.muted
	switch {
	case busy:
	case b.Pressed():
		b.Background(k.press)
	case b.Hovered():
		ink = k.text
		b.Background(k.hover)
	}
	if b.Clicked() && !busy {
		s.refreshPage()
	}
	// The same button, either way: the keyboard's focus stays on it.
	b.Children(func() {
		if busy {
			ui.Spinner(c).Size(16, 16).Label("Refreshing")
			return
		}
		ui.Icon(c, icon("refresh")).FontSize(18).TextColor(ink)
	})
}
