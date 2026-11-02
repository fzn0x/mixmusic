package main

import (
	"net/url"
	"runtime"
	"strings"
	"time"

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
	// reveal is the group a link to it in its section scrolls into view,
	// once the section shows.
	reveal string
	// typedAt is when the query changed, and told the query whose matches
	// screen readers heard of.
	typedAt time.Time
	told    string
	// focus has the box take the keyboard focus once it shows.
	focus bool

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
		m = allIn(f.cur, searchFold(k.section+" "+k.group+" "+title+" "+text+" "+more))
		f.seen[k] = m
	}
	return m
}

// spellings turn the words the settings spell one way, and users may spell
// another, into their other spelling, both ways: colour and color,
// equaliser and equalizer, levelling and leveling, licence and license,
// normalise and normalize.
var spellings = strings.NewReplacer(
	"colour", "color", "color", "colour",
	"equalis", "equaliz", "equaliz", "equalis",
	"levell", "level", "level", "levell",
	"licenc", "licens", "licens", "licenc",
	"normalis", "normaliz", "normaliz", "normalis",
)

// searchFold folds a text the search of the settings looks through as fold
// does, and adds it spelled the other way: either spelling then finds it,
// at every letter typed. The query is only folded.
func searchFold(text string) string {
	text = fold(text)
	if other := spellings.Replace(text); other != text {
		return text + " " + other
	}
	return text
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

// findHead titles the group of the first thing found in it, with a link to
// the group in its section, which Back returns from, and the group's own
// links.
func (s *shell) findHead(c *ui.Context) {
	f := &s.find
	if !f.head {
		return
	}
	f.head = false
	section, group, links := f.section, f.group, f.links
	s.sectionHead(c, group, func() {
		// Screen readers hear the group in the link's name, as the
		// groups of a section show a link each.
		in := s.linkOf(c.Key("find-in-"+section), "show in "+section, "").FontSize(s.lk.smallSize()).
			Label("Show " + group + " in " + section)
		if in.Clicked() {
			f.reveal = group
			s.navigate(settingsPath(section, ""))
		}
		if links != nil {
			links()
		}
	})
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

// searchSettings opens the settings with their search box focused: on a
// page of the settings, the search it shows stays; from another page, the
// settings open unsearched.
func (s *shell) searchSettings() {
	tab, q := "", ""
	if p := s.router.Path(); p == "/settings" || strings.HasPrefix(p, "/settings/") {
		tab, q = strings.TrimPrefix(strings.TrimPrefix(p, "/settings"), "/"), s.router.Query("q")
	}
	s.navigate(settingsPath(tab, q))
	s.find.focus = true
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
		// The page that slides in takes the focus, not the one leaving.
		focus := current && f.focus
		if focus {
			f.focus = false
		}
		in := s.filterBox(c, "settings-find", "search the settings", "Search the settings", query, focus, func() {
			f.query, f.route = "", ""
			s.router.Replace(settingsPath(tab, ""))
		})
		if current && in.Changed() {
			f.route, f.typedAt = f.query, c.Now()
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

// tellAfter is how long the typing pauses before screen readers hear what
// the search of the settings found.
const tellAfter = 600 * time.Millisecond

// findTell tells screen readers how many things the search of the page
// found, once the typing pauses: "3 matches", or "No setting matches
// crossfade".
func (s *shell) findTell(c *ui.Context) {
	f := &s.find
	q := strings.TrimSpace(f.q)
	if f.told == q {
		return
	}
	if wait := tellAfter - c.Now().Sub(f.typedAt); wait > 0 {
		c.After(wait)
		return
	}
	f.told = q
	switch {
	case !f.on():
	case f.found == 0:
		c.Announce("No setting matches " + q)
	default:
		c.Announce(plural(f.found, "match", "matches"))
	}
}
