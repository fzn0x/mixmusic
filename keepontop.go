package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Keep on top keeps the main window above the other windows, while the
// settings say so (#350): from Settings › Desktop, the command palette,
// macOS's Window menu or settings.json. The mini player stays on top
// whatever they say, as its window's options have it.

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

// onTop is what the main window was last told of staying on top, and
// macOS's Window › Keep on Top, which shows it.
type onTop struct {
	kept bool
	item *mygo.MenuItem
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
// setting changed, wherever it changed, and checks the Window menu's item
// with it.
func (s *shell) followOnTop() {
	want := s.set.KeepOnTop
	if !s.main || want == s.onTop.kept || !keepAbove(s.win, want) {
		return
	}
	s.onTop.kept = want
	if s.onTop.item != nil {
		s.onTop.item.SetChecked(want)
	}
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

// windowMenu is macOS's Window menu: the system's, which lists the windows
// too, with Keep on Top, a checkbox. MyGo builds a menu as it opens, so
// the item's label is known before, and its check follows the setting
// (followOnTop).
func (s *shell) windowMenu() *mygo.MenuItem {
	s.onTop.item = &mygo.MenuItem{Label: "Keep on Top", Type: mygo.MenuItemCheckbox, Checked: s.set.KeepOnTop,
		// MyGo checks it, or not, before it calls Click.
		Click: func(item *mygo.MenuItem, _ *mygo.Window) {
			on := item.IsChecked()
			s.update(func() {
				s.set.KeepOnTop = on
				s.save()
			})
		}}
	return &mygo.MenuItem{Role: mygo.RoleWindowMenu, Submenu: []*mygo.MenuItem{
		{Role: mygo.RoleMinimize},
		{Role: mygo.RoleZoom},
		mygo.Separator(),
		s.onTop.item,
		mygo.Separator(),
		{Role: mygo.RoleFront},
	}}
}
