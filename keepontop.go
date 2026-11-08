package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Keep on top keeps the main window above the other windows, while the
// settings say so (#350): from Settings › Desktop, the command palette or
// settings.json. The mini player stays on top whatever they say, as its
// window's options have it.

func init() { addSettings(desktopRows, 15, (*shell).keepOnTopRow) }

// keepOnTopTitle and keepOnTopDescription tell of the setting, in Settings
// › On the desktop.
const (
	keepOnTopTitle       = "Keep on top"
	keepOnTopDescription = "MixMusic's window stays above your other windows. The mini player stays on top either way."
)

// keepOnTopRow turns keeping the main window on top on and off.
func (s *shell) keepOnTopRow(c *ui.Context) {
	s.row(c, keepOnTopTitle, keepOnTopDescription, func() {
		if s.switchOf(c.Key("keep-on-top"), &s.set.KeepOnTop).Label(keepOnTopTitle).Changed() {
			s.save()
		}
	}, "always on top float pin above other windows")
}

// onTop is what the main window was last told of staying on top.
type onTop struct {
	kept bool
}

// keepAbove keeps a window above the other windows, or lets them cover it
// again, and reports whether it could: not before the shell has its
// window. Tests, which have none, replace it.
var keepAbove = func(win *mygo.Window, on bool) bool {
	if win == nil {
		return false
	}
	win.SetAlwaysOnTop(on)
	return true
}

// startOnTop is whether the main window opens on top, which it then is.
func (s *shell) startOnTop() bool {
	s.onTop.kept = s.set.KeepOnTop
	return s.onTop.kept
}

// followOnTop keeps the main window on top, or lets it go, once the
// setting changed, wherever it changed.
func (s *shell) followOnTop() {
	want := s.set.KeepOnTop
	if !s.main || want == s.onTop.kept || !keepAbove(s.win, want) {
		return
	}
	s.onTop.kept = want
}

// toggleOnTop keeps the main window on top, or stops, from the command
// palette, and tells which: nothing else shows it.
func (s *shell) toggleOnTop() {
	s.set.KeepOnTop = !s.set.KeepOnTop
	s.save()
	msg := "MixMusic stays on top of other windows"
	if !s.set.KeepOnTop {
		msg = "MixMusic no longer stays on top"
	}
	s.notes = append(s.notes, notice(msg))
}
