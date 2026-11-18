package main

// Editing the Library's tabs: Edit tabs, at the end of the Library's
// tabs, lists them in their order, as Edit home lists Home's shelves, with
// the same rows (homeedit.go) and arrangement (homelayout.go): the user
// drags a tab onto another, or moves it with Alt+Up and Alt+Down, hides
// it, brings it back from the list under them, and Reset puts them back as
// they come. settings.json keeps them as libraryTabs, by their keys. A
// hidden tab still opens from a link or a command that leads to it, and
// shows among the tabs while it does. Tabs that don't apply stay out, as
// they do of the Library: Spotify and YouTube Music signed out, Disliked
// while empty.

import (
	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/locales"
)

// tabsArrange is the state of Edit tabs: whether the Library shows it,
// whether Reset asks, and the tab the keys moved last, which takes the
// focus at its new place.
type tabsArrange struct {
	editing, resetting bool
	focus              string
}

// libraryTabRows are the Library's tabs as they come, by their keys and
// their labels.
func libraryTabRows() []homeRow {
	rows := make([]homeRow, len(librarySections))
	for i, sec := range librarySections {
		rows[i] = homeRow{key: sec.id, title: sec.label()}
	}
	return rows
}

// libraryTabs returns the Library's tabs in the order the user put them,
// each hidden or not: those the arrangement doesn't name after the
// others, in their own order. None of its own goes.
func (s *shell) libraryTabs() []settings.Arranged {
	return arranged(s.set.LibraryTabs, libraryTabRows(), func(string) bool { return false })
}

// setLibraryTabs keeps layout as the arrangement of the Library's tabs,
// by their keys: their labels are the language's.
func (s *shell) setLibraryTabs(layout []settings.Arranged) {
	out := make([]settings.Arranged, len(layout))
	for i, e := range layout {
		out[i] = settings.Arranged{Key: e.Key, Hidden: e.Hidden}
	}
	s.set.LibraryTabs = out
	s.save()
}

// editTabsButton opens Edit tabs, at the end of the Library's tabs.
func (s *shell) editTabsButton(c *ui.Context) {
	if s.iconButton(c, "library-edit-tabs", "edit", locales.LibraryEditTabs(), false, 18).Clicked() {
		s.tabs.editing = true
	}
}

// tabsEditor is the Library as the user edits its tabs, as the package's
// comment tells; applies tells the tabs that apply now, which it lists.
func (s *shell) tabsEditor(c *ui.Context, applies func(id string) bool) {
	k := &s.lk
	full := s.libraryTabs()
	var shown, hidden []settings.Arranged
	for _, e := range full {
		switch {
		case !applies(e.Key):
		case e.Hidden:
			hidden = append(hidden, e)
		default:
			shown = append(shown, e)
		}
	}
	a := arrangement{list: "tabs", full: full, shown: shown, set: s.setLibraryTabs, focus: &s.tabs.focus}
	ui.Row(c).Gap(8).Wrap().Margin(0, 0, 16).Children(func() {
		if s.button(c, "tabs-done", locales.HomeEditDone(), true).Clicked() {
			s.tabs.editing = false
		}
		if s.button(c, "tabs-reset", locales.LibraryEditTabsReset(), false).Disabled(len(s.set.LibraryTabs) == 0).Clicked() {
			s.tabs.resetting = true
		}
	})
	ui.Text(c, locales.LibraryEditTabsHint()).FontSize(k.bodySize()).TextColor(k.muted).MaxWidth(640).Margin(0, 0, 24)
	ui.Column(c.Key("library-tabs")).Gap(4).Margin(0, 0, k.sections()).Children(func() {
		for i, e := range shown {
			s.arrangeRow(c, a, i, e, e.Title, "", func() {
				if len(shown) > 1 && s.iconButton(c, "tabs-hide-"+e.Key, "close", locales.HomeEditHide(e.Title), false, 16).Clicked() {
					a.set(withHidden(full, e.Key, true))
				}
			})
		}
	})
	if len(hidden) > 0 {
		s.sectionTitle(c, locales.HomeEditHidden())
		ui.Column(c.Key("tabs-hidden")).Gap(4).Children(func() {
			for _, e := range hidden {
				s.arrangeHiddenRow(c, a, e, "")
			}
		})
	}
	if s.confirm(c, "tabs-reset", &s.tabs.resetting, locales.LibraryEditTabsReset(), locales.LibraryEditTabsResetBody(), locales.HomeEditResetButton()) {
		s.setLibraryTabs(nil)
	}
}
