package main

import (
	"net/url"
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"
)

// settingsFind is the search of the settings: what the user typed, and,
// as a page of the settings builds, where the page is in what it finds.
//
// The builders of the settings search as they build: group and groupWith,
// row and rowWith, hint and choices. A row made with row in a group is
// found by its title, its description, its group's title and its
// section's name, with nothing more to do, and row's words give it more
// words to be found by. Anything else a group shows goes through found,
// with the words it is found by, or it would show in every search:
// TestSettingsFindNothingElse fails for a block that doesn't.
type settingsFind struct {
	// query is the box's text, and route the location's query it took
	// last: a link, Back or the palette sets the search through the
	// location.
	query, route string
	// words are the folded words of the queries searched, and seen whether
	// what was searched held them all, by query.
	words map[string][]string
	seen  map[findKey]bool

	// As a page builds: whether it is the page the router shows, the query
	// it searches and its words; the section and the group that build;
	// head tells that the group's title waits for the first thing found in
	// it, whose links are links; whole that the group's title or its
	// section's name holds the words, which shows all of the group; inside
	// that a thing found builds, which shows whole. inGroup and found count
	// what showed, in the group and in the page.
	current        bool
	q              string
	cur            []string
	section, group string
	links          func()
	head, whole    bool
	inside         bool
	inGroup, found int
}

// findKey is what a search looks through, for a query: a thing's section,
// group, title, text and more words.
type findKey struct{ query, section, group, title, text, more string }

// begin starts the build of a page of the settings that searches q, the
// page the router shows if current: two build in a frame while one slides
// in over the other.
func (f *settingsFind) begin(q string, current bool) {
	if f.words == nil || len(f.words) > 8 || len(f.seen) > 4096 {
		f.words, f.seen = map[string][]string{}, map[findKey]bool{}
	}
	w, ok := f.words[q]
	if !ok {
		w = strings.Fields(fold(q))
		f.words[q] = w
	}
	f.current, f.q, f.cur = current, q, w
	f.section, f.group, f.links = "", "", nil
	f.head, f.whole, f.inside = false, false, false
	f.inGroup, f.found = 0, 0
}

// end ends the build of a page of the settings: what builds after it
// doesn't search.
func (f *settingsFind) end() { f.current, f.q, f.cur = false, "", nil }

// on tells whether the page that builds searches the settings.
func (f *settingsFind) on() bool { return len(f.cur) > 0 }

// sifting tells whether what builds next shows only if it holds the words
// typed: while the page searches, but in a thing found, which shows whole,
// and in a group found whole.
func (f *settingsFind) sifting() bool { return f.on() && !f.inside && !f.whole }

// holds tells whether a thing of the group that builds holds every word of
// the query, with its group's title and its section's name, as it did the
// first time it was asked.
func (f *settingsFind) holds(title, text, more string) bool {
	k := findKey{f.q, f.section, f.group, title, text, more}
	m, ok := f.seen[k]
	if !ok {
		m = allIn(f.cur, fold(k.section+" "+k.group+" "+title+" "+text+" "+more))
		f.seen[k] = m
	}
	return m
}

// findable tells whether the thing title and text name shows, which words
// name too: always, but while the user searches the settings, when it
// holds every word typed, with its group's title and its section's name.
func (s *shell) findable(title, text string, words ...string) bool {
	f := &s.find
	if !f.sifting() {
		return true
	}
	more := ""
	if len(words) > 0 {
		more = strings.Join(words, " ")
	}
	return f.holds(title, text, more)
}

// found builds what title and text name, unless the search leaves it out.
func (s *shell) found(c *ui.Context, title, text string, build func()) {
	if s.findable(title, text) {
		s.show(c, build)
	}
}

// show builds a thing the search found: under its group's title, the
// first, and over a hairline, the others, as the rows of a group divide.
// What it builds shows whole, its parts divided as the group's rows are.
func (s *shell) show(c *ui.Context, build func()) {
	f := &s.find
	if !f.on() || f.inside {
		build()
		return
	}
	s.findHead(c)
	f.inGroup++
	f.found++
	item := ui.Column(c).Dividers(1, s.lk.border)
	if f.inGroup > 1 {
		item.BorderWidth(1, 0, 0, 0).BorderColor(s.lk.border)
	}
	item.Children(func() {
		f.inside = true
		build()
		f.inside = false
	})
}

// findHead titles the group of the first thing found in it, with its
// links.
func (s *shell) findHead(c *ui.Context) {
	f := &s.find
	if !f.head {
		return
	}
	f.head = false
	s.sectionHead(c, f.group, f.links)
}

// findGroup builds the rows of a group of the settings searched: under the
// group's title, what holds the words; all of them, when the group's title
// or its section's name does; else nothing.
func (s *shell) findGroup(c *ui.Context, title string, links, rows func()) {
	f := &s.find
	group, before, head, whole, inGroup := f.group, f.links, f.head, f.whole, f.inGroup
	f.group, f.links, f.head, f.inGroup = title, links, true, 0
	f.whole = f.holds("", "", "")
	g := ui.Column(c)
	g.Children(rows)
	if f.inGroup > 0 {
		g.Margin(0, 0, 48)
	}
	f.group, f.links, f.head, f.whole, f.inGroup = group, before, head, whole, inGroup
}

// settingsPath is the location of the settings' section tab, or of the
// first, searched for query, if any.
func settingsPath(tab, query string) string {
	p := "/settings"
	if tab != "" {
		p += "/" + tab
	}
	if query != "" {
		p += "?q=" + url.QueryEscape(query)
	}
	return p
}

// findBox is the box that searches the settings, under the sections' names,
// on the page of the settings' section tab. On the page the router shows,
// it holds what the user types; on the one that slides away, its own
// search, q.
func (s *shell) findBox(c *ui.Context, tab, q string, current bool) {
	f := &s.find
	query := &q
	if current {
		query = &f.query
	}
	ui.Row(c).Margin(0, 0, 32).Children(func() {
		in := s.filterBox(c, "settings-find", "search the settings", "Search the settings", query, false, func() {
			f.query, f.route = "", ""
			s.router.Replace(settingsPath(tab, ""))
		})
		if current && in.Changed() {
			f.route = f.query
			// A shortcut being recorded stops: its row may go.
			s.recording = ""
			s.router.Replace(settingsPath(tab, f.query))
		}
	})
}

// shortcutWords are the words the search of the settings finds a command's
// shortcut by: as Settings shows it, and with Mod spelled as the key's
// name, "ctrl+k", or "cmd+k" on macOS.
func shortcutWords(keys string) string {
	if keys == "" {
		return ""
	}
	mod := "Ctrl+"
	if runtime.GOOS == "darwin" {
		mod = "Cmd+"
	}
	return shortcutLabel(keys) + " " + strings.ReplaceAll(keys, "Mod+", mod)
}
